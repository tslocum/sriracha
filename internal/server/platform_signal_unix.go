//go:build unix

package server

import (
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"os/signal"

	"golang.org/x/sys/unix"
)

func (s *Server) _handleSignal(signals chan os.Signal) {
	for {
		// Wait until SIGHUP, SIGINT or SIGTERM is received.
		sig := <-signals

		// Rebuild static files when SIGHUP is received.
		if sig == unix.SIGHUP {
			s.lock.Lock()

			// Rebuild staic files.
			fmt.Printf("Rebuilding...\n")
			db := s.begin()
			s.rebuildAll(db)
			db.Commit()

			// Reload HTTPS certificate and private key.
			if s.config.HTTPS != "" {
				cert, err := tls.LoadX509KeyPair(s.config.HTTPSCert, s.config.HTTPSKey)
				if err != nil {
					log.Fatalf("failed to load HTTPS certificate %s and key %s: %s", s.config.HTTPSCert, s.config.HTTPSKey, err)
				}
				s.httpsCert = &cert
				fmt.Printf("Reloaded HTTPS certificate and private key.\n")
			}

			s.lock.Unlock()

			var extra string
			if s.config.HTTPS != "" {
				extra = " and https://" + s.config.HTTPS
			}
			fmt.Printf("Serving http://%s%s\n", s.config.HTTP, extra)
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
