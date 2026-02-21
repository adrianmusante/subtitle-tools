package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/adrianmusante/subtitle-tools/internal/fs"
	"github.com/adrianmusante/subtitle-tools/internal/run"
)

const (
	defaultOwner = "adrianmusante"
	defaultRepo  = "subtitle-tools"
)

type Options struct {
	Owner          string
	Repo           string
	APIKey         string
	CurrentVersion string
	ExePath        string
	DryRun         bool
	WorkDir        string
	HTTPClient     *http.Client
}

type Result struct {
	Updated   bool
	Version   string
	AssetName string
	ExePath   string
}

type release struct {
	TagName string  `json:"tag_name"`
	Assets  []asset `json:"assets"`
}

type asset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"url"`
}

func validateAndDefaultOptions(opts Options) (Options, error) {
	if opts.WorkDir == "" {
		return Options{}, errors.New("workdir is required")
	}
	if opts.Owner == "" {
		opts.Owner = defaultOwner
	}
	if opts.Repo == "" {
		opts.Repo = defaultRepo
	}
	if opts.ExePath == "" {
		exePath, err := getExePath()
		if err != nil {
			return Options{}, err
		}
		opts.ExePath = exePath
	}
	return opts, nil
}

func Run(ctx context.Context, opts Options) (Result, error) {
	opts, err := validateAndDefaultOptions(opts)
	if err != nil {
		return Result{}, err
	}

	slog.Info("Update check started", "owner", opts.Owner, "repo", opts.Repo, "current_version", opts.CurrentVersion, "exe_path", opts.ExePath)

	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	rel, err := fetchLatestRelease(ctx, client, opts.Owner, opts.Repo, opts.APIKey)
	if err != nil {
		return Result{}, err
	}

	version := normalizeVersion(rel.TagName)
	asset, err := findAsset(rel.Assets, version, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return Result{}, err
	}

	if isUpToDate(opts.CurrentVersion, version) {
		return Result{Updated: false, Version: version, AssetName: asset.Name, ExePath: opts.ExePath}, nil
	}

	namer := run.NewTempNamer(opts.WorkDir, opts.ExePath)

	newPath, err := downloadAndExtract(ctx, client, namer, asset, opts.APIKey, runtime.GOOS)
	if err != nil {
		return Result{}, err
	}

	outputPath := opts.ExePath
	if opts.DryRun {
		outputPath = namer.Step("exec")
	}
	err = moveFileWithFallback(newPath, outputPath)
	if err != nil {
		return Result{}, err
	}
	return Result{Updated: true, Version: version, AssetName: asset.Name, ExePath: outputPath}, nil
}

func fetchLatestRelease(ctx context.Context, client *http.Client, owner, repo, apiKey string) (release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return release{}, err
	}
	setGitHubHeaders(req, apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return release{}, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			slog.Error("close response body", "error", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return release{}, fmt.Errorf("github api error: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var rel release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return release{}, fmt.Errorf("decode release json: %w", err)
	}
	if rel.TagName == "" {
		return release{}, errors.New("github release has no tag_name")
	}
	return rel, nil
}

func findAsset(assets []asset, version, goos, goarch string) (asset, error) {
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	expected := fmt.Sprintf("subtitle-tools_%s_%s_%s%s", version, goos, goarch, ext)
	for _, a := range assets {
		if a.Name == expected {
			return a, nil
		}
	}
	return asset{}, fmt.Errorf("no asset found for %s/%s (expected %s)", goos, goarch, expected)
}

func normalizeVersion(tag string) string {
	return strings.TrimPrefix(strings.TrimSpace(tag), "v")
}

func isUpToDate(current, latest string) bool {
	if current == "" || current == "dev" {
		return false
	}
	return normalizeVersion(current) == latest
}

func downloadAndExtract(ctx context.Context, client *http.Client,
	namer run.TempNamer,
	a asset, apiKey string, goos string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.DownloadURL, nil)
	if err != nil {
		return "", err
	}
	setGitHubHeaders(req, apiKey)
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			slog.Error("close response body", "error", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return "", fmt.Errorf("download error: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	binaryName := expectedBinaryName(goos)
	if strings.HasSuffix(a.Name, ".tar.gz") {
		return extractTarGz(resp.Body, namer, binaryName)
	}
	if strings.HasSuffix(a.Name, ".zip") {
		return extractZip(resp.Body, namer, binaryName)
	}
	return "", fmt.Errorf("unsupported asset format: %s", a.Name)
}

func expectedBinaryName(goos string) string {
	if goos == "windows" {
		return "subtitle-tools.exe"
	}
	return "subtitle-tools"
}

func extractTarGz(r io.Reader, namer run.TempNamer, binaryName string) (string, error) {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return "", fmt.Errorf("read gzip: %w", err)
	}
	defer fs.CloseOrLog(gzr, "close gzip")

	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read tar: %w", err)
		}
		if hdr.FileInfo().IsDir() {
			continue
		}
		if filepath.Base(hdr.Name) != binaryName {
			continue
		}
		return writeTempBinary(namer, hdr.FileInfo().Mode(), tr)
	}
	return "", fmt.Errorf("binary %s not found in archive", binaryName)
}

func extractZip(r io.Reader, namer run.TempNamer, binaryName string) (string, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("read zip: %w", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf), int64(len(buf)))
	if err != nil {
		return "", fmt.Errorf("open zip: %w", err)
	}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if filepath.Base(f.Name) != binaryName {
			continue
		}
		fr, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("open zip file: %w", err)
		}
		path, err := writeTempBinary(namer, f.Mode(), fr)
		_ = fr.Close()
		if err != nil {
			return "", err
		}
		return path, nil
	}
	return "", fmt.Errorf("binary %s not found in zip", binaryName)
}

func writeTempBinary(namer run.TempNamer, mode os.FileMode, r io.Reader) (string, error) {
	outputTmpPath := namer.Step("download")
	err := fs.WriteFile(r, outputTmpPath)
	if err != nil {
		return "", err
	}
	if mode == 0 {
		mode = 0o755
	}
	if err = os.Chmod(outputTmpPath, mode); err != nil {
		return "", fmt.Errorf("chmod temp binary: %w", err)
	}
	return outputTmpPath, nil
}

func setGitHubHeaders(req *http.Request, apiKey string) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "subtitle-tools-update")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
}

func getExePath() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve current executable: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return "", fmt.Errorf("resolve executable symlink: %w", err)
	}
	return exePath, nil
}

// moveFileWithFallback attempts to move src to dst.
// On Windows, if the destination file is in use (sharing violation), it attempts a fallback:
// rename dst to dst.old and then move src to dst.
func moveFileWithFallback(src, dst string) error {
	if err := fs.MoveFile(src, dst); err != nil {
		if fs.IsFileInUseError(err) {
			slog.Info("File in use, attempting fallback rename strategy", "path", dst, "error", err)

			// Try to rename the old file to .old
			oldPath := dst + ".old"
			// If a previous .old file exists (for example from a prior failed update),
			// attempt to remove it first so that the rename does not fail with "file exists".
			if err := os.Remove(oldPath); err != nil && !errors.Is(err, os.ErrNotExist) {
				slog.Warn("Could not remove existing .old file before rename", "path", oldPath, "error", err)
			}
			if err := os.Rename(dst, oldPath); err != nil {
				return fmt.Errorf("could not rename existing file to .old: %w", err)
			}

			// Now move again the new file to the destination
			if err := fs.MoveFile(src, dst); err != nil {
				// If move fails, log the issue but still return an error
				return fmt.Errorf("could not move new file to destination: %w", err)
			}

			// Clean up the old file. If this fails, log a warning but don't return an error since the update succeeded.
			if err := os.Remove(oldPath); err != nil {
				slog.Warn("Could not remove old file after successful update", "path", oldPath, "error", err)
			}

			return nil
		}
		return err
	}
	return nil
}
