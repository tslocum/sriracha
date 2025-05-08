package sriracha

import (
	"context"
	"crypto/sha512"
	"embed"
	"encoding/base64"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"plugin"
	"regexp"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/fsnotify/fsnotify"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/leonelquinteros/gotext"
	"github.com/r3labs/diff/v3"
	"golang.org/x/exp/constraints"
	"golang.org/x/sys/unix"
	"gopkg.in/yaml.v3"
)

var SrirachaVersion = "DEV"

var alphaNumericAndSymbols = regexp.MustCompile(`^[0-9A-Za-z_-]+$`)

//go:embed locale
var localeFS embed.FS

var srirachaServer *Server

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

const newDirPermission = 0755
const newFilePermission = 0600

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
	SiteName     string
	SiteHome     string
	News         NewsOption
	BoardIndex   bool
	CAPTCHA      bool
	Refresh      int
	Uploads      []*uploadType
	Embeds       [][2]string
	OekakiWidth  int
	OekakiHeight int
}

type Server struct {
	Boards []*Board

	rangeBans map[*Ban]*regexp.Regexp

	config Config
	dbPool *pgxpool.Pool
	opt    ServerOptions
	tpl    *template.Template
	lock   sync.Mutex
}

func NewServer() *Server {
	srirachaServer = &Server{}
	return srirachaServer
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

	s.config = config
	s.config.importMode = s.config.Import.Enabled()
	return nil
}

func (s *Server) loadPlugin(pluginPath string) error {
	info, err := os.Stat(pluginPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("failed to load plugin %s: file or directory not found", pluginPath)
	} else if info.IsDir() {
		return filepath.WalkDir(pluginPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			} else if path == pluginPath {
				return nil
			}
			return s.loadPlugin(path)
		})
	} else if !strings.HasSuffix(pluginPath, ".so") {
		return nil
	}

	_, err = plugin.Open(pluginPath)
	if err != nil {
		return fmt.Errorf("failed to load plugin %s: %s", pluginPath, err)
	}
	return err
}

func (s *Server) loadPlugins() error {
	for _, pluginPath := range flag.Args() {
		err := s.loadPlugin(pluginPath)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) setDefaultServerConfig() error {
	conn, err := s.dbPool.Acquire(context.Background())
	if err != nil {
		return err
	}
	defer conn.Release()

	_, err = conn.Exec(context.Background(), "BEGIN")
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %s", err)
	}

	db := &Database{
		conn: conn,
	}

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

	_, err = conn.Exec(context.Background(), "COMMIT")
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %s", err)
	}
	return nil
}

func (s *Server) setDefaultPluginConfig() error {
	conn, err := s.dbPool.Acquire(context.Background())
	if err != nil {
		return err
	}
	defer conn.Release()

	_, err = conn.Exec(context.Background(), "BEGIN")
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %s", err)
	}

	db := &Database{
		conn: conn,
	}

	for i, info := range allPluginInfo {
		db.plugin = strings.ToLower(info.Name)

		for i, config := range info.Config {
			if !db.HaveConfig(config.Name) {
				db.SaveString(config.Name, config.Value)
			} else {
				info.Config[i].Value = db.GetString(config.Name)
			}
		}

		p := allPlugins[i]
		pUpdate, ok := p.(PluginWithUpdate)
		if ok {
			for _, config := range info.Config {
				pUpdate.Update(db, config.Name)
			}
		}
	}

	_, err = conn.Exec(context.Background(), "COMMIT")
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %s", err)
	}
	return nil
}

func (s *Server) parseTemplates(standardDir string, customDir string) error {
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
	if standardDir == "" {
		s.tpl = template.New("sriracha").Funcs(templateFuncMap)

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
		s.tpl = template.New("sriracha").Funcs(templateFuncMap)
		err := parseDir(standardDir)
		if err != nil {
			return err
		}
	}

	if customDir != "" {
		return parseDir(customDir)
	}
	return nil
}

func (s *Server) watchTemplates() error {
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
				err := s.parseTemplates("template", s.config.Template)
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

	err = watcher.Add("template")
	if err == nil && s.config.Template != "" {
		err = watcher.Add(s.config.Template)
	}
	return err
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

func (s *Server) deletePost(db *Database, p *Post) {
	posts := db.allPostsInThread(p.ID, false)
	for _, post := range posts {
		s.deletePostFiles(post)
	}

	db.deletePost(p.ID)
}

func (s *Server) buildData(db *Database, w http.ResponseWriter, r *http.Request) *templateData {
	if strings.HasPrefix(r.URL.Path, "/sriracha/logout") {
		http.SetCookie(w, &http.Cookie{
			Name:  "sriracha_session",
			Value: "",
			Path:  "/",
		})
		http.Redirect(w, r, "/sriracha/", http.StatusFound)
		return newTemplateData()
	}

	if r.URL.Path == "/sriracha/" || r.URL.Path == "/sriracha" {
		var failedLogin bool
		username := r.FormValue("username")
		if len(username) != 0 {
			failedLogin = true
			password := r.FormValue("password")
			if len(password) != 0 {
				account := db.loginAccount(username, password)
				if account != nil {
					http.SetCookie(w, &http.Cookie{
						Name:  "sriracha_session",
						Value: account.Session,
						Path:  "/",
					})
					if s.config.importMode {
						http.Redirect(w, r, "/sriracha/import/", http.StatusFound)
					}
					return &templateData{
						Account: account,
						Manage: &manageData{
							Plugins: allPluginInfo,
						},
					}
				}
			}
		}
		if failedLogin {
			return &templateData{
				Info:     "Invalid username or password.",
				Template: "manage_error",
				Manage: &manageData{
					Plugins: allPluginInfo,
				},
			}
		}
	}

	cookies := r.CookiesNamed("sriracha_session")
	if len(cookies) > 0 {
		account := db.accountBySessionKey(cookies[0].Value)
		if account != nil {
			return &templateData{
				Account: account,
				Manage: &manageData{
					Plugins: allPluginInfo,
				},
			}
		}
	}
	return newTemplateData()
}

func (s *Server) writeThread(db *Database, board *Board, postID int) {
	posts := db.allPostsInThread(postID, true)
	if len(posts) == 0 {
		return
	}

	if board.Unique == 0 {
		board.Unique = db.uniqueUserPosts(board)
	}

	f, err := os.OpenFile(filepath.Join(s.config.Root, board.Dir, "res", fmt.Sprintf("%d.html", postID)), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatal(err)
	}

	data := &templateData{
		Board:     board,
		Boards:    db.allBoards(),
		Threads:   [][]*Post{posts},
		ReplyMode: postID,
		Manage:    &manageData{},
		Template:  "board_page",
	}
	data.execute(f)
}

func (s *Server) writeIndexes(db *Database, board *Board) {
	if board.Unique == 0 {
		board.Unique = db.uniqueUserPosts(board)
	}

	threads := db.allThreads(board, true)
	data := &templateData{
		Board:     board,
		Boards:    db.allBoards(),
		ReplyMode: 1,
		Manage:    &manageData{},
		Template:  "board_catalog",
	}

	// Write catalog.
	if board.Type == TypeImageboard {
		catalogFile, err := os.OpenFile(filepath.Join(s.config.Root, board.Dir, "catalog.html"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			log.Fatal(err)
		}

		for _, thread := range threads {
			data.Threads = append(data.Threads, []*Post{thread})
		}
		data.execute(catalogFile)

		catalogFile.Close()
	}

	// Write indexes.

	data.ReplyMode = 0
	data.Template = "board_page"
	data.Pages = pageCount(len(threads), board.Threads)
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
		end := len(threads)
		if board.Threads != 0 && end > start+board.Threads {
			end = start + board.Threads
		}

		data.Threads = data.Threads[:0]
		for _, thread := range threads[start:end] {
			posts := []*Post{thread}
			if board.Type == TypeImageboard {
				posts = append(posts, db.allReplies(thread.ID, board.Replies, true)...)
			}
			data.Threads = append(data.Threads, posts)
		}
		data.Page = page
		data.execute(indexFile)

		indexFile.Close()
	}
}

func (s *Server) rebuildThread(db *Database, post *Post) {
	s.writeThread(db, post.Board, post.Thread())
	s.writeIndexes(db, post.Board)
}

func (s *Server) rebuildBoard(db *Database, board *Board) {
	for _, post := range db.allThreads(board, true) {
		s.writeThread(db, board, post.ID)
	}
	s.writeIndexes(db, board)
}

func (s *Server) writeNewsItem(db *Database, n *News) {
	if n.ID <= 0 {
		return
	}

	data := &templateData{
		Boards:   db.allBoards(),
		Manage:   &manageData{},
		Template: "news",
		AllNews:  []*News{n},
		Pages:    1,
		Extra:    "view",
	}

	itemFile, err := os.OpenFile(filepath.Join(s.config.Root, fmt.Sprintf("news-%d.html", n.ID)), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatal(err)
	}
	data.execute(itemFile)
	itemFile.Close()
}

func (s *Server) writeNewsIndexes(db *Database) {
	allNews := db.allNews(true)
	data := &templateData{
		Boards:   db.allBoards(),
		Manage:   &manageData{},
		Template: "news",
	}

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

func (s *Server) rebuildNewsItem(db *Database, n *News) {
	s.writeNewsItem(db, n)
	s.writeNewsIndexes(db)
}

func (s *Server) rebuildAllNews(db *Database) {
	for _, n := range db.allNews(true) {
		s.writeNewsItem(db, n)
	}
	s.writeNewsIndexes(db)
}

func (s *Server) reloadBans(db *Database) {
	var rangeBans = make(map[*Ban]*regexp.Regexp)
	bans := db.allBans(true)
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

func (s *Server) serveManage(db *Database, w http.ResponseWriter, r *http.Request) {
	data := s.buildData(db, w, r)
	if strings.HasPrefix(r.URL.Path, "/sriracha/logout") {
		return
	}
	var skipExecute bool

	if len(data.Info) != 0 {
		w.Header().Set("Content-Type", "text/html")
		data.Template = "manage_error"
		data.execute(w)
		return
	}

	if strings.HasPrefix(r.URL.Path, "/sriracha/oekaki/") {
		postID := pathInt(r, "/sriracha/oekaki/")
		post := db.postByID(postID)
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
		db.updateAccountLastActive(data.Account.ID)
	}

	data.Template = "manage_login"

	if data.Account == nil {
		w.Header().Set("Content-Type", "text/html")
		data.execute(w)
		return
	} else if s.config.importMode {
		if data.Account.Role != RoleSuperAdmin {
			w.Header().Set("Content-Type", "text/html")
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
	w.Header().Set("Content-Type", "text/html")
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

	conn, err := s.dbPool.Acquire(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Release()

	_, err = conn.Exec(context.Background(), "BEGIN")
	if err != nil {
		log.Fatal(err)
	}

	db := &Database{
		conn: conn,
	}
	var handled bool

	if db.deleteExpiredBans() > 0 {
		s.reloadBans(db)
	}

	// Check IP range ban.
	ip := requestIP(r)
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
		ban := db.banByIP(hashIP(r))
		if ban != nil {
			data := s.buildData(db, w, r)
			data.ManageError("You are banned. " + ban.Info() + fmt.Sprintf(" (Ban #%d)", ban.ID))
			data.execute(w)
			handled = true
		} else if strings.HasPrefix(r.URL.Path, "/sriracha/post/") {
			postID := pathInt(r, "/sriracha/post/")
			post := db.postByID(postID)
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
		if s.config.importMode && action != "" {
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

	_, err = conn.Exec(context.Background(), "COMMIT")
	if err != nil {
		log.Fatalf("failed to commit transaction: %s", err)
	}
}

func (s *Server) listen() error {
	mux := http.NewServeMux()
	mux.Handle("/static/", http.FileServerFS(staticFS))
	mux.HandleFunc("/sriracha/", s.serve)
	mux.Handle("/", http.FileServer(http.Dir(s.config.Root)))

	fmt.Printf("Serving http://%s\n", s.config.Serve)
	return http.ListenAndServe(s.config.Serve, mux)
}

func (s *Server) Run() error {
	printInfo := func() {
		fmt.Fprintf(os.Stderr, "\nSriracha imageboard and forum\n  https://codeberg.org/tslocum/sriracha\nGNU LESSER GENERAL PUBLIC LICENSE\n  https://codeberg.org/tslocum/sriracha/src/branch/main/LICENSE\n")
	}
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n  sriracha [OPTION...] [PLUGIN...]\n\nOptions:\n")
		flag.PrintDefaults()
		printInfo()
	}
	var configFile string
	var devMode bool
	var printVersion bool
	flag.StringVar(&configFile, "config", "", "path to configuration file (default: ~/.config/sriracha/config.yml)")
	flag.BoolVar(&devMode, "dev", false, "run in development mode (monitor template files and apply changes)")
	flag.BoolVar(&printVersion, "version", false, "print version information and exit")
	flag.Parse()

	if printVersion {
		fmt.Fprintf(os.Stderr, "Sriracha %s\n", SrirachaVersion)
		printInfo()
		return nil
	}

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

	if devMode {
		_, err := os.Stat("template")
		if os.IsNotExist(err) {
			log.Fatal("error: could not find standard template directory, start sriracha in the same directory as the file README.md")
		}
	}

	if s.config.Template != "" {
		_, err := os.Stat(s.config.Template)
		if os.IsNotExist(err) {
			log.Fatalf("error: custom template directory %s does not exist", s.config.Template)
		}
	}

	if s.config.Locale != "" && s.config.Locale != "en" {
		buf, err := localeFS.ReadFile(fmt.Sprintf("locale/%s/%s.po", s.config.Locale, s.config.Locale))
		if err != nil {
			log.Fatalf("failed to load locale %s: %s", s.config.Locale, err)
		}

		po := gotext.NewPo()
		po.Parse(buf)
		gotext.GetStorage().AddTranslator("sriracha", po)
	}

	if devMode {
		err := s.watchTemplates()
		if err != nil {
			log.Fatalf("failed to watch templates for changes: %s", err)
		}
		fmt.Println("Running in development mode. Template files are monitored for changes.")
	}

	s.dbPool, err = connectDatabase(s.config)
	if err != nil {
		return err
	}

	err = s.setDefaultServerConfig()
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
		err := os.Mkdir(captchaDir, newDirPermission)
		if err != nil {
			log.Fatalf("failed to create captcha dir: %s", err)
		}
	}

	siteIndexFile := filepath.Join(s.config.Root, "index.html")
	_, err = os.Stat(siteIndexFile)
	if os.IsNotExist(err) {
		err = os.WriteFile(siteIndexFile, siteIndexHTML, newFilePermission)
		if err != nil {
			log.Fatalf("failed to write site index at %s: %s", siteIndexFile, err)
		}
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, unix.SIGINT, unix.SIGTERM)
	go func() {
		for {
			<-signals
			fmt.Println("Shutting down...")
			s.lock.Lock()
			os.Exit(0)
		}
	}()

	return s.listen()
}

func hashData(data string) string {
	checksum := sha512.Sum384([]byte(data + srirachaServer.config.SaltData))
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

func _hashIP(address string) string {
	if address == "" {
		return ""
	}
	return hashData(parseAddress(address))
}

func requestIP(r *http.Request) string {
	var address string
	if srirachaServer == nil {
		log.Panicf("sriracha server not running")
	} else if srirachaServer.config.Header != "" {
		values := r.Header[srirachaServer.config.Header]
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

func hashIP(r *http.Request) string {
	return _hashIP(requestIP(r))
}

func encryptPassword(password string) string {
	hash, err := argon2id.CreateHash(password+srirachaServer.config.SaltPass, argon2idParameters)
	debug.FreeOSMemory() // Hashing is memory intensive. Return memory to the OS.
	if err != nil {
		log.Fatal(err)
	}
	return hash
}

func comparePassword(password string, hash string) bool {
	match, err := argon2id.ComparePasswordAndHash(password+srirachaServer.config.SaltPass, hash)
	debug.FreeOSMemory() // Hashing is memory intensive. Return memory to the OS.
	if err != nil {
		log.Fatal(err)
	}
	return match
}

func parseInt(v string) int {
	i, err := strconv.Atoi(v)
	if err == nil && i > 0 {
		return i
	}
	return 0
}

func parseInt64(v string) int64 {
	i, err := strconv.ParseInt(v, 10, 64)
	if err == nil && i > 0 {
		return i
	}
	return 0
}

func parseFloat(v string) float64 {
	i, err := strconv.ParseFloat(v, 64)
	if err == nil && i > 0 {
		return i
	}
	return 0
}

func formString(r *http.Request, key string) string {
	return strings.TrimSpace(r.FormValue(key))
}

func formMultiString(r *http.Request, key string) []string {
	formKeys := make([]string, len(r.Form))
	var i int
	for key := range r.Form {
		formKeys[i] = key
		i++
	}
	sort.Slice(formKeys, func(i, j int) bool {
		return formKeys[i] < formKeys[j]
	})
	var values []string
	for _, formKey := range formKeys {
		formValues := r.Form[formKey]
		if strings.HasPrefix(formKey, key+"_") {
			for _, v := range formValues {
				if strings.TrimSpace(v) == "" {
					continue
				}
				values = append(values, v)
			}
		}
	}
	return values
}

func formInt(r *http.Request, key string) int {
	v, err := strconv.Atoi(formString(r, key))
	if err == nil && v >= 0 {
		return v
	}
	return 0
}

func formInt64(r *http.Request, key string) int64 {
	v, err := strconv.ParseInt(formString(r, key), 10, 64)
	if err == nil && v >= 0 {
		return v
	}
	return 0
}

func formBool(r *http.Request, key string) bool {
	return formInt(r, key) == 1
}

func formRange[T constraints.Integer](r *http.Request, key string, min T, max T) T {
	v := formInt(r, key)
	if v >= int(min) && v <= int(max) {
		return T(v)
	}
	return min
}

func pathInt(r *http.Request, prefix string) int {
	pathValue := pathString(r, prefix)
	if pathValue != "" {
		v, err := strconv.Atoi(pathValue)
		if err == nil && v > 0 {
			return v
		}
	}
	return 0
}

func pathString(r *http.Request, prefix string) string {
	if !strings.HasPrefix(r.URL.Path, prefix) {
		return ""
	}
	return strings.TrimPrefix(r.URL.Path, prefix)
}

func formatTimestamp(timestamp int64) string {
	return time.Unix(timestamp, 0).Format("2006/01/02(Mon)15:04:05")
}

func formatFileSize(size int64) string {
	v := float64(size)
	for _, unit := range []string{"", "K", "M", "G", "T", "P", "E", "Z"} {
		if math.Abs(v) < 1024.0 {
			return fmt.Sprintf("%.0f%sB", v, unit)
		}
		v /= 1024.0
	}
	return fmt.Sprintf("%.0fYB", v)
}

func formatValue(v interface{}) interface{} {
	if role, ok := v.(AccountRole); ok {
		return formatRole(role)
	} else if t, ok := v.(BoardType); ok {
		return formatBoardType(t)
	} else if t, ok := v.(BoardLock); ok {
		return formatBoardLock(t)
	} else if t, ok := v.(BoardApproval); ok {
		return formatBoardApproval(t)
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

		label += fmt.Sprintf(` [%s: "%v" > "%v"]`, name, formatValue(from), formatValue(to))
	}
	return label
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
