//go:build !linux

package server

import "os"

func allocateFile(file *os.File, size int64) {
	file.Truncate(size)
}
