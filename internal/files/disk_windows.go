//go:build windows

package files

import (
	"io/fs"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// LinkKey identifies a file that several folders share through a hard link, so a storage total counts it once.
// The second result is false for a file with a single name, which needs no key.
//
// Windows keeps the link count and the file's identity off FileInfo, so this opens the file to ask for them. The
// handle wants attributes only, which is cheap and never blocks a render or an upload holding the same file.
func LinkKey(path string, _ fs.FileInfo) ([2]uint64, bool) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return [2]uint64{}, false
	}
	h, err := windows.CreateFile(p, windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return [2]uint64{}, false
	}
	defer windows.CloseHandle(h)
	var info windows.ByHandleFileInformation
	if windows.GetFileInformationByHandle(h, &info) != nil || info.NumberOfLinks <= 1 {
		return [2]uint64{}, false
	}
	return [2]uint64{uint64(info.VolumeSerialNumber), uint64(info.FileIndexHigh)<<32 | uint64(info.FileIndexLow)}, true
}

// FreeSpace is how many bytes are still free on the disk holding dir, or 0 if that can't be read.
func FreeSpace(dir string) uint64 {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return 0
	}
	p, err := windows.UTF16PtrFromString(abs)
	if err != nil {
		return 0
	}
	var free, total, totalFree uint64
	if windows.GetDiskFreeSpaceEx(p, &free, &total, &totalFree) != nil {
		return 0
	}
	return free // what this user may still use, which a disk quota can make smaller than the disk's own free space
}
