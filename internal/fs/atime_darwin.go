//go:build darwin

package fs

import (
	"os"
	"syscall"
	"time"
)

func getAtime(fi os.FileInfo) (time.Time, bool) {
	sys, ok := fi.Sys().(*syscall.Stat_t)
	if !ok || sys == nil {
		return time.Time{}, false
	}
	return time.Unix(int64(sys.Atimespec.Sec), int64(sys.Atimespec.Nsec)), true
}
