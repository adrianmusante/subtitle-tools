//go:build !darwin && !linux

package fs

import (
	"os"
	"time"
)

// getAtime returns the file's access time if available on this platform.
//
// ok is false when the platform doesn't expose atime in os.FileInfo.Sys().
func getAtime(fi os.FileInfo) (t time.Time, ok bool) {
	return fi.ModTime(), false
}
