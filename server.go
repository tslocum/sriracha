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
	"regexp"
	"strconv"
	"strings"

	"github.com/fsnotify/fsnotify"
	"golang.org/x/sys/unix"
	"gopkg.in/yaml.v3"
)

const manageTemplate = "manage"

var alphaNumericAndSymbols = regexp.MustCompile(`^[A-Za-z_-]+$`)

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

	if config.Root == "" {
		return fmt.Errorf("root (lowercase!) must be set in %s to the root folder (where board files are written)", configFile)
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
	if strings.HasPrefix(r.URL.Path, "/imgboard/logout/") {
		http.SetCookie(w, &http.Cookie{
			Name:  "sriracha_session",
			Value: "",
			Path:  "/",
		})
		http.Redirect(w, r, "/imgboard/", http.StatusFound)
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
					Manage:  &manageData{},
				}
			}
		}
	}
	if failedLogin {
		return &templateData{
			Error:  "Invalid username or password.",
			Manage: &manageData{},
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
				Manage:  &manageData{},
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
	defer func() {
		w.Header().Set("Content-Type", "text/html")
		err := s.tpl.ExecuteTemplate(w, page, data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}()
	if len(data.Error) != 0 {
		page = "manage_error"
	} else {
		if data.Account != nil {
			if strings.HasPrefix(r.URL.Path, "/imgboard/board") {
				page = "manage_board"

				boardID, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/imgboard/board/"))
				if err == nil && boardID > 0 {
					data.Manage.Board, err = s.db.boardByID(boardID)
					if err != nil {
						log.Fatal(err)
					}

					if data.Manage.Board != nil && r.Method == http.MethodPost {
						oldDir := data.Manage.Board.Dir
						data.Manage.Board.Dir = strings.TrimSpace(r.FormValue("dir"))
						data.Manage.Board.Name = strings.TrimSpace(r.FormValue("name"))
						data.Manage.Board.Description = strings.TrimSpace(r.FormValue("description"))
						typeString := r.FormValue("type")
						if typeString == "1" {
							data.Manage.Board.Type = TypeForum
						} else {
							data.Manage.Board.Type = TypeImageboard
						}
						err := data.Manage.Board.validate()
						if err != nil {
							page = "manage_error"
							data.Error = err.Error()
							return
						}

						if data.Manage.Board.Dir != oldDir {
							_, err := os.Stat(filepath.Join(s.config.Root, data.Manage.Board.Dir))
							if err != nil {
								if !os.IsNotExist(err) {
									log.Fatal(err)
								}
							} else {
								page = "manage_error"
								data.Error = "New directory already exists"
								return
							}
						}

						err = s.db.updateBoard(data.Manage.Board)
						if err != nil {
							page = "manage_error"
							data.Error = err.Error()
							return
						}

						if data.Manage.Board.Dir != oldDir {
							err := os.Rename(filepath.Join(s.config.Root, oldDir), filepath.Join(s.config.Root, data.Manage.Board.Dir))
							if err != nil {
								page = "manage_error"
								data.Error = fmt.Sprintf("Failed to rename board directory: %s", err)
								return
							}
						}

						http.Redirect(w, r, "/imgboard/board/", http.StatusFound)
						return
					}
				} else {
					if r.Method == http.MethodPost {
						b := &Board{
							Dir:         strings.TrimSpace(r.FormValue("dir")),
							Name:        strings.TrimSpace(r.FormValue("name")),
							Description: strings.TrimSpace(r.FormValue("description")),
							Type:        TypeImageboard,
						}
						typeString := r.FormValue("type")
						if typeString == "1" {
							b.Type = TypeForum
						}

						err := data.Manage.Board.validate()
						if err != nil {
							page = "manage_error"
							data.Error = err.Error()
							return
						}

						err = os.Mkdir(filepath.Join(s.config.Root, b.Dir), 0755)
						if err != nil {
							page = "manage_error"
							if os.IsExist(err) {
								data.Error = fmt.Sprintf("Board directory %s already exists.", b.Dir)
							} else {
								data.Error = fmt.Sprintf("Failed to create board directory %s: %s", b.Dir, err)
							}
							return
						}

						err = s.db.addBoard(b)
						if err != nil {
							page = "manage_error"
							data.Error = err.Error()
							return
						}

						http.Redirect(w, r, "/imgboard/board/", http.StatusFound)
						return
					}
					data.Manage.Boards, err = s.db.allBoards()
					if err != nil {
						log.Fatal(err)
					}
				}
			} else {
				page = "manage_index"
			}
		} else {
			page = "manage_login"
		}
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
	mux.HandleFunc("/imgboard/", s.serve)
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

		err = watcher.Add("template")
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

	s.db, err = connectDatabase(s.config.Address, s.config.Username, s.config.Password, s.config.Schema)
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
