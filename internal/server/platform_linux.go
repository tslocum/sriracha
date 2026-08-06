//go:build linux

package server

import (
	"os"

	"golang.org/x/sys/unix"
)

func allocateFile(file *os.File, size int64) {
	unix.Fallocate(int(file.Fd()), 0, 0, size)
	file.Truncate(size)
}
