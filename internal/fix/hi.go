package fix

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/adrianmusante/subtitle-tools/internal/srt"
)

const DefaultStripHIMode = StripHIModeStandard

// Hearing impaired (HI) modes
const (
	StripHIModeSafe         = "safe"
	StripHIModeSafePlus     = "safe-plus"
	StripHIModeStandard     = "standard"
	StripHIModeStandardPlus = "standard-plus"
)

type hiCuePatterns struct {
	onlyLine    *regexp.Regexp
	leading     *regexp.Regexp
	dashLeading *regexp.Regexp
	trailing    *regexp.Regexp
	inlineCue   *regexp.Regexp
}

type hiLine struct {
	text            string
	hadDialogueDash bool
}

type hiLineTransformer func(*hiLine, hiCuePatterns)

type hiModeConfig struct {
	cues         hiCuePatterns
	delimiters   []hiDelimiterPair
	transformers []hiLineTransformer
}

func buildHICuePatterns(includeExtendedCues bool) hiCuePatterns {
	mainCuePattern := `\[[^\[\]]+\]`
	inlineCuePattern := `\[[^\[\]]*\]`
	if includeExtendedCues {
		mainCuePattern = `(?:\[[^\[\]]+\]|\([^()]+\)|\{[^{}]+\})`
		inlineCuePattern = `\[[^\[\]]*\]|\([^()]*\)|\{[^{}]*\}`
	}

	return hiCuePatterns{
		onlyLine:    regexp.MustCompile(`^(?:-\s*)?` + mainCuePattern + `(?:\s*[:\-]\s*)?$`),
		leading:     regexp.MustCompile(`^` + mainCuePattern + `(?:\s*[:\-]\s*|\s+)(.+)$`),
		dashLeading: regexp.MustCompile(`^-\s*` + mainCuePattern + `(?:\s*[:\-]\s*|\s+)(.+)$`),
		trailing:    regexp.MustCompile(`^(.+?)\s+` + mainCuePattern + `$`),
		inlineCue:   regexp.MustCompile(inlineCuePattern),
	}
}

var (
	hiBaseCuePatterns         = buildHICuePatterns(false)
	hiPlusCuePatterns         = buildHICuePatterns(true)
	hiSpeakerPrefixPattern    = regexp.MustCompile(`^[A-Z][A-Z0-9 .'-]{1,30}:\s*`)
	hiLeadingCueArtifact      = regexp.MustCompile(`^[:;]+\s*`)
	hiMultiSpacePattern       = regexp.MustCompile(`\s{2,}`)
	hiSpaceBeforePunctPattern = regexp.MustCompile(`\s+([,.;:!?])`)
	hiSafeTransformers        = []hiLineTransformer{
		hiDropCueOnlyLine,
		hiStripDashLeadingCues,
		hiStripLeadingCues,
		hiStripTrailingCues,
	}
	hiStandardTransformers = []hiLineTransformer{
		hiCaptureDialogueDash,
		hiStripInlineCues,
		hiStripSpeakerPrefix,
		hiNormalizeSpacing,
		hiStripLeadingCueArtifacts,
		hiRestoreDialogueDash,
	}
)

func isValidStripHIMode(mode string) bool {
	return mode == StripHIModeSafe ||
		mode == StripHIModeSafePlus ||
		mode == StripHIModeStandard ||
		mode == StripHIModeStandardPlus
}

func normalizeStripHIMode(mode string) string {
	return strings.ToLower(strings.TrimSpace(mode))
}

type hiDelimiterPair struct {
	open  rune
	close rune
}

var hiBaseDelimiterPairs = []hiDelimiterPair{{open: '[', close: ']'}}
var hiPlusDelimiterPairs = []hiDelimiterPair{{open: '[', close: ']'}, {open: '(', close: ')'}, {open: '{', close: '}'}}

func stripSubtitleHI(text string, mode string) string {
	if mode == "" {
		mode = DefaultStripHIMode
	}
	mode = normalizeStripHIMode(mode)
	return stripSubtitleHIWithConfig(text, buildHIModeConfig(mode))
}

func buildHIModeConfig(mode string) hiModeConfig {
	config := hiModeConfig{
		cues:         hiBaseCuePatterns,
		delimiters:   hiBaseDelimiterPairs,
		transformers: append([]hiLineTransformer(nil), hiSafeTransformers...),
	}

	if mode == StripHIModeSafePlus || mode == StripHIModeStandardPlus {
		config.cues = hiPlusCuePatterns
		config.delimiters = hiPlusDelimiterPairs
	}

	if mode == StripHIModeStandard || mode == StripHIModeStandardPlus {
		config.transformers = append(config.transformers, hiStandardTransformers...)
	}

	return config
}

func stripSubtitleHIWithConfig(text string, config hiModeConfig) string {
	text = stripClosedMultilineHIBlocks(text, config.delimiters)
	text = srt.CleanText(text)
	if text == "" {
		return text
	}

	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))

	for _, rawLine := range lines {
		line := hiLine{text: strings.TrimSpace(rawLine)}
		if line.text == "" {
			continue
		}

		for _, transform := range config.transformers {
			if line.text == "" {
				break
			}
			transform(&line, config.cues)
		}

		if !shouldKeepHILine(line.text, config.cues) {
			continue
		}
		kept = append(kept, line.text)
	}

	return srt.CleanText(strings.Join(kept, "\n"))
}

func stripClosedMultilineHIBlocks(text string, delimiters []hiDelimiterPair) string {
	if text == "" || len(delimiters) == 0 {
		return text
	}

	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		current := runes[i]
		for _, pair := range delimiters {
			if current != pair.open {
				continue
			}

			closeIdx := -1
			hasNewline := false
			for j := i + 1; j < len(runes); j++ {
				if runes[j] == '\n' {
					hasNewline = true
				}
				if runes[j] == pair.close {
					closeIdx = j
					break
				}
			}
			if closeIdx == -1 || !hasNewline {
				break
			}

			for k := i; k <= closeIdx; k++ {
				if runes[k] != '\n' {
					runes[k] = ' '
				}
			}
			i = closeIdx
			break
		}
	}

	return string(runes)
}

func hiDropCueOnlyLine(line *hiLine, cues hiCuePatterns) {
	if cues.onlyLine.MatchString(line.text) {
		line.text = ""
	}
}

func hiStripDashLeadingCues(line *hiLine, cues hiCuePatterns) {
	line.text = repeatHITransform(line.text, cues.dashLeading, func(m []string) string {
		return "- " + strings.TrimSpace(m[1])
	})
}

func hiStripLeadingCues(line *hiLine, cues hiCuePatterns) {
	line.text = repeatHITransform(line.text, cues.leading, func(m []string) string {
		return strings.TrimSpace(m[1])
	})
}

func hiStripTrailingCues(line *hiLine, cues hiCuePatterns) {
	line.text = repeatHITransform(line.text, cues.trailing, func(m []string) string {
		return strings.TrimSpace(m[1])
	})
}

func hiCaptureDialogueDash(line *hiLine, _ hiCuePatterns) {
	if !strings.HasPrefix(line.text, "-") {
		return
	}
	line.hadDialogueDash = true
	line.text = strings.TrimSpace(strings.TrimPrefix(line.text, "-"))
}

func hiStripInlineCues(line *hiLine, cues hiCuePatterns) {
	line.text = cues.inlineCue.ReplaceAllString(line.text, " ")
}

func hiStripSpeakerPrefix(line *hiLine, _ hiCuePatterns) {
	line.text = hiSpeakerPrefixPattern.ReplaceAllString(line.text, "")
}

func hiNormalizeSpacing(line *hiLine, _ hiCuePatterns) {
	line.text = hiSpaceBeforePunctPattern.ReplaceAllString(line.text, "$1")
	line.text = hiMultiSpacePattern.ReplaceAllString(line.text, " ")
	line.text = strings.TrimSpace(line.text)
}

func hiStripLeadingCueArtifacts(line *hiLine, _ hiCuePatterns) {
	line.text = strings.TrimSpace(hiLeadingCueArtifact.ReplaceAllString(line.text, ""))
}

func hiRestoreDialogueDash(line *hiLine, _ hiCuePatterns) {
	if !line.hadDialogueDash || line.text == "" {
		return
	}
	line.text = "- " + strings.TrimSpace(strings.TrimPrefix(line.text, "-"))
}

func repeatHITransform(text string, pattern *regexp.Regexp, transform func([]string) string) string {
	for {
		matches := pattern.FindStringSubmatch(text)
		if matches == nil {
			return text
		}
		text = transform(matches)
	}
}

func shouldKeepHILine(line string, cues hiCuePatterns) bool {
	line = strings.TrimSpace(line)
	return line != "" && !cues.onlyLine.MatchString(line) && containsDialogueContent(line)
}

func containsDialogueContent(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}
