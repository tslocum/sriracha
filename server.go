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

func (s *Server) buildData(r *http.Request) *templateData {
	var sessionKey string
	username := r.FormValue("username")
	if len(username) != 0 {
		password := r.FormValue("password")
		if len(password) != 0 {
			log.Println("PASSWORD LOGIN")
		}
	}
	cookies := r.CookiesNamed("sriracha_key")
	if len(cookies) > 0 {
		sessionKey = cookies[0].Value
	}
	if len(sessionKey) == 0 {
		return guestData
	}
	return &templateData{
		Account: &Account{ID: 1, Name: "TODO"},
	}
}

func (s *Server) writeIndex() {
}

func (s *Server) servePost(w http.ResponseWriter, r *http.Request) {
}

func (s *Server) serveManage(w http.ResponseWriter, r *http.Request) {
	data := s.buildData(r)

	page := "manage_login"
	if data.Account != nil {
		page = "manage_index"
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
	mux := http.NewServeMux()
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

func (s *Server) parseTemplates() error {
	s.tpl = template.New("sriracha")
	entries, err := templatesFS.ReadDir("template")
	if err != nil {
		return err
	}
	for _, f := range entries {
		buf, err := templatesFS.ReadFile(filepath.Join("template", f.Name()))
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
	flag.StringVar(&configFile, "config", "", "path to configuration file (default: ~/.config/sriracha/config.yml)")
	flag.Parse()

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

	loadPlugin := func(pluginPath string) error {
		_, err = plugin.Open(pluginPath)
		if err != nil {
			return fmt.Errorf("failed to load plugin %s: %s", pluginPath, err)
		}
		return err
	}
	for _, pluginPath := range flag.Args() {
		info, err := os.Stat(pluginPath)
		if os.IsNotExist(err) {
			return fmt.Errorf("failed to load plugin %s: file or directory not found", pluginPath)
		}
		if info.IsDir() {
			filepath.WalkDir(pluginPath, func(path string, d fs.DirEntry, err error) error {
				log.Println(path)
				return nil
			})
		} else {
			err = loadPlugin(pluginPath)
			if err != nil {
				return err
			}
		}
	}

	s.db, err = connectDatabase(s.config.Address, s.config.Username, s.config.Password, s.config.Schema)
	if err != nil {
		return err
	}

	err = s.parseTemplates()
	if err != nil {
		return fmt.Errorf("failed to parse templates: %s", err)
	}

	return s.listen()
}
