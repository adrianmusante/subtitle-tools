//go:build windows

package fs

import (
	"errors"
	"os"
	"syscall"
)

const (
	errSharingViolation syscall.Errno = 32 // ERROR_SHARING_VIOLATION
	errNotSameDevice    syscall.Errno = 17 // ERROR_NOT_SAME_DEVICE
)

func IsFileInUseError(err error) bool {
	return errors.Is(err, syscall.ERROR_ACCESS_DENIED) ||
		errors.Is(err, errSharingViolation)
}

func isCrossDeviceError(err error) bool {
	var linkErr *os.LinkError
	if !errors.As(err, &linkErr) {
		return false
	}
	if errors.Is(linkErr.Err, errNotSameDevice) {
		return true
	}
	return errors.Is(linkErr.Err, syscall.EXDEV)
}
