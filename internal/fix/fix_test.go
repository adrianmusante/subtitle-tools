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

func TestFixFile_StripHI_RemovesHICues(t *testing.T) {
	workdir, cleanup, err := run.NewWorkdir("", "test")
	if err != nil {
		t.Fatalf("NewWorkdir: %v", err)
	}
	defer cleanup()

	input := filepath.Join(workdir, "in.srt")
	orig := strings.Join([]string{
		"1",
		"00:00:01,000 --> 00:00:02,000",
		"[MUSIC]",
		"",
		"2",
		"00:00:03,000 --> 00:00:04,000",
		"(sighs) Hola.",
		"",
		"3",
		"00:00:05,000 --> 00:00:06,000",
		"No me voy [door opens]",
		"",
		"4",
		"00:00:07,000 --> 00:00:08,000",
		"- [whispers]",
		"",
		"5",
		"00:00:09,000 --> 00:00:10,000",
		"- [Sra. Cobel] He dicho si estás bien.",
		"",
	}, "\n")
	if err := os.WriteFile(input, []byte(orig), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	expected := strings.Join([]string{
		"1",
		"00:00:03,000 --> 00:00:04,000",
		"(sighs) Hola.",
		"",
		"2",
		"00:00:05,000 --> 00:00:06,000",
		"No me voy",
		"",
		"3",
		"00:00:09,000 --> 00:00:10,000",
		"- He dicho si estás bien.",
		"",
		"",
	}, "\n")

	opts := Options{
		InputPath:      input,
		OutputPath:     "",
		DryRun:         true,
		WorkDir:        workdir,
		MaxLineLength:  DefaultMaxLineLength,
		MinWordsMerge:  DefaultMinWordsForMerging,
		StripHI:        true,
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

func TestStripSubtitleHI_ModeLayers(t *testing.T) {
	tests := []struct {
		name  string
		mode  string
		input string
		want  string
	}{
		{
			name:  "safe uses conservative base cleanup",
			mode:  StripHIModeSafe,
			input: "[over radio]: JOHN: Run [alarm blares]",
			want:  "JOHN: Run",
		},
		{
			name:  "safe drops cue-only lines with trailing colon",
			mode:  StripHIModeSafe,
			input: "[voice fading]:",
			want:  "",
		},
		{
			name:  "standard builds on safe and removes speaker prefix",
			mode:  StripHIModeStandard,
			input: "[over radio]: JOHN: Run [alarm blares]",
			want:  "Run",
		},
		{
			name:  "safe plus extends the conservative layer to parens and braces",
			mode:  StripHIModeSafePlus,
			input: "(whispers) JOHN: Run {door slams}",
			want:  "JOHN: Run",
		},
		{
			name:  "standard plus extends standard with parens and braces",
			mode:  StripHIModeStandardPlus,
			input: "(whispers) JOHN: Run {door slams}",
			want:  "Run",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := stripSubtitleHI(tc.input, tc.mode)
			if got != tc.want {
				t.Fatalf("stripSubtitleHI(%q, %q) = %q, want %q", tc.input, tc.mode, got, tc.want)
			}
		})
	}
}

func TestFixFile_StripHI_SafePlus_RemovesClosedMultilineCue(t *testing.T) {
	workdir, cleanup, err := run.NewWorkdir("", "test")
	if err != nil {
		t.Fatalf("NewWorkdir: %v", err)
	}
	defer cleanup()

	input := filepath.Join(workdir, "in.srt")
	orig := strings.Join([]string{
		"1",
		"00:00:01,000 --> 00:00:02,000",
		"(sighs) Hola.",
		"",
		"2",
		"00:00:03,000 --> 00:00:04,000",
		"{door closes}",
		"",
		"3",
		"00:00:05,000 --> 00:00:06,000",
		"No me voy [door opens]",
		"",
	}, "\n")
	if err := os.WriteFile(input, []byte(orig), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	expected := strings.Join([]string{
		"1",
		"00:00:01,000 --> 00:00:02,000",
		"Hola.",
		"",
		"2",
		"00:00:05,000 --> 00:00:06,000",
		"No me voy",
		"",
		"",
	}, "\n")

	opts := Options{
		InputPath:      input,
		OutputPath:     "",
		DryRun:         true,
		WorkDir:        workdir,
		MaxLineLength:  DefaultMaxLineLength,
		MinWordsMerge:  DefaultMinWordsForMerging,
		StripHI:        true,
		StripHIMode:    StripHIModeSafePlus,
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

func TestFixFile_StripHI_Safe_RemovesLineReducedToDash(t *testing.T) {
	workdir, cleanup, err := run.NewWorkdir("", "test")
	if err != nil {
		t.Fatalf("NewWorkdir: %v", err)
	}
	defer cleanup()

	input := filepath.Join(workdir, "in.srt")
	orig := strings.Join([]string{
		"1",
		"00:00:01,000 --> 00:00:02,000",
		"- [man whoops] -[Kay laughs]",
		"",
		"2",
		"00:00:03,000 --> 00:00:04,000",
		"Hello.",
		"",
		"",
	}, "\n")
	if err := os.WriteFile(input, []byte(orig), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	expected := strings.Join([]string{
		"1",
		"00:00:03,000 --> 00:00:04,000",
		"Hello.",
		"",
		"",
	}, "\n")

	res, err := Run(context.Background(), Options{
		InputPath:      input,
		OutputPath:     "",
		DryRun:         true,
		WorkDir:        workdir,
		MaxLineLength:  DefaultMaxLineLength,
		MinWordsMerge:  DefaultMinWordsForMerging,
		StripHI:        true,
		StripHIMode:    StripHIModeSafe,
		SkipTranslator: true,
		CreateBackup:   false,
		BackupExt:      ".bak",
	})
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

func TestFixFile_StripHI_Safe_RemovesLeadingCueWithColon(t *testing.T) {
	workdir, cleanup, err := run.NewWorkdir("", "test")
	if err != nil {
		t.Fatalf("NewWorkdir: %v", err)
	}
	defer cleanup()

	input := filepath.Join(workdir, "in.srt")
	orig := strings.Join([]string{
		"1",
		"00:00:01,000 --> 00:00:02,000",
		"[over radio]: Utah... hit by",
		"machine-gun fire. Casualties...",
		"",
		"",
	}, "\n")
	if err := os.WriteFile(input, []byte(orig), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	expected := strings.Join([]string{
		"1",
		"00:00:01,000 --> 00:00:02,000",
		"Utah... hit by",
		"machine-gun fire. Casualties...",
		"",
		"",
	}, "\n")

	res, err := Run(context.Background(), Options{
		InputPath:      input,
		OutputPath:     "",
		DryRun:         true,
		WorkDir:        workdir,
		MaxLineLength:  DefaultMaxLineLength,
		MinWordsMerge:  DefaultMinWordsForMerging,
		StripHI:        true,
		StripHIMode:    StripHIModeSafe,
		SkipTranslator: true,
		CreateBackup:   false,
		BackupExt:      ".bak",
	})
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

func TestFixFile_StripHI_Standard_RemovesLeadingCueWithColon(t *testing.T) {
	workdir, cleanup, err := run.NewWorkdir("", "test")
	if err != nil {
		t.Fatalf("NewWorkdir: %v", err)
	}
	defer cleanup()

	input := filepath.Join(workdir, "in.srt")
	orig := strings.Join([]string{
		"1",
		"00:00:01,000 --> 00:00:02,000",
		"[over radio]: Utah... hit by",
		"machine-gun fire. Casualties...",
		"",
		"",
	}, "\n")
	if err := os.WriteFile(input, []byte(orig), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	expected := strings.Join([]string{
		"1",
		"00:00:01,000 --> 00:00:02,000",
		"Utah... hit by",
		"machine-gun fire. Casualties...",
		"",
		"",
	}, "\n")

	res, err := Run(context.Background(), Options{
		InputPath:      input,
		OutputPath:     "",
		DryRun:         true,
		WorkDir:        workdir,
		MaxLineLength:  DefaultMaxLineLength,
		MinWordsMerge:  DefaultMinWordsForMerging,
		StripHI:        true,
		StripHIMode:    StripHIModeStandard,
		SkipTranslator: true,
		CreateBackup:   false,
		BackupExt:      ".bak",
	})
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

func TestFixFile_StripHI_StandardMode(t *testing.T) {
	workdir, cleanup, err := run.NewWorkdir("", "test")
	if err != nil {
		t.Fatalf("NewWorkdir: %v", err)
	}
	defer cleanup()

	input := filepath.Join(workdir, "in.srt")
	orig := strings.Join([]string{
		"1",
		"00:00:01,000 --> 00:00:02,000",
		"JOHN: ♪ (whispers) corre! [door slams]",
		"",
		"",
	}, "\n")
	if err := os.WriteFile(input, []byte(orig), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	expected := strings.Join([]string{
		"1",
		"00:00:01,000 --> 00:00:02,000",
		"♪ (whispers) corre!",
		"",
		"",
	}, "\n")

	opts := Options{
		InputPath:      input,
		OutputPath:     "",
		DryRun:         true,
		WorkDir:        workdir,
		MaxLineLength:  DefaultMaxLineLength,
		MinWordsMerge:  DefaultMinWordsForMerging,
		StripHI:        true,
		StripHIMode:    StripHIModeStandard,
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

func TestFixFile_StripHI_StandardPlusMode(t *testing.T) {
	workdir, cleanup, err := run.NewWorkdir("", "test")
	if err != nil {
		t.Fatalf("NewWorkdir: %v", err)
	}
	defer cleanup()

	input := filepath.Join(workdir, "in.srt")
	orig := strings.Join([]string{
		"1",
		"00:00:01,000 --> 00:00:02,000",
		"JOHN: ♪ {nervous} (whispers) corre! [door slams]",
		"",
		"",
	}, "\n")
	if err := os.WriteFile(input, []byte(orig), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	expected := strings.Join([]string{
		"1",
		"00:00:01,000 --> 00:00:02,000",
		"♪ corre!",
		"",
		"",
	}, "\n")

	opts := Options{
		InputPath:      input,
		OutputPath:     "",
		DryRun:         true,
		WorkDir:        workdir,
		MaxLineLength:  DefaultMaxLineLength,
		MinWordsMerge:  DefaultMinWordsForMerging,
		StripHI:        true,
		StripHIMode:    StripHIModeStandardPlus,
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

func TestFixFile_StripStyleThenHI(t *testing.T) {
	workdir, cleanup, err := run.NewWorkdir("", "test")
	if err != nil {
		t.Fatalf("NewWorkdir: %v", err)
	}
	defer cleanup()

	input := filepath.Join(workdir, "in.srt")
	orig := strings.Join([]string{
		"1",
		"00:00:01,000 --> 00:00:02,000",
		"<i>[MUSIC]</i>",
		"",
		"2",
		"00:00:03,000 --> 00:00:04,000",
		"<font>(sighs)</font> Hola",
		"",
	}, "\n")
	if err := os.WriteFile(input, []byte(orig), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	expected := strings.Join([]string{
		"1",
		"00:00:03,000 --> 00:00:04,000",
		"(sighs) Hola",
		"",
		"",
	}, "\n")

	opts := Options{
		InputPath:      input,
		OutputPath:     "",
		DryRun:         true,
		WorkDir:        workdir,
		MaxLineLength:  DefaultMaxLineLength,
		MinWordsMerge:  DefaultMinWordsForMerging,
		StripStyle:     true,
		StripHI:        true,
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
	shiftTime := time.Duration(0)

	outPath, err := shiftTimeSubtitles(input, shiftTime, namer)
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
	shiftTime := 2 * time.Second

	outPath, err := shiftTimeSubtitles(input, shiftTime, namer)
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
	shiftTime := -500 * time.Millisecond

	outPath, err := shiftTimeSubtitles(input, shiftTime, namer)
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
	shiftTime := -2 * time.Second // -2s, causes 1s - 2s = -1s

	_, err = shiftTimeSubtitles(input, shiftTime, namer)
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

func TestFixFile_InvalidStripHIMode_ReturnsError(t *testing.T) {
	workdir, cleanup, err := run.NewWorkdir("", "test")
	if err != nil {
		t.Fatalf("NewWorkdir: %v", err)
	}
	defer cleanup()

	input := filepath.Join(workdir, "in.srt")
	orig := "1\n00:00:01,000 --> 00:00:02,000\nHola\n\n"
	if err := os.WriteFile(input, []byte(orig), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err = Run(context.Background(), Options{
		InputPath:      input,
		OutputPath:     "",
		DryRun:         true,
		WorkDir:        workdir,
		MaxLineLength:  DefaultMaxLineLength,
		MinWordsMerge:  DefaultMinWordsForMerging,
		StripHI:        true,
		StripHIMode:    "super-aggressive",
		SkipTranslator: true,
		CreateBackup:   false,
		BackupExt:      ".bak",
	})
	if err == nil {
		t.Fatal("expected error for invalid strip-hi mode")
	}
}

func TestFixFile_StripHI_Standard_PreservesDialogueDash(t *testing.T) {
	workdir, cleanup, err := run.NewWorkdir("", "test")
	if err != nil {
		t.Fatalf("NewWorkdir: %v", err)
	}
	defer cleanup()

	input := filepath.Join(workdir, "in.srt")
	orig := strings.Join([]string{
		"1",
		"00:00:01,000 --> 00:00:02,000",
		"- Thank you.",
		"",
		"",
	}, "\n")
	if err := os.WriteFile(input, []byte(orig), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	expected := strings.Join([]string{
		"1",
		"00:00:01,000 --> 00:00:02,000",
		"- Thank you.",
		"",
		"",
	}, "\n")

	res, err := Run(context.Background(), Options{
		InputPath:      input,
		OutputPath:     "",
		DryRun:         true,
		WorkDir:        workdir,
		MaxLineLength:  DefaultMaxLineLength,
		MinWordsMerge:  DefaultMinWordsForMerging,
		StripHI:        true,
		StripHIMode:    StripHIModeStandard,
		SkipTranslator: true,
		CreateBackup:   false,
		BackupExt:      ".bak",
	})
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

func TestFixFile_StripHI_Standard_PreservesMusicWithLyrics_AndRemovesEmptyMusicLine(t *testing.T) {
	workdir, cleanup, err := run.NewWorkdir("", "test")
	if err != nil {
		t.Fatalf("NewWorkdir: %v", err)
	}
	defer cleanup()

	input := filepath.Join(workdir, "in.srt")
	orig := strings.Join([]string{
		"1",
		"00:00:01,000 --> 00:00:02,000",
		"♪ But then his number came up ♪",
		"",
		"2",
		"00:00:03,000 --> 00:00:04,000",
		"♪ He had a boogie style",
		"that no one else could play ♪",
		"",
		"3",
		"00:00:05,000 --> 00:00:06,000",
		"♪ ♪",
		"",
		"",
	}, "\n")
	if err := os.WriteFile(input, []byte(orig), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	expected := strings.Join([]string{
		"1",
		"00:00:01,000 --> 00:00:02,000",
		"♪ But then his number came up ♪",
		"",
		"2",
		"00:00:03,000 --> 00:00:04,000",
		"♪ He had a boogie style",
		"that no one else could play ♪",
		"",
		"",
	}, "\n")

	res, err := Run(context.Background(), Options{
		InputPath:      input,
		OutputPath:     "",
		DryRun:         true,
		WorkDir:        workdir,
		MaxLineLength:  DefaultMaxLineLength,
		MinWordsMerge:  DefaultMinWordsForMerging,
		StripHI:        true,
		StripHIMode:    StripHIModeStandard,
		SkipTranslator: true,
		CreateBackup:   false,
		BackupExt:      ".bak",
	})
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

func TestFixFile_RemovesEmptyMusicLines(t *testing.T) {
	workdir, cleanup, err := run.NewWorkdir("", "test")
	if err != nil {
		t.Fatalf("NewWorkdir: %v", err)
	}
	defer cleanup()

	input := filepath.Join(workdir, "in.srt")
	orig := strings.Join([]string{
		"1",
		"00:00:01,000 --> 00:00:02,000",
		"♪ ♪",
		"",
		"2",
		"00:00:03,000 --> 00:00:04,000",
		"♪ la la ♪",
		"",
		"3",
		"00:00:05,000 --> 00:00:07,000",
		"Hola",
		"♪   ♪",
		"Chau",
		"",
		"",
	}, "\n")
	if err := os.WriteFile(input, []byte(orig), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	expected := strings.Join([]string{
		"1",
		"00:00:03,000 --> 00:00:04,000",
		"♪ la la ♪",
		"",
		"2",
		"00:00:05,000 --> 00:00:07,000",
		"Hola",
		"Chau",
		"",
		"",
	}, "\n")

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
	if string(b) != expected {
		t.Fatalf("output mismatch\nexpected:\n%s\n\nactual:\n%s", expected, string(b))
	}
}

func TestFixFile_EmptyOutput_FallsBackToInputContentForAlternateOutput(t *testing.T) {
	workdir, cleanup, err := run.NewWorkdir("", "test")
	if err != nil {
		t.Fatalf("NewWorkdir: %v", err)
	}
	defer cleanup()

	// A single subtitle whose only content is a bare dash.
	// With StripHI enabled the dash is removed, leaving no subtitles at all.
	input := filepath.Join(workdir, "in.srt")
	output := filepath.Join(workdir, "out.srt")
	orig := "1\n00:00:00,000 --> 00:00:01,000\n-\n\n"
	if err := os.WriteFile(input, []byte(orig), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	res, err := Run(context.Background(), Options{
		InputPath:     input,
		OutputPath:    output,
		DryRun:        false,
		WorkDir:       workdir,
		MaxLineLength: DefaultMaxLineLength,
		MinWordsMerge: DefaultMinWordsForMerging,
		StripHI:       true,
		StripHIMode:   StripHIModeSafe,
		CreateBackup:  false,
		BackupExt:     ".bak",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.WasEmpty {
		t.Fatalf("expected WasEmpty=true, got false")
	}
	if res.WrittenPath != output {
		t.Fatalf("expected WrittenPath to be output path; got %q", res.WrittenPath)
	}

	// Original file must be intact.
	b, err := os.ReadFile(input)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(b) != orig {
		t.Fatalf("original file was modified:\nwant: %q\ngot:  %q", orig, string(b))
	}

	outB, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("ReadFile output: %v", err)
	}
	if string(outB) != orig {
		t.Fatalf("output file mismatch:\nwant: %q\ngot:  %q", orig, string(outB))
	}

	// No backup should have been created either.
	if _, statErr := os.Stat(input + ".bak"); statErr == nil {
		t.Fatalf("unexpected backup file created")
	}
}

func TestIsDecorativeLine(t *testing.T) {
	tests := []struct {
		name string
		line string
		want bool
	}{
		{name: "single-dash", line: "-", want: true},
		{name: "many-dashes-with-spaces", line: "- - -", want: true},
		{name: "many-colons", line: ":::", want: true},
		{name: "ellipsis", line: "...", want: true},
		{name: "many-underscores", line: "____", want: true},
		{name: "single-treble-note", line: "♪", want: true},
		{name: "many-treble-notes", line: "♪ ♪ ♪", want: true},
		{name: "single-beamed-note", line: "♫", want: true},
		{name: "many-beamed-notes", line: "♫ ♫", want: true},
		{name: "mixed-symbols", line: "-:_", want: false},
		{name: "mixed-music-symbols", line: "♪ ♫", want: false},
		{name: "dash-with-text", line: "- Hello", want: false},
		{name: "speaker-with-colon", line: "JOHN:", want: false},
		{name: "letters-only", line: "Hola", want: false},
		{name: "empty", line: "   ", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isDecorativeLine(tc.line)
			if got != tc.want {
				t.Fatalf("isDecorativeLine(%q)=%v, want %v", tc.line, got, tc.want)
			}
		})
	}
}

func TestFixFile_RemovesDecorativeLines(t *testing.T) {
	workdir, cleanup, err := run.NewWorkdir("", "test")
	if err != nil {
		t.Fatalf("NewWorkdir: %v", err)
	}
	defer cleanup()

	input := filepath.Join(workdir, "in.srt")
	orig := strings.Join([]string{
		"1",
		"00:00:01,000 --> 00:00:02,000",
		"-",
		"",
		"2",
		"00:00:03,000 --> 00:00:04,000",
		"...",
		"",
		"3",
		"00:00:05,000 --> 00:00:06,000",
		": : :",
		"",
		"4",
		"00:00:07,000 --> 00:00:08,000",
		"Hola",
		"____",
		"Chau",
		"",
		"5",
		"00:00:09,000 --> 00:00:10,000",
		"- Thank you.",
		"",
		"",
	}, "\n")
	if err := os.WriteFile(input, []byte(orig), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	expected := strings.Join([]string{
		"1",
		"00:00:07,000 --> 00:00:08,000",
		"Hola",
		"Chau",
		"",
		"2",
		"00:00:09,000 --> 00:00:10,000",
		"- Thank you.",
		"",
		"",
	}, "\n")

	res, err := Run(context.Background(), Options{
		InputPath:      input,
		OutputPath:     "",
		DryRun:         true,
		WorkDir:        workdir,
		MaxLineLength:  DefaultMaxLineLength,
		MinWordsMerge:  DefaultMinWordsForMerging,
		SkipTranslator: true,
		CreateBackup:   false,
		BackupExt:      ".bak",
	})
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

func TestFixFile_StripHI_Safe_RemovesClosedMultilineSquareCue_AndPreservesDialogue(t *testing.T) {
	workdir, cleanup, err := run.NewWorkdir("", "test")
	if err != nil {
		t.Fatalf("NewWorkdir: %v", err)
	}
	defer cleanup()

	input := filepath.Join(workdir, "in.srt")
	orig := strings.Join([]string{
		"1",
		"00:00:01,000 --> 00:00:02,000",
		"[overlapping radio chatter",
		"continues]",
		"",
		"2",
		"00:00:03,000 --> 00:00:04,000",
		"Take cover!",
		"[over radio",
		"continues]",
		"Now!",
		"",
		"",
	}, "\n")
	if err := os.WriteFile(input, []byte(orig), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	expected := strings.Join([]string{
		"1",
		"00:00:03,000 --> 00:00:04,000",
		"Take cover!",
		"Now!",
		"",
		"",
	}, "\n")

	res, err := Run(context.Background(), Options{
		InputPath:      input,
		OutputPath:     "",
		DryRun:         true,
		WorkDir:        workdir,
		MaxLineLength:  DefaultMaxLineLength,
		MinWordsMerge:  DefaultMinWordsForMerging,
		StripHI:        true,
		StripHIMode:    StripHIModeSafe,
		SkipTranslator: true,
		CreateBackup:   false,
		BackupExt:      ".bak",
	})
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

func TestFixFile_StripHI_SafePlus_RemovesClosedMultilineParenCue(t *testing.T) {
	workdir, cleanup, err := run.NewWorkdir("", "test")
	if err != nil {
		t.Fatalf("NewWorkdir: %v", err)
	}
	defer cleanup()

	input := filepath.Join(workdir, "in.srt")
	orig := strings.Join([]string{
		"1",
		"00:00:01,000 --> 00:00:02,000",
		"(overlapping radio chatter",
		"continues)",
		"",
		"2",
		"00:00:03,000 --> 00:00:04,000",
		"Go, go, go!",
		"",
		"",
	}, "\n")
	if err := os.WriteFile(input, []byte(orig), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	expected := strings.Join([]string{
		"1",
		"00:00:03,000 --> 00:00:04,000",
		"Go, go, go!",
		"",
		"",
	}, "\n")

	res, err := Run(context.Background(), Options{
		InputPath:      input,
		OutputPath:     "",
		DryRun:         true,
		WorkDir:        workdir,
		MaxLineLength:  DefaultMaxLineLength,
		MinWordsMerge:  DefaultMinWordsForMerging,
		StripHI:        true,
		StripHIMode:    StripHIModeSafePlus,
		SkipTranslator: true,
		CreateBackup:   false,
		BackupExt:      ".bak",
	})
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

func TestFixFile_StripHI_SafePlus_MixedDelimiterMultilineCue_IsPreserved(t *testing.T) {
	workdir, cleanup, err := run.NewWorkdir("", "test")
	if err != nil {
		t.Fatalf("NewWorkdir: %v", err)
	}
	defer cleanup()

	input := filepath.Join(workdir, "in.srt")
	orig := strings.Join([]string{
		"1",
		"00:00:01,000 --> 00:00:02,000",
		"(overlapping radio chatter",
		"continues]",
		"",
		"",
	}, "\n")
	if err := os.WriteFile(input, []byte(orig), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	res, err := Run(context.Background(), Options{
		InputPath:      input,
		OutputPath:     "",
		DryRun:         true,
		WorkDir:        workdir,
		MaxLineLength:  DefaultMaxLineLength,
		MinWordsMerge:  DefaultMinWordsForMerging,
		StripHI:        true,
		StripHIMode:    StripHIModeSafePlus,
		SkipTranslator: true,
		CreateBackup:   false,
		BackupExt:      ".bak",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	b, err := os.ReadFile(res.WrittenPath)
	if err != nil {
		t.Fatalf("ReadFile output: %v", err)
	}
	if string(b) != orig {
		t.Fatalf("mixed delimiters should be preserved\nexpected:\n%s\n\nactual:\n%s", orig, string(b))
	}
}

func TestFixFile_StripHI_Safe_UnclosedMultilineCue_IsPreserved(t *testing.T) {
	workdir, cleanup, err := run.NewWorkdir("", "test")
	if err != nil {
		t.Fatalf("NewWorkdir: %v", err)
	}
	defer cleanup()

	input := filepath.Join(workdir, "in.srt")
	orig := strings.Join([]string{
		"1",
		"00:00:01,000 --> 00:00:02,000",
		"[overlapping radio chatter",
		"continues without closure",
		"",
		"",
	}, "\n")
	if err := os.WriteFile(input, []byte(orig), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	res, err := Run(context.Background(), Options{
		InputPath:      input,
		OutputPath:     "",
		DryRun:         true,
		WorkDir:        workdir,
		MaxLineLength:  DefaultMaxLineLength,
		MinWordsMerge:  DefaultMinWordsForMerging,
		StripHI:        true,
		StripHIMode:    StripHIModeSafe,
		SkipTranslator: true,
		CreateBackup:   false,
		BackupExt:      ".bak",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	b, err := os.ReadFile(res.WrittenPath)
	if err != nil {
		t.Fatalf("ReadFile output: %v", err)
	}
	if string(b) != orig {
		t.Fatalf("unclosed multiline cue should be preserved\nexpected:\n%s\n\nactual:\n%s", orig, string(b))
	}
}

func TestFixFile_StripHI_SafePlus_RemovesClosedMultilineBraceCue(t *testing.T) {
	workdir, cleanup, err := run.NewWorkdir("", "test")
	if err != nil {
		t.Fatalf("NewWorkdir: %v", err)
	}
	defer cleanup()

	input := filepath.Join(workdir, "in.srt")
	orig := strings.Join([]string{
		"1",
		"00:00:01,000 --> 00:00:02,000",
		"{overlapping radio chatter",
		"continues}",
		"",
		"2",
		"00:00:03,000 --> 00:00:04,000",
		"Move!",
		"{over radio",
		"continues}",
		"Now!",
		"",
		"",
	}, "\n")
	if err := os.WriteFile(input, []byte(orig), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	expected := strings.Join([]string{
		"1",
		"00:00:03,000 --> 00:00:04,000",
		"Move!",
		"Now!",
		"",
		"",
	}, "\n")

	res, err := Run(context.Background(), Options{
		InputPath:      input,
		OutputPath:     "",
		DryRun:         true,
		WorkDir:        workdir,
		MaxLineLength:  DefaultMaxLineLength,
		MinWordsMerge:  DefaultMinWordsForMerging,
		StripHI:        true,
		StripHIMode:    StripHIModeSafePlus,
		SkipTranslator: true,
		CreateBackup:   false,
		BackupExt:      ".bak",
	})
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
