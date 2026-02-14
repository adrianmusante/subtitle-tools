package fs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestValidatePathWritable_DirDoesNotExist(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "missing", "out.srt")
	if err := ValidatePathWritable(out); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidatePathWritable_FileDoesNotExistButDirWritable(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "out.srt")
	if err := ValidatePathWritable(out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(out); err == nil {
		t.Fatalf("expected validator to not create final output file")
	}
}

func TestValidatePathWritable_FileExistsAndWritable(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "out.srt")
	if err := os.WriteFile(out, []byte("hi"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := ValidatePathWritable(out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePathWritable_DirNotWritable(t *testing.T) {
	// This test can be flaky if executed with elevated privileges (e.g. root).
	if os.Geteuid() == 0 {
		t.Skip("running as root; directory permissions won't prevent writes")
	}
	if runtime.GOOS == "windows" {
		t.Skip("directory permissions are not POSIX on Windows")
	}

	base := t.TempDir()
	dir := filepath.Join(base, "nowrite")
	if err := os.Mkdir(dir, 0o555); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	out := filepath.Join(dir, "out.srt")
	if err := ValidatePathWritable(out); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCopyFileContentsSync_PreservesModeAndMtime(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src.txt")
	dst := filepath.Join(tmp, "dst.txt")

	wantMode := os.FileMode(0o640)
	if err := os.WriteFile(src, []byte("hello"), wantMode); err != nil {
		t.Fatalf("write src: %v", err)
	}

	wantMTime := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(src, wantMTime, wantMTime); err != nil {
		t.Fatalf("chtimes src: %v", err)
	}

	if err := copyFileContentsSync(src, dst); err != nil {
		t.Fatalf("copy: %v", err)
	}

	st, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("stat dst: %v", err)
	}

	if runtime.GOOS != "windows" {
		gotMode := st.Mode() & os.ModePerm
		if gotMode != wantMode {
			t.Fatalf("mode mismatch: got %o want %o", gotMode, wantMode)
		}
	}

	// Filesystems may have different timestamp granularities; allow small skew.
	gotMTime := st.ModTime().UTC()
	if d := gotMTime.Sub(wantMTime); d < -2*time.Second || d > 2*time.Second {
		t.Fatalf("mtime mismatch: got %s want %s (delta %s)", gotMTime, wantMTime, d)
	}
}
