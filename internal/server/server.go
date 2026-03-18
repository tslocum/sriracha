// Package server is the Sriracha imageboard and forum server.
package server

import (
	"context"
	"crypto/md5"
	"crypto/sha3"
	"crypto/sha512"
	"crypto/tls"
	"embed"
	"encoding/base64"
	"flag"
	"fmt"
	"hash"
	"html/template"
	"image"
	"io"
	"io/fs"
	"log"
	"maps"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"codeberg.org/tslocum/sriracha"
	"codeberg.org/tslocum/sriracha/internal/database"
	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
	"github.com/fsnotify/fsnotify"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/leonelquinteros/gotext"
	"github.com/r3labs/diff/v3"
	"golang.org/x/sys/unix"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
	"gopkg.in/yaml.v3"
)

// SrirachaVersion is the version of the software. In official releases, this
// variable is replaced during compilation with the version of the release.
var SrirachaVersion = "DEV"

//go:embed locale
var localeFS embed.FS

// Default server settings.
const (
	defaultServerSiteName     = "Sriracha"
	defaultServerSiteHome     = "/"
	defaultServerOekakiWidth  = 540
	defaultServerOekakiHeight = 540
	defaultServerRefresh      = 30
)

// defaultServerEmbeds is a list of default oEmbed services.
var defaultServerEmbeds = [][2]string{
	{"YouTube", "https://youtube.com/oembed?format=json&url=SRIRACHA_EMBED"},
	{"Vimeo", "https://vimeo.com/api/oembed.json?url=SRIRACHA_EMBED"},
	{"SoundCloud", "https://soundcloud.com/oembed?format=json&url=SRIRACHA_EMBED"},
}

// Banner options.
const (
	bannerOverboard = -1
	bannerNews      = -2
	bannerPages     = -3
)

// NewsOption represents a News setting option.
type NewsOption int

// News options.
const (
	NewsDisable      NewsOption = 0
	NewsWriteToNews  NewsOption = 1
	NewsWriteToIndex NewsOption = 2
)

type categoryInfo struct {
	Name        string
	Description string
	Boards      []*Board
	Recent      []*Post
}

type HashAlgorithm int8

const (
	AlgorithmSHA2 HashAlgorithm = 0
	AlgorithmSHA3 HashAlgorithm = 1
)

const HashSize = 48 // Bytes.

// ServerOptions represents server configuration options and related data.
type ServerOptions struct {
	SiteName         string
	SiteHome         string
	SiteIndex        bool
	News             NewsOption
	BoardIndex       bool
	CAPTCHA          bool
	Refresh          int
	Uploads          []*UploadType
	Embeds           [][2]string
	OekakiWidth      int
	OekakiHeight     int
	Overboard        string
	OverboardType    BoardType
	OverboardThreads int
	OverboardReplies int
	Identifiers      bool
	Locale           string
	Locales          map[string]string
	LocalesSorted    []string
	Algorithm        HashAlgorithm
	Access           map[string]string
	Banners          map[int][]*Banner
	Rules            map[int][]template.HTML
	Categories       []*categoryInfo
	Notifications    bool
	DevMode          bool
	FuncMaps         map[string]template.FuncMap
}

// DefaultLocaleName returns the name of the configured default locale.
func (opt *ServerOptions) DefaultLocaleName() string {
	if opt.Locale == "" || opt.Locale == "en" {
		return "English"
	}
	name := opt.Locales[opt.Locale]
	if name != "" {
		return name
	}
	return opt.Locale
}

type cachedKeyword struct {
	id int            // ID.
	p  *regexp.Regexp // Pattern.
	a  string         // Action.
}

// rebuildInfo contains information used to request rebuilding a thread.
type rebuildInfo struct {
	post *Post
	wg   *sync.WaitGroup
}

// Server is the Sriracha imageboard and forum server.
type Server struct {
	Boards []*Board

	rangeBans map[*Ban]*regexp.Regexp

	keywordCache map[int][]*cachedKeyword

	config *Config
	dbPool *pgxpool.Pool
	opt    ServerOptions

	importDatabases []*importInfo

	tpl             *template.Template
	original        *template.Template
	customTemplates []string

	notifications          []notification
	notificationsPattern   *regexp.Regexp
	notificationsWaitGroup sync.WaitGroup
	shutdownNotifications  chan struct{}

	rebuildQueue     chan *rebuildInfo
	rebuildWaitGroup sync.WaitGroup
	rebuildLock      sync.Mutex

	httpClient *http.Client

	httpServer *http.Server

	lock sync.Mutex
}

// NewServer returns a new server.
func NewServer() *Server {
	httpClient := &http.Client{
		Timeout: 15 * time.Second,
	}
	return &Server{
		keywordCache: make(map[int][]*cachedKeyword),
		opt: ServerOptions{
			Banners: make(map[int][]*Banner),
			Rules:   make(map[int][]template.HTML),
		},
		shutdownNotifications: make(chan struct{}),
		rebuildQueue:          make(chan *rebuildInfo),
		httpClient:            httpClient,
	}
}

// parseBuildInfo parses version control information embedded in the binary
// during compilation. This is only used in unofficial releases.
func (s *Server) parseBuildInfo() {
	if SrirachaVersion == "" {
		SrirachaVersion = "DEV"
	} else if SrirachaVersion != "DEV" {
		return
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	buildTag := info.Main.Version
	if buildTag != "" && buildTag[0] == 'v' {
		SrirachaVersion = buildTag[1:]
		firstHyphen := strings.IndexRune(SrirachaVersion, '-')
		firstPlus := strings.IndexRune(SrirachaVersion, '+')
		if firstHyphen == -1 && firstPlus == -1 {
			return
		}
		if firstHyphen != -1 {
			SrirachaVersion = SrirachaVersion[:firstHyphen]
			firstPlus = strings.IndexRune(SrirachaVersion, '+')
		}
		if firstPlus != -1 {
			SrirachaVersion = SrirachaVersion[:firstPlus]
		}
		SrirachaVersion += "-DEV"
	}
	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" {
			revision := setting.Value
			if len(revision) > 10 {
				revision = revision[:10]
			}
			SrirachaVersion += "-" + revision
			return
		}
	}
}

// forbidden returns whether a user is forbidden from performing an accion.
// When forbidden, an error page is written to the web request automatically.
func (s *Server) forbidden(w http.ResponseWriter, data *templateData, action string) bool {
	var required AccountRole
	switch s.config.Access[action] {
	case "mod":
		required = RoleMod
	case "admin":
		required = RoleAdmin
	case "super-admin":
		required = RoleSuperAdmin
	}
	return data.forbidden(w, required)
}

// parseConfig parses a YAML configuration file.
func (s *Server) parseConfig(configFile string) error {
	buf, err := os.ReadFile(configFile)
	if err != nil {
		return err
	}

	config := &Config{
		Access: make(map[string]string),
	}
	err = yaml.Unmarshal(buf, config)
	if err != nil {
		return err
	}

	switch {
	case config.Root == "":
		return fmt.Errorf("root (lowercase!) must be set in %s to the root directory (where board files are written)", configFile)
	case config.Serve == "":
		return fmt.Errorf("serve (lowercase!) must be set in %s to the HTTP server listen address (hostname:port)", configFile)
	case config.SaltData == "":
		return fmt.Errorf("saltdata (lowercase!) must be set in %s to the one-way secure data hashing salt (a long string of random data which, once set, never changes)", configFile)
	case config.SaltPass == "":
		return fmt.Errorf("saltpass (lowercase!) must be set in %s to the two-way secure data hashing salt (a long string of random data which, once set, never changes)", configFile)
	case config.SaltTrip == "":
		return fmt.Errorf("salttrip (lowercase!) must be set in %s to the secure tripcode generation salt (a long string of random data which, once set, never changes)", configFile)
	}

	if config.DBURL == "" {
		switch {
		case config.Address == "":
			return fmt.Errorf("address (lowercase!) must be set in %s to the database address (hostname:port)", configFile)
		case config.Username == "":
			return fmt.Errorf("username (lowercase!) must be set in %s to the database username", configFile)
		case config.Password == "":
			return fmt.Errorf("password (lowercase!) must be set in %s to the database password", configFile)
		case config.DBName == "":
			return fmt.Errorf("dbname (lowercase!) must be set in %s to the database name", configFile)
		}
	}

	if config.Algorithm == "" {
		config.Algorithm = "sha-2"
	}
	switch config.Algorithm {
	case "sha-2":
		s.opt.Algorithm = AlgorithmSHA2
	case "sha-3":
		s.opt.Algorithm = AlgorithmSHA3
	default:
		return fmt.Errorf("algorithm must be set to sha-3 or sha-2")
	}

	if config.Locale == "" {
		config.Locale = "en"
	}

	if config.MailFrom != "" && ParseEmail(config.MailFrom) == "" {
		return fmt.Errorf("mailfrom is not a valid email address: %s", config.MailFrom)
	} else if config.MailReplyTo != "" && ParseEmail(config.MailReplyTo) == "" {
		return fmt.Errorf("mailreplyto is not a valid email address: %s", config.MailReplyTo)
	}

	if config.Mentions <= 0 {
		config.Mentions = 60
	}
	if config.Notifications <= 0 {
		config.Notifications = 1440
	}

	defaultAccess := map[string]string{
		"ban.add":         "mod",
		"ban.shorten":     "admin",
		"ban.lengthen":    "mod",
		"ban.delete":      "admin",
		"banfile.add":     "mod",
		"banfile.delete":  "admin",
		"banner.add":      "admin",
		"banner.update":   "admin",
		"banner.delete":   "super-admin",
		"board.add":       "admin",
		"board.update":    "admin",
		"board.delete":    "super-admin",
		"category.add":    "admin",
		"category.update": "admin",
		"category.delete": "super-admin",
		"keyword.add":     "admin",
		"keyword.update":  "admin",
		"keyword.delete":  "admin",
		"page.add":        "admin",
		"page.update":     "admin",
		"page.delete":     "admin",
		"post.sticky":     "mod",
		"post.lock":       "mod",
		"post.move":       "mod",
		"post.delete":     "mod",
	}
	validateAccess := func(name string, v string) error {
		if _, ok := defaultAccess[name]; !ok && name != "default" {
			return fmt.Errorf("access configuration contains unrecognized action %s", name)
		}
		switch v {
		case "mod", "admin", "super-admin", "disable":
			return nil
		default:
			return fmt.Errorf("action %s has unknown access level %s: must be 'mod', 'admin', 'super-admin' or 'disable'", name, v)
		}
	}
	var defaultRequirement string
	for name, v := range config.Access {
		err = validateAccess(name, v)
		if err != nil {
			return fmt.Errorf("access configuration is invalid: %s", err)
		} else if name == "default" {
			defaultRequirement = v
			delete(config.Access, name)
		}
	}
	for name, v := range defaultAccess {
		if config.Access[name] != "" {
			continue
		} else if defaultRequirement != "" {
			config.Access[name] = defaultRequirement
			continue
		}
		config.Access[name] = v
	}

	s.config = config
	s.config.ImportMode = s.config.Import.Enabled()

	if s.config.MailDomains != "" {
		s.notificationsPattern, err = regexp.Compile(s.config.MailDomains)
		if err != nil {
			return fmt.Errorf("failed to parse maildomains regular expression: %s", err)
		}
	}
	return nil
}

// parseLocales parses locale files.
func (s *Server) parseLocales() error {
	return fs.WalkDir(localeFS, "locale", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		} else if d.IsDir() || !strings.HasSuffix(p, ".po") {
			return nil
		}
		id := filepath.Base(strings.TrimSuffix(p, ".po"))

		buf, err := localeFS.ReadFile(fmt.Sprintf("locale/%s/%s.po", id, id))
		if err != nil {
			return fmt.Errorf("failed to load locale %s: %s", id, err)
		}

		po := gotext.NewPo()
		po.Parse(buf)
		gotext.GetStorage().AddTranslator(fmt.Sprintf("sriracha-%s", id), po)
		return nil
	})
}

// connectToMailServer connects to the configured mail server and returns a SMTP client.
func (s *Server) connectToMailServer() (*smtp.Client, error) {
	if s.config.MailAddress == "" {
		return nil, nil // Email notifications are disabled.
	}

	// Parse hostname and set default port.
	address := s.config.MailAddress
	hostname, _, err := net.SplitHostPort(s.config.MailAddress)
	if err != nil {
		hostname = s.config.MailAddress
		if strings.ContainsRune(s.config.MailAddress, ':') {
			address = fmt.Sprintf("[%s]:25", address)
		} else {
			address = address + ":25"
		}
	}
	tlsConfig := &tls.Config{
		InsecureSkipVerify: s.config.MailInsecure,
		ServerName:         hostname,
	}

	// Connect to mail server.
	var conn net.Conn
	if s.config.MailTLS {
		conn, err = tls.Dial("tcp", address, tlsConfig)
		if err != nil {
			log.Fatalf("failed to connect to SMTP server with TLS enabled at %s: %s", address, err)
		}
	} else {
		conn, err = net.Dial("tcp", address)
		if err != nil {
			log.Fatalf("failed to connect to SMTP server without TLS enabled at %s: %s", address, err)
		}
	}

	// Initialize client,
	client, err := smtp.NewClient(conn, hostname)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to initialize SMTP client: %s", err)
	}

	// Upgrade to TLS connection when available.
	if !s.config.MailTLS {
		ok, _ := client.Extension("STARTTLS")
		if ok {
			err = client.StartTLS(tlsConfig)
			if err != nil {
				client.Close()
				return nil, fmt.Errorf("failed to upgrade plain text connection to TLS even though support for it was advertised")
			}
		}
	}

	// Authenticate.
	var auth smtp.Auth
	switch s.config.MailAuth {
	case "challenge":
		auth = smtp.CRAMMD5Auth(s.config.MailUsername, s.config.MailPassword)
	case "plain":
		auth = smtp.PlainAuth("", s.config.MailUsername, s.config.MailPassword, hostname)
	case "", "none":
		// Do nothing.
	default:
		client.Close()
		return nil, fmt.Errorf("unrecognized mailauth configuration value %s: must be challenge / plain / none", s.config.MailAuth)
	}
	if auth != nil {
		err := client.Auth(auth)
		if err != nil {
			client.Close()
			return nil, fmt.Errorf("failed to authenticate with SMTP server: %s", err)
		}
	}

	// Send NOOP command to verify connection and authentication were successful.
	if err = client.Noop(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to verify SMTP server connection by sending NOOP command: %s", err)
	}
	return client, nil
}

// sendMail sends an email.
func (s *Server) sendMail(client *smtp.Client, recipient string, subject string, message string) error {
	// Build mail body.
	var body []byte
	if s.config.MailFrom != "" {
		body = fmt.Appendf(body, "From: %s\n", s.config.MailFrom)
	}
	body = fmt.Appendf(body, "To: %s\nSubject: %s\n", recipient, subject)
	if s.config.MailReplyTo != "" {
		body = fmt.Appendf(body, "Reply-To: %s\n", s.config.MailReplyTo)
	}
	body = fmt.Appendf(body, "\n%s", message)

	// Reset state.
	if err := client.Reset(); err != nil {
		return fmt.Errorf("failed to reset state: %s", err)
	}

	// Set "From" and "To" addresses.
	if s.config.MailFrom != "" {
		if err := client.Mail(s.config.MailFrom); err != nil {
			return fmt.Errorf("failed to set from address: %s", err)
		}
	}
	if err := client.Rcpt(recipient); err != nil {
		return fmt.Errorf("failed to set recipient address: %s", err)
	}

	// Initiate data transfer.
	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to send DATA command: %s", err)
	}

	// Write mail body.
	_, err = wc.Write(body)
	if err != nil {
		return fmt.Errorf("failed to write email body: %s", err)
	}

	// Complete data transfer.
	err = wc.Close()
	if err != nil {
		return fmt.Errorf("failed to write email body: %s", err)
	}
	return nil
}

// begin acquires a database connection from the pool and starts a transaction.
func (s *Server) begin() *database.DB {
	return database.Begin(s.dbPool, s.config)
}

// loadServerConfig loads the server configuration and sets default values.
func (s *Server) loadServerConfig() error {
	db := s.begin()
	defer db.Commit()

	siteName := db.GetString("sitename")
	if siteName == "" {
		siteName = defaultServerSiteName
	}
	s.opt.SiteName = siteName

	siteHome := db.GetString("sitehome")
	if siteHome == "" {
		siteHome = defaultServerSiteHome
	}
	s.opt.SiteHome = siteHome

	siteIndex := db.GetString("siteindex")
	s.opt.SiteIndex = siteIndex == "" || siteIndex == "1"

	news := NewsOption(db.GetInt("news"))
	if news == NewsDisable || news == NewsWriteToNews || news == NewsWriteToIndex {
		s.opt.News = news
	}

	boardIndex := db.GetString("boardindex")
	s.opt.BoardIndex = boardIndex == "" || boardIndex == "1"

	s.opt.CAPTCHA = db.GetBool("captcha")

	oekakiWidth := db.GetInt("oekakiwidth")
	if oekakiWidth == 0 {
		oekakiWidth = defaultServerOekakiWidth
	}
	s.opt.OekakiWidth = oekakiWidth

	oekakiHeight := db.GetInt("oekakiheight")
	if oekakiHeight == 0 {
		oekakiHeight = defaultServerOekakiHeight
	}
	s.opt.OekakiHeight = oekakiHeight

	if !db.HaveConfig("refresh") {
		s.opt.Refresh = defaultServerRefresh
	} else {
		s.opt.Refresh = db.GetInt("refresh")
	}

	s.opt.Overboard = db.GetString("overboard")
	s.opt.OverboardType = BoardType(db.GetInt("overboardtype"))
	s.opt.OverboardThreads = db.GetInt("overboardthreads")
	s.opt.OverboardReplies = db.GetInt("overboardreplies")

	s.opt.Uploads = s.config.UploadTypes()

	s.opt.Embeds = nil
	if !db.HaveConfig("embeds") {
		s.opt.Embeds = append(s.opt.Embeds, defaultServerEmbeds...)
	} else {
		embeds := db.GetMultiString("embeds")
		for _, v := range embeds {
			split := strings.SplitN(v, " ", 2)
			if len(split) != 2 {
				continue
			}
			s.opt.Embeds = append(s.opt.Embeds, [2]string{split[0], split[1]})
		}
	}
	db.ClearBoardCache()
	s.removeInvalidBoardOptions(db)

	s.reloadBans(db)

	s.opt.Identifiers = s.config.Identifiers

	s.opt.Locale = s.config.Locale

	s.opt.Locales = make(map[string]string)
	english := display.English.Languages()
	fs.WalkDir(localeFS, "locale", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		} else if d.IsDir() || !strings.HasSuffix(p, ".po") {
			return nil
		}
		id := filepath.Base(strings.TrimSuffix(p, ".po"))

		name := id
		tag, err := language.Parse(id)
		if err == nil {
			tagName := english.Name(tag)
			if tagName != "" {
				name = tagName
			}
		}

		s.opt.Locales[id] = name
		return nil
	})
	s.opt.Locales["en@pirate"] = "Pirate English"
	if s.opt.Locale != "" && s.opt.Locale != "en" {
		s.opt.Locales["en"] = "English"
	}
	s.opt.LocalesSorted = slices.SortedFunc(maps.Keys(s.opt.Locales), func(s1, s2 string) int {
		return strings.Compare(s.opt.Locales[s1], s.opt.Locales[s2])
	})

	templateFuncMaps = make(map[string]template.FuncMap)
	templateFuncMaps[""] = newTemplateFuncMap(s.opt.Locale)
	for id := range s.opt.Locales {
		templateFuncMaps[id] = newTemplateFuncMap(id)
	}

	s.opt.Access = make(map[string]string)
	maps.Copy(s.opt.Access, s.config.Access)
	return nil
}

// setDefaultPluginConfig sets default plugin configuration values.
func (s *Server) setDefaultPluginConfig() error {
	db := s.begin()
	defer db.Commit()

	for i, info := range allPluginInfo {
		db.Plugin = info.Name

		for i, config := range info.Config {
			if !db.HaveConfig(config.Name) {
				db.SaveString(config.Name, config.Value)
			} else {
				info.Config[i].Value = db.GetString(config.Name)
			}
		}

		p := allPlugins[i]
		pUpdate, ok := p.(sriracha.PluginWithUpdate)
		if ok {
			for _, config := range info.Config {
				pUpdate.Update(db, config.Name)
			}
		}
	}
	return nil
}

// loadPluginConfig loads plugin configuration options.
func (s *Server) loadPluginConfig() error {
	db := s.begin()
	defer db.Commit()

	for _, info := range allPluginInfo {
		db.Plugin = info.Name
		for i, c := range info.Config {
			v := db.GetString(strings.ToLower(info.Name + "." + c.Name))
			if v != "" {
				info.Config[i].Value = v
			}
		}
	}
	db.Plugin = ""
	return nil
}

// officialTemplateDir searches for the path to the official template directory and returns it.
func (s *Server) officialTemplateDir() string {
	officialDir := "internal/server/template"
	_, err := os.Stat(officialDir)
	if !os.IsNotExist(err) {
		return officialDir
	}
	officialDir = "template"
	_, err = os.Stat(officialDir)
	if !os.IsNotExist(err) {
		return officialDir
	}
	return ""
}

// validateTemplateConfig validates whether the official and custom template
// directories are unique and accessible.
func (s *Server) validateTemplateConfig(officialDir string) error {
	if s.config.Template == "" {
		return nil
	}
	_, err := os.Stat(s.config.Template)
	if os.IsNotExist(err) {
		return fmt.Errorf("custom template directory %s does not exist", s.config.Template)
	}
	officialTemplate, err := os.Stat(officialDir)
	if err != nil {
		return fmt.Errorf("failed to locate official template directory: start sriracha in the same directory as the file README.md")
	}
	customTemplate, err := os.Stat(s.config.Template)
	if err != nil {
		return fmt.Errorf("custom template directory %s is inaccessible", s.config.Template)
	}
	if os.SameFile(officialTemplate, customTemplate) {
		return fmt.Errorf("official templates and custom templates must be located in separate directories")
	}
	return nil
}

// parseTemplates parses official and custom templates. Provide an empty
// officialDir to load official templates from the embedded file system.
// Otherwise, official templates are loaded from disk. When customDir is set,
// custom templates are loaded from disk.
func (s *Server) parseTemplates(officialDir string, customDir string) error {
	s.customTemplates = s.customTemplates[:0]
	wrapError := func(name string, err error) error {
		var source string
		if !slices.Contains(s.customTemplates, name) {
			source = "official"
		} else {
			source = "custom"
		}
		return fmt.Errorf("failed to parse %s template file %s: %s", source, name, err)
	}
	parseDir := func(dir string, custom bool) error {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, f := range entries {
			if !strings.HasSuffix(f.Name(), ".gohtml") {
				continue
			} else if custom {
				s.customTemplates = append(s.customTemplates, f.Name())
			}

			buf, err := os.ReadFile(filepath.Join(dir, f.Name()))
			if err != nil {
				return wrapError(f.Name(), err)
			}

			_, err = s.tpl.New(f.Name()).Parse(string(buf))
			if err != nil {
				return wrapError(f.Name(), err)
			}
		}
		return nil
	}
	if officialDir == "" {
		s.tpl = template.New("sriracha").Funcs(templateFuncMaps[""])

		entries, err := templateFS.ReadDir("template")
		if err != nil {
			return err
		}
		for _, f := range entries {
			if !strings.HasSuffix(f.Name(), ".gohtml") {
				continue
			}

			buf, err := templateFS.ReadFile(filepath.Join("template", f.Name()))
			if err != nil {
				return wrapError(f.Name(), err)
			}

			_, err = s.tpl.New(f.Name()).Parse(string(buf))
			if err != nil {
				return wrapError(f.Name(), err)
			}
		}
	} else {
		s.tpl = template.New("sriracha").Funcs(templateFuncMaps[""])

		err := parseDir(officialDir, false)
		if err != nil {
			return err
		}
	}

	if customDir != "" {
		err := parseDir(customDir, true)
		if err != nil {
			return err
		}
	}

	var err error
	s.original, err = s.tpl.Clone()
	return err
}

func (s *Server) _watchTemplates(officialDir string, watcher *fsnotify.Watcher) {
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			} else if !event.Has(fsnotify.Create) && !event.Has(fsnotify.Write) && !event.Has(fsnotify.Remove) && !event.Has(fsnotify.Rename) {
				continue
			}
			err := s.parseTemplates(officialDir, s.config.Template)
			if err != nil {
				log.Printf("failed to parse template files: %s", err)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("fsnotify error: %s", err)
		}
	}
}

// watchTemplates watches the official and custom template directories for changes.
func (s *Server) watchTemplates(officialDir string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	go s._watchTemplates(officialDir, watcher)

	err = watcher.Add(officialDir)
	if err == nil && s.config.Template != "" {
		err = watcher.Add(s.config.Template)
	}
	return err
}

// log adds an entry to the audit log.
func (s *Server) log(db *database.DB, account *Account, board *Board, action string, info string) {
	user := "system"
	if account != nil && account.ID != 0 {
		if account.Role == RoleSuperAdmin || account.Role == RoleAdmin {
			user = "admin"
		} else {
			user = "mod"
		}
	}
	for _, handlerInfo := range allPluginAuditHandlers {
		db.Plugin = handlerInfo.Name
		err := handlerInfo.Handler(db, user, action, info)
		if err != nil {
			log.Fatalf("plugin %s failed to process audit event: %s", handlerInfo.Name, err)
		}
	}
	db.Plugin = ""

	db.AddLog(&Log{
		Account: account,
		Board:   board,
		Message: action,
		Changes: info,
	})
}

// refreshBannerCache refreshes the banner cache.
func (s *Server) refreshBannerCache(db *database.DB) {
	banners := s.opt.Banners
	for id := range banners {
		banners[id] = banners[id][:0]
	}

	for _, banner := range db.AllBanners() {
		for _, board := range banner.Boards {
			banners[board.ID] = append(banners[board.ID], banner)
		}
		if banner.Overboard {
			banners[bannerOverboard] = append(banners[bannerOverboard], banner)
		}
		if banner.News {
			banners[bannerNews] = append(banners[bannerNews], banner)
		}
		if banner.Pages {
			banners[bannerPages] = append(banners[bannerPages], banner)
		}
	}

	for id := range banners {
		if len(banners[id]) == 0 {
			delete(banners, id)
		}
	}
}

// refreshRulesCache refreshes the board rules cache.
func (s *Server) refreshRulesCache(db *database.DB) {
	rules := s.opt.Rules
	for id := range rules {
		rules[id] = rules[id][:0]
	}

	for _, board := range db.AllBoards() {
		for _, info := range allPluginRulesHandlers {
			rulesHTML, err := info.Handler(db, board)
			if err != nil {
				log.Fatalf("failed to refresh rules cache: plugin %s encountered an error: %s", info.Name, err)
			}
			rulesText := strings.TrimSpace(string(rulesHTML))
			if rulesText == "" {
				continue
			}
			for _, rulesLine := range strings.Split(rulesText, "\n") {
				if rulesLine == "" {
					continue
				}
				rules[board.ID] = append(rules[board.ID], template.HTML(rulesLine))
			}
		}
	}

	for id := range rules {
		if len(rules[id]) == 0 {
			delete(rules, id)
		}
	}
}

// refreshKeywordCache refreshes the keyword cache.
func (s *Server) refreshKeywordCache(db *database.DB) {
	for boardID := range s.keywordCache {
		s.keywordCache[boardID] = s.keywordCache[boardID][:0]
	}

	for _, k := range db.AllKeywords() {
		var err error
		kw := &cachedKeyword{
			id: k.ID,
			a:  k.Action,
		}
		kw.p, err = regexp.Compile(k.Text)
		if err != nil {
			log.Fatalf("failed to parse keyword %s as regular expression: %s", k.Text, err)
		}
		for _, board := range k.Boards {
			s.keywordCache[board.ID] = append(s.keywordCache[board.ID], kw)
		}
	}

	for boardID := range s.keywordCache {
		if len(s.keywordCache[boardID]) == 0 {
			delete(s.keywordCache, boardID)
		}
	}
}

func (s *Server) _processCategory(c *Category) {
	for _, cat := range c.Categories {
		s._processCategory(cat)
	}
	if len(c.Boards) == 0 {
		return
	}
	info := &categoryInfo{
		Name:        c.Name,
		Description: c.Description,
		Boards:      c.Boards,
	}
	s.opt.Categories = append(s.opt.Categories, info)
}

// refreshCategoryCache refreshes the category cache.
func (s *Server) refreshCategoryCache(db *database.DB) {
	s.opt.Categories = s.opt.Categories[:0]
	for _, c := range db.AllCategories() {
		if c.Parent == nil {
			s._processCategory(c)
		}
	}
	// Create pseudo-category.
	if len(s.opt.Categories) == 0 {
		info := &categoryInfo{}
		for _, b := range db.AllBoards() {
			if b.Hide == HideIndex || b.Hide == HideEverywhere {
				continue
			}
			info.Boards = append(info.Boards, b)
		}
		s.opt.Categories = append(s.opt.Categories, info)
	}
}

func (s *Server) refreshRecentPosts(db *database.DB) {
	for _, info := range s.opt.Categories {
		info.Recent = info.Recent[:0]
		for _, b := range info.Boards {
			info.Recent = append(info.Recent, db.LastPostByBoard(b))
		}
	}
}

// deletePostFiles deletes files associated with a post.
func (s *Server) deletePostFiles(p *Post) {
	if p.Board == nil {
		return
	} else if p.ID != 0 && p.Parent == 0 {
		os.Remove(filepath.Join(s.config.Root, p.Board.Dir, "res", fmt.Sprintf("%d.html", p.ID)))
	}

	if p.File == "" {
		return
	}
	srcPath := filepath.Join(s.config.Root, p.Board.Dir, "src", p.File)
	os.Remove(srcPath)

	if p.Thumb == "" {
		return
	}
	thumbPath := filepath.Join(s.config.Root, p.Board.Dir, "thumb", p.Thumb)
	os.Remove(thumbPath)
}

// deletePost deletes a post from the database as well as any associated files.
func (s *Server) deletePost(db *database.DB, p *Post) {
	posts := db.AllPostsInThread(p.ID, false)
	for _, post := range posts {
		s.deletePostFiles(post)
	}

	db.DeletePost(p.ID)
}

// httpResponse executes a HTTP request and returns the response.
func (s *Server) httpResponse(r *http.Request) (*http.Response, error) {
	r.Header.Set("User-Agent", "Sriracha imageboard and forum server")
	return s.httpClient.Do(r)
}

// buildData returns a new template data instance.
func (s *Server) buildData(db *database.DB, w http.ResponseWriter, r *http.Request) *templateData {
	if strings.HasPrefix(r.URL.Path, "/sriracha/logout") {
		http.SetCookie(w, &http.Cookie{
			Name:  "sriracha_session",
			Value: "",
			Path:  "/",
		})
		http.Redirect(w, r, "/sriracha/", http.StatusFound)
		return s.newTemplateData()
	}

	if r.URL.Path == "/sriracha/" || r.URL.Path == "/sriracha" {
		var failedLogin bool
		username := r.FormValue("username")
		if len(username) != 0 {
			failedLogin = true
			password := r.FormValue("password")
			if len(password) != 0 {
				if !s.opt.DevMode {
					// Verify CAPTCHA.
					var solved bool
					ipHash := s.hashIP(r)
					challenge := db.GetCAPTCHA(ipHash)
					if challenge != nil {
						solution := FormString(r, "captcha")
						if strings.ToLower(solution) == challenge.Text {
							solved = true
							db.DeleteCAPTCHA(ipHash)
							os.Remove(filepath.Join(s.config.Root, "captcha", challenge.Image+".png"))
						}
					}
					if !solved {
						data := s.newTemplateData()
						data.Info = "Invalid CAPTCHA."
						data.Template = "manage_error"
						return data
					}
				}

				// Verify username and password.
				account := db.LoginAccount(username, password)
				if account != nil {
					http.SetCookie(w, &http.Cookie{
						Name:  "sriracha_session",
						Value: account.Session,
						Path:  "/",
					})
					data := s.newTemplateData()
					data.Account = account
					if s.config.ImportMode {
						data.Redirect(w, r, "/sriracha/import/")
					}
					return data
				}
			}
		}
		if failedLogin {
			data := s.newTemplateData()
			data.Info = "Invalid username or password."
			data.Template = "manage_error"
			return data
		}
	}

	cookies := r.CookiesNamed("sriracha_session")
	if len(cookies) > 0 {
		account := db.AccountBySessionKey(cookies[0].Value)
		if account != nil {
			data := s.newTemplateData()
			data.Account = account
			return data
		}
	}
	return s.newTemplateData()
}

// writeThread writes a thread res page to disk.
func (s *Server) writeThread(db *database.DB, board *Board, postID int) {
	posts := db.AllPostsInThread(postID, true)
	if len(posts) == 0 {
		return
	}

	if board.Unique == 0 {
		board.Unique = db.UniqueUserPosts(board)
	}

	f, err := os.OpenFile(filepath.Join(s.config.Root, board.Dir, "res", fmt.Sprintf("%d.html", postID)), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, NewFilePermission)
	if err != nil {
		log.Fatal(err)
	}

	data := s.newTemplateData()
	data.Board = board
	data.Boards = db.AllBoards()
	data.Threads = [][]*Post{posts}
	data.ReplyMode = postID
	data.Template = "board_page"
	data.execute(f)
}

// writeBoardIndexes writes board index pages to disk.
func (s *Server) writeBoardIndexes(db *database.DB, board *Board) {
	if board.Unique == 0 {
		board.Unique = db.UniqueUserPosts(board)
	}

	data := s.newTemplateData()
	data.Board = board
	data.Boards = db.AllBoards()
	data.ReplyMode = 1
	data.Template = "board_catalog"

	threadInfo := db.AllThreads(board, true)

	// Write catalog.
	if board.Type == TypeImageboard {
		catalogFile, err := os.OpenFile(filepath.Join(s.config.Root, board.Dir, "catalog.html"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, NewFilePermission)
		if err != nil {
			log.Fatal(err)
		}

		for _, info := range threadInfo {
			thread := db.PostByID(info[0])
			thread.Replies = info[1]
			data.Threads = append(data.Threads, []*Post{thread})
		}
		data.execute(catalogFile)

		catalogFile.Close()
	}

	// Write indexes.

	data.ReplyMode = 0
	data.Template = "board_page"
	data.Pages = pageCount(len(threadInfo), board.Threads)
	for page := 0; page < data.Pages; page++ {
		fileName := "index.html"
		if page > 0 {
			fileName = fmt.Sprintf("%d.html", page)
		}

		indexFile, err := os.OpenFile(filepath.Join(s.config.Root, board.Dir, fileName), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, NewFilePermission)
		if err != nil {
			log.Fatal(err)
		}

		start := page * board.Threads
		end := len(threadInfo)
		if board.Threads != 0 && end > start+board.Threads {
			end = start + board.Threads
		}

		data.Threads = data.Threads[:0]
		for _, info := range threadInfo[start:end] {
			thread := db.PostByID(info[0])
			thread.Replies = info[1]
			posts := []*Post{thread}
			if board.Type == TypeImageboard {
				posts = append(posts, db.AllReplies(thread.ID, board.Replies, true)...)
			}
			data.Threads = append(data.Threads, posts)
		}
		data.Page = page
		data.execute(indexFile)

		indexFile.Close()
	}
}

// writeOverboard writes overboard pages to disk.
func (s *Server) writeOverboard(db *database.DB) {
	var overboardDir string
	if s.opt.Overboard != "/" {
		overboardDir = s.opt.Overboard
	}

	overboard := &Board{
		ID:      -1,
		Type:    s.opt.OverboardType,
		Name:    gotext.Get("Overboard"),
		Dir:     overboardDir,
		Threads: s.opt.OverboardThreads,
		Replies: s.opt.OverboardReplies,
	}

	data := s.newTemplateData()
	data.Board = overboard
	data.Boards = db.AllBoards()
	data.ReplyMode = 1
	data.Template = "board_catalog"

	threadInfo := db.AllThreads(nil, true)

	// Write catalog.
	if overboard.Type == TypeImageboard {
		catalogFile, err := os.OpenFile(filepath.Join(s.config.Root, overboardDir, "catalog.html"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, NewFilePermission)
		if err != nil {
			log.Fatal(err)
		}

		for _, info := range threadInfo {
			thread := db.PostByID(info[0])
			thread.Replies = info[1]
			data.Threads = append(data.Threads, []*Post{thread})
		}
		data.execute(catalogFile)

		catalogFile.Close()
	}

	// Write indexes.

	data.ReplyMode = 0
	data.Template = "board_page"
	data.Pages = pageCount(len(threadInfo), overboard.Threads)
	for page := 0; page < data.Pages; page++ {
		fileName := "index.html"
		if page > 0 {
			fileName = fmt.Sprintf("%d.html", page)
		}

		indexFile, err := os.OpenFile(filepath.Join(s.config.Root, overboardDir, fileName), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, NewFilePermission)
		if err != nil {
			log.Fatal(err)
		}

		start := page * overboard.Threads
		end := len(threadInfo)
		if overboard.Threads != 0 && end > start+overboard.Threads {
			end = start + overboard.Threads
		}

		data.Threads = data.Threads[:0]
		for _, info := range threadInfo[start:end] {
			thread := db.PostByID(info[0])
			thread.Replies = info[1]
			posts := []*Post{thread}
			if overboard.Type == TypeImageboard {
				posts = append(posts, db.AllReplies(thread.ID, overboard.Replies, true)...)
			}
			data.Threads = append(data.Threads, posts)
		}
		data.Page = page
		data.execute(indexFile)

		indexFile.Close()
	}
}

// newPageTemplate returns a new collection of templates with read-only database access.
func (s *Server) newPageTemplate(db *database.DB) *template.Template {
	tpl, err := s.original.Clone()
	if err != nil {
		log.Fatal(err)
	}
	return tpl.Funcs(map[string]any{
		// Board.
		"BoardByID":       db.BoardByID,
		"BoardByDir":      db.BoardByDir,
		"UniqueUserPosts": db.UniqueUserPosts,
		"AllBoards":       db.AllBoards,
		// News.
		"NewsByID": db.NewsByID,
		"AllNews":  db.AllNews,
		// Page.
		"PageByID":   db.PageByID,
		"PageByPath": db.PageByPath,
		"AllPages":   db.AllPages,
		// Post.
		"AllThreads":       db.AllThreads,
		"AllPostsInThread": db.AllPostsInThread,
		"AllReplies":       db.AllReplies,
		"PendingPosts":     db.PendingPosts,
		"PostByID":         db.PostByID,
		"PostsByIP":        db.PostsByIP,
		"PostsByFileHash":  db.PostsByFileHash,
		"PostByField":      db.PostByField,
		"LastPostByIP":     db.LastPostByIP,
		"ReplyCount":       db.ReplyCount,
	})
}

// writePages writes custom pages to disk.
func (s *Server) writePages(db *database.DB, pages []*Page, dryRun bool) error {
	data := s.newTemplateData()
	data.Boards = db.AllBoards()
	data.Template = "page"

	tpl := s.newPageTemplate(db)
	for _, p := range pages {
		err := p.Validate()
		if err != nil {
			if dryRun {
				return err
			}
			log.Printf("Warning: skipped invalid page %s: %s", p.Path, err)
			continue
		}

		dir := filepath.Dir(p.Path)
		if dir != "" {
			dirPath := filepath.Join(s.config.Root, dir)
			_, err := os.Stat(dirPath)
			if os.IsNotExist(err) {
				os.MkdirAll(dirPath, NewDirPermission)
			}
		}

		var w io.Writer
		var pageFile *os.File
		if dryRun {
			w = io.Discard
		} else {
			pageFile, err = os.OpenFile(filepath.Join(s.config.Root, p.Path+".html"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, NewFilePermission)
			if err != nil {
				log.Fatal(err)
			}
			w = pageFile
		}

		data.tpl, err = tpl.Clone()
		if err != nil {
			log.Fatal(err)
		}
		data.tpl, err = data.tpl.New("line").Parse(p.Content)
		if err != nil {
			if dryRun {
				return err
			}
			log.Printf("Warning: failed to parse content of page %s: %s", p.Path, err)
			pageFile.Close()
			continue
		}

		if strings.HasPrefix(p.Content, doctypePrefx) {
			data.Template = "line"
		} else {
			data.Template = "page"
		}
		err = data.executeWithError(w)
		if err != nil {
			if dryRun {
				return err
			}
			log.Printf("Warning: failed to render page %s: %s", p.Path, err)
			pageFile.Close()
			continue
		}

		if !dryRun {
			pageFile.Close()
		}
	}
	return nil
}

// rebuildBoard rebuilds a thread res page and board index pages.
func (s *Server) rebuildThread(db *database.DB, post *Post) {
	s.writeThread(db, post.Board, post.Thread())
	s.writeBoardIndexes(db, post.Board)
	if s.opt.Overboard != "" {
		s.writeOverboard(db)
	}
}

// rebuildBoard rebuilds all pages in a board.
func (s *Server) rebuildBoard(db *database.DB, board *Board) {
	for _, info := range db.AllThreads(board, true) {
		s.writeThread(db, board, info[0])
	}
	s.writeBoardIndexes(db, board)
}

// rebuildAll rebuilds all board, overboard, news and custom pages.
func (s *Server) rebuildAll(db *database.DB, verbose bool) {
	allPages := db.AllPages()
	if len(allPages) > 0 {
		if verbose {
			fmt.Println("Rebuilding pages...")
		}
		s.writePages(db, allPages, false)
	}
	published := len(db.AllNews(true))
	if published > 0 {
		if verbose {
			fmt.Println("Rebuilding news...")
		}
		s.rebuildNews(db)
	}
	if s.opt.Overboard != "" {
		if verbose {
			fmt.Println("Rebuilding overboard...")
		}
		s.writeOverboard(db)
	}
	for _, b := range db.AllBoards() {
		if verbose {
			fmt.Printf("Rebuilding %s...\n", b.Path())
		}
		s.rebuildBoard(db, b)
	}
	s.writeSiteIndex(db)
	s.writeVisitorGuide(db)
}

// writeNewsItem writes a news entry page to disk.
func (s *Server) writeNewsItem(db *database.DB, n *News) {
	if n.ID <= 0 {
		return
	}

	data := s.newTemplateData()
	data.Boards = db.AllBoards()
	data.Template = "news"
	data.AllNews = []*News{n}
	data.Pages = 1
	data.Extra = "view"

	itemFile, err := os.OpenFile(filepath.Join(s.config.Root, fmt.Sprintf("news-%d.html", n.ID)), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, NewFilePermission)
	if err != nil {
		log.Fatal(err)
	}
	data.execute(itemFile)
	itemFile.Close()
}

// writeNewsIndexes writes news index pages to disk.
func (s *Server) writeNewsIndexes(db *database.DB) {
	allNews := db.AllNews(true)
	data := s.newTemplateData()
	data.Boards = db.AllBoards()
	data.Template = "news"

	const newsCount = 10
	data.Pages = pageCount(len(allNews), newsCount)
	for page := 0; page < data.Pages; page++ {
		fileName := "news.html"
		if s.opt.News == NewsWriteToIndex {
			fileName = "index.html"
		}
		if page > 0 {
			fileName = fmt.Sprintf("news-p%d.html", page)
		}

		indexFile, err := os.OpenFile(filepath.Join(s.config.Root, fileName), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, NewFilePermission)
		if err != nil {
			log.Fatal(err)
		}

		start := page * newsCount
		end := len(allNews)
		if newsCount != 0 && end > start+newsCount {
			end = start + newsCount
		}

		data.AllNews = allNews[start:end]
		data.Page = page
		data.execute(indexFile)

		indexFile.Close()
	}
}

// rebuildNewsItem rebuilds a news entry.
func (s *Server) rebuildNewsItem(db *database.DB, n *News) {
	s.writeNewsItem(db, n)
	s.writeNewsIndexes(db)
}

// rebuildNews rebuilds all news entries.
func (s *Server) rebuildNews(db *database.DB) {
	for _, n := range db.AllNews(true) {
		s.writeNewsItem(db, n)
	}
	s.writeNewsIndexes(db)
}

// writeVisitorGuide writes the visitor guide to disk.
func (s *Server) writeVisitorGuide(db *database.DB) {
	data := s.newTemplateData()
	data.Template = "guide"
	data.Boards = db.AllBoards()

	file, err := os.OpenFile(filepath.Join(s.config.Root, "guide.html"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, NewFilePermission)
	if err != nil {
		log.Fatal(err)
	}
	data.execute(file)
	file.Close()
}

// writeSiteIndex writes the site index page to disk.
func (s *Server) writeSiteIndex(db *database.DB) {
	if !s.opt.SiteIndex || s.opt.News == NewsWriteToIndex || s.opt.Overboard == "/" || db.BoardByDir("") != nil {
		return
	}
	allBoards := db.AllBoards()
	var keep []*Board
	for _, board := range allBoards {
		if board.Hide == HideIndex || board.Hide == HideEverywhere {
			continue
		}
		keep = append(keep, board)
	}
	allBoards = keep
	if len(allBoards) < 2 {
		return
	}
	data := s.newTemplateData()
	data.Template = "index"

	data.Boards = allBoards

	if s.opt.News != NewsDisable {
		allNews := db.AllNews(true)
		var latest *News
		if len(allNews) > 0 {
			latest = allNews[0]
		}
		data.News = latest
	}

	s.refreshRecentPosts(db)

	indexFile, err := os.OpenFile(filepath.Join(s.config.Root, "index.html"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, NewFilePermission)
	if err != nil {
		log.Fatal(err)
	}
	data.execute(indexFile)
	indexFile.Close()
}

// removeInvalidBoardOptions removes invalid board options from the database.
func (s *Server) removeInvalidBoardOptions(db *database.DB) {
	for _, b := range db.AllBoards() {
		var keep []string
		var modified bool
		for _, boardEmbed := range b.Embeds {
			var found bool
			for _, serverEmbed := range s.opt.Embeds {
				if serverEmbed[0] == boardEmbed {
					found = true
					break
				}
			}
			if !found {
				modified = true
				continue
			}
			keep = append(keep, boardEmbed)
		}
		if modified {
			b.Embeds = keep
			db.UpdateBoard(b)
		}
	}
}

// reloadBans refreshes the range ban regular expression cache.
func (s *Server) reloadBans(db *database.DB) {
	var rangeBans = make(map[*Ban]*regexp.Regexp)
	bans := db.AllBans(true)
	for _, ban := range bans {
		pattern, err := regexp.Compile(ban.IP[2:])
		if err != nil {
			log.Printf("warning: failed to compile IP range ban `%s` as regular expression: %s", ban.IP[2:], err)
			return
		}
		rangeBans[ban] = pattern
	}
	s.rangeBans = rangeBans
}

// serveManage serves management panel web requests.
func (s *Server) serveManage(db *database.DB, w http.ResponseWriter, r *http.Request) {
	data := s.buildData(db, w, r)
	if strings.HasPrefix(r.URL.Path, "/sriracha/logout") {
		return
	}
	var skipExecute bool

	if len(data.Info) != 0 {
		data.Template = "manage_error"
		data.execute(w)
		return
	}

	if strings.HasPrefix(r.URL.Path, "/sriracha/oekaki/") {
		postID := PathInt(r, "/sriracha/oekaki/")
		post := db.PostByID(postID)
		if post == nil || !post.IsOekaki() {
			data.BoardError(w, "invalid or deleted post")
			return
		}

		data := s.buildData(db, w, r)
		data.Template = "oekaki"
		data.Message2 = template.HTML(`
		<script type="text/javascript">
		Tegaki.open({
			width: ` + strconv.Itoa(s.opt.OekakiWidth) + `,
			height: ` + strconv.Itoa(s.opt.OekakiHeight) + `,
			replayMode: true,
			replayURL: '` + post.Board.Path() + `src/` + post.File + `'
		});
		document.getElementById('tegaki-finish-btn').addEventListener('click', function(e) {
			window.close();
			return false;
		});
		</script>`)
		data.execute(w)
		return
	}

	if data.Account != nil {
		db.UpdateAccountLastActive(data.Account.ID)
	}

	data.Template = "manage_login"

	if data.Account == nil {
		data.execute(w)
		return
	} else if s.config.ImportMode {
		if data.Account.Role != RoleSuperAdmin {
			data.ManageError("Sriracha is running in import mode. Only super-administrators may log in.")
			data.execute(w)
			return
		} else if !strings.HasPrefix(r.URL.Path, "/sriracha/import/") {
			data.Redirect(w, r, "/sriracha/import/")
			return
		}
		data.Info = "IMPORT MODE"
	}

	switch {
	case strings.HasPrefix(r.URL.Path, "/sriracha/preference"):
		s.servePreference(data, db, w, r)
	case strings.HasPrefix(r.URL.Path, "/sriracha/account"):
		s.serveAccount(data, db, w, r)
	case strings.HasPrefix(r.URL.Path, "/sriracha/banner"):
		s.serveBanner(data, db, w, r)
	case strings.HasPrefix(r.URL.Path, "/sriracha/ban"):
		s.serveBan(data, db, w, r)
	case strings.HasPrefix(r.URL.Path, "/sriracha/board"):
		skipExecute = s.serveBoard(data, db, w, r)
	case strings.HasPrefix(r.URL.Path, "/sriracha/category"):
		s.serveCategory(data, db, w, r)
	case strings.HasPrefix(r.URL.Path, "/sriracha/import"):
		s.serveImport(data, db, w, r)
	case strings.HasPrefix(r.URL.Path, "/sriracha/keyword"):
		s.serveKeyword(data, db, w, r)
	case strings.HasPrefix(r.URL.Path, "/sriracha/log"):
		s.serveLog(data, db, w, r)
	case strings.HasPrefix(r.URL.Path, "/sriracha/mod"):
		s.serveMod(data, db, w, r)
	case strings.HasPrefix(r.URL.Path, "/sriracha/news"):
		s.serveNews(data, db, w, r)
	case strings.HasPrefix(r.URL.Path, "/sriracha/page"):
		s.servePage(data, db, w, r)
	case strings.HasPrefix(r.URL.Path, "/sriracha/plugin"):
		s.servePlugin(data, db, w, r)
	case strings.HasPrefix(r.URL.Path, "/sriracha/setting"):
		s.serveSetting(data, db, w, r)
	default:
		s.serveStatus(data, db, w, r)
	}

	if skipExecute {
		return
	}
	data.execute(w)
}

// serve serves web requests.
func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	s.lock.Lock()
	defer s.lock.Unlock()

	w.Header().Set("Server", "Sriracha GNU LGPL")

	if r.Method == http.MethodPost {
		const maxMemory = 32 << 20 // 32 megabytes.
		r.ParseMultipartForm(maxMemory)
		if r.MultipartForm != nil {
			defer r.MultipartForm.RemoveAll()
		}

		var modified bool
		f := make(url.Values)
		for key, values := range r.Form {
			f[key] = make([]string, len(values))
			for i := range values {
				modified = true
				f[key][i] = strings.ReplaceAll(values[i], "\r", "")
			}
		}
		if modified {
			r.Form = f
		}
	}

	var action string
	if r.URL.Path == "/sriracha/" || r.URL.Path == "/sriracha" {
		action = r.FormValue("action")
		if action == "" {
			values := r.URL.Query()
			action = values.Get("action")
		}
	} else if strings.HasPrefix(r.URL.Path, "/sriracha/captcha/") {
		action = "captcha"
	}

	db := s.begin()
	defer db.Commit()

	db.DeleteExpiredSubscriptions()

	if db.DeleteExpiredBans() > 0 {
		s.reloadBans(db)
	}

	// Check IP range ban.
	ip := s.requestIP(r)
	for ban, pattern := range s.rangeBans {
		if pattern.MatchString(ip) {
			data := s.buildData(db, w, r)
			data.ManageError("You are banned. " + ban.Info() + fmt.Sprintf(" (Ban #%d)", ban.ID))
			data.execute(w)
			return
		}
	}

	// Check static IP ban.
	ban := db.BanByIP(s.hashIP(r))
	if ban != nil {
		data := s.buildData(db, w, r)
		data.ManageError("You are banned. " + ban.Info() + fmt.Sprintf(" (Ban #%d)", ban.ID))
		data.execute(w)
		return
	} else if strings.HasPrefix(r.URL.Path, "/sriracha/post/") {
		postID := PathInt(r, "/sriracha/post/")
		post := db.PostByID(postID)
		data := s.buildData(db, w, r)
		if post == nil {
			data.BoardError(w, "Invalid or deleted post.")
		} else {
			data.Redirect(w, r, fmt.Sprintf("%sres/%d.html#%d", post.Board.Path(), post.Thread(), post.ID))
		}
		return
	}

	if strings.HasPrefix(r.URL.Path, "/sriracha/subscribe") {
		action = "subscribe"
	}

	if s.config.ImportMode && action != "" {
		data := s.buildData(db, w, r)
		data.BoardError(w, "Sriracha is running in import mode. All boards are currently locked. Please wait and try again.")
	} else {
		switch action {
		case "post":
			s.servePost(db, w, r)
		case "report":
			s.serveReport(db, w, r)
		case "delete":
			s.serveDelete(db, w, r)
		case "captcha":
			s.serveCAPTCHA(db, w, r)
		case "subscribe":
			s.serveSubscribe(db, w, r)
		default:
			s.serveManage(db, w, r)
		}
	}
}

var cachePattern = regexp.MustCompile(`^.*\.(aac|avi|css|flac|gif|ico|jpg|js|mp3|mp4|ogg|opus|png|svg|swf|wasm|wav|webm|webp|woff)$`)

func withCacheHeader(fs http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cachePattern.MatchString(r.URL.Path) {
			// Cache static files.
			w.Header().Set("Cache-Control", "public, max-age=1209600, immutable")
		} else {
			// Revalidate HTML files.
			w.Header().Set("Cache-Control", "public, no-cache")
		}
		fs.ServeHTTP(w, r)
	}
}

// listen listens for HTTP connections and sends the error returned by the HTTP
// server via the provided channel.
func (s *Server) listen(httpErrors chan error) {
	info, err := os.Stat("static/css/futaba.css")
	if err != nil || info.IsDir() {
		httpErrors <- fmt.Errorf("failed to locate static directory, unable to serve CSS and JS files: run sriracha from the directory that contains static as a subdirectory")
	}

	mux := http.NewServeMux()
	mux.Handle("/static/", withCacheHeader(http.StripPrefix("/static/", http.FileServer(http.Dir("static")))))
	mux.HandleFunc("/sriracha/", s.serve)
	mux.Handle("/", withCacheHeader(http.FileServer(http.Dir(s.config.Root))))

	s.httpServer = &http.Server{
		Addr:    s.config.Serve,
		Handler: mux,
	}
	httpErrors <- s.httpServer.ListenAndServe()
}

// handleRebuild handles requests to rebuild threads.
func (s *Server) handleRebuild() {
	defer s.rebuildWaitGroup.Done()

	minWait := 1 * time.Second
	maxWait := 10 * time.Second

	lastBuild := time.Now()

	var info *rebuildInfo
	var pending []*rebuildInfo
	var boards []*Board
	var threads []int
	var shutdown bool
	var t *time.Timer
	for {
		// Process queue.
		info = <-s.rebuildQueue
		if info == nil {
			return // Shut down.
		}
		pending = append(pending, info)
		if time.Since(lastBuild) < maxWait {
			for {
				// Drain queue until minimum wait time has passed.
				t = time.NewTimer(minWait)
				var found bool
			DRAINQUEUE:
				for {
					select {
					case info = <-s.rebuildQueue:
						if info == nil {
							shutdown = true
						} else {
							pending = append(pending, info)
							found = true
						}
					case <-t.C:
						break DRAINQUEUE
					}
				}
				if !found {
					break
				}
				// Check if maximum wait time has passed.
				if time.Since(lastBuild) >= maxWait {
					break
				}
			}
		}
		if shutdown && len(pending) == 0 {
			return
		}

		// Flush queue.
		db := s.begin()
		for _, info := range pending {
			thread := info.post.Thread()
			if !slices.Contains(threads, thread) {
				s.writeThread(db, info.post.Board, thread)
				threads = append(threads, thread)
			}
			if !slices.Contains(boards, info.post.Board) {
				s.writeBoardIndexes(db, info.post.Board)
				boards = append(boards, info.post.Board)
			}
		}
		if s.opt.Overboard != "" {
			s.writeOverboard(db)
		}
		s.writeSiteIndex(db)
		if s.opt.Notifications {
			for _, info := range pending {
				s.queueNotifications(db, info.post)
			}
		}
		db.Commit()

		for _, info := range pending {
			info.wg.Done()
		}

		pending = pending[:0]
		boards = boards[:0]
		threads = threads[:0]

		lastBuild = time.Now()

		if shutdown {
			return
		}
	}
}

func (s *Server) _handleSignal(signals chan os.Signal) {
	for {
		// Wait until SIGHUP, SIGINT or SIGTERM is received.
		sig := <-signals

		// Rebuild static files when SIGHUP is received.
		if sig == unix.SIGHUP {
			db := s.begin()
			s.rebuildAll(db, true)
			db.Commit()
			fmt.Printf("Serving http://%s\n", s.config.Serve)
			continue
		}

		// Shut down server when SIGINT or SIGTERM is received.
		s.Stop()
		return
	}
}

// startSignalHandler starts the signal handler which rebuilds static files on
// SIGHUP and shuts down the server on SIGINT or SIGTERM.
func (s *Server) startSignalHandler() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, unix.SIGHUP, unix.SIGINT, unix.SIGTERM)
	go s._handleSignal(signals)
}

// Run initializes the server and starts listening for connections.
func (s *Server) Run() error {
	s.parseBuildInfo()

	// Parse flags and arguments.
	printInfo := func() {
		fmt.Fprintf(os.Stderr, "\nSriracha imageboard and forum server\n  https://codeberg.org/tslocum/sriracha\nGNU LESSER GENERAL PUBLIC LICENSE\n  https://codeberg.org/tslocum/sriracha/src/branch/main/LICENSE\n")
	}
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n  sriracha [OPTION...] [PLUGIN...]\n\nOptions:\n")
		flag.PrintDefaults()
		printInfo()
	}
	var (
		configFile   string
		exportPath   string
		importPath   string
		devMode      bool
		rebuild      bool
		printVersion bool
	)
	flag.StringVar(&configFile, "config", "", "path to configuration file (default: ~/.config/sriracha/config.yml)")
	flag.StringVar(&exportPath, "export", "", "export posts to zip file at specified path")
	flag.StringVar(&importPath, "import", "", "import posts from zip file or sqlite database file at specified path")
	flag.BoolVar(&devMode, "dev", false, "run in development mode (watch official and custom template files for changes)")
	flag.BoolVar(&rebuild, "rebuild", false, "rebuild static files before serving any requests")
	flag.BoolVar(&printVersion, "version", false, "print version information and exit")
	flag.Parse()

	// Print version information and exit.
	if printVersion {
		fmt.Fprintf(os.Stderr, "Sriracha version %s\n", SrirachaVersion)
		printInfo()
		return nil
	}

	// Start rebuild queue handler.
	s.rebuildWaitGroup.Add(1)
	go s.handleRebuild()

	// Start SIGINT and SIGTERM signal handler.
	s.startSignalHandler()

	// Set default server configuration file path.
	if configFile == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			configFile = path.Join(homeDir, ".config", "sriracha", "config.yml")
		}
	}

	// Parse server YAML configuration file.
	err := s.parseConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to parse configuration %s: %s", configFile, err)
	}
	s.config.StartTime = time.Now()

	// Set default gettext domain.
	gotext.SetDomain("sriracha")

	// Parse locale files.
	err = s.parseLocales()
	if err != nil {
		log.Fatalf("failed to parse locale files: %s", err)
	}

	// Locate official templates and validate custom template configuration.
	var officialDir string
	if devMode {
		s.opt.DevMode = true

		officialDir = s.officialTemplateDir()
		if officialDir == "" {
			return fmt.Errorf("failed to locate official template directory: start sriracha in the same directory as the file README.md")
		}

		err = s.validateTemplateConfig(officialDir)
		if err != nil {
			return fmt.Errorf("invalid custom template directory: %s", err)
		}
	}

	// Verify mail server configuration.
	s.opt.Notifications = s.config.MailAddress != ""
	if s.opt.Notifications && !devMode {
		fmt.Println("Verifying mail server configuration...")
		client, err := s.connectToMailServer()
		if err != nil {
			log.Fatalf("failed to verify mail server configuration: %s", err)
		}
		client.Close()
	}

	// Initialize database connection pool, which contains one connection.
	s.dbPool, err = database.Connect(s.config)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %s", err)
	}

	// Load server configuration and set default values.
	err = s.loadServerConfig()
	if err != nil {
		return fmt.Errorf("failed to set default server configuration: %s", err)
	}

	// Load plugin configuration.
	err = s.loadPluginConfig()
	if err != nil {
		return fmt.Errorf("failed to load plugin configuration: %s", err)
	}

	// Load plugins.
	err = s.loadPlugins()
	if err != nil {
		return fmt.Errorf("failed to load plugins: %s", err)
	}

	// Set default plugin configuration.
	err = s.setDefaultPluginConfig()
	if err != nil {
		return fmt.Errorf("failed to set default plugin configuration: %s", err)
	}

	// Parse template files.
	err = s.parseTemplates(officialDir, s.config.Template)
	if err != nil {
		return fmt.Errorf("failed to parse template files: %s", err)
	}

	// Export posts.
	if exportPath != "" {
		db := s.begin()
		defer db.Commit()

		err := s.exportPosts(db, exportPath)
		if err != nil {
			return fmt.Errorf("failed to export posts: %s", err)
		}
		return nil
	}

	// Import posts.
	if importPath != "" {
		s.config.ImportMode = true
		err := s.importPosts(importPath)
		if err != nil {
			return fmt.Errorf("failed to import posts: %s", err)
		}
	}

	// Verify root directory is writable.
	if unix.Access(s.config.Root, unix.W_OK) != nil {
		return fmt.Errorf("configured root directory %s is not writable", s.config.Root)
	}

	// Create captcha directory.
	captchaDir := filepath.Join(s.config.Root, "captcha")
	_, err = os.Stat(captchaDir)
	if os.IsNotExist(err) {
		err := os.Mkdir(captchaDir, NewDirPermission)
		if err != nil {
			log.Fatalf("failed to create captcha directory: %s", err)
		}
	}

	// Create banner directory.
	bannerDir := filepath.Join(s.config.Root, "banner")
	_, err = os.Stat(bannerDir)
	if os.IsNotExist(err) {
		err := os.Mkdir(bannerDir, NewDirPermission)
		if err != nil {
			log.Fatalf("failed to create banner directory: %s", err)
		}
	}

	// Write default site index file.
	siteIndexFile := filepath.Join(s.config.Root, "index.html")
	_, err = os.Stat(siteIndexFile)
	if os.IsNotExist(err) {
		err = os.WriteFile(siteIndexFile, siteIndexHTML, NewFilePermission)
		if err != nil {
			log.Fatalf("failed to write site index at %s: %s", siteIndexFile, err)
		}
	}

	// Lock server until initialization is complete.
	s.lock.Lock()

	// Start listening for HTTP connections.
	httpErrors := make(chan error)
	go s.listen(httpErrors)

	// Rebuild everything on startup when explicitly requested and after upgrading.
	db := s.begin()
	s.refreshBannerCache(db)
	s.refreshRulesCache(db)
	s.refreshCategoryCache(db)
	s.refreshKeywordCache(db)
	sv := db.GetString("sv") // Sriracha version.
	if sv != SrirachaVersion {
		if sv != "" {
			fmt.Printf("Upgraded from Sriracha version %s to %s, rebuilding...\n", sv, SrirachaVersion)
			rebuild = true
		}
		db.SaveString("sv", SrirachaVersion)
	}
	if rebuild {
		s.rebuildAll(db, true)
	}
	db.Commit()

	// Watch template directories.
	if devMode {
		dir := "directory"
		if s.config.Template != "" {
			dir = "directories"
		}
		fmt.Printf("Development mode enabled. Monitoring template %s...\n", dir)
		err = s.watchTemplates(officialDir)
		if err != nil {
			s.lock.Unlock()
			return fmt.Errorf("failed to watch templates for changes: %s", err)
		}
	}

	// Start notification queue handler.
	if s.config.MailAddress != "" {
		s.notificationsWaitGroup.Add(1)
		go s.handleNotifications()
	}

	if s.config.ImportMode {
		fmt.Println("Import mode enabled. Visitors may not create posts until import mode is disabled.")
	}

	// Initialization complete. Unlock server.
	fmt.Printf("Serving http://%s\n", s.config.Serve)
	s.lock.Unlock()

	// Wait until the HTTP server returns an error.
	err = <-httpErrors

	// Shut down gracefully.
	if err == http.ErrServerClosed {
		// Wait until all web requests have been processed.
		s.rebuildWaitGroup.Wait()
		// Wait until all notifications have been sent.
		s.notificationsWaitGroup.Wait()
		return nil
	}
	return err
}

// newHash returns a new hash digest.
func (s *Server) newHash() hash.Hash {
	if s.opt.Algorithm == AlgorithmSHA3 {
		return sha3.New384()
	}
	return sha512.New384()
}

// hashBytes returns the hash of the provided bytes and optional salt.
func (s *Server) hashBytes(buf []byte, salt string) string {
	hash := s.newHash()
	hash.Write(buf)
	if salt != "" {
		hash.Write([]byte(salt))
	}
	var sum [HashSize]byte
	hash.Sum(sum[:0])
	return base64.URLEncoding.EncodeToString(sum[:])
}

// hashData returns the salted hash of the provided data.
func (s *Server) hashData(data string) string {
	return s.hashBytes([]byte(data), s.config.SaltData)
}

// md5Sum returns the MD5 sum of the provided data.
func md5Sum(data string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(data)))
}

// parseHostname returns the hostname portion of an address.
func parseHostname(address string) string {
	if address == "" {
		return ""
	}
	hostname, port, err := net.SplitHostPort(address)
	if err != nil || port == "" {
		return address
	}
	return hostname
}

// requestIP returns the remote IP address of a request.
func (s *Server) requestIP(r *http.Request) string {
	var address string
	if s.config.Header != "" {
		values := r.Header[s.config.Header]
		if len(values) > 0 {
			address = values[0]
		}
	} else {
		address = r.RemoteAddr
	}
	if address == "" {
		log.Fatal("Error: No client IP address specified in HTTP request. Are you sure the header server option is correct? See MANUAL.md for more info.")
	}
	return parseHostname(address)
}

func (s *Server) _hashIP(address string) string {
	if address == "" {
		return ""
	}
	return s.hashData(parseHostname(address))
}

// hashIP returns the salted hash of a request's IP address.
func (s *Server) hashIP(r *http.Request) string {
	return s._hashIP(s.requestIP(r))
}

// imageDimensions returns the width and height of a JPG, PNG or GIF image.
func (s *Server) imageDimensions(reader io.Reader) (int, int) {
	imgConfig, _, err := image.DecodeConfig(reader)
	if err != nil {
		return 0, 0
	}
	return imgConfig.Width, imgConfig.Height
}

// Stop shuts down the server gracefully.
func (s *Server) Stop() {
	fmt.Println("Shutting down...")

	// Stop serving new web requests.
	if s.httpServer != nil {
		s.httpServer.Shutdown(context.Background())
	}

	// Wait until existing web requests finish processing.
	s.lock.Lock()
	s.rebuildLock.Lock()
	s.rebuildQueue <- nil
	s.rebuildWaitGroup.Wait()

	// Flush notification queue.
	if s.opt.Notifications {
		s.shutdownNotifications <- struct{}{}
	}

	// If the HTTP server hasn't started yet, exit immediately.
	if s.httpServer == nil {
		os.Exit(0)
	}
}

// pluginByName returns the specified plugin instance and associated plugin information.
func pluginByName(name string) (any, *pluginInfo) {
	name = strings.ToLower(name)
	for i, info := range allPluginInfo {
		if strings.ToLower(info.Name) == name {
			return allPlugins[i], info
		}
	}
	return nil, nil
}

// FormatValue formats a value as a human-readable string.
func FormatValue(v interface{}) interface{} {
	if role, ok := v.(AccountRole); ok {
		return FormatRole(role)
	} else if t, ok := v.(BoardType); ok {
		return FormatBoardType(t)
	} else if t, ok := v.(BoardHide); ok {
		return FormatBoardHide(t)
	} else if t, ok := v.(BoardLock); ok {
		return FormatBoardLock(t)
	} else if t, ok := v.(BoardApproval); ok {
		return FormatBoardApproval(t)
	} else if t, ok := v.(BoardIdentifiers); ok {
		return FormatBoardIdentifiers(t)
	}
	return v
}

// printChanges returns the difference between two structs as a human-readable string.
func printChanges(old interface{}, new interface{}) string {
	const mask = "***"
	diff, err := diff.Diff(old, new)
	if err != nil {
		log.Fatal(err)
	} else if len(diff) == 0 {
		return ""
	}
	var label string
	for _, change := range diff {
		from := change.From
		to := change.To

		var name string
		if len(change.Path) > 0 {
			name = change.Path[0]
			if name == "Password" {
				from = mask
				to = mask
			}
		}

		label += fmt.Sprintf(` [%s: "%v" > "%v"]`, name, FormatValue(from), FormatValue(to))
	}
	return label
}

// pageCount returns the number of pages required to display the provided number of items.
func pageCount(items int, pageSize int) int {
	if items == 0 || pageSize == 0 {
		return 1
	}
	pages := items / pageSize
	if items%pageSize != 0 {
		pages++
	}
	return pages
}

// doctypePrefx is an HTML prefix which may be used in custom pages to skip
// including the default page header and footer templates.
const doctypePrefx = "<!DOCTYPE html>"

// siteIndexHTML is an HTML page written to index.html when such a file does not already exist.
var siteIndexHTML = []byte(`
<!DOCTYPE html>
<html>
	<body>
		<meta http-equiv="refresh" content="0; url=/sriracha/">
		<a href="/sriracha/">Redirecting...</a>
	</body>
</html>
`)
