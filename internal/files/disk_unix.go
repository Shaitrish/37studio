//go:build !windows

package files

import (
	"io/fs"
	"syscall"
)

// LinkKey identifies a file that several folders share through a hard link, so a storage total counts it once.
// The second result is false for a file with a single name, which needs no key.
func LinkKey(_ string, info fs.FileInfo) ([2]uint64, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || st.Nlink <= 1 {
		return [2]uint64{}, false
	}
	return [2]uint64{uint64(st.Dev), uint64(st.Ino)}, true
}

// FreeSpace is how many bytes are still free on the disk holding dir, or 0 if that can't be read.
func FreeSpace(dir string) uint64 {
	var st syscall.Statfs_t
	if syscall.Statfs(dir, &st) != nil {
		return 0
	}
	return st.Bavail * uint64(st.Bsize)
}
