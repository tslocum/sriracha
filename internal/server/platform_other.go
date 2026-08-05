//go:build !linux

package server

import "os"

func preallocateFile(file *os.File, size int64) {
	// Preallocation is only supported on Linux.
}
