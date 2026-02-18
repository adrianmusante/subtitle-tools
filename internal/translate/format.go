package translate

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Format contract for the LLM:
// We send NDJSON: one JSON object per line.
// Example:
//   {"idx":1,"text":"Hello"}
//   {"idx":2,"text":"Line 1\nLine 2"}
// Output must be exactly the same shape.

// wire item: one JSON object per line (NDJSON)
// Example:
//   {"idx":196,"text":"Line 1\nLine 2"}
// The JSON string can contain newlines via standard JSON escaping; callers see real "\n".

const AbbreviationMax = 250

var errNoTranslatedLinesParsed = errors.New("no translated lines parsed")

type wireItem struct {
	Idx  int    `json:"idx"`
	Text string `json:"text"`
}

func FormatOneForTranslation(idx int, text string) ([]byte, error) {
	item := wireItem{Idx: idx, Text: strings.ReplaceAll(text, "\r\n", "\n")}
	return json.Marshal(item)
}

func FormatForTranslation(idxs []int, texts []string) (string, error) {
	if len(idxs) != len(texts) {
		return "", errors.New("idxs and texts length mismatch")
	}
	var b strings.Builder
	for i := range idxs {
		if idxs[i] <= 0 {
			return "", errors.New("idx must be positive")
		}
		if i > 0 {
			b.WriteByte('\n')
		}
		enc, err := FormatOneForTranslation(idxs[i], texts[i])
		if err != nil {
			return "", err
		}
		b.Write(enc)
	}
	return b.String(), nil
}

type ParsedLine struct {
	Idx  int
	Text string
}

func ParseTranslatedLines(out string) ([]ParsedLine, error) {
	out = strings.ReplaceAll(out, "\r\n", "\n")
	out = stripCodeFences(out)
	out = strings.TrimSpace(out)
	if out == "" {
		return nil, errors.New("empty translation output")
	}

	if strings.HasPrefix(out, "[") {
		return parseWireItemsJSONArray(out)
	}

	// Robust mode: extract balanced JSON objects and unmarshal each.
	// This tolerates whitespace, code fences already stripped, and even cases where
	// objects are not strictly one-per-line.
	if res, err := parseWireItemsByBraces(out); err == nil {
		return res, nil
	}

	// Strict/diagnostic mode: try line-by-line NDJSON to return a more precise error
	// message when the output looks like NDJSON but one line is broken.
	if res, err := parseWireItemsByLines(out); err == nil {
		return res, nil
	}

	// Safer salvage: try to repair only the broken NDJSON lines. This minimizes the
	// risk of heuristics over unrelated content.
	if res, salvaged, err := parseWireItemsByLinesWithRepair(out); err == nil {
		logSalvagedNDJSONLines(salvaged)
		return res, nil
	}

	// Last-resort mode: attempt to salvage items when the LLM returned almost-JSON
	// but broke string escaping (most commonly: unescaped double quotes inside text).
	if res, err := parseWireItemsByRepairingText(out); err == nil {
		slog.Debug("salvaged invalid json output by repairing extracted json objects")
		return res, nil
	}

	// If we got here, return the strict error (it tends to be most actionable).
	_, err := parseWireItemsByLines(out)
	return nil, err
}

func parseWireItemsJSONArray(trim string) ([]ParsedLine, error) {
	var items []wireItem
	if err := json.Unmarshal([]byte(trim), &items); err != nil {
		return nil, fmt.Errorf("invalid json array: %w", err)
	}
	res := make([]ParsedLine, 0, len(items))
	for _, it := range items {
		if it.Idx <= 0 {
			return nil, fmt.Errorf("invalid idx in item: %d", it.Idx)
		}
		res = append(res, ParsedLine{Idx: it.Idx, Text: it.Text})
	}
	if len(res) == 0 {
		return nil, errNoTranslatedLinesParsed
	}
	return res, nil
}

func logSalvagedNDJSONLines(salvaged int) {
	if salvaged > 0 {
		slog.Debug("salvaged invalid json lines in translation output", "salvaged", salvaged)
	}
}

func parseWireItemsByLines(trim string) ([]ParsedLine, error) {
	lines := strings.Split(trim, "\n")
	res := make([]ParsedLine, 0, len(lines))
	for lineNo, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var it wireItem
		if err := json.Unmarshal([]byte(line), &it); err != nil {
			return nil, fmt.Errorf("invalid json line %d: %w (line=%q)", lineNo+1, err, abbreviate(line, AbbreviationMax))
		}
		if it.Idx <= 0 {
			return nil, fmt.Errorf("invalid idx in item at line %d: %d (line=%q)", lineNo+1, it.Idx, abbreviate(line, AbbreviationMax))
		}
		res = append(res, ParsedLine{Idx: it.Idx, Text: it.Text})
	}
	if len(res) == 0 {
		return nil, errNoTranslatedLinesParsed
	}
	return res, nil
}

func parseWireItemsByLinesWithRepair(trim string) ([]ParsedLine, int, error) {
	lines := strings.Split(trim, "\n")
	res := make([]ParsedLine, 0, len(lines))
	salvaged := 0
	for lineNo, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var it wireItem
		if err := json.Unmarshal([]byte(line), &it); err == nil {
			if it.Idx <= 0 {
				return nil, salvaged, fmt.Errorf("invalid idx in item at line %d: %d (line=%q)", lineNo+1, it.Idx, abbreviate(line, AbbreviationMax))
			}
			res = append(res, ParsedLine{Idx: it.Idx, Text: it.Text})
			continue
		}

		// Preserve the strict error for diagnostics.
		strictErr := json.Unmarshal([]byte(line), &it)

		idx, text, ok, sErr := extractIdxAndTextBestEffort(line)
		if sErr != nil || !ok || idx <= 0 {
			// Preserve the strict/diagnostic error.
			return nil, salvaged, fmt.Errorf("invalid json line %d: %w (line=%q)", lineNo+1, strictErr, abbreviate(line, AbbreviationMax))
		}
		res = append(res, ParsedLine{Idx: idx, Text: text})
		salvaged++
	}
	if len(res) == 0 {
		return nil, salvaged, errNoTranslatedLinesParsed
	}
	return res, salvaged, nil
}

func parseWireItemsByBraces(s string) ([]ParsedLine, error) {
	objs := extractJSONObjectSegmentsWithOffsets(s)
	if len(objs) == 0 {
		return nil, errNoTranslatedLinesParsed
	}

	res := make([]ParsedLine, 0, len(objs))
	for i, obj := range objs {
		var it wireItem
		if err := json.Unmarshal([]byte(obj.JSON), &it); err != nil {
			return nil, fmt.Errorf("invalid json object #%d at offset %d: %w (obj=%q)", i+1, obj.Start, err, abbreviate(obj.JSON, AbbreviationMax))
		}
		if it.Idx <= 0 {
			return nil, fmt.Errorf("invalid idx in item in object #%d at offset %d: %d", i+1, obj.Start, it.Idx)
		}
		res = append(res, ParsedLine{Idx: it.Idx, Text: it.Text})
	}
	return res, nil
}

type jsonSegment struct {
	Start int
	JSON  string
}

func extractJSONObjectSegmentsWithOffsets(s string) []jsonSegment {
	var res []jsonSegment
	inStr := false
	esc := false
	depth := 0
	start := -1
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inStr {
			if esc {
				esc = false
				continue
			}
			if c == '\\' {
				esc = true
				continue
			}
			if c == '"' {
				inStr = false
			}
			continue
		}

		switch c {
		case '"':
			inStr = true
		case '{':
			if depth == 0 {
				start = i
			}
			depth++
		case '}':
			if depth > 0 {
				depth--
				if depth == 0 && start >= 0 {
					seg := strings.TrimSpace(s[start : i+1])
					res = append(res, jsonSegment{Start: start, JSON: seg})
					start = -1
				}
			}
		}
	}
	return res
}

func abbreviate(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func stripCodeFences(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	if strings.HasPrefix(s, "```") {
		// Remove first fence line.
		if i := strings.Index(s, "\n"); i >= 0 {
			s = s[i+1:]
		}
		// Remove last fence.
		if j := strings.LastIndex(s, "```"); j >= 0 {
			s = s[:j]
		}
	}
	return s
}

func parseWireItemsByRepairingText(s string) ([]ParsedLine, error) {
	segs := extractJSONObjectSegmentsWithOffsets(s)
	if len(segs) == 0 {
		return nil, errNoTranslatedLinesParsed
	}

	res := make([]ParsedLine, 0, len(segs))
	for i, seg := range segs {
		idx, text, ok, err := extractIdxAndTextBestEffort(seg.JSON)
		if err != nil {
			return nil, fmt.Errorf("cannot salvage json object #%d at offset %d: %w (obj=%q)", i+1, seg.Start, err, abbreviate(seg.JSON, AbbreviationMax))
		}
		if !ok {
			return nil, fmt.Errorf("cannot salvage json object #%d at offset %d (obj=%q)", i+1, seg.Start, abbreviate(seg.JSON, AbbreviationMax))
		}
		if idx <= 0 {
			return nil, fmt.Errorf("invalid idx in salvaged item in object #%d at offset %d: %d", i+1, seg.Start, idx)
		}

		// Re-encode as valid JSON to let encoding/json validate the escaping.
		fixed, mErr := json.Marshal(wireItem{Idx: idx, Text: text})
		if mErr != nil {
			return nil, fmt.Errorf("cannot marshal salvaged item in object #%d at offset %d: %w", i+1, seg.Start, mErr)
		}
		var it wireItem
		if uErr := json.Unmarshal(fixed, &it); uErr != nil {
			return nil, fmt.Errorf("cannot unmarshal salvaged item in object #%d at offset %d: %w (fixed=%q)", i+1, seg.Start, uErr, abbreviate(string(fixed), AbbreviationMax))
		}
		res = append(res, ParsedLine{Idx: it.Idx, Text: it.Text})
	}
	if len(res) == 0 {
		return nil, errNoTranslatedLinesParsed
	}
	return res, nil
}

// extractIdxAndTextBestEffort tries to recover idx and text from an object that
// looks like: {"idx":119,"text":"..."}
// but where the text may contain unescaped double quotes.
func extractIdxAndTextBestEffort(obj string) (idx int, text string, ok bool, err error) {
	obj = strings.TrimSpace(obj)
	if obj == "" {
		return 0, "", false, nil
	}

	// Find "idx":<number>
	idxPos := strings.Index(obj, "\"idx\"")
	if idxPos < 0 {
		return 0, "", false, nil
	}
	colon := strings.IndexByte(obj[idxPos:], ':')
	if colon < 0 {
		return 0, "", false, errors.New("missing ':' after idx")
	}
	p := idxPos + colon + 1
	for p < len(obj) && (obj[p] == ' ' || obj[p] == '\t' || obj[p] == '\n' || obj[p] == '\r') {
		p++
	}
	if p >= len(obj) {
		return 0, "", false, errors.New("missing idx value")
	}
	startNum := p
	if obj[p] == '-' {
		p++
	}
	for p < len(obj) && obj[p] >= '0' && obj[p] <= '9' {
		p++
	}
	if p == startNum || (obj[startNum] == '-' && p == startNum+1) {
		return 0, "", false, errors.New("invalid idx number")
	}
	parsedIdx, convErr := strconv.Atoi(strings.TrimSpace(obj[startNum:p]))
	if convErr != nil {
		return 0, "", false, fmt.Errorf("invalid idx: %w", convErr)
	}

	// Find "text":"<string...>
	textKey := strings.Index(obj, "\"text\"")
	if textKey < 0 {
		return 0, "", false, nil
	}
	colon2 := strings.IndexByte(obj[textKey:], ':')
	if colon2 < 0 {
		return 0, "", false, errors.New("missing ':' after text")
	}
	q := textKey + colon2 + 1
	for q < len(obj) && (obj[q] == ' ' || obj[q] == '\t' || obj[q] == '\n' || obj[q] == '\r') {
		q++
	}
	if q >= len(obj) || obj[q] != '"' {
		return 0, "", false, errors.New("text value is not a string")
	}
	q++ // past opening quote

	// Heuristic: the real end of the text string is the quote that is followed by
	// optional whitespace and then either '}' (end of object) OR ',"' (next key).
	// Any other quote is considered part of the text and will be treated as an unescaped quote.
	var raw strings.Builder
	for q < len(obj) {
		c := obj[q]
		if c == '"' {
			k := q + 1
			for k < len(obj) && (obj[k] == ' ' || obj[k] == '\t' || obj[k] == '\n' || obj[k] == '\r') {
				k++
			}
			if k < len(obj) {
				// End of object: ..."}
				if obj[k] == '}' {
					break
				}
				// Next key: ...", "foo": ...  (we only accept comma if immediately followed by a quote)
				if obj[k] == ',' {
					k2 := k + 1
					for k2 < len(obj) && (obj[k2] == ' ' || obj[k2] == '\t' || obj[k2] == '\n' || obj[k2] == '\r') {
						k2++
					}
					if k2 < len(obj) && obj[k2] == '"' {
						break
					}
				}
			}

			// Unescaped quote inside text.
			raw.WriteByte('\\')
			raw.WriteByte('"')
			q++
			continue
		}

		// Preserve existing escapes (\n, \", \uXXXX, etc.) as-is.
		if c == '\\' {
			// Copy backslash and the next byte/rune if present.
			raw.WriteByte('\\')
			q++
			if q >= len(obj) {
				// dangling backslash; keep it literal.
				break
			}
			// If next is part of UTF-8 multibyte, just copy bytes safely.
			if obj[q] < utf8.RuneSelf {
				raw.WriteByte(obj[q])
				q++
				continue
			}
			r, size := utf8.DecodeRuneInString(obj[q:])
			if r == utf8.RuneError && size == 1 {
				raw.WriteByte(obj[q])
				q++
				continue
			}
			raw.WriteRune(r)
			q += size
			continue
		}

		if c < utf8.RuneSelf {
			raw.WriteByte(c)
			q++
			continue
		}
		r, size := utf8.DecodeRuneInString(obj[q:])
		if r == utf8.RuneError && size == 1 {
			raw.WriteByte(c)
			q++
			continue
		}
		raw.WriteRune(r)
		q += size
	}
	if q >= len(obj) {
		return 0, "", false, errors.New("unterminated text string")
	}

	// Unescape the raw JSON string content (with potentially repaired escapes).
	// We can do this by wrapping it in quotes to make it a JSON string.
	wrapped := "\"" + raw.String() + "\""
	var decoded string
	if uErr := json.Unmarshal([]byte(wrapped), &decoded); uErr != nil {
		return 0, "", false, fmt.Errorf("cannot decode text: %w", uErr)
	}

	return parsedIdx, decoded, true, nil
}
