package translate

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResolveBaseURLForModel(t *testing.T) {
	t.Run("explicit overrides", func(t *testing.T) {
		got, err := resolveBaseURLForModel("gemini-1.5-pro", "http://example.com/base")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if got != "http://example.com/base" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("gemini", func(t *testing.T) {
		got, err := resolveBaseURLForModel("gemini-1.5-pro", "")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if got != "https://generativelanguage.googleapis.com/v1beta/openai" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("gpt", func(t *testing.T) {
		got, err := resolveBaseURLForModel("gpt-4o-mini", "")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if got != "https://api.openai.com" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("unknown model errors", func(t *testing.T) {
		_, err := resolveBaseURLForModel("claude-3", "")
		if err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestOpenAIClient_APIKeysCSV(t *testing.T) {
	c := OpenAIClient{APIKey: " k1, ,k2 ,k3 ,, "}
	got := c.apiKeys()
	if len(got) != 3 {
		t.Fatalf("expected 3 keys, got %d: %#v", len(got), got)
	}
	if got[0] != "k1" || got[1] != "k2" || got[2] != "k3" {
		t.Fatalf("unexpected keys: %#v", got)
	}
}

func TestOpenAIClient_429RotatesAPIKey(t *testing.T) {
	var authHeaders []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeaders = append(authHeaders, r.Header.Get("Authorization"))
		// First attempt returns 429, second succeeds.
		if len(authHeaders) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte("rate limit"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"idx\":1,\"text\":\"Hola\"}"}}]}`))
	}))
	defer server.Close()

	var logBuf bytes.Buffer
	h := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })

	c := OpenAIClient{
		BaseURL: server.URL,
		APIKey:  "key_1,key_2",
		Model:   "gpt-test",
		RetryOptions: RetryOptions{
			MaxAttempts: 2,
			BaseDelay:   0,
			MaxDelay:    0,
			Jitter:      0,
		},
	}

	out, err := (&c).TranslateBatch(t.Context(), "es", `{"idx":1,"text":"Hello"}`)
	if err != nil {
		t.Fatalf("TranslateBatch: %v", err)
	}
	if strings.TrimSpace(out) == "" {
		t.Fatalf("expected non-empty output")
	}
	if len(authHeaders) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(authHeaders))
	}
	if authHeaders[0] == authHeaders[1] {
		t.Fatalf("expected Authorization header to rotate on 429; got %q then %q", authHeaders[0], authHeaders[1])
	}
	if !strings.Contains(logBuf.String(), "rotating api key") {
		t.Fatalf("expected log to mention api key rotation on 429; got logs: %s", logBuf.String())
	}
}
