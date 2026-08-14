//go:build openbsd

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
	if stat.F_bavail > 0 {
		avail = int64(stat.F_bavail) * int64(stat.F_bsize)
	}
	return avail, fmt.Sprintf("%v", stat.F_fsid)
}
