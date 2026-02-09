//go:build !windows
// +build !windows

package iteminfo

import (
	"os"
	"time"
)

// GetBirthTime returns the creation time (birth time) of a file.
// Unix/Linux implementation - currently falling back to ModTime as birth time
// is not consistently available across all Unix-like systems via standard syscalls in Go without cgo or complex reflection.
// For the purpose of this feature, ModTime is an acceptable fallback on Linux where heatmap scans might be less intrusive or handled differently.
func GetBirthTime(info os.FileInfo) time.Time {
	// TODO: Implement specific logic for macOS (syscall.Stat_t.Birthtimespec)
	// and newer Linux kernels (syscall.Statx) if needed in future.
	return info.ModTime()
}
