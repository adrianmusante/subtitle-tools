package fs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveAbsPath_Empty(t *testing.T) {
	got, err := ResolveAbsPath("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestResolveAbsPath_RelativeBecomesAbsolute(t *testing.T) {
	d := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	defer func() { _ = os.Chdir(old) }()
	if err := os.Chdir(d); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	got, err := ResolveAbsPath("foo/bar")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// It should be absolute and end with the requested relative suffix.
	wantSuffix := string(filepath.Separator) + filepath.Join("foo", "bar")
	if !strings.HasSuffix(filepath.Clean(got), filepath.Clean(wantSuffix)) {
		t.Fatalf("expected %q to end with %q", got, wantSuffix)
	}

	// And its base directory should match the (possibly symlink-normalized) temp dir.
	gotBase := filepath.Dir(filepath.Dir(got)) // .../foo/bar -> base
	if resolved, err := filepath.EvalSymlinks(gotBase); err == nil {
		gotBase = resolved
	}
	wantBase := d
	if resolved, err := filepath.EvalSymlinks(wantBase); err == nil {
		wantBase = resolved
	}
	if gotBase != wantBase {
		t.Fatalf("base mismatch: want %q, got %q", wantBase, gotBase)
	}
}

func TestSameFilePath_BestEffort(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "a.txt")
	if err := os.WriteFile(p, []byte("hi"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	abs, err := ResolveAbsPath(p)
	if err != nil {
		t.Fatalf("ResolveAbsPath: %v", err)
	}

	if !SameFilePath(p, abs) {
		t.Fatalf("expected SameFilePath to be true")
	}
}
