package srt

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Subtitle struct {
	Idx      int
	FromTime time.Duration
	ToTime   time.Duration
	Text     string
}

var timeFramePattern = regexp.MustCompile(`(\d+):(\d+):(\d+),(\d+) --> (\d+):(\d+):(\d+),(\d+)`)

func getDuration(parts []string) time.Duration {
	hour, _ := strconv.Atoi(parts[0])
	minute, _ := strconv.Atoi(parts[1])
	second, _ := strconv.Atoi(parts[2])
	millisecond, _ := strconv.Atoi(parts[3])
	return time.Millisecond*time.Duration(millisecond) +
		time.Second*time.Duration(second) +
		time.Minute*time.Duration(minute) +
		time.Hour*time.Duration(hour)
}

func formatDuration(duration time.Duration) string {
	hour := duration / time.Hour
	duration -= hour * time.Hour
	minute := duration / time.Minute
	duration -= minute * time.Minute
	second := duration / time.Second
	duration -= second * time.Second
	millisecond := duration / time.Millisecond
	return fmt.Sprintf(`%02d:%02d:%02d,%03d`, hour, minute, second, millisecond)
}

func CleanText(text string) string { return strings.Trim(text, "\n ") }

func ReadOne(scanner *bufio.Scanner) (*Subtitle, error) {
	if !scanner.Scan() {
		return nil, nil
	}
	idxRaw := scanner.Text()
	idx, err := strconv.Atoi(idxRaw)
	if err != nil {
		return nil, errors.New("invalid subtitle index")
	}
	if !scanner.Scan() {
		return nil, errors.New("could not find subtitle timing")
	}
	timing := timeFramePattern.FindStringSubmatch(scanner.Text())
	if timing == nil {
		return nil, errors.New("invalid subtitle timing")
	}
	fromTime := getDuration(timing[1:5])
	toTime := getDuration(timing[5:9])
	if !scanner.Scan() {
		return nil, errors.New("could not find subtitle text")
	}
	content := scanner.Text()
	for scanner.Scan() && scanner.Text() != "" {
		content += "\n" + scanner.Text()
	}
	content = CleanText(content)
	return &Subtitle{Idx: idx, FromTime: fromTime, ToTime: toTime, Text: content}, nil
}

func ReadAll(r io.Reader) ([]*Subtitle, error) {
	scanner := bufio.NewScanner(r)
	var subs []*Subtitle
	for {
		s, err := ReadOne(scanner)
		if err != nil {
			return nil, err
		}
		if s == nil {
			break
		}
		subs = append(subs, s)
	}
	return subs, nil
}

func WriteOne(w io.Writer, subtitle *Subtitle, idx *int) error {
	_, err := fmt.Fprint(w,
		*idx, "\n",
		formatDuration(subtitle.FromTime), " --> ", formatDuration(subtitle.ToTime), "\n",
		CleanText(subtitle.Text), "\n\n")
	*idx++
	return err
}

func WriteAll(w io.Writer, subs []*Subtitle) error {
	idx := 1
	for _, s := range subs {
		if err := WriteOne(w, s, &idx); err != nil {
			return err
		}
	}
	return nil
}

// Sort sorts subtitles in-place by FromTime; if equal, by ToTime; if still equal, by Idx.
func Sort(subtitles []*Subtitle) {
	sort.Slice(subtitles, func(i, j int) bool {
		if subtitles[i].FromTime != subtitles[j].FromTime {
			return subtitles[i].FromTime < subtitles[j].FromTime
		}
		if subtitles[i].ToTime != subtitles[j].ToTime {
			return subtitles[i].ToTime < subtitles[j].ToTime
		}
		return subtitles[i].Idx < subtitles[j].Idx
	})
}

// ValidateSequentialIdx ensures subtitle indexes start at 1 and are sequential by slice order.
func ValidateSequentialIdx(subtitles []*Subtitle) error {
	for i, s := range subtitles {
		if s == nil {
			return fmt.Errorf("nil subtitle at position %d", i+1)
		}
		expected := i + 1
		if s.Idx != expected {
			return fmt.Errorf("invalid subtitle index at position %d: expected %d, got %d", i+1, expected, s.Idx)
		}
	}
	return nil
}

// Reindex updates subtitle indexes in-place to be sequential starting at 1.
func Reindex(subtitles []*Subtitle) {
	for i, s := range subtitles {
		if s == nil {
			continue
		}
		s.Idx = i + 1
	}
}
