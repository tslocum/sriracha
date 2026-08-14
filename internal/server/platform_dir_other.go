//go:build !unix

package server

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	. "codeberg.org/tslocum/sriracha/util"
)

func writeable(dir string) bool {
	name := fmt.Sprintf("%d.txt", time.Now().Unix())
	err := os.WriteFile(filepath.Join(dir, name), []byte("w"), NewFilePermission)
	os.Remove(filepath.Join(dir, name))
	return err == nil
}

func (s *Server) handleRefreshDiskSpace() {
	// Monitoring disk space is only supported on UNIX platforms.
	for {
		<-s.refreshDisks
	}
}
