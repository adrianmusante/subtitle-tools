package srt

import "testing"

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
