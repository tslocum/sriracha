//go:build !unix && !openbsd

package server

func remainingDiskSpace(dir string) (int64, string) {
	return 0, ""
}
