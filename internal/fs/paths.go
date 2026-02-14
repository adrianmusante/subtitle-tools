package fs

import (
	"os"
	"path/filepath"
)

// ResolveAbsPath returns a cleaned absolute path.
//
// If the path exists, it also resolves symlinks to make comparisons more reliable.
func ResolveAbsPath(p string) (string, error) {
	if p == "" {
		return "", nil
	}
	abs, err := filepath.Abs(filepath.Clean(p))
	if err != nil {
		return "", err
	}

	// If the path exists, resolve fully.
	if _, err := os.Lstat(abs); err == nil {
		if resolved, err := filepath.EvalSymlinks(abs); err == nil {
			return resolved, nil
		}
		return abs, nil
	}

	// If it doesn't exist (common for comparisons / joins), try to resolve symlinks
	// on the nearest existing parent directory to normalize paths like /var -> /private/var on macOS.
	parent := filepath.Dir(abs)
	if parent != "" && parent != abs {
		if _, err := os.Lstat(parent); err == nil {
			if resolvedParent, err := filepath.EvalSymlinks(parent); err == nil {
				base := filepath.Base(abs)
				return filepath.Join(resolvedParent, base), nil
			}
		}
	}

	return abs, nil
}

// SameFilePath returns true if both paths refer to the same file.
//
// It attempts to use os.SameFile when both paths exist; otherwise it falls back
// to best-effort string comparison.
func SameFilePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	ai, errA := os.Stat(a)
	bi, errB := os.Stat(b)
	if errA == nil && errB == nil {
		return os.SameFile(ai, bi)
	}
	return filepath.Clean(a) == filepath.Clean(b)
}
