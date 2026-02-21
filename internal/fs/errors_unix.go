//go:build !windows

package fs

import (
	"errors"
	"os"
	"syscall"
)

func IsFileInUseError(err error) bool {
	// En Unix/Linux, el acceso denegado se manifiesta como ErrPermission
	return errors.Is(err, os.ErrPermission)
}

func isCrossDeviceError(err error) bool {
	var linkErr *os.LinkError
	if !errors.As(err, &linkErr) {
		return false
	}
	return errors.Is(linkErr.Err, syscall.EXDEV)
}
