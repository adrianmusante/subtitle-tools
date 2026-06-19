package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adrianmusante/subtitle-tools/internal/fs"
	"github.com/spf13/cobra"
)

const (
	updateFixGoldenEnv         = "UPDATE_FIX_GOLDEN"
	mismatchContextBeforeLines = 10
	mismatchContextAfterLines  = 2
)

type fixRegressionCase struct {
	Args           []string `json:"args"`
	ExpectedTarget string   `json:"expected_target"`
}

func TestFixCLI_RegressionCases(t *testing.T) {
	testCasesRoot := filepath.Join("testdata", "fix", "cases")
	entries, err := os.ReadDir(testCasesRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			t.Skipf("no regression cases found under %s", testCasesRoot)
		}
		t.Fatalf("ReadDir(%s): %v", testCasesRoot, err)
	}

	updateGolden := os.Getenv(updateFixGoldenEnv) == "1"

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		caseName := entry.Name()
		caseDir := filepath.Join(testCasesRoot, caseName)

		t.Run(caseName, func(t *testing.T) {
			t.Parallel()

			cfg, err := loadFixRegressionCase(filepath.Join(caseDir, "case.json"))
			if err != nil {
				t.Fatalf("load case config: %v", err)
			}

			inputFixturePath := filepath.Join(caseDir, "input.srt")
			expectedFixturePath := filepath.Join(caseDir, "expected.srt")
			runDir := t.TempDir()
			workDir := filepath.Join(runDir, "workdir")
			inputRunPath := filepath.Join(runDir, "input.srt")
			outputRunPath := filepath.Join(runDir, "output.srt")

			if err := fs.CopyFile(inputFixturePath, inputRunPath); err != nil {
				t.Fatalf("copy input fixture: %v", err)
			}

			args := materializeArgs(cfg.Args, inputRunPath, outputRunPath, workDir)
			cmd := newFixTestCommand()
			cmd.SetContext(context.Background())
			cmd.SetArgs(args)

			if err := cmd.Execute(); err != nil {
				t.Fatalf("command execution failed: %v", err)
			}

			actualPath := resolveActualPath(cfg.ExpectedTarget, inputRunPath, outputRunPath)
			if updateGolden {
				if err := fs.CopyFile(actualPath, expectedFixturePath); err != nil {
					t.Fatalf("update golden expected: %v", err)
				}
				return
			}

			actualB, readActualErr := os.ReadFile(actualPath)
			expectedB, readExpectedErr := os.ReadFile(expectedFixturePath)
			if readActualErr != nil || readExpectedErr != nil {
				t.Fatalf("failed to read files for comparison (actualErr=%v, expectedErr=%v)", readActualErr, readExpectedErr)
			}

			if !bytes.Equal(normalizeLineEndings(expectedB), normalizeLineEndings(actualB)) {
				report := buildMismatchReport(expectedFixturePath, actualPath, expectedB, actualB)
				t.Fatalf("output mismatch\n%s", report)
			}
		})
	}
}

func buildMismatchReport(expectedPath, actualPath string, expectedB, actualB []byte) string {
	expectedLines := strings.Split(string(normalizeLineEndings(expectedB)), "\n")
	actualLines := strings.Split(string(normalizeLineEndings(actualB)), "\n")

	diffIdx := firstDifferentLine(expectedLines, actualLines)
	if diffIdx < 0 {
		return fmt.Sprintf("files differ but no line-level mismatch found\nexpected: %s\nactual: %s", expectedPath, actualPath)
	}

	lineNumber := diffIdx + 1
	start := diffIdx - mismatchContextBeforeLines
	if start < 0 {
		start = 0
	}
	endExpected := windowEnd(expectedLines, diffIdx, mismatchContextAfterLines)
	endActual := windowEnd(actualLines, diffIdx, mismatchContextAfterLines)

	var b strings.Builder
	_, _ = fmt.Fprintf(&b, "first difference at line %d\n", lineNumber)
	_, _ = fmt.Fprintf(&b, "expected (%s), showing lines %d-%d:\n%s\n", expectedPath, start+1, endExpected+1, formatLineWindow(expectedLines, start, diffIdx, endExpected))
	_, _ = fmt.Fprintf(&b, "actual (%s), showing lines %d-%d:\n%s", actualPath, start+1, endActual+1, formatLineWindow(actualLines, start, diffIdx, endActual))
	return b.String()
}

func normalizeLineEndings(b []byte) []byte {
	// Normalize CRLF to LF so fixture checks remain stable across OS defaults.
	return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
}

func firstDifferentLine(expectedLines, actualLines []string) int {
	maxShared := len(expectedLines)
	if len(actualLines) < maxShared {
		maxShared = len(actualLines)
	}
	for i := 0; i < maxShared; i++ {
		if expectedLines[i] != actualLines[i] {
			return i
		}
	}
	if len(expectedLines) != len(actualLines) {
		return maxShared
	}
	return -1
}

func windowEnd(lines []string, diffIdx, contextAfter int) int {
	if diffIdx >= len(lines) {
		// Length mismatch where this side already ended; only show a single EOF marker line.
		return diffIdx
	}
	end := diffIdx + contextAfter
	if end >= len(lines) {
		return len(lines) - 1
	}
	return end
}

func formatLineWindow(lines []string, start, diffIdx, end int) string {
	var b strings.Builder
	for i := start; i <= end; i++ {
		lineContent := "<EOF>"
		if i < len(lines) {
			lineContent = strings.TrimSuffix(lines[i], "\r")
		}

		prefix := " "
		if i == diffIdx {
			prefix = ">"
		}
		_, _ = fmt.Fprintf(&b, "%s %6d | %s\n", prefix, i+1, lineContent)
	}
	return strings.TrimRight(b.String(), "\n")
}

func loadFixRegressionCase(path string) (fixRegressionCase, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return fixRegressionCase{}, err
	}
	var cfg fixRegressionCase
	if err := json.Unmarshal(b, &cfg); err != nil {
		return fixRegressionCase{}, err
	}
	if len(cfg.Args) == 0 {
		return fixRegressionCase{}, fmt.Errorf("case has no args: %s", path)
	}
	cfg.ExpectedTarget = strings.ToLower(strings.TrimSpace(cfg.ExpectedTarget))
	if cfg.ExpectedTarget == "" {
		cfg.ExpectedTarget = "input"
	}
	if cfg.ExpectedTarget != "input" && cfg.ExpectedTarget != "output" {
		return fixRegressionCase{}, fmt.Errorf("invalid expected_target %q in %s", cfg.ExpectedTarget, path)
	}
	return cfg, nil
}

func materializeArgs(args []string, inputPath, outputPath, workdir string) []string {
	replacer := strings.NewReplacer(
		"{{input}}", inputPath,
		"{{output}}", outputPath,
		"{{workdir}}", workdir,
	)
	resolved := make([]string, 0, len(args))
	for _, arg := range args {
		resolved = append(resolved, replacer.Replace(arg))
	}
	return resolved
}

func resolveActualPath(expectedTarget, inputPath, outputPath string) string {
	if expectedTarget == "output" {
		return outputPath
	}
	return inputPath
}

func newFixTestCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           fixCmd.Use,
		Short:         fixCmd.Short,
		Args:          fixCmd.Args,
		RunE:          fixCmd.RunE,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	registerFixFlags(cmd)

	return cmd
}
