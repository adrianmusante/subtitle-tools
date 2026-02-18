package translate

import (
	"strings"
)

const (
	LanguageEnglish        = "English"
	LanguageEnglishUS      = "English (US)"
	LanguageEnglishUK      = "English (UK)"
	LanguageSpanishLatin   = "Spanish (Neutral Latin American)"
	LanguageSpanishSpain   = "Spanish (Spain)"
	LanguageSpanishNeutral = "Spanish (Neutral)"
)

var exactLanguageLabels = map[string]string{
	"en":    LanguageEnglish,
	"en-us": LanguageEnglishUS,
	"en-gb": LanguageEnglishUK,
	"es":    LanguageSpanishNeutral,
	"spa":   LanguageSpanishNeutral,
	"es-es": LanguageSpanishSpain,
	"ea":    LanguageSpanishLatin,
	"spl":   LanguageSpanishLatin,
}

// normalizeTargetLanguage takes user input (often BCP-47-ish tags like "es", "es-MX",
// "es_419", or patterns like "es-*"), normalizes it, and returns:
// - tag: normalized tag/pattern for traceability
// - label: a human-friendly variant that is better suited for prompts
//
// This is intentionally conservative: it only maps a small set of common values
// and otherwise falls back to the normalized input.
func normalizeTargetLanguage(input string) (tag string, label string) {
	tag = strings.TrimSpace(input)
	tag = strings.ReplaceAll(tag, "_", "-")
	for strings.Contains(tag, "--") {
		tag = strings.ReplaceAll(tag, "--", "-")
	}
	if tag == "" {
		return "", ""
	}

	// Normalize to canonical-ish casing for language/region tags.
	parts := strings.Split(tag, "-")
	if len(parts) >= 1 {
		parts[0] = strings.ToLower(parts[0])
	}
	if len(parts) >= 2 {
		// Region is usually 2 letters or 3 digits.
		if len(parts[1]) == 2 {
			parts[1] = strings.ToUpper(parts[1])
		} else if len(parts[1]) == 3 {
			parts[1] = strings.ToLower(parts[1])
		}
	}
	tag = strings.Join(parts, "-")
	lower := strings.ToLower(tag)

	if label, ok := exactLanguageLabels[lower]; ok {
		return tag, label
	}
	if strings.HasPrefix(lower, "en-") {
		return tag, LanguageEnglish
	}
	if strings.HasPrefix(lower, "es-") {
		return tag, LanguageSpanishLatin
	}

	return tag, tag
}

func normalizeTargetLanguageLabel(input string) (label string) {
	_, label = normalizeTargetLanguage(input)
	if label == "" {
		label = input
	}
	return label
}
