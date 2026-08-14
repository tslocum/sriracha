//go:build unix && !bsd

package server

import (
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
	var lastRefresh time.Time
	const maxRefresh = 6 * time.Hour
	const minRefresh = 5 * time.Minute
	for {
		s.lock.Lock()

		db := s.begin()
		var fullDisks [][]string
		var nearDisks [][]string
		for _, b := range db.AllBoards() {
			boardDir := filepath.Join(s.config.Root, b.Dir)
			avail, fsID := remainingDiskSpace(boardDir)
			if slices.Contains(fsIDs, fsID) {
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
		wait := maxRefresh
		if len(s.opt.FullDisks) > 0 || len(s.opt.NearDisks) > 0 {
			wait = minRefresh
		}
		t := time.NewTimer(wait)
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
