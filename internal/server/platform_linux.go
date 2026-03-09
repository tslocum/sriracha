//go:build linux

package server

import (
	"os"
	"syscall"
)

func preallocateFile(file *os.File, size int64) error {
	return syscall.Fallocate(int(file.Fd()), 0, 0, size)
}
