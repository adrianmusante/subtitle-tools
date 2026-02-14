package run

import "os"

// NewWorkdir creates a unique per-run working directory.
//
// If baseDir is empty, it creates a system temp directory and returns a cleanup
// function that removes it.
//
// If baseDir is provided, it ensures baseDir exists, creates a unique subdir
// inside it, and returns a no-op cleanup function.
func NewWorkdir(baseDir, prefix string) (runDir string, cleanup func(), err error) {
	if baseDir == "" {
		d, err := os.MkdirTemp("", "subtitle-tools-"+prefix+"-")
		if err != nil {
			return "", nil, err
		}
		return d, func() { _ = os.RemoveAll(d) }, nil
	}
	// ensure base exists
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return "", nil, err
	}
	d, err := os.MkdirTemp(baseDir, "subtitle-tools-"+prefix+"-")
	if err != nil {
		return "", nil, err
	}
	return d, func() {}, nil
}
