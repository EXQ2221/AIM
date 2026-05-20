package embedding

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
)

type retryClient struct {
	base       Client
	maxRetries int
	backoff    time.Duration
}

func wrapWithRetry(base Client, cfg Config) Client {
	if base == nil {
		return nil
	}
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		return base
	}
	backoff := cfg.RetryBackoff
	if backoff <= 0 {
		backoff = 1200 * time.Millisecond
	}
	return &retryClient{
		base:       base,
		maxRetries: maxRetries,
		backoff:    backoff,
	}
}

func (c *retryClient) Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error) {
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		resp, err := c.base.Embed(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if attempt == c.maxRetries || !isRetryableEmbeddingError(err) {
			break
		}
		wait := c.backoff * time.Duration(1<<attempt)
		if wait > 10*time.Second {
			wait = 10 * time.Second
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, lastErr
}

func isRetryableEmbeddingError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var timeoutErr interface{ Timeout() bool }
	if errors.As(err, &timeoutErr) && timeoutErr.Timeout() {
		return true
	}
	var statusErr *HTTPStatusError
	if errors.As(err, &statusErr) {
		return statusErr.StatusCode == http.StatusTooManyRequests || statusErr.StatusCode >= http.StatusInternalServerError
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "connection reset") ||
		strings.Contains(text, "connection refused") ||
		strings.Contains(text, "i/o timeout") ||
		strings.Contains(text, "tls: handshake timeout") ||
		strings.Contains(text, "server closed idle connection")
}

var _ Client = (*retryClient)(nil)
