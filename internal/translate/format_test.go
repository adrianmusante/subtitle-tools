package translate

import "testing"

func TestFormatAndParse_NDJSON(t *testing.T) {
	payload, err := FormatForTranslation([]int{1, 2}, []string{"Hola", "L1\nL2"})
	if err != nil {
		t.Fatalf("FormatForTranslation: %v", err)
	}
	parsed, err := ParseTranslatedLines(payload)
	if err != nil {
		t.Fatalf("ParseTranslatedLines: %v", err)
	}
	if len(parsed) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(parsed))
	}
	if parsed[0].Idx != 1 || parsed[0].Text != "Hola" {
		t.Fatalf("line0 mismatch: %+v", parsed[0])
	}
	if parsed[1].Idx != 2 || parsed[1].Text != "L1\nL2" {
		t.Fatalf("line1 mismatch: %+v", parsed[1])
	}
}

func TestParseTranslatedLines_ToleratesCodeFences(t *testing.T) {
	out := "```json\n" +
		"{\"idx\":1,\"text\":\"Hola\"}\n" +
		"{\"idx\":2,\"text\":\"L1\\nL2\"}\n" +
		"```"
	parsed, err := ParseTranslatedLines(out)
	if err != nil {
		t.Fatalf("ParseTranslatedLines: %v", err)
	}
	if len(parsed) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(parsed))
	}
	if parsed[1].Text != "L1\nL2" {
		t.Fatalf("expected newline in idx=2, got %q", parsed[1].Text)
	}
}

func TestParseTranslatedLines_ToleratesObjectsNotOnePerLine(t *testing.T) {
	out := "{\"idx\":1,\"text\":\"Hola\"} {\"idx\":2,\"text\":\"Chau\"}\n"
	parsed, err := ParseTranslatedLines(out)
	if err != nil {
		t.Fatalf("ParseTranslatedLines: %v", err)
	}
	if len(parsed) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(parsed))
	}
	if parsed[0].Idx != 1 || parsed[0].Text != "Hola" {
		t.Fatalf("line0 mismatch: %+v", parsed[0])
	}
	if parsed[1].Idx != 2 || parsed[1].Text != "Chau" {
		t.Fatalf("line1 mismatch: %+v", parsed[1])
	}
}

func TestParseTranslatedLines_SalvagesUnescapedQuotesInText(t *testing.T) {
	out := "{\"idx\":119,\"text\":\"♪No diré \"gracias\", te sonrojarías\\ny lo ignorarías riendo♪\"}"
	parsed, err := ParseTranslatedLines(out)
	if err != nil {
		t.Fatalf("ParseTranslatedLines: %v", err)
	}
	if len(parsed) != 1 {
		t.Fatalf("expected 1 line, got %d", len(parsed))
	}
	if parsed[0].Idx != 119 {
		t.Fatalf("expected idx=119, got %d", parsed[0].Idx)
	}
	want := "♪No diré \"gracias\", te sonrojarías\ny lo ignorarías riendo♪"
	if parsed[0].Text != want {
		t.Fatalf("text mismatch:\nwant: %q\n got: %q", want, parsed[0].Text)
	}
}

func TestParseTranslatedLines_SalvagesMixedEscapedAndUnescapedQuotes(t *testing.T) {
	out := "{\"idx\":1,\"text\":\"Ella dijo \\\"hola\\\" y luego \"chau\"\\nfin\"}"
	parsed, err := ParseTranslatedLines(out)
	if err != nil {
		t.Fatalf("ParseTranslatedLines: %v", err)
	}
	if len(parsed) != 1 {
		t.Fatalf("expected 1 line, got %d", len(parsed))
	}
	want := "Ella dijo \"hola\" y luego \"chau\"\nfin"
	if parsed[0].Text != want {
		t.Fatalf("text mismatch:\nwant: %q\n got: %q", want, parsed[0].Text)
	}
}

func TestParseTranslatedLines_SalvagesOnlyBrokenLines(t *testing.T) {
	out := "{\"idx\":1,\"text\":\"Hola\"}\n" +
		"{\"idx\":2,\"text\":\"Ella dijo \"chau\"\"}\n" + // invalid JSON: unescaped quotes around chau
		"{\"idx\":3,\"text\":\"Fin\"}"

	parsed, err := ParseTranslatedLines(out)
	if err != nil {
		t.Fatalf("ParseTranslatedLines: %v", err)
	}
	if len(parsed) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(parsed))
	}
	if parsed[0].Idx != 1 || parsed[0].Text != "Hola" {
		t.Fatalf("line0 mismatch: %+v", parsed[0])
	}
	if parsed[1].Idx != 2 || parsed[1].Text != "Ella dijo \"chau\"" {
		t.Fatalf("line1 mismatch: %+v", parsed[1])
	}
	if parsed[2].Idx != 3 || parsed[2].Text != "Fin" {
		t.Fatalf("line2 mismatch: %+v", parsed[2])
	}
}

func TestParseTranslatedLines_SalvagesUnescapedQuotesFollowedByComma(t *testing.T) {
	out := "{\"idx\":10,\"text\":\"Dijo \"hola\", y se fue\"}"
	parsed, err := ParseTranslatedLines(out)
	if err != nil {
		t.Fatalf("ParseTranslatedLines: %v", err)
	}
	if len(parsed) != 1 {
		t.Fatalf("expected 1 line, got %d", len(parsed))
	}
	want := "Dijo \"hola\", y se fue"
	if parsed[0].Idx != 10 || parsed[0].Text != want {
		t.Fatalf("mismatch: %+v (want text %q)", parsed[0], want)
	}
}
