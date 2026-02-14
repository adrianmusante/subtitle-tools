//go:build linux

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
	// On linux, Stat_t exposes atime as Atim.
	return time.Unix(int64(sys.Atim.Sec), int64(sys.Atim.Nsec)), true
}
