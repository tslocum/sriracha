package server

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"

	"codeberg.org/tslocum/sriracha"
	"gopkg.in/yaml.v3"
)

type ServerConfig struct {
	Serve string
	Salt  string

	Address  string
	Username string
	Password string
	Database string
}

type Server struct {
	Boards []*sriracha.Board

	config ServerConfig
}

func New() *Server {
	return &Server{}
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
	if config.Database == "" {
		return fmt.Errorf("database (lowercase!) must be set in %s to the database schema name", configFile)
	}
	s.config = config
	return nil
}

func (s *Server) listen() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "" {
			http.NotFound(w, r)
			return
		}
		log.Println(r.Method, r.URL)
		log.Println("GET REQ", r.Header)
	})

	fmt.Printf("Serving http://%s\n", s.config.Serve)
	return http.ListenAndServe(s.config.Serve, mux)
}

func (s *Server) Run() error {
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

	return s.listen()
}
