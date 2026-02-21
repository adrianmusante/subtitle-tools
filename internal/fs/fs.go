package fs

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func CloseOrLog(c io.Closer, what string) {
	if err := c.Close(); err != nil {
		slog.Error("failed to close: "+what, "err", err)
	}
}

func WriteFile(r io.Reader, destPath string) error {
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer CloseOrLog(out, destPath)
	if _, err := io.Copy(out, r); err != nil {
		return err
	}
	return nil
}

func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer CloseOrLog(in, src)

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func FilesEqual(pathA, pathB string) (bool, error) {
	if SameFilePath(pathA, pathB) {
		return true, nil
	}
	stA, err := os.Stat(pathA)
	if err != nil {
		return false, err
	}
	stB, err := os.Stat(pathB)
	if err != nil {
		return false, err
	}
	if stA.Size() != stB.Size() {
		return false, nil
	}

	fa, err := os.Open(pathA)
	if err != nil {
		return false, err
	}
	defer CloseOrLog(fa, pathA)

	fb, err := os.Open(pathB)
	if err != nil {
		return false, err
	}
	defer CloseOrLog(fb, pathB)

	const chunk = 32 * 1024
	bufA := make([]byte, chunk)
	bufB := make([]byte, chunk)

	for {
		nA, errA := fa.Read(bufA)
		nB, errB := fb.Read(bufB)
		if nA != nB {
			return false, nil
		}
		if nA > 0 && !strings.EqualFold(string(bufA[:nA]), string(bufB[:nB])) {
			// Note: content compare must be byte-based; use io.Equal-style compare
			if !equalBytes(bufA[:nA], bufB[:nB]) {
				return false, nil
			}
		}

		if errA == nil && errB == nil {
			continue
		}
		if errors.Is(errA, io.EOF) && errors.Is(errB, io.EOF) {
			return true, nil
		}
		if errA != nil && !errors.Is(errA, io.EOF) {
			return false, errA
		}
		if errB != nil && !errors.Is(errB, io.EOF) {
			return false, errB
		}
		// One EOF before the other (shouldn't happen if sizes match, but be safe).
		if errors.Is(errA, io.EOF) != errors.Is(errB, io.EOF) {
			return false, nil
		}
	}
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ValidatePathWritable validates that path's parent directory exists and is writable.
//
// Behavior:
//   - If the parent directory doesn't exist => error.
//   - If the parent exists but isn't writable => error.
//   - If the file exists => attempts a safe "touch" by opening it with
//     O_WRONLY|O_APPEND (without truncating).
//   - If the file doesn't exist => creates a temporary file in the same
//     directory (never using the final path itself), closes it, and removes it.
func ValidatePathWritable(path string) error {
	if path == "" {
		return errors.New("path is empty")
	}

	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		dir = string(os.PathSeparator)
	}

	dirInfo, err := os.Stat(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("directory does not exist: %s", dir)
		}
		return fmt.Errorf("stat directory %s: %w", dir, err)
	}
	if !dirInfo.IsDir() {
		return fmt.Errorf("directory is not a directory: %s", dir)
	}

	// If the file already exists, touch it without truncating.
	if fi, err := os.Stat(path); err == nil {
		if fi.IsDir() {
			return fmt.Errorf("path is a directory: %s", path)
		}
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
		if err != nil {
			return fmt.Errorf("file exists but is not writable: %s: %w", path, err)
		}
		_ = f.Close()
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat path %s: %w", path, err)
	}

	// File doesn't exist. Validate writability by creating a temp file in the same dir.
	f, err := os.CreateTemp(dir, ".subtitle-tools-*.tmp")
	if err != nil {
		return fmt.Errorf("directory is not writable: %s: %w", dir, err)
	}
	name := f.Name()
	_ = f.Close()
	if err := os.Remove(name); err != nil {
		return fmt.Errorf("created temp file but failed to remove it (%s): %w", name, err)
	}
	return nil
}

// RenameOrMove renames src => dst.
//
// It prefers os.Rename (atomic within the same filesystem). If the operation
// fails due to a cross-device move (EXDEV), it falls back to copy+sync+remove,
// which works across different filesystems/mounts (e.g. SMB/CIFS/Samba).
func RenameOrMove(src, dst string) error {
	if err := os.Rename(src, dst); err != nil {
		var linkErr *os.LinkError
		if errors.As(err, &linkErr) && errors.Is(linkErr.Err, syscall.EXDEV) {
			if err2 := copyFileContentsSync(src, dst); err2 != nil {
				return fmt.Errorf("cross-device move: copy %s -> %s: %w", src, dst, err2)
			}
			if err2 := os.Remove(src); err2 != nil {
				return fmt.Errorf("cross-device move: remove %s: %w", src, err2)
			}
			return nil
		}
		return err
	}
	return nil
}

func copyFileContentsSync(src, dst string) error {
	st, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Preserve basic permissions (best-effort; some mounts may not support it fully).
	mode := st.Mode() & os.ModePerm

	// Preserve mtime always; atime is best-effort (platform-specific).
	mtime := st.ModTime()
	atime := mtime
	if t, ok := getAtime(st); ok {
		atime = t
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer CloseOrLog(in, src)

	// Create dst with source perms; Chmod after create to avoid umask differences.
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}

	_, copyErr := io.Copy(out, in)
	syncErr := out.Sync()
	closeErr := out.Close()

	if copyErr != nil {
		_ = os.Remove(dst)
		return copyErr
	}
	if syncErr != nil {
		_ = os.Remove(dst)
		return syncErr
	}
	if closeErr != nil {
		_ = os.Remove(dst)
		return closeErr
	}

	if err := os.Chmod(dst, mode); err != nil {
		_ = os.Remove(dst)
		return err
	}

	if err := os.Chtimes(dst, atime, mtime); err != nil {
		// Some FS/mounts may not support setting times; treat as best-effort.
		// Keep the file if data copy succeeded.
	}

	return nil
}
