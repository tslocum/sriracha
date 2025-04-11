package sriracha

import (
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
	"strings"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

const manageTemplate = "manage"

type ServerConfig struct {
	Serve string
	Salt  string

	Address  string
	Username string
	Password string
	Schema   string
}

type Server struct {
	Boards []*Board

	config ServerConfig
	db     *Database
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

	var config ServerConfig
	err = yaml.Unmarshal(buf, &config)
	if err != nil {
		return err
	}

	if config.Serve == "" {
		return fmt.Errorf("serve (lowercase!) must be set in %s to the HTTP server listen address (hostname:port)", configFile)
	}
	if config.Salt == "" {
		return fmt.Errorf("salt (lowercase!) must be set in %s to the secure data hashing salt (a long string of random data which, once set, never changes)", configFile)
	}
	if config.Address == "" {
		return fmt.Errorf("address (lowercase!) must be set in %s to the database address (hostname:port)", configFile)
	}
	if config.Username == "" {
		return fmt.Errorf("username (lowercase!) must be set in %s to the database username", configFile)
	}
	if config.Password == "" {
		return fmt.Errorf("password (lowercase!) must be set in %s to the database password", configFile)
	}
	if config.Schema == "" {
		return fmt.Errorf("schema (lowercase!) must be set in %s to the database schema name", configFile)
	}
	s.config = config
	return nil
}

func (s *Server) buildData(w http.ResponseWriter, r *http.Request) *templateData {
	if r.URL.Path == "/imgboard/logout" {
		http.SetCookie(w, &http.Cookie{
			Name:  "sriracha_session",
			Value: "",
			Path:  "/",
		})
		return guestData
	}

	var failedLogin bool
	username := r.FormValue("username")
	if len(username) != 0 {
		failedLogin = true
		password := r.FormValue("password")
		if len(password) != 0 {
			var err error
			account, err := s.db.accountByUsernamePassword(username, password)
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
				}
			}
		}
	}
	if failedLogin {
		return &templateData{
			Error: "Invalid username or password.",
		}
	}

	cookies := r.CookiesNamed("sriracha_session")
	if len(cookies) > 0 {
		account, err := s.db.accountBySessionKey(cookies[0].Value)
		if err != nil {
			log.Fatal(err)
		} else if account != nil {
			return &templateData{
				Account: account,
			}
		}
	}
	return guestData
}

func (s *Server) writeIndex() {
}

func (s *Server) servePost(w http.ResponseWriter, r *http.Request) {
}

func (s *Server) serveManage(w http.ResponseWriter, r *http.Request) {
	var page string
	data := s.buildData(w, r)
	if len(data.Error) != 0 {
		page = "manage_error"
	} else {
		if data.Account != nil {
			if r.URL.Path == "/imgboard/board" {
				page = "manage_board"
			} else {
				page = "manage_index"
			}
		} else {
			page = "manage_login"
		}
	}

	w.Header().Set("Content-Type", "text/html")
	err := s.tpl.ExecuteTemplate(w, page, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	action := r.FormValue("action")
	if action == "" {
		values := r.URL.Query()
		action = values.Get("action")
	}
	switch action {
	case "post":
		s.servePost(w, r)
	default:
		s.serveManage(w, r)
	}
}

func (s *Server) listen() error {
	subFS, err := fs.Sub(templateFS, "template")
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("/css/", http.FileServerFS(subFS))
	mux.HandleFunc("/imgboard/board", s.serve)
	mux.HandleFunc("/imgboard/logout", s.serve)
	mux.HandleFunc("/imgboard", s.serve)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "" {
			http.NotFound(w, r)
			return
		}
		s.serve(w, r)
	})

	fmt.Printf("Serving http://%s\n", s.config.Serve)
	return http.ListenAndServe(s.config.Serve, mux)
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
					}
					if event.Has(fsnotify.Write) {
						err := s.parseTemplates(".")
						if err != nil {
							log.Printf("error: failed to parse templates: %s", err)
						}
					}
				case err, ok := <-watcher.Errors:
					if !ok {
						return
					}
					log.Printf("fsnotify error: %s", err)
				}
			}
		}()

		var monitorChanges func(dir string)
		monitorChanges = func(dir string) {
			entries, err := os.ReadDir(dir)
			if err != nil {
				log.Fatal(err)
			}
			for _, entry := range entries {
				if entry.IsDir() {
					monitorChanges(path.Join(dir, entry.Name()))
					continue
				}
				// TODO handle reloading CSS
				if strings.HasSuffix(entry.Name(), ".gohtml") {
					err = watcher.Add(path.Join("template", entry.Name()))
					if err != nil {
						log.Fatal(err)
					}
				}
			}
		}
		monitorChanges("template")
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

	s.db, err = connectDatabase(s.config.Address, s.config.Username, s.config.Password, s.config.Schema)
	if err != nil {
		return err
	}

	err = s.parseTemplates("")
	if err != nil {
		return fmt.Errorf("failed to parse templates: %s", err)
	}

	return s.listen()
}
