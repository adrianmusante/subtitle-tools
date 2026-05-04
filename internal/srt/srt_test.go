package srt

import (
	"bufio"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestValidateSequentialIdx_OK(t *testing.T) {
	subs := []*Subtitle{{Idx: 1}, {Idx: 2}, {Idx: 3}}
	if err := ValidateSequentialIdx(subs); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateSequentialIdx_StartsAtOne(t *testing.T) {
	subs := []*Subtitle{{Idx: 2}, {Idx: 3}}
	if err := ValidateSequentialIdx(subs); err == nil {
		t.Fatalf("expected error for non-1 start")
	}
}

func TestValidateSequentialIdx_Gap(t *testing.T) {
	subs := []*Subtitle{{Idx: 1}, {Idx: 3}}
	if err := ValidateSequentialIdx(subs); err == nil {
		t.Fatalf("expected error for gap")
	}
}

func TestReindex(t *testing.T) {
	subs := []*Subtitle{{Idx: 10}, {Idx: 20}, {Idx: 30}}
	Reindex(subs)
	if subs[0].Idx != 1 || subs[1].Idx != 2 || subs[2].Idx != 3 {
		t.Fatalf("unexpected indexes after reindex: %d, %d, %d", subs[0].Idx, subs[1].Idx, subs[2].Idx)
	}
}

func TestCleanText_TrimSpace(t *testing.T) {
	cases := []struct {
		name string
		in   string
		out  string
	}{
		{name: "leading_trailing_spaces", in: "  hello  ", out: "hello"},
		{name: "tabs", in: "\thello\t", out: "hello"},
		{name: "newlines", in: "\nhello\n", out: "hello"},
		{name: "mixed_whitespace", in: " \t\nhello\r\n\t ", out: "hello"},
		{name: "internal_whitespace_preserved", in: "  he\tllo\nworld  ", out: "he\tllo\nworld"},
		{name: "trim_each_line", in: "  hello  \n  world  ", out: "hello\nworld"},
		{name: "blank_lines_removed", in: "hello\n\n\nworld", out: "hello\nworld"},
		{name: "whitespace_only_lines_removed", in: "hello\n \t \nworld", out: "hello\nworld"},
		{name: "crlf_lines_trimmed", in: "  hello  \r\n  world  \r\n", out: "hello\nworld"},
		{name: "empty", in: "", out: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CleanText(tc.in); got != tc.out {
				t.Fatalf("unexpected CleanText result: %q", got)
			}
		})
	}
}

func TestReadOne_CRLF(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader("1\r\n00:00:01,000 --> 00:00:02,500\r\n hello \r\n world \r\n\r\n"))

	sub, err := ReadOne(scanner)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if sub == nil {
		t.Fatal("expected subtitle, got nil")
	}
	if sub.Idx != 1 {
		t.Fatalf("unexpected idx: %d", sub.Idx)
	}
	if sub.FromTime != time.Second {
		t.Fatalf("unexpected from time: %v", sub.FromTime)
	}
	if sub.ToTime != 2500*time.Millisecond {
		t.Fatalf("unexpected to time: %v", sub.ToTime)
	}
	if sub.Text != "hello\nworld" {
		t.Fatalf("unexpected text: %q", sub.Text)
	}
}

func TestReadOne_UTF8BOMAndCRLF(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader("\ufeff1\r\n00:00:01,000 --> 00:00:02,500\r\nHello\r\n\r\n"))

	sub, err := ReadOne(scanner)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if sub == nil {
		t.Fatal("expected subtitle, got nil")
	}
	if sub.Idx != 1 {
		t.Fatalf("unexpected idx: %d", sub.Idx)
	}
	if sub.Text != "Hello" {
		t.Fatalf("unexpected text: %q", sub.Text)
	}
}

func TestReadOne_UTF8BOMOnStructuralLineAfterLeadingSpace(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader(" \ufeff1\n00:00:01,000 --> 00:00:02,500\nHello\n\n"))

	sub, err := ReadOne(scanner)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if sub == nil {
		t.Fatal("expected subtitle, got nil")
	}
	if sub.Idx != 1 {
		t.Fatalf("unexpected idx: %d", sub.Idx)
	}
}

func TestReadOne_PreservesUTF8BOMInContent(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader("1\n00:00:01,000 --> 00:00:02,500\n\ufeffHello\n\n"))

	sub, err := ReadOne(scanner)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if sub == nil {
		t.Fatal("expected subtitle, got nil")
	}
	if sub.Text != "\ufeffHello" {
		t.Fatalf("unexpected text: %q", sub.Text)
	}
}

func TestReadOne_WhitespaceOnlyLineInsideCue(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader("1\n00:00:01,000 --> 00:00:02,500\nHello\n   \nWorld\n\n"))

	sub, err := ReadOne(scanner)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if sub == nil {
		t.Fatal("expected subtitle, got nil")
	}
	if sub.Text != "Hello\nWorld" {
		t.Fatalf("unexpected text: %q", sub.Text)
	}

	next, err := ReadOne(scanner)
	if err != nil {
		t.Fatalf("expected no error on next read, got %v", err)
	}
	if next != nil {
		t.Fatalf("expected no next subtitle, got %+v", next)
	}
}

func TestReadOne_MissingCueContentStillReturnsFriendlyError(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader("1\n00:00:01,000 --> 00:00:02,500\n"))

	_, err := ReadOne(scanner)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "could not find subtitle text" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadOne_MissingCueContentBeforeNextCueDoesNotConsumeNextCue(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader("1\n00:00:01,000 --> 00:00:02,500\n\n2\n00:00:03,000 --> 00:00:04,500\nWorld\n\n"))

	sub, err := ReadOne(scanner)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "could not find subtitle text" {
		t.Fatalf("unexpected error: %v", err)
	}
	if sub != nil {
		t.Fatalf("expected nil subtitle on error, got %+v", sub)
	}

	next, err := ReadOne(scanner)
	if err != nil {
		t.Fatalf("expected no error reading next cue, got %v", err)
	}
	if next == nil {
		t.Fatal("expected next subtitle, got nil")
	}
	if next.Idx != 2 {
		t.Fatalf("expected next subtitle idx 2, got %d", next.Idx)
	}
	if next.Text != "World" {
		t.Fatalf("unexpected next subtitle text: %q", next.Text)
	}

	last, err := ReadOne(scanner)
	if err != nil {
		t.Fatalf("expected no error on final read, got %v", err)
	}
	if last != nil {
		t.Fatalf("expected no more subtitles, got %+v", last)
	}
}

func TestReadOne_PropagatesErrTooLongOnFirstCueLine(t *testing.T) {
	longLine := strings.Repeat("A", bufio.MaxScanTokenSize+1)
	input := strings.NewReader("1\n00:00:01,000 --> 00:00:02,500\n" + longLine + "\n\n")
	scanner := bufio.NewScanner(input)

	_, err := ReadOne(scanner)
	if err == nil {
		t.Fatal("expected scanner error, got nil")
	}
	if !errors.Is(err, bufio.ErrTooLong) {
		t.Fatalf("expected bufio.ErrTooLong, got %v", err)
	}
}

func TestReadOne_PropagatesErrTooLongOnLaterCueLine(t *testing.T) {
	longLine := strings.Repeat("B", bufio.MaxScanTokenSize+1)
	input := strings.NewReader("1\n00:00:01,000 --> 00:00:02,500\nshort\n" + longLine + "\n\n")
	scanner := bufio.NewScanner(input)

	_, err := ReadOne(scanner)
	if err == nil {
		t.Fatal("expected scanner error, got nil")
	}
	if !errors.Is(err, bufio.ErrTooLong) {
		t.Fatalf("expected bufio.ErrTooLong, got %v", err)
	}
}

func TestReadAll_CRLF(t *testing.T) {
	input := strings.NewReader("1\r\n00:00:01,000 --> 00:00:02,000\r\nHello\r\n\r\n2\r\n00:00:03,000 --> 00:00:04,000\r\nWorld\r\nAgain\r\n\r\n")

	subs, err := ReadAll(input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(subs) != 2 {
		t.Fatalf("expected 2 subtitles, got %d", len(subs))
	}
	if subs[0].Text != "Hello" {
		t.Fatalf("unexpected first subtitle text: %q", subs[0].Text)
	}
	if subs[1].Text != "World\nAgain" {
		t.Fatalf("unexpected second subtitle text: %q", subs[1].Text)
	}
}

func TestReadAll_MixedLineEndingsAndLeadingBlankLines(t *testing.T) {
	input := strings.NewReader("\r\n\r\n1\r\n00:00:01,000 --> 00:00:02,000\r\nFirst line\r\n\r\n2\n00:00:02,500 --> 00:00:03,500\nSecond line\n\n")

	subs, err := ReadAll(input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(subs) != 2 {
		t.Fatalf("expected 2 subtitles, got %d", len(subs))
	}
	if subs[0].Idx != 1 || subs[1].Idx != 2 {
		t.Fatalf("unexpected indexes: %d, %d", subs[0].Idx, subs[1].Idx)
	}
	if subs[0].Text != "First line" || subs[1].Text != "Second line" {
		t.Fatalf("unexpected texts: %q / %q", subs[0].Text, subs[1].Text)
	}
}

func TestReadAll_UTF8BOMAfterLeadingBlankLines(t *testing.T) {
	input := strings.NewReader("\r\n\r\n\ufeff1\r\n00:00:01,000 --> 00:00:02,000\r\nFirst\r\n\r\n2\r\n00:00:03,000 --> 00:00:04,000\r\nSecond\r\n\r\n")

	subs, err := ReadAll(input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(subs) != 2 {
		t.Fatalf("expected 2 subtitles, got %d", len(subs))
	}
	if subs[0].Idx != 1 || subs[1].Idx != 2 {
		t.Fatalf("unexpected indexes: %d, %d", subs[0].Idx, subs[1].Idx)
	}
}

func TestReadOne_EmptyInputReturnsNilNil(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader(""))

	sub, err := ReadOne(scanner)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if sub != nil {
		t.Fatalf("expected nil subtitle, got %+v", sub)
	}
}

func TestReadAll_PropagatesScannerErrTooLong(t *testing.T) {
	longLine := strings.Repeat("1", bufio.MaxScanTokenSize+1)
	input := strings.NewReader(longLine + "\n00:00:01,000 --> 00:00:02,000\nHello\n\n")

	_, err := ReadAll(input)
	if err == nil {
		t.Fatal("expected scanner error, got nil")
	}
	if !errors.Is(err, bufio.ErrTooLong) {
		t.Fatalf("expected bufio.ErrTooLong, got %v", err)
	}
}
