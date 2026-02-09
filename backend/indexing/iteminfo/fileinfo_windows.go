//go:build windows
// +build windows

package iteminfo

import (
	"os"
	"syscall"
	"time"
)

// GetBirthTime returns the creation time (birth time) of a file.
// Windows implementation using syscall.Win32FileAttributeData
func GetBirthTime(info os.FileInfo) time.Time {
	// Get platform-specific stat data
	stat, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		// Fallback to ModTime if we can't get syscall data
		return info.ModTime()
	}

	// Convert Windows FILETIME to Unix time
	nsec := stat.CreationTime.Nanoseconds()
	return time.Unix(0, nsec)
}
