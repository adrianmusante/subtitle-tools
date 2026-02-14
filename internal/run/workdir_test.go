package run

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewWorkdir_EmptyBaseDir_CreatesTempAndCleansUp(t *testing.T) {
	d, cleanup, err := NewWorkdir("", "test")
	if err != nil {
		t.Fatalf("NewWorkdir: %v", err)
	}
	if d == "" {
		t.Fatalf("expected non-empty dir")
	}
	if _, err := os.Stat(d); err != nil {
		t.Fatalf("expected dir to exist: %v", err)
	}

	cleanup()
	if _, err := os.Stat(d); !os.IsNotExist(err) {
		t.Fatalf("expected dir to be removed, got err=%v", err)
	}
}

func TestNewWorkdir_WithBaseDir_CreatesSubdir_NoCleanup(t *testing.T) {
	base := t.TempDir()
	d, cleanup, err := NewWorkdir(base, "test")
	if err != nil {
		t.Fatalf("NewWorkdir: %v", err)
	}
	if d == "" {
		t.Fatalf("expected non-empty dir")
	}
	if !filepath.IsAbs(d) {
		t.Fatalf("expected absolute dir, got %q", d)
	}
	if _, err := os.Stat(d); err != nil {
		t.Fatalf("expected dir to exist: %v", err)
	}
	if rel, err := filepath.Rel(base, d); err != nil || rel == "." || rel == ".." {
		// not a perfect check, but ensures it's inside base in the common case.
		t.Fatalf("expected dir within base %q, got %q (rel=%q err=%v)", base, d, rel, err)
	}

	cleanup() // no-op expected
	if _, err := os.Stat(d); err != nil {
		t.Fatalf("expected dir to still exist after cleanup no-op: %v", err)
	}
}
