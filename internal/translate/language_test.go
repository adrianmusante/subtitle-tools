package translate

import "testing"

func TestNormalizeTargetLanguage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		in    string
		tag   string
		label string
	}{
		{name: "empty", in: "", tag: "", label: ""},
		{name: "en base", in: "en", tag: "en", label: "English"},
		{name: "spl", in: "spl", tag: "spl", label: "Spanish (Neutral Latin American)"},
		{name: "en-us", in: "en-us", tag: "en-US", label: "English (US)"},
		{name: "en-gb", in: "EN_gb", tag: "en-GB", label: "English (UK)"},
		{name: "trim and underscores", in: "  es_MX  ", tag: "es-MX", label: "Spanish (Neutral Latin American)"},
		{name: "wildcard", in: "es-*", tag: "es-*", label: "Spanish (Neutral Latin American)"},
		{name: "casing normalization", in: "ES-mx", tag: "es-MX", label: "Spanish (Neutral Latin American)"},
		{name: "es-419", in: "es-419", tag: "es-419", label: "Spanish (Neutral Latin American)"},
		{name: "fallback", in: "fr-CA", tag: "fr-CA", label: "fr-CA"},
		{name: "casing normalization by fallback", in: "FR-CA", tag: "fr-CA", label: "fr-CA"},
		{name: "es-AR", in: "es-AR", tag: "es-AR", label: "Spanish (Neutral Latin American)"},
		{name: "es-ES", in: "es-ES", tag: "es-ES", label: "Spanish (Spain)"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tag, label := normalizeTargetLanguage(tt.in)
			if tag != tt.tag {
				t.Fatalf("tag: got %q want %q", tag, tt.tag)
			}
			if label != tt.label {
				t.Fatalf("label: got %q want %q", label, tt.label)
			}
		})
	}
}
