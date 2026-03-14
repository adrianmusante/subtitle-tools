package fix

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adrianmusante/subtitle-tools/internal/run"
)

func TestFixFile_DryRun_WritesTempAndKeepsOriginal(t *testing.T) {
	workdir, cleanup, err := run.NewWorkdir("", "test")
	if err != nil {
		t.Fatalf("NewWorkdir: %v", err)
	}
	defer cleanup()

	input := filepath.Join(workdir, "in.srt")
	orig := "1\n00:00:01,000 --> 00:00:02,000\nHello\n\n"
	if err := os.WriteFile(input, []byte(orig), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	opts := Options{
		InputPath:      input,
		OutputPath:     "", // force temp output
		DryRun:         true,
		WorkDir:        workdir,
		MaxLineLength:  DefaultMaxLineLength,
		MinWordsMerge:  DefaultMinWordsForMerging,
		SkipTranslator: true,
		CreateBackup:   false,
		BackupExt:      ".bak",
	}

	res, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.WrittenPath == "" {
		t.Fatalf("expected WrittenPath")
	}
	if res.WrittenPath == input {
		t.Fatalf("dry run should not write to input path")
	}

	b, err := os.ReadFile(input)
	if err != nil {
		t.Fatalf("ReadFile input: %v", err)
	}
	if string(b) != orig {
		t.Fatalf("original file changed in dry-run")
	}

	if _, err := os.Stat(res.WrittenPath); err != nil {
		t.Fatalf("expected output file to exist: %v", err)
	}
}

func TestFixFile_InPlace_CreatesBackup(t *testing.T) {
	workdir, cleanup, err := run.NewWorkdir("", "test")
	if err != nil {
		t.Fatalf("NewWorkdir: %v", err)
	}
	defer cleanup()

	input := filepath.Join(workdir, "in.srt")
	// This fixture MUST change after fix.Run, otherwise we won't create a backup anymore.
	// Two overlapping subtitles will be merged into a single one.
	orig := "1\n00:00:01,000 --> 00:00:02,000\nHello\n\n2\n00:00:01,500 --> 00:00:03,000\nWorld\n\n"
	if err := os.WriteFile(input, []byte(orig), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	opts := Options{
		InputPath:      input,
		OutputPath:     input, // in-place
		DryRun:         false,
		WorkDir:        workdir,
		MaxLineLength:  DefaultMaxLineLength,
		MinWordsMerge:  DefaultMinWordsForMerging,
		SkipTranslator: true,
		CreateBackup:   true,
		BackupExt:      ".bak",
	}

	res, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.WrittenPath != input {
		t.Fatalf("expected WrittenPath to be input path; got %q", res.WrittenPath)
	}

	bak := input + ".bak"
	b, err := os.ReadFile(bak)
	if err != nil {
		t.Fatalf("expected backup to exist: %v", err)
	}
	if string(b) != orig {
		t.Fatalf("backup contents mismatch")
	}
}

func TestFixFile_InPlace_NoChanges_DoesNotCreateBackup(t *testing.T) {
	workdir, cleanup, err := run.NewWorkdir("", "test")
	if err != nil {
		t.Fatalf("NewWorkdir: %v", err)
	}
	defer cleanup()

	input := filepath.Join(workdir, "in.srt")
	orig := "1\n00:00:01,000 --> 00:00:02,000\nHello\n\n"
	if err := os.WriteFile(input, []byte(orig), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	opts := Options{
		InputPath:      input,
		OutputPath:     input,
		DryRun:         false,
		WorkDir:        workdir,
		MaxLineLength:  DefaultMaxLineLength,
		MinWordsMerge:  DefaultMinWordsForMerging,
		SkipTranslator: true,
		CreateBackup:   true,
		BackupExt:      ".bak",
	}

	res, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.WrittenPath != input {
		t.Fatalf("expected WrittenPath to be input path; got %q", res.WrittenPath)
	}

	// Because there are no changes, we should not create a backup.
	if _, err := os.Stat(input + ".bak"); err == nil {
		t.Fatalf("did not expect backup file to exist")
	}

	b, err := os.ReadFile(input)
	if err != nil {
		t.Fatalf("ReadFile input: %v", err)
	}
	if string(b) != orig {
		t.Fatalf("input contents changed unexpectedly")
	}
}

func TestFixFile_StripStyle_RemovesTags(t *testing.T) {
	workdir, cleanup, err := run.NewWorkdir("", "test")
	if err != nil {
		t.Fatalf("NewWorkdir: %v", err)
	}
	defer cleanup()

	input := filepath.Join(workdir, "in.srt")
	orig := strings.Join([]string{
		"1",
		"00:00:01,000 --> 00:00:02,000",
		"<font face=\"A\">Hola<br/>Chau</font>",
		"",
		"2",
		"00:00:03,000 --> 00:00:04,000",
		"<i>Ah... </i>",
		"",
		"3",
		"00:00:05,000 --> 00:00:06,000",
		"<FONT>Solo<br></FONT>",
		"",
		"4",
		"00:00:07,000 --> 00:00:08,000",
		"<i><b>nested</b></i>",
		"",
		"5",
		"00:00:09,000 --> 00:00:10,000",
		"<font>Unclosed",
		"",
	}, "\n")
	if err := os.WriteFile(input, []byte(orig), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	expected := strings.Join([]string{
		"1",
		"00:00:01,000 --> 00:00:02,000",
		"Hola",
		"Chau",
		"",
		"2",
		"00:00:03,000 --> 00:00:04,000",
		"Ah...",
		"",
		"3",
		"00:00:05,000 --> 00:00:06,000",
		"Solo",
		"",
		"4",
		"00:00:07,000 --> 00:00:08,000",
		"nested",
		"",
		"5",
		"00:00:09,000 --> 00:00:10,000",
		"<font>Unclosed",
		"",
		"",
	}, "\n")

	opts := Options{
		InputPath:      input,
		OutputPath:     "", // force temp output
		DryRun:         true,
		WorkDir:        workdir,
		MaxLineLength:  DefaultMaxLineLength,
		MinWordsMerge:  DefaultMinWordsForMerging,
		StripStyle:     true,
		SkipTranslator: true,
		CreateBackup:   false,
		BackupExt:      ".bak",
	}

	res, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	b, err := os.ReadFile(res.WrittenPath)
	if err != nil {
		t.Fatalf("ReadFile output: %v", err)
	}
	if string(b) != expected {
		t.Fatalf("output mismatch\nexpected:\n%s\n\nactual:\n%s", expected, string(b))
	}
}

func TestShiftTimeSubtitles_ZeroShift_ReturnsSamePath(t *testing.T) {
	workdir, cleanup, err := run.NewWorkdir("", "test")
	if err != nil {
		t.Fatalf("NewWorkdir: %v", err)
	}
	defer cleanup()

	input := filepath.Join(workdir, "in.srt")
	orig := "1\n00:00:01,000 --> 00:00:02,000\nHello\n\n"
	if err := os.WriteFile(input, []byte(orig), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	namer := run.NewTempNamer(workdir, input)
	opts := Options{ShiftTime: 0}

	outPath, err := shiftTimeSubtitles(input, opts, namer)
	if err != nil {
		t.Fatalf("shiftTimeSubtitles: %v", err)
	}
	if outPath != input {
		t.Fatalf("zero shift should return input path unchanged; got %q", outPath)
	}
}

func TestShiftTimeSubtitles_PositiveShift(t *testing.T) {
	workdir, cleanup, err := run.NewWorkdir("", "test")
	if err != nil {
		t.Fatalf("NewWorkdir: %v", err)
	}
	defer cleanup()

	input := filepath.Join(workdir, "in.srt")
	orig := strings.Join([]string{
		"1",
		"00:00:01,000 --> 00:00:02,000",
		"Hello",
		"",
		"2",
		"00:00:03,000 --> 00:00:04,500",
		"World",
		"",
		"",
	}, "\n")
	if err := os.WriteFile(input, []byte(orig), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	expected := strings.Join([]string{
		"1",
		"00:00:03,000 --> 00:00:04,000",
		"Hello",
		"",
		"2",
		"00:00:05,000 --> 00:00:06,500",
		"World",
		"",
		"",
	}, "\n")

	namer := run.NewTempNamer(workdir, input)
	opts := Options{ShiftTime: 2_000_000_000} // +2s

	outPath, err := shiftTimeSubtitles(input, opts, namer)
	if err != nil {
		t.Fatalf("shiftTimeSubtitles: %v", err)
	}

	b, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(b) != expected {
		t.Fatalf("output mismatch\nexpected:\n%s\n\nactual:\n%s", expected, string(b))
	}
}

func TestShiftTimeSubtitles_NegativeShift(t *testing.T) {
	workdir, cleanup, err := run.NewWorkdir("", "test")
	if err != nil {
		t.Fatalf("NewWorkdir: %v", err)
	}
	defer cleanup()

	input := filepath.Join(workdir, "in.srt")
	orig := strings.Join([]string{
		"1",
		"00:00:02,000 --> 00:00:03,000",
		"Hello",
		"",
		"2",
		"00:00:04,000 --> 00:00:05,500",
		"World",
		"",
		"",
	}, "\n")
	if err := os.WriteFile(input, []byte(orig), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	expected := strings.Join([]string{
		"1",
		"00:00:01,500 --> 00:00:02,500",
		"Hello",
		"",
		"2",
		"00:00:03,500 --> 00:00:05,000",
		"World",
		"",
		"",
	}, "\n")

	namer := run.NewTempNamer(workdir, input)
	opts := Options{ShiftTime: -500 * time.Millisecond} // -500ms

	outPath, err := shiftTimeSubtitles(input, opts, namer)
	if err != nil {
		t.Fatalf("shiftTimeSubtitles: %v", err)
	}

	b, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(b) != expected {
		t.Fatalf("output mismatch\nexpected:\n%s\n\nactual:\n%s", expected, string(b))
	}
}

func TestShiftTimeSubtitles_NegativeResult_ReturnsError(t *testing.T) {
	workdir, cleanup, err := run.NewWorkdir("", "test")
	if err != nil {
		t.Fatalf("NewWorkdir: %v", err)
	}
	defer cleanup()

	input := filepath.Join(workdir, "in.srt")
	orig := "1\n00:00:01,000 --> 00:00:02,000\nHello\n\n"
	if err := os.WriteFile(input, []byte(orig), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	namer := run.NewTempNamer(workdir, input)
	opts := Options{ShiftTime: -2 * time.Second} // -2s, causes 1s - 2s = -1s

	_, err = shiftTimeSubtitles(input, opts, namer)
	if err == nil {
		t.Fatal("expected an error for negative subtitle time, got nil")
	}
}

func TestFixFile_KeepStyle_Default(t *testing.T) {
	workdir, cleanup, err := run.NewWorkdir("", "test")
	if err != nil {
		t.Fatalf("NewWorkdir: %v", err)
	}
	defer cleanup()

	input := filepath.Join(workdir, "in.srt")
	orig := strings.Join([]string{
		"1",
		"00:00:01,000 --> 00:00:02,000",
		"<font face=\"A\">Hola<br/>Chau</font>",
		"",
		"2",
		"00:00:03,000 --> 00:00:04,000",
		"<i>Ah... </i>",
		"",
		"",
	}, "\n")
	if err := os.WriteFile(input, []byte(orig), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	opts := Options{
		InputPath:      input,
		OutputPath:     "",
		DryRun:         true,
		WorkDir:        workdir,
		MaxLineLength:  DefaultMaxLineLength,
		MinWordsMerge:  DefaultMinWordsForMerging,
		SkipTranslator: true,
		CreateBackup:   false,
		BackupExt:      ".bak",
	}

	res, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	b, err := os.ReadFile(res.WrittenPath)
	if err != nil {
		t.Fatalf("ReadFile output: %v", err)
	}
	if string(b) != orig {
		t.Fatalf("output mismatch\nexpected:\n%s\n\nactual:\n%s", orig, string(b))
	}
}
