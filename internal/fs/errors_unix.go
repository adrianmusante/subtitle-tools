//go:build !windows

package fs

import (
	"errors"
	"os"
	"syscall"
)

func IsFileInUseError(err error) bool {
	// On non-Windows platforms we don't reliably detect "file in use".
	// Returning false avoids treating generic permission errors as "file in use"
	// and prevents masking the original failure in higher-level fallbacks.
	_ = err
	return false
}

func isCrossDeviceError(err error) bool {
	var linkErr *os.LinkError
	if !errors.As(err, &linkErr) {
		return false
	}
	return errors.Is(linkErr.Err, syscall.EXDEV)
}
