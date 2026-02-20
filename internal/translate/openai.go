package translate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"log/slog"

	"github.com/adrianmusante/subtitle-tools/internal/run"
)

type OpenAIClient struct {
	HTTPClient   *http.Client
	BaseURL      string // e.g. https://api.openai.com
	APIKey       string // can be a single key or a comma-separated list of keys
	Model        string
	Timeout      time.Duration
	RetryOptions RetryOptions

	apiKeyRR uint32 // round-robin counter for multi-key rotation
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionsRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
}

type chatCompletionsResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (c *OpenAIClient) apiKeys() []string {
	// Accept comma-separated keys, trimming whitespace, ignoring empties.
	// If no comma is present, still returns a 1-item slice.
	raw := strings.TrimSpace(c.APIKey)
	if raw == "" {
		return []string{}
	}
	parts := strings.Split(raw, ",")
	keys := make([]string, 0, len(parts))
	for _, p := range parts {
		k := strings.TrimSpace(p)
		if k == "" {
			continue
		}
		keys = append(keys, k)
	}
	return keys
}

func (c *OpenAIClient) pickAPIKey(keys []string, rotated bool) (string, int) {
	if len(keys) == 0 {
		return "", -1
	}
	if len(keys) == 1 {
		return keys[0], 0
	}

	base := int(atomic.LoadUint32(&c.apiKeyRR))
	idx := base % len(keys)
	if rotated {
		idx = (idx + 1) % len(keys)
	}
	return keys[idx], idx
}

func (c *OpenAIClient) advanceAPIKeyRR() {
	atomic.AddUint32(&c.apiKeyRR, 1)
}

func (c *OpenAIClient) TranslateBatch(ctx context.Context, sourceLanguage string, targetLanguage string, payload string) (string, error) {
	if c.Model == "" {
		return "", errors.New("model is required")
	}
	if targetLanguage == "" {
		return "", errors.New("target language is required")
	}

	keys := c.apiKeys()

	hc := c.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: c.Timeout}
	}

	base, err := resolveBaseURLForModel(c.Model, c.BaseURL)
	if err != nil {
		return "", err
	}
	u, err := buildURL(base, "/v1/chat/completions")
	if err != nil {
		return "", err
	}

	messages := buildPrompt(sourceLanguage, targetLanguage, payload)

	reqBody := chatCompletionsRequest{
		Model:       c.Model,
		Messages:    messages,
		Temperature: 0,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	retry := c.RetryOptions
	rotatedOnReject := false

	return requestWithRetry[string](ctx, retry, func(attempt int) (string, retryDecision) {
		apiKey, _ := c.pickAPIKey(keys, rotatedOnReject)
		rotatedOnReject = false

		r, err := doJSONPost(ctx, hc, u.String(), apiKey, body)
		if err != nil {
			if isRetryableNetErr(err) {
				return "", retryDecision{err: err, retry: true}
			}
			return "", retryDecision{err: err}
		}

		if r.statusCode < 200 || r.statusCode >= 300 {
			hErr := fmt.Errorf("translation api error: status=%d body=%s", r.statusCode, strings.TrimSpace(string(r.bodyBytes)))

			if isRejectedHTTPStatus(r.statusCode) {
				if len(keys) > 1 {
					slog.Warn("translation api rejected request; rotating api key",
						"attempt", attempt,
						"status_code", r.statusCode,
						"status_text", http.StatusText(r.statusCode),
						"rejected_key", run.MaskKey(apiKey),
						"keys", len(keys),
					)
					rotatedOnReject = true
				}
			}

			if rotatedOnReject || isRetryableHTTPStatus(r.statusCode) {
				return "", retryDecision{err: hErr, retry: true, delay: retryDelayFromHeader(r.header)}
			}
			return "", retryDecision{err: hErr}
		}

		// Success: advance RR so the next request starts from the next key.
		if len(keys) > 1 {
			c.advanceAPIKeyRR()
		}

		content, err := parseChatCompletionContent(r.bodyBytes)
		if err != nil {
			return "", retryDecision{err: err, retry: true}
		}
		return content, retryDecision{}
	})
}

func resolveBaseURLForModel(model string, explicitBaseURL string) (string, error) {
	explicitBaseURL = strings.TrimSpace(explicitBaseURL)
	if explicitBaseURL != "" {
		return explicitBaseURL, nil
	}

	m := strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.HasPrefix(m, "gemini-"):
		return "https://generativelanguage.googleapis.com/v1beta/openai", nil
	case strings.HasPrefix(m, "gpt-"):
		return "https://api.openai.com", nil
	default:
		return "", fmt.Errorf("cannot resolve base url for model %q; set BaseURL explicitly", model)
	}
}

func buildPrompt(sourceLanguage string, targetLanguage string, input string) []ChatMessage {
	sourcePromptLabel := normalizeTargetLanguageLabel(sourceLanguage)
	targetPromptLabel := normalizeTargetLanguageLabel(targetLanguage)

	system := ChatMessage{Role: "system", Content: "You are a translation engine. Output must follow the requested format exactly. Do not add commentary."}
	userContent := "Translate the following subtitles"
	if sourcePromptLabel != "" {
		userContent += " from `" + sourcePromptLabel + "`"
	}
	userContent += " to: `" + targetPromptLabel + "`\n"
	userContent += "\n" +
		"Rules:\n" +
		"- Output MUST contain the same number of items as the input.\n" +
		"- Preserve idx values exactly and do not reorder.\n" +
		"- Output MUST be NDJSON: one JSON object per line (no surrounding array).\n" +
		"- Each output line MUST be valid JSON with exactly two keys: idx (number) and text (string).\n" +
		"- Do not output markdown, code fences, headers, or explanations.\n" +
		"\n" +
		"Example:\n" +
		"Input:\n" +
		"{\"idx\":1,\"text\":\"Hello\\nworld\"}\n" +
		"{\"idx\":2,\"text\":\"How are you?\"}\n" +
		"Output:\n" +
		"{\"idx\":1,\"text\":\"Hola\\nmundo\"}\n" +
		"{\"idx\":2,\"text\":\"¿Cómo estás?\"}\n" +
		"\n" +
		"Input:\n\n" + input + "\n"
	user := ChatMessage{Role: "user", Content: userContent}

	return []ChatMessage{system, user}
}
