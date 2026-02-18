package fix

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/adrianmusante/subtitle-tools/internal/fs"
	"github.com/adrianmusante/subtitle-tools/internal/run"
	"github.com/adrianmusante/subtitle-tools/internal/srt"
)

const DefaultMinWordsForMerging = 3
const DefaultMaxLineLength = 70
const DefaultMaxLinesPerSubtitle = 6

// DefaultMinSubtitleDurationForDedup is the max duration to consider a subtitle
// "super-short" and eligible for deduplication/merge if it repeats previous text.
const DefaultMinSubtitleDurationForDedup = 150 * time.Millisecond

var translatorPattern = regexp.MustCompile(`(?i)traductor|traducción|traduccion|translate|translator`)

var ErrSubtitlesOutOfOrder = errors.New("subtitles are out of order")

type Options struct {
	InputPath  string
	OutputPath string
	DryRun     bool
	WorkDir    string

	MaxLineLength int
	MinWordsMerge int

	StripStyle     bool
	SkipTranslator bool
	CreateBackup   bool
	BackupExt      string
}

type Result struct {
	WrittenPath string
}

func Run(ctx context.Context, opts Options) (Result, error) {
	_ = ctx
	if opts.InputPath == "" {
		return Result{}, errors.New("input path is required")
	}
	if opts.MaxLineLength <= 0 {
		opts.MaxLineLength = DefaultMaxLineLength
	}
	if opts.MinWordsMerge <= 0 {
		opts.MinWordsMerge = DefaultMinWordsForMerging
	}
	if opts.CreateBackup && opts.BackupExt == "" {
		return Result{}, errors.New("backup ext is required")
	}
	if opts.WorkDir == "" {
		return Result{}, errors.New("workdir is required (create one with run.NewWorkdir)")
	}

	slog.Info("fixing subtitles file", "input_path", opts.InputPath)

	namer := run.NewTempNamer(opts.WorkDir, opts.InputPath)

	tmpOutputPath, err := mergeSubtitles(opts.InputPath, opts, namer)
	if err != nil {
		if !errors.Is(err, ErrSubtitlesOutOfOrder) {
			return Result{}, err
		}
		slog.Warn("Subtitles out of order. Trying to sort and remerge.")
		// Attempt sort + remerge
		sortedPath, err2 := sortSubtitles(tmpOutputPath, namer)
		if err2 != nil {
			return Result{}, fmt.Errorf("out of order; sorting failed: %w", err2)
		}
		mergedSortedFilePath, err3 := mergeSubtitles(sortedPath, opts, namer)
		if err3 != nil {
			return Result{}, fmt.Errorf("out of order; remerge failed: %w", err3)
		}
		tmpOutputPath = mergedSortedFilePath
	}

	outputPath := opts.OutputPath
	if opts.DryRun {
		// In dry-run, always write to temp file.
		outputPath = namer.Step("output")
	} else if outputPath == "" {
		// Non-dry-run default is in-place overwrite.
		outputPath = opts.InputPath
	}

	// If the destination already exists and has the same content as what we
	// generated, don't overwrite it (avoids unnecessary file replacement / trash).
	outputEquals, err := fs.FilesEqual(outputPath, tmpOutputPath)
	if outputEquals {
		slog.Info("output identical to existing file; not overwriting", "path", outputPath)
	} else {
		// If output overwrites input, do atomic-ish replace with optional backup.
		if opts.CreateBackup && fs.SameFilePath(outputPath, opts.InputPath) {
			backupFilePath := opts.InputPath + opts.BackupExt
			_ = os.Remove(backupFilePath)
			if err := fs.RenameOrMove(opts.InputPath, backupFilePath); err != nil {
				return Result{}, err
			}
		}
		if err := fs.RenameOrMove(tmpOutputPath, outputPath); err != nil {
			return Result{}, err
		}
	}

	return Result{WrittenPath: outputPath}, nil
}

func isContinueLine(s string) bool {
	if len(s) == 0 {
		return true
	}
	r := []rune(s)[0]
	return r == '&' || r == ',' || unicode.IsLower(r)
}

func isEndLine(s string) bool {
	if len(s) == 0 {
		return false
	}
	runes := []rune(s)
	r := runes[len(runes)-1]
	return r == '.' || r == '>'
}

func normalizeSubtitleText(text string, opts Options) string {
	text = srt.CleanText(text)
	if opts.StripStyle {
		text = stripSubtitleStyles(text)
	}
	return srt.CleanText(text)
}

func mergeShortLines(text string, minWords int, maxLineLen int) string {
	lines := strings.Split(text, "\n")
	var merged []string
	var buffer string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !isHtmlTagLine(line) &&
			len(strings.Fields(line)) <= minWords &&
			(buffer == "" || (isContinueLine(line) && !isEndLine(buffer))) {
			var candidate string
			if len(buffer) > 0 {
				candidate = buffer + " " + line
			} else {
				candidate = line
			}
			if len(candidate) >= maxLineLen {
				if len(buffer) > 0 {
					merged = append(merged, buffer)
				}
				buffer = line
			} else {
				buffer = candidate
			}
		} else {
			if len(buffer) > 0 {
				merged = append(merged, buffer)
			}
			buffer = line
		}
	}
	if len(buffer) > 0 {
		merged = append(merged, buffer)
	}
	return srt.CleanText(strings.Join(merged, "\n"))
}

func wrapSubtitleLines(text string, maxLen int) string {
	lines := strings.Split(text, "\n")
	var result []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if isHtmlTagLine(line) {
			result = append(result, line)
			continue
		}
		words := strings.Fields(line)
		var currentLine string
		var currentLen int

		for _, word := range words {
			extra := 0
			if currentLen > 0 {
				extra = 1
			}
			if currentLen+len(word)+extra > maxLen {
				result = append(result, currentLine)
				currentLine = word
				currentLen = len(word)
			} else {
				if currentLen > 0 {
					currentLine += " "
					currentLen++
				}
				currentLine += word
				currentLen += len(word)
			}
		}
		if currentLen > 0 {
			result = append(result, currentLine)
		}
	}
	return srt.CleanText(strings.Join(result, "\n"))
}

func mergeSubtitles(inputPath string, opts Options, namer run.TempNamer) (string, error) {
	if inputPath == "" {
		return "", errors.New("empty file path")
	}
	outputTmpPath := namer.Step("merge")

	f, err := os.Open(inputPath)
	if err != nil {
		return "", err
	}
	defer fs.CloseOrLog(f, inputPath)

	out, err := os.Create(outputTmpPath)
	if err != nil {
		return "", err
	}
	defer fs.CloseOrLog(out, outputTmpPath)

	scanner := bufio.NewScanner(f)

	newIdx := 1
	var lastSubtitle *srt.Subtitle
	var processed []*srt.Subtitle
	outOfOrder := false

	for {
		subtitle, err := srt.ReadOne(scanner)
		if err != nil {
			return outputTmpPath, err
		}

		if subtitle != nil { // Normalize text early to improve deduplication and translator skipping.
			normalizedText := normalizeSubtitleText(subtitle.Text, opts)
			if normalizedText != subtitle.Text {
				subtitle.Text = normalizedText
			}
		}

		if lastSubtitle == nil {
			if subtitle != nil && opts.SkipTranslator && translatorPattern.MatchString(subtitle.Text) {
				slog.Debug("skipping translator subtitle", "subtitle", subtitle)
				continue
			}
		} else {
			if subtitle != nil {
				if len(subtitle.Text) == 0 {
					continue
				}
				if subtitle.FromTime > subtitle.ToTime {
					continue
				}
				duplicate := false
				for _, s := range processed {
					if subtitle.Text == s.Text && subtitle.FromTime == s.FromTime && subtitle.ToTime == s.ToTime {
						duplicate = true
						break
					}
				}
				if duplicate {
					continue
				}
				processed = append(processed, &srt.Subtitle{FromTime: subtitle.FromTime, ToTime: subtitle.ToTime, Text: subtitle.Text})

				if subtitle.ToTime < lastSubtitle.FromTime { // Subtitles may not be synchronized when translations or descriptions are added that appear on the screen (tag: hi).
					outOfOrder = true
				} else { // Check for overlapping subtitles
					if subtitle.FromTime-lastSubtitle.ToTime < 0 {
						// If the next subtitle overlaps the previous one, merge the text and extend the end time.
						lastSubtitle.Text = strings.Join([]string{lastSubtitle.Text, subtitle.Text}, "\n")
						lastSubtitle.ToTime = subtitle.ToTime
						continue
					}
					// Skip super-short subtitles that mostly repeat the previous text; extend the previous subtitle instead.
					if subtitle.ToTime-subtitle.FromTime < DefaultMinSubtitleDurationForDedup && strings.Contains(lastSubtitle.Text, subtitle.Text) {
						lastSubtitle.ToTime = subtitle.ToTime
						continue
					}
				}
			}

			lastSubtitle.Text = srt.CleanText(lastSubtitle.Text)
			if len(lastSubtitle.Text) > 0 {
				lastSubtitle.Text = wrapSubtitleLines(lastSubtitle.Text, opts.MaxLineLength)
				lines := strings.Split(lastSubtitle.Text, "\n")
				if len(lines) > DefaultMaxLinesPerSubtitle {
					lastSubtitle.Text = mergeShortLines(lastSubtitle.Text, opts.MinWordsMerge, opts.MaxLineLength)
				}
				if err := srt.WriteOne(out, lastSubtitle, &newIdx); err != nil {
					return outputTmpPath, err
				}
			}
		}

		if subtitle == nil {
			break
		}
		lastSubtitle = subtitle
	}

	if outOfOrder {
		return outputTmpPath, ErrSubtitlesOutOfOrder
	}
	return outputTmpPath, nil
}

func sortSubtitles(inputPath string, namer run.TempNamer) (string, error) {
	if inputPath == "" {
		return "", errors.New("empty file path")
	}
	outputPath := namer.Step("sort")

	f, err := os.Open(inputPath)
	if err != nil {
		return "", err
	}
	defer fs.CloseOrLog(f, inputPath)

	subtitles, err := srt.ReadAll(f)
	if err != nil {
		return outputPath, err
	}

	srt.Sort(subtitles)

	out, err := os.Create(outputPath)
	if err != nil {
		return "", err
	}
	defer fs.CloseOrLog(out, outputPath)

	err = srt.WriteAll(out, subtitles)
	if err != nil {
		return outputPath, err
	}

	return outputPath, nil
}
