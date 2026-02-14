package fix

import (
	"strings"
	"unicode"

	"github.com/adrianmusante/subtitle-tools/internal/srt"
)

type subtitleTokenKind int

type subtitleToken struct {
	kind    subtitleTokenKind
	raw     string
	tagName string
	tagType subtitleTagType
	remove  bool
}

type subtitleTagType int

const (
	subtitleTokenText subtitleTokenKind = iota
	subtitleTokenTag
)

const (
	subtitleTagOpen subtitleTagType = iota
	subtitleTagClose
	subtitleTagSelf
)

func isHtmlTagLine(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">")
}

func stripSubtitleStyles(text string) string {
	tokens := tokenizeSubtitleText(text)
	if !tokensContainTags(tokens) {
		return text
	}

	var stack []int
	for i := range tokens {
		tok := &tokens[i]
		if tok.kind != subtitleTokenTag {
			continue
		}
		switch tok.tagType {
		case subtitleTagSelf:
			tok.remove = true
		case subtitleTagOpen:
			stack = append(stack, i)
		case subtitleTagClose:
			if len(stack) == 0 {
				continue
			}
			last := stack[len(stack)-1]
			if strings.EqualFold(tokens[last].tagName, tok.tagName) {
				tokens[last].remove = true
				tok.remove = true
				stack = stack[:len(stack)-1]
			}
		}
	}

	var b strings.Builder
	changed := false
	for i, tok := range tokens {
		if tok.kind == subtitleTokenText {
			b.WriteString(tok.raw)
			continue
		}
		if !tok.remove {
			b.WriteString(tok.raw)
			continue
		}
		changed = true
		if tok.tagName == "br" {
			if hasTextBeforeNewline(tokens, i) {
				b.WriteString("\n")
			}
			continue
		}
	}

	if !changed {
		return text
	}

	lines := strings.Split(b.String(), "\n")
	var kept []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		kept = append(kept, trimmed)
	}
	return srt.CleanText(strings.Join(kept, "\n"))
}

func tokensContainTags(tokens []subtitleToken) bool {
	for _, tok := range tokens {
		if tok.kind == subtitleTokenTag {
			return true
		}
	}
	return false
}

func tokenizeSubtitleText(text string) []subtitleToken {
	var tokens []subtitleToken
	for i := 0; i < len(text); {
		if text[i] != '<' {
			next := strings.IndexByte(text[i:], '<')
			if next == -1 {
				tokens = append(tokens, subtitleToken{kind: subtitleTokenText, raw: text[i:]})
				break
			}
			tokens = append(tokens, subtitleToken{kind: subtitleTokenText, raw: text[i : i+next]})
			i += next
			continue
		}
		end := strings.IndexByte(text[i:], '>')
		if end == -1 {
			tokens = append(tokens, subtitleToken{kind: subtitleTokenText, raw: text[i:]})
			break
		}
		end += i
		raw := text[i : end+1]
		name, tagType, ok := parseSubtitleTag(raw)
		if !ok {
			tokens = append(tokens, subtitleToken{kind: subtitleTokenText, raw: raw})
		} else {
			tokens = append(tokens, subtitleToken{kind: subtitleTokenTag, raw: raw, tagName: name, tagType: tagType})
		}
		i = end + 1
	}
	return tokens
}

func parseSubtitleTag(raw string) (string, subtitleTagType, bool) {
	if len(raw) < 3 || raw[0] != '<' || raw[len(raw)-1] != '>' {
		return "", subtitleTagOpen, false
	}
	inner := strings.TrimSpace(raw[1 : len(raw)-1])
	if inner == "" {
		return "", subtitleTagOpen, false
	}
	if strings.HasPrefix(inner, "!") || strings.HasPrefix(inner, "?") {
		return "", subtitleTagOpen, false
	}
	closing := false
	if strings.HasPrefix(inner, "/") {
		closing = true
		inner = strings.TrimSpace(inner[1:])
	}
	selfClosing := false
	if strings.HasSuffix(inner, "/") {
		selfClosing = true
		inner = strings.TrimSpace(inner[:len(inner)-1])
	}
	fields := strings.Fields(inner)
	if len(fields) == 0 {
		return "", subtitleTagOpen, false
	}
	name := fields[0]
	if !isValidSubtitleTagName(name) {
		return "", subtitleTagOpen, false
	}
	name = strings.ToLower(name)
	if name == "br" && !closing {
		return name, subtitleTagSelf, true
	}
	if closing {
		if selfClosing {
			return "", subtitleTagOpen, false
		}
		return name, subtitleTagClose, true
	}
	if selfClosing {
		return name, subtitleTagSelf, true
	}
	return name, subtitleTagOpen, true
}

func isValidSubtitleTagName(name string) bool {
	for i, r := range name {
		if i == 0 {
			if !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == ':' || r == '_') {
			return false
		}
	}
	return name != ""
}

func hasTextBeforeNewline(tokens []subtitleToken, startIdx int) bool {
	for i := startIdx + 1; i < len(tokens); i++ {
		tok := tokens[i]
		if tok.kind == subtitleTokenTag {
			if tok.remove {
				continue
			}
			continue
		}
		if tok.raw == "" {
			continue
		}
		if idx := strings.IndexByte(tok.raw, '\n'); idx != -1 {
			segment := tok.raw[:idx]
			return strings.TrimSpace(segment) != ""
		}
		if strings.TrimSpace(tok.raw) != "" {
			return true
		}
	}
	return false
}
