// Package httpx provides a small JSON HTTP helper shared by the Jira and Tempo
// clients: JSON encoding/decoding, error bodies and retry with backoff on
// HTTP 429 (and transient 5xx).
package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// Doer performs HTTP requests with a retry policy. It wraps an *http.Client so
// tests can inject an httptest server.
type Doer struct {
	Client     *http.Client
	MaxRetries int
	// Sleep is the delay function; overridable in tests to avoid real waiting.
	Sleep func(time.Duration)
}

// NewDoer returns a Doer with sensible defaults.
func NewDoer(client *http.Client) *Doer {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &Doer{Client: client, MaxRetries: 4, Sleep: time.Sleep}
}

// APIError describes a non-2xx HTTP response.
type APIError struct {
	StatusCode int
	Status     string
	Body       string
	Method     string
	URL        string
}

func (e *APIError) Error() string {
	body := e.Body
	if len(body) > 300 {
		body = body[:300] + "…"
	}
	return fmt.Sprintf("%s %s: %s: %s", e.Method, e.URL, e.Status, body)
}

// NewJSONRequest builds a request with a JSON body (when body != nil) and the
// appropriate headers.
func NewJSONRequest(ctx context.Context, method, url string, body any) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// A generic client identifier — never includes user identity.
	req.Header.Set("User-Agent", "lazytempo")
	return req, nil
}

// DoJSON executes the request and decodes a JSON response into out (which may be
// nil to discard the body). It retries on 429 and transient 5xx using
// Retry-After when present, otherwise exponential backoff.
//
// For requests with a body, the body is buffered so it can be replayed on retry.
func (d *Doer) DoJSON(req *http.Request, out any) error {
	var bodyBytes []byte
	if req.Body != nil && req.GetBody == nil {
		b, err := io.ReadAll(req.Body)
		if err != nil {
			return err
		}
		_ = req.Body.Close()
		bodyBytes = b
	}

	var lastErr error
	for attempt := 0; attempt <= d.MaxRetries; attempt++ {
		if bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			req.ContentLength = int64(len(bodyBytes))
		}
		resp, err := d.Client.Do(req)
		if err != nil {
			lastErr = err
			if !d.backoff(req.Context(), attempt, 0) {
				return err
			}
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
			drain(resp)
			lastErr = &APIError{StatusCode: resp.StatusCode, Status: resp.Status, Method: req.Method, URL: req.URL.String()}
			if attempt == d.MaxRetries || !d.backoff(req.Context(), attempt, retryAfter) {
				return lastErr
			}
			continue
		}

		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			return &APIError{
				StatusCode: resp.StatusCode,
				Status:     resp.Status,
				Body:       string(data),
				Method:     req.Method,
				URL:        req.URL.String(),
			}
		}
		if out == nil {
			drain(resp)
			return nil
		}
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil && err != io.EOF {
			return fmt.Errorf("decode response: %w", err)
		}
		return nil
	}
	return lastErr
}

// backoff waits before the next attempt. It returns false if the context is
// cancelled while waiting.
func (d *Doer) backoff(ctx context.Context, attempt int, retryAfter time.Duration) bool {
	delay := retryAfter
	if delay <= 0 {
		delay = time.Duration(1<<uint(attempt)) * 250 * time.Millisecond
	}
	if delay > 10*time.Second {
		delay = 10 * time.Second
	}
	if d.Sleep != nil {
		// Respect context cancellation during the wait.
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return false
		case <-timer.C:
			return true
		}
	}
	return true
}

func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

func drain(resp *http.Response) {
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	_ = resp.Body.Close()
}
