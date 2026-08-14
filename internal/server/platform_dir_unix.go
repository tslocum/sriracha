//go:build unix && !bsd

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
	var fsIDs []string
	var err error
	var lastRefresh time.Time
	const minRefresh = 5 * time.Minute
	for {
		s.lock.Lock()

		db := s.begin()
		var fullDisks [][]string
		var nearDisks [][]string
		for _, b := range db.AllBoards() {
			boardDir := filepath.Join(s.config.Root, b.Dir)
			avail, fsID := remainingDiskSpace(boardDir)
			if err != nil {
				log.Fatalf("failed to stat directory %s: %s", boardDir, err)
			} else if slices.Contains(fsIDs, fsID) {
				continue
			}

			if avail < s.config.MinFree {
				fullDisks = append(fullDisks, []string{boardDir + "/", FormatFileSize(avail)})
			} else if avail < s.config.WarnFree {
				nearDisks = append(nearDisks, []string{boardDir + "/", FormatFileSize(avail)})
			}
			fsIDs = append(fsIDs, fsID)
		}
		s.opt.FullDisks = fullDisks
		s.opt.NearDisks = nearDisks
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
