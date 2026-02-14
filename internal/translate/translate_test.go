package translate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestTranslateFile_Batched_ReconstructsSRT(t *testing.T) {
	// Mock OpenAI-compatible server.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always return a fixed translation for 2 lines (format: NDJSON).
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"idx\":1,\"text\":\"Hola\"}\n{\"idx\":2,\"text\":\"Adios\"}"}}]}`))
	}))
	defer server.Close()

	workdir, err := os.MkdirTemp("", "subtitle-tools-translate-test-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer func() { _ = os.RemoveAll(workdir) }()

	inPath := filepath.Join(workdir, "in.srt")
	outPath := filepath.Join(workdir, "out.srt")

	input := strings.Join([]string{
		"1",
		"00:00:01,000 --> 00:00:02,000",
		"Hello",
		"",
		"2",
		"00:00:03,000 --> 00:00:04,000",
		"Bye",
		"",
		"",
	}, "\n")
	if err := os.WriteFile(inPath, []byte(input), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	res, err := Run(context.Background(), Options{
		InputPath:        inPath,
		OutputPath:       outPath,
		DryRun:           false,
		WorkDir:          workdir,
		TargetLanguage:   "es",
		APIKey:           "test",
		Model:            "gpt-test",
		BaseURL:          server.URL,
		MaxBatchChars:    12000,
		MaxWorkers:       1,
		RPS:              0,
		RetryMaxAttempts: DefaultRetryMaxAttempts,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.WrittenPath != outPath {
		t.Fatalf("WrittenPath mismatch: %s", res.WrittenPath)
	}

	b, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	out := string(b)
	if !strings.Contains(out, "00:00:01,000 --> 00:00:02,000") {
		t.Fatalf("expected original timing preserved")
	}
	if !strings.Contains(out, "Hola") || !strings.Contains(out, "Adios") {
		t.Fatalf("expected translated text in output, got:\n%s", out)
	}
}

func TestTranslateFile_RetryOnParseFailure(t *testing.T) {
	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if c == 1 {
			// Invalid content -> ParseTranslatedLines should fail.
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"NOT NDJSON"}}]}`))
			return
		}
		// Valid NDJSON on retry.
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"idx\":1,\"text\":\"Hola\"}\n{\"idx\":2,\"text\":\"Adios\"}"}}]}`))
	}))
	defer server.Close()

	workdir, err := os.MkdirTemp("", "subtitle-tools-translate-test-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer func() { _ = os.RemoveAll(workdir) }()

	inPath := filepath.Join(workdir, "in.srt")
	outPath := filepath.Join(workdir, "out.srt")

	input := strings.Join([]string{
		"1",
		"00:00:01,000 --> 00:00:02,000",
		"Hello",
		"",
		"2",
		"00:00:03,000 --> 00:00:04,000",
		"Bye",
		"",
		"",
	}, "\n")
	if err := os.WriteFile(inPath, []byte(input), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err = Run(context.Background(), Options{
		InputPath:             inPath,
		OutputPath:            outPath,
		DryRun:                false,
		WorkDir:               workdir,
		TargetLanguage:        "es",
		APIKey:                "test",
		Model:                 "gpt-test",
		BaseURL:               server.URL,
		MaxBatchChars:         12000,
		MaxWorkers:            1,
		RPS:                   0,
		RetryMaxAttempts:      1, // keep HTTP retry out of the way; this is parse-retry.
		RetryParseMaxAttempts: 2,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := calls.Load(); got < 2 {
		t.Fatalf("expected at least 2 calls due to parse retry, got %d", got)
	}

	b, readErr := os.ReadFile(outPath)
	if readErr != nil {
		t.Fatalf("ReadFile: %v", readErr)
	}
	out := string(b)
	if !strings.Contains(out, "Hola") || !strings.Contains(out, "Adios") {
		t.Fatalf("expected translated text in output, got:\n%s", out)
	}
}
