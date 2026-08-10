//go:build unix

package server

import "golang.org/x/sys/unix"

func writeable(dir string) bool {
	return unix.Access(dir, unix.W_OK) == nil
}
