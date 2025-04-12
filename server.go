package sriracha

import (
	"context"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"plugin"
	"regexp"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sys/unix"
	"gopkg.in/yaml.v3"
)

const manageTemplate = "manage"

var alphaNumericAndSymbols = regexp.MustCompile(`^[0-9A-Za-z_-]+$`)

type Server struct {
	Boards []*Board

	config Config
	dbPool *pgxpool.Pool
	tpl    *template.Template
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
		return fmt.Errorf("root (lowercase!) must be set in %s to the root folder (where board files are written)", configFile)
	case config.Serve == "":
		return fmt.Errorf("serve (lowercase!) must be set in %s to the HTTP server listen address (hostname:port)", configFile)
	case config.Salt == "":
		return fmt.Errorf("salt (lowercase!) must be set in %s to the secure data hashing salt (a long string of random data which, once set, never changes)", configFile)
	case config.Min <= 0:
		return fmt.Errorf("min (lowercase!) must be set in %s to the minimum number of connections of the database connection pool (1 is a reasonable choice)", configFile)
	case config.Max <= 0:
		return fmt.Errorf("max (lowercase!) must be set in %s to the maximum number of connections of the database connection pool (4 is a reasonable choice)", configFile)
	case config.Max < config.Min:
		return fmt.Errorf("max must be greater than or equal to min in %s", configFile)
	case config.Address == "":
		return fmt.Errorf("address (lowercase!) must be set in %s to the database address (hostname:port)", configFile)
	case config.Username == "":
		return fmt.Errorf("username (lowercase!) must be set in %s to the database username", configFile)
	case config.Password == "":
		return fmt.Errorf("password (lowercase!) must be set in %s to the database password", configFile)
	case config.Schema == "":
		return fmt.Errorf("schema (lowercase!) must be set in %s to the database schema name", configFile)
	default:
		s.config = config
		return nil
	}
}

func (s *Server) parseTemplates(dir string) error {
	if dir != "" {
		s.tpl = template.New("sriracha")
		entries, err := os.ReadDir("template")
		if err != nil {
			return err
		}
		for _, f := range entries {
			if !strings.HasSuffix(f.Name(), ".gohtml") {
				continue
			}

			buf, err := os.ReadFile(filepath.Join("template", f.Name()))
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

	s.tpl = template.New("sriracha")
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
	return nil
}

func (s *Server) watchTemplates() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				} else if !event.Has(fsnotify.Write) {
					continue
				}
				err := s.parseTemplates(".")
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

	return watcher.Add("template")
}

func (s *Server) buildData(db *Database, w http.ResponseWriter, r *http.Request) *templateData {
	if strings.HasPrefix(r.URL.Path, "/imgboard/logout/") {
		http.SetCookie(w, &http.Cookie{
			Name:  "sriracha_session",
			Value: "",
			Path:  "/",
		})
		http.Redirect(w, r, "/imgboard/", http.StatusFound)
		return guestData
	}

	if r.URL.Path == "/imgboard/" || r.URL.Path == "/imgboard" {
		var failedLogin bool
		username := r.FormValue("username")
		if len(username) != 0 {
			failedLogin = true
			password := r.FormValue("password")
			if len(password) != 0 {
				var err error
				account, err := db.loginAccount(username, password)
				if err != nil {
					log.Fatal(err)
				} else if account != nil {
					http.SetCookie(w, &http.Cookie{
						Name:  "sriracha_session",
						Value: account.Session,
						Path:  "/",
					})
					return &templateData{
						Account: account,
						Manage:  &manageData{},
					}
				}
			}
		}
		if failedLogin {
			return &templateData{
				Info:     "Invalid username or password.",
				Template: "manage_error",
				Manage:   &manageData{},
			}
		}
	}

	cookies := r.CookiesNamed("sriracha_session")
	if len(cookies) > 0 {
		account, err := db.accountBySessionKey(cookies[0].Value)
		if err != nil {
			log.Fatal(err)
		} else if account != nil {
			return &templateData{
				Account: account,
				Manage:  &manageData{},
			}
		}
	}
	return guestData
}

func (s *Server) writeThread(post *Post) {
	// TODO
}

func (s *Server) writeIndexes(board *Board) {
	f, err := os.OpenFile(filepath.Join(s.config.Root, board.Dir, "index.html"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatal(err)
	}

	data := &templateData{
		Board:  board,
		Manage: &manageData{},
	}
	err = s.tpl.ExecuteTemplate(f, "board_index.gohtml", data)
	if err != nil {
		log.Fatal(err)
	}
}

func (s *Server) writeBoard(board *Board) {
	s.writeIndexes(board)
	// for all threads, write thread
}

func (s *Server) serveManage(db *Database, w http.ResponseWriter, r *http.Request) {
	data := s.buildData(db, w, r)
	defer func() {
		w.Header().Set("Content-Type", "text/html")
		err := s.tpl.ExecuteTemplate(w, data.Template+".gohtml", data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}()

	if len(data.Info) != 0 {
		data.Template = "manage_error"
		return
	}

	if data.Account != nil {
		err := db.updateAccountLastActive(data.Account.ID)
		if err != nil {
			log.Fatal(err)
		}
	}

	data.Template = "manage_login"

	if data.Account == nil {
		return
	}
	switch {
	case strings.HasPrefix(r.URL.Path, "/imgboard/password"):
		s.serveChangePassword(data, db, w, r)
	case strings.HasPrefix(r.URL.Path, "/imgboard/account"):
		s.serveAccount(data, db, w, r)
	case strings.HasPrefix(r.URL.Path, "/imgboard/board"):
		s.serveBoard(data, db, w, r)
	case strings.HasPrefix(r.URL.Path, "/imgboard/keyword"):
		s.serveKeyword(data, db, w, r)
	default:
		data.Template = "manage_index"
	}
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	action := r.FormValue("action")
	if action == "" {
		values := r.URL.Query()
		action = values.Get("action")
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

	switch action {
	case "post":
		//s.servePost(db, w, r)
	default:
		s.serveManage(db, w, r)
	}

	_, err = conn.Exec(context.Background(), "COMMIT")
	if err != nil {
		log.Fatal(err)
	}
}

func (s *Server) listen() error {
	subFS, err := fs.Sub(templateFS, "template")
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("/css/", http.FileServerFS(subFS))
	mux.HandleFunc("/imgboard/", s.serve)
	mux.Handle("/", http.FileServer(http.Dir(s.config.Root)))

	fmt.Printf("Serving http://%s\n", s.config.Serve)
	return http.ListenAndServe(s.config.Serve, mux)
}

func (s *Server) Run() error {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n  sriracha [OPTION...] [PLUGIN...]\n\nOptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nsriracha imageboard and forum\n  https://codeberg.org/tslocum/sriracha\nGNU LESSER GENERAL PUBLIC LICENSE\n  https://codeberg.org/tslocum/sriracha/src/branch/main/LICENSE\n")
	}
	var configFile string
	var devMode bool
	flag.StringVar(&configFile, "config", "", "path to configuration file (default: ~/.config/sriracha/config.yml)")
	flag.BoolVar(&devMode, "dev", false, "run in development mode (monitor template files and apply changes)")
	flag.Parse()

	if configFile == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			configFile = path.Join(homeDir, ".config", "sriracha", "config.yml")
		}
	}

	if devMode {
		err := s.watchTemplates()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Running in development mode. Template files are monitored for changes.")
	}

	err := s.parseConfig(configFile)
	if err != nil {
		return err
	}

	var loadPlugin func(pluginPath string) error
	loadPlugin = func(pluginPath string) error {
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
				return loadPlugin(path)
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
	for _, pluginPath := range flag.Args() {
		err = loadPlugin(pluginPath)
		if err != nil {
			return err
		}
	}

	s.dbPool, err = connectDatabase(s.config.Address, s.config.Username, s.config.Password, s.config.Schema, s.config.Min, s.config.Max)
	if err != nil {
		return err
	}

	err = s.parseTemplates("")
	if err != nil {
		return fmt.Errorf("failed to parse templates: %s", err)
	}

	if unix.Access(s.config.Root, unix.W_OK) != nil {
		return fmt.Errorf("failed to set root: %s is not writable", s.config.Root)
	}

	return s.listen()
}
