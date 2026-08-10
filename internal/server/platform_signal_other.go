//go:build !unix

package server

import (
	"os"
	"os/signal"
)

func (s *Server) _handleSignal(signals chan os.Signal) {
	for {
		// Wait until Interrupt or Kill signal is received.
		<-signals

		// Shut down server.
		s.Stop()
		return
	}
}
func (s *Server) startSignalHandler() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, os.Kill)
	go s._handleSignal(signals)
}
