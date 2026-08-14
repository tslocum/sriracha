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
	var lastRefresh time.Time
	const minRefresh = 5 * time.Minute
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

		fsIDs = fsIDs[:0]

		lastRefresh = time.Now()

		// Refresh disk space at least every six hours and at most every five minutes.
		t := time.NewTimer(6 * time.Hour)
		select {
		case <-t.C:
		case <-s.refreshDisks:
			delta := minRefresh - time.Since(lastRefresh)
			if delta <= 0 {
				break
			}
			tt := time.NewTimer(delta)
			var timeout bool
			for {
				select {
				case <-tt.C:
					timeout = true
				case <-s.refreshDisks:
				}
				if timeout {
					break
				}
			}
		}
	}
}
