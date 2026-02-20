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

// keys are normalized to lowercase for case-insensitive matching
var languageLabels = map[string]string{
	"en":    LanguageEnglish,
	"en-us": LanguageEnglishUS,
	"en-gb": LanguageEnglishUK,
	"es":    LanguageSpanishNeutral,
	"spa":   LanguageSpanishNeutral,
	"es-es": LanguageSpanishSpain,
	"ea":    LanguageSpanishLatin,
	"spl":   LanguageSpanishLatin,

	// If a specific region isn't recognized, but the language is, we can still apply a more general label.
	"en-*": LanguageEnglish,
	"es-*": LanguageSpanishLatin,
}

const LanguageSeparator = "-"

// normalizeTargetLanguage takes user input (often BCP-47-ish tags like "es", "es-MX",
// "es_419", or patterns like "es-*"), normalizes it, and returns:
// - tag: normalized tag/pattern for traceability
// - label: a human-friendly variant that is better suited for prompts
//
// This is intentionally conservative: it only maps a small set of common values
// and otherwise falls back to the normalized input.
func normalizeTargetLanguage(input string) (tag string, label string) {
	tag = strings.TrimSpace(input)
	tag = strings.ReplaceAll(tag, "_", LanguageSeparator)
	for strings.Contains(tag, "--") {
		tag = strings.ReplaceAll(tag, "--", LanguageSeparator)
	}
	if tag == "" {
		return "", ""
	}

	// Normalize to canonical-ish casing for language/region tags.
	parts := strings.Split(tag, LanguageSeparator)
	if len(parts) >= 1 {
		parts[0] = strings.ToLower(parts[0])
	}
	wildcardLang := ""
	if len(parts) >= 2 {
		// Region is usually 2 letters or 3 digits.
		if len(parts[1]) == 2 {
			parts[1] = strings.ToUpper(parts[1])
		} else if len(parts[1]) == 3 {
			parts[1] = strings.ToLower(parts[1])
		}
		wildcardLang = parts[0] + LanguageSeparator + "*" // e.g. "es-AR" would match "es-*"
	}
	tag = strings.Join(parts, LanguageSeparator)
	lower := strings.ToLower(tag)

	if label, ok := languageLabels[lower]; ok {
		return tag, label
	}

	if wildcardLang != "" {
		if label, ok := languageLabels[wildcardLang]; ok {
			return tag, label
		}
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
