//go:build unix && !openbsd

package server

import (
	"fmt"
	"log"

	"golang.org/x/sys/unix"
)

func remainingDiskSpace(dir string) (int64, string) {
	var stat unix.Statfs_t
	err := unix.Statfs(dir, &stat)
	if err != nil {
		log.Fatalf("failed to stat directory %s: %s", dir, err)
	}
	var avail int64
	if stat.Bavail > 0 {
		avail = int64(stat.Bavail) * int64(stat.Bsize)
	}
	return avail, fmt.Sprintf("%v", stat.Fsid)
}
