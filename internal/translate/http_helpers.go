package translate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

type httpResult struct {
	statusCode int
	header     http.Header
	bodyBytes  []byte
}

func doJSONPost(
	ctx context.Context,
	hc *http.Client,
	u string,
	authBearer string,
	body []byte,
) (httpResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return httpResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if authBearer != "" {
		req.Header.Set("Authorization", "Bearer "+authBearer)
	}

	resp, err := hc.Do(req)
	if err != nil {
		return httpResult{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return httpResult{}, err
	}
	return httpResult{statusCode: resp.StatusCode, header: resp.Header.Clone(), bodyBytes: bodyBytes}, nil
}

func retryDelayFromHeader(h http.Header) time.Duration {
	ra := strings.TrimSpace(h.Get("Retry-After"))
	if ra == "" {
		return 0
	}
	secs, err := strconv.Atoi(ra)
	if err != nil || secs < 0 {
		return 0
	}
	return time.Duration(secs) * time.Second
}

func parseChatCompletionContent(bodyBytes []byte) (string, error) {
	var out chatCompletionsResponse
	if err := json.Unmarshal(bodyBytes, &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", errors.New("no choices in response")
	}
	content := strings.TrimSpace(out.Choices[0].Message.Content)
	if content == "" {
		return "", errors.New("empty content in response")
	}
	return content, nil
}

func buildURL(baseUrl, urlPath string) (*url.URL, error) {
	baseUrl = strings.TrimRight(baseUrl, "/")
	if baseUrl == "" {
		return nil, errors.New("base URL is required")
	}
	u, err := url.Parse(baseUrl)
	if err != nil {
		return nil, err
	}
	u.Path = path.Join(u.Path, urlPath)
	return u, nil
}
