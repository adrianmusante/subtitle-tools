package run

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestTempNamer_ExtDefaultAndStepFormat(t *testing.T) {
	n := NewTempNamer(t.TempDir(), "input")
	p := n.Step("merge")
	if !filepath.IsAbs(p) {
		t.Fatalf("expected absolute path, got %q", p)
	}
	if filepath.Dir(p) != n.workDir {
		t.Fatalf("expected path inside workdir %q, got %q", n.workDir, p)
	}
	base := filepath.Base(p)
	if !strings.HasPrefix(base, "input.") {
		t.Fatalf("unexpected name %q", base)
	}
	if !strings.HasSuffix(base, "merge.tmp") {
		t.Fatalf("expected .tmp suffix, got %q", base)
	}
}

func TestTempNamer_PreservesExtension(t *testing.T) {
	n := NewTempNamer(t.TempDir(), "/a/b/movie.es.srt")
	p := n.Step("sort")
	base := filepath.Base(p)
	if !strings.HasPrefix(base, "movie.es.") {
		t.Fatalf("unexpected name %q", base)
	}
	if !strings.HasSuffix(base, ".sort.srt") {
		t.Fatalf("expected .srt suffix, got %q", base)
	}
}
