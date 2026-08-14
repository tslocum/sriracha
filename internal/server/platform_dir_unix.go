//go:build unix

package server

import (
	"log"
	"path/filepath"
	"slices"
	"time"

	. "codeberg.org/tslocum/sriracha/util"
	"golang.org/x/sys/unix"
)

func writeable(dir string) bool {
	return unix.Access(dir, unix.W_OK) == nil
}

func (s *Server) handleRefreshDiskSpace() {
	var fsIDs []unix.Fsid
	var stat unix.Statfs_t
	var err error
	for {
		s.lock.Lock()

		db := s.begin()
		var emptyDisks [][]string
		var lowDisks [][]string
		for _, b := range db.AllBoards() {
			boardDir := filepath.Join(s.config.Root, b.Dir)
			err = unix.Statfs(boardDir, &stat)
			if err != nil {
				log.Fatalf("failed to stat directory %s: %s", boardDir, err)
			} else if slices.Contains(fsIDs, stat.Fsid) {
				continue
			}

			var avail int64
			if stat.Bavail > 0 {
				avail = int64(stat.Bavail) * stat.Bsize
			}
			if avail < s.config.MinFree {
				emptyDisks = append(emptyDisks, []string{boardDir + "/", FormatFileSize(avail)})
			} else if avail < s.config.WarnFree {
				lowDisks = append(lowDisks, []string{boardDir + "/", FormatFileSize(avail)})
			}
			fsIDs = append(fsIDs, stat.Fsid)
		}
		s.opt.EmptyDisks = emptyDisks
		s.opt.LowDisks = lowDisks
		db.Commit()

		s.lock.Unlock()

		t := time.NewTimer(6 * time.Hour)
		select {
		case <-t.C:
		case <-s.refreshFreeSpace:
		}
	}
}
