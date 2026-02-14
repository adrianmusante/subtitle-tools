package run

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// TempNamer creates deterministic-ish file names for intermediate artifacts
// inside a per-run working directory.
//
// The produced names are based on the original file base name + an arbitrary
// step label + a high-resolution UTC timestamp to minimize collisions.
type TempNamer struct {
	workDir string
	base    string
	ext     string
}

// NewTempNamer creates a TempNamer rooted at workDir.
//
// originalInputPath is only used to derive a nice base name and extension.
// If originalInputPath has no extension, ".srt" is used.
func NewTempNamer(workDir, originalInputPath string) TempNamer {
	baseFile := filepath.Base(originalInputPath)
	ext := filepath.Ext(baseFile)
	base := strings.TrimSuffix(baseFile, ext)
	if ext == "" {
		ext = ".tmp"
	}
	return TempNamer{workDir: workDir, base: base, ext: ext}
}

// Step returns a path inside workDir for the given step.
func (n TempNamer) Step(step string) string {
	// Use a high-res timestamp to minimize collisions even in quick pipelines.
	now := time.Now().UTC()
	ts := now.Format("20060102150405") + fmt.Sprintf("%09d", now.Nanosecond()) // ex: now.Format("20060102T150405.000000000Z")
	name := fmt.Sprintf("%s.%s.%s%s", n.base, ts, step, n.ext)
	return filepath.Join(n.workDir, name)
}
