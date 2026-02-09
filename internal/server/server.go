package server

import (
	"crypto/sha512"
	"embed"
	"encoding/base64"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"maps"
	"net/http"
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

var SrirachaVersion = "DEV"

//go:embed locale
var localeFS embed.FS

const (
	defaultServerSiteName     = "Sriracha"
	defaultServerSiteHome     = "/"
	defaultServerOekakiWidth  = 540
	defaultServerOekakiHeight = 540
	defaultServerRefresh      = 30
)

var defaultServerEmbeds = [][2]string{
	{"YouTube", "https://youtube.com/oembed?format=json&url=SRIRACHA_EMBED"},
	{"Vimeo", "https://vimeo.com/api/oembed.json?url=SRIRACHA_EMBED"},
	{"SoundCloud", "https://soundcloud.com/oembed?format=json&url=SRIRACHA_EMBED"},
}

func init() {
	gotext.SetDomain("sriracha")
}

type HTMLError struct {
	Page string
}

func (e *HTMLError) Error() string {
	return e.Page
}

type NewsOption int

const (
	NewsDisable      NewsOption = 0
	NewsWriteToNews  NewsOption = 1
	NewsWriteToIndex NewsOption = 2
)

type ServerOptions struct {
	SiteName         string
	SiteHome         string
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
	DevMode          bool
	FuncMaps         map[string]template.FuncMap
}

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

type rebuildInfo struct {
	post *Post
	wg   *sync.WaitGroup
}

type Server struct {
	Boards []*Board

	rangeBans map[*Ban]*regexp.Regexp

	config *Config
	dbPool *pgxpool.Pool
	opt    ServerOptions
	tpl    *template.Template

	rebuildQueue     chan *rebuildInfo
	rebuildWaitGroup sync.WaitGroup
	rebuildLock      sync.Mutex

	lock sync.Mutex
}

func NewServer() *Server {
	return &Server{
		rebuildQueue: make(chan *rebuildInfo),
	}
}

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

func (s *Server) parseConfig(configFile string) error {
	buf, err := os.ReadFile(configFile)
	if err != nil {
		return err
	}

	var config Config
	err = yaml.Unmarshal(buf, &config)
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

	if config.Locale == "" {
		config.Locale = "en"
	}

	s.config = &config
	s.config.ImportMode = s.config.Import.Enabled()
	return nil
}

func (s *Server) begin() *database.DB {
	return database.Begin(s.dbPool, s.config)
}

func (s *Server) setDefaultServerConfig() error {
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
	return nil
}

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

func (s *Server) parseTemplates(officialDir string, customDir string) error {
	parseDir := func(dir string) error {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, f := range entries {
			if !strings.HasSuffix(f.Name(), ".gohtml") {
				continue
			}

			buf, err := os.ReadFile(filepath.Join(dir, f.Name()))
			if err != nil {
				return err
			}

			_, err = s.tpl.New(f.Name()).Parse(string(buf))
			if err != nil {
				return err
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
				return err
			}

			_, err = s.tpl.New(f.Name()).Parse(string(buf))
			if err != nil {
				return err
			}
		}
	} else {
		s.tpl = template.New("sriracha").Funcs(templateFuncMaps[""])
		err := parseDir(officialDir)
		if err != nil {
			return err
		}
	}

	if customDir != "" {
		return parseDir(customDir)
	}
	return nil
}

func (s *Server) watchTemplates(officialDir string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	go func() {
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
					log.Printf("error: failed to parse templates: %s", err)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("fsnotify error: %s", err)
			}
		}
	}()

	err = watcher.Add(officialDir)
	if err == nil && s.config.Template != "" {
		err = watcher.Add(s.config.Template)
	}
	return err
}

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

func (s *Server) deletePost(db *database.DB, p *Post) {
	posts := db.AllPostsInThread(p.ID, false)
	for _, post := range posts {
		s.deletePostFiles(post)
	}

	db.DeletePost(p.ID)
}

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
					if s.config.ImportMode {
						http.Redirect(w, r, "/sriracha/import/", http.StatusFound)
					}
					data := s.newTemplateData()
					data.Account = account
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

func (s *Server) writeThread(db *database.DB, board *Board, postID int) {
	posts := db.AllPostsInThread(postID, true)
	if len(posts) == 0 {
		return
	}

	if board.Unique == 0 {
		board.Unique = db.UniqueUserPosts(board)
	}

	f, err := os.OpenFile(filepath.Join(s.config.Root, board.Dir, "res", fmt.Sprintf("%d.html", postID)), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
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

func (s *Server) writeIndexes(db *database.DB, board *Board) {
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
		catalogFile, err := os.OpenFile(filepath.Join(s.config.Root, board.Dir, "catalog.html"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
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

		indexFile, err := os.OpenFile(filepath.Join(s.config.Root, board.Dir, fileName), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
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
		catalogFile, err := os.OpenFile(filepath.Join(s.config.Root, overboardDir, "catalog.html"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
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

		indexFile, err := os.OpenFile(filepath.Join(s.config.Root, overboardDir, fileName), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
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

func (s *Server) rebuildThread(db *database.DB, post *Post) {
	s.writeThread(db, post.Board, post.Thread())
	s.writeIndexes(db, post.Board)
	if s.opt.Overboard != "" {
		s.writeOverboard(db)
	}
}

func (s *Server) rebuildBoard(db *database.DB, board *Board) {
	for _, info := range db.AllThreads(board, true) {
		s.writeThread(db, board, info[0])
	}
	s.writeIndexes(db, board)
}

func (s *Server) rebuildAll(db *database.DB) {
	for _, b := range db.AllBoards() {
		s.rebuildBoard(db, b)
	}

	s.rebuildNews(db)

	if s.opt.Overboard != "" {
		s.writeOverboard(db)
	}
}

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

	itemFile, err := os.OpenFile(filepath.Join(s.config.Root, fmt.Sprintf("news-%d.html", n.ID)), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatal(err)
	}
	data.execute(itemFile)
	itemFile.Close()
}

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

		indexFile, err := os.OpenFile(filepath.Join(s.config.Root, fileName), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
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

func (s *Server) rebuildNewsItem(db *database.DB, n *News) {
	s.writeNewsItem(db, n)
	s.writeNewsIndexes(db)
}

func (s *Server) rebuildNews(db *database.DB) {
	for _, n := range db.AllNews(true) {
		s.writeNewsItem(db, n)
	}
	s.writeNewsIndexes(db)
}

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
			http.Redirect(w, r, "/sriracha/import/", http.StatusFound)
			return
		}
		data.Info = "IMPORT MODE"
	}

	switch {
	case strings.HasPrefix(r.URL.Path, "/sriracha/preference"):
		s.servePreference(data, db, w, r)
	case strings.HasPrefix(r.URL.Path, "/sriracha/account"):
		s.serveAccount(data, db, w, r)
	case strings.HasPrefix(r.URL.Path, "/sriracha/ban"):
		s.serveBan(data, db, w, r)
	case strings.HasPrefix(r.URL.Path, "/sriracha/board"):
		skipExecute = s.serveBoard(data, db, w, r)
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

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if r.Method == http.MethodPost {
		const maxMemory = 32 << 20 // 32 megabytes.
		r.ParseMultipartForm(maxMemory)

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
	var handled bool

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
			handled = true
			break
		}
	}

	// Check static IP ban.
	if !handled {
		ban := db.BanByIP(s.hashIP(r))
		if ban != nil {
			data := s.buildData(db, w, r)
			data.ManageError("You are banned. " + ban.Info() + fmt.Sprintf(" (Ban #%d)", ban.ID))
			data.execute(w)
			handled = true
		} else if strings.HasPrefix(r.URL.Path, "/sriracha/post/") {
			postID := PathInt(r, "/sriracha/post/")
			post := db.PostByID(postID)
			if post == nil {
				data := s.buildData(db, w, r)
				data.BoardError(w, "Invalid or deleted post.")
			} else {
				http.Redirect(w, r, fmt.Sprintf("%sres/%d.html#%d", post.Board.Path(), post.Thread(), post.ID), http.StatusFound)
			}
			handled = true
		}
	}

	if !handled {
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
			default:
				s.serveManage(db, w, r)
			}
		}
	}
}

func (s *Server) listen() error {
	info, err := os.Stat("static/css/futaba.css")
	if err != nil || info.IsDir() {
		return fmt.Errorf("failed to locate static directory, unable to serve CSS and JS files: run sriracha from the directory that contains static as a subdirectory")
	}

	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.HandleFunc("/sriracha/", s.serve)
	mux.Handle("/", http.FileServer(http.Dir(s.config.Root)))

	fmt.Printf("Serving http://%s\n", s.config.Serve)
	return http.ListenAndServe(s.config.Serve, mux)
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
	for {
		// Process queue.
		info = <-s.rebuildQueue
		if info == nil {
			shutdown = true
		} else {
			pending = append(pending, info)
		}
		for {
			// Sleep until minimum wait time has passed.
			time.Sleep(minWait)
			// Drain queue.
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
				default:
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
				s.writeIndexes(db, info.post.Board)
				boards = append(boards, info.post.Board)
			}
		}
		if s.opt.Overboard != "" {
			s.writeOverboard(db)
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

func (s *Server) Run() error {
	s.parseBuildInfo()

	printInfo := func() {
		fmt.Fprintf(os.Stderr, "\nSriracha imageboard and forum\n  https://codeberg.org/tslocum/sriracha\nGNU LESSER GENERAL PUBLIC LICENSE\n  https://codeberg.org/tslocum/sriracha/src/branch/main/LICENSE\n")
	}
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n  sriracha [OPTION...] [PLUGIN...]\n\nOptions:\n")
		flag.PrintDefaults()
		printInfo()
	}
	var configFile string
	var rebuild bool
	var devMode bool
	var printVersion bool
	flag.StringVar(&configFile, "config", "", "path to configuration file (default: ~/.config/sriracha/config.yml)")
	flag.BoolVar(&rebuild, "rebuild", false, "rebuild static files on startup")
	flag.BoolVar(&devMode, "dev", false, "run in development mode (watch template files for changes)")
	flag.BoolVar(&printVersion, "version", false, "print version information and exit")
	flag.Parse()

	if printVersion {
		fmt.Fprintf(os.Stderr, "Sriracha version %s\n", SrirachaVersion)
		printInfo()
		return nil
	}

	s.rebuildWaitGroup.Add(1)
	go s.handleRebuild()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, unix.SIGINT, unix.SIGTERM)
	go func() {
		// Wait until SIGINT or SIGTERM is received.
		<-signals
		// Shut down server.
		fmt.Println("Shutting down...")
		s.lock.Lock()
		s.rebuildLock.Lock()
		s.rebuildQueue <- nil
		s.rebuildWaitGroup.Wait()
		os.Exit(0)
	}()

	if configFile == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			configFile = path.Join(homeDir, ".config", "sriracha", "config.yml")
		}
	}

	err := s.parseConfig(configFile)
	if err != nil {
		return err
	}

	s.config.StartTime = time.Now()

	// Parse locale files.
	err = fs.WalkDir(localeFS, "locale", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		} else if d.IsDir() || !strings.HasSuffix(p, ".po") {
			return nil
		}
		id := filepath.Base(strings.TrimSuffix(p, ".po"))

		buf, err := localeFS.ReadFile(fmt.Sprintf("locale/%s/%s.po", id, id))
		if err != nil {
			log.Fatalf("failed to load locale %s: %s", id, err)
		}

		po := gotext.NewPo()
		po.Parse(buf)
		gotext.GetStorage().AddTranslator(fmt.Sprintf("sriracha-%s", id), po)
		return nil
	})
	if err != nil {
		log.Fatalf("failed to parse locale files: %s", err)
	}

	officialDir := "internal/server/template"
	if devMode {
		_, err := os.Stat(officialDir)
		if os.IsNotExist(err) {
			officialDir = "template"
			_, err := os.Stat(officialDir)
			if os.IsNotExist(err) {
				log.Fatal("error: could not find official template directory, start sriracha in the same directory as the file README.md")
			}
		}
	}

	if s.config.Template != "" {
		_, err := os.Stat(s.config.Template)
		if os.IsNotExist(err) {
			log.Fatalf("error: custom template directory %s does not exist", s.config.Template)
		}
		officialTemplate, err := os.Stat(officialDir)
		if err != nil {
			log.Fatal("error: could not find official template directory, start sriracha in the same directory as the file README.md")
		}
		customTemplate, err := os.Stat(s.config.Template)
		if err != nil {
			log.Fatalf("error: custom template directory %s is inaccessible", s.config.Template)
		}
		if os.SameFile(officialTemplate, customTemplate) {
			log.Fatalf("error: official templates and custom templates must be located in separate directories")
		}
	}

	if devMode {
		s.opt.DevMode = true
		err := s.watchTemplates(officialDir)
		if err != nil {
			log.Fatalf("failed to watch templates for changes: %s", err)
		}
		fmt.Println("Running in development mode. Template files are monitored for changes.")
	}

	s.dbPool, err = database.Connect(s.config)
	if err != nil {
		return err
	}

	err = s.setDefaultServerConfig()
	if err != nil {
		return err
	}

	err = s.loadPluginConfig()
	if err != nil {
		return err
	}

	err = s.loadPlugins()
	if err != nil {
		return err
	}

	err = s.setDefaultPluginConfig()
	if err != nil {
		return err
	}

	err = s.parseTemplates("", s.config.Template)
	if err != nil {
		return fmt.Errorf("failed to parse templates: %s", err)
	}

	if unix.Access(s.config.Root, unix.W_OK) != nil {
		return fmt.Errorf("failed to set root: %s is not writable", s.config.Root)
	}

	captchaDir := filepath.Join(s.config.Root, "captcha")
	_, err = os.Stat(captchaDir)
	if os.IsNotExist(err) {
		err := os.Mkdir(captchaDir, NewDirPermission)
		if err != nil {
			log.Fatalf("failed to create captcha dir: %s", err)
		}
	}

	siteIndexFile := filepath.Join(s.config.Root, "index.html")
	_, err = os.Stat(siteIndexFile)
	if os.IsNotExist(err) {
		err = os.WriteFile(siteIndexFile, siteIndexHTML, NewFilePermission)
		if err != nil {
			log.Fatalf("failed to write site index at %s: %s", siteIndexFile, err)
		}
	}

	// Rebuild everything on startup when explicitly requested and after upgrading.
	db := s.begin()
	sv := db.GetString("sv") // Sriracha version.
	if sv != SrirachaVersion {
		if sv != "" {
			fmt.Printf("Upgraded from Sriracha version %s to %s\n", sv, SrirachaVersion)
			rebuild = true
		}
		db.SaveString("sv", SrirachaVersion)
	}
	if rebuild {
		published := len(db.AllNews(true))
		if published > 0 {
			fmt.Println("Rebuilding news...")
			s.rebuildNews(db)
		}
		if s.opt.Overboard != "" {
			fmt.Println("Rebuilding overboard...")
			s.writeOverboard(db)
		}
		for _, b := range db.AllBoards() {
			fmt.Printf("Rebuilding %s...\n", b.Path())
			s.rebuildBoard(db, b)
		}
	}
	db.Commit()

	return s.listen()
}

func (s *Server) hashData(data string) string {
	checksum := sha512.Sum384([]byte(data + s.config.SaltData))
	return base64.URLEncoding.EncodeToString(checksum[:])
}

func parseAddress(address string) string {
	if address == "" {
		return ""
	}
	leftBracket, rightBracket := strings.IndexByte(address, '['), strings.IndexByte(address, ']')
	if leftBracket != -1 && rightBracket != -1 && rightBracket > leftBracket {
		address = address[1:rightBracket]
	} else if strings.IndexByte(address, '.') != -1 {
		colon := strings.IndexByte(address, ':')
		if colon != -1 {
			address = address[:colon]
		}
	}
	return address
}

func (s *Server) _hashIP(address string) string {
	if address == "" {
		return ""
	}
	return s.hashData(parseAddress(address))
}

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
	return parseAddress(address)
}

func (s *Server) hashIP(r *http.Request) string {
	return s._hashIP(s.requestIP(r))
}

func pluginByName(name string) (any, *pluginInfo) {
	name = strings.ToLower(name)
	for i, info := range allPluginInfo {
		if strings.ToLower(info.Name) == name {
			return allPlugins[i], info
		}
	}
	return nil, nil
}

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

func calculateFileHash(buf []byte) string {
	checksum := sha512.Sum384(buf)
	return base64.URLEncoding.EncodeToString(checksum[:])
}

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

var siteIndexHTML = []byte(`
<!DOCTYPE html>
<html>
	<body>
		<meta http-equiv="refresh" content="0; url=/sriracha/">
		<a href="/sriracha/">Redirecting...</a>
	</body>
</html>
`)
