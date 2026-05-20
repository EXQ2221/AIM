package embedding

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeClient struct {
	calls int
	fn    func(int) (*EmbedResponse, error)
}

func (f *fakeClient) Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error) {
	f.calls++
	return f.fn(f.calls)
}

func TestRetryClientRetriesOnTimeout(t *testing.T) {
	base := &fakeClient{
		fn: func(call int) (*EmbedResponse, error) {
			if call < 3 {
				return nil, context.DeadlineExceeded
			}
			return &EmbedResponse{Embeddings: [][]float32{{1, 2, 3}}}, nil
		},
	}

	client := wrapWithRetry(base, Config{
		MaxRetries:   2,
		RetryBackoff: 2 * time.Millisecond,
	})
	resp, err := client.Embed(context.Background(), EmbedRequest{})
	if err != nil {
		t.Fatalf("expected success after retry, got error: %v", err)
	}
	if len(resp.Embeddings) != 1 {
		t.Fatalf("unexpected embedding count: %d", len(resp.Embeddings))
	}
	if base.calls != 3 {
		t.Fatalf("expected 3 calls, got %d", base.calls)
	}
}

func TestRetryClientNoRetryOnBadRequest(t *testing.T) {
	base := &fakeClient{
		fn: func(call int) (*EmbedResponse, error) {
			return nil, errors.New("embedding input text is empty")
		},
	}

	client := wrapWithRetry(base, Config{
		MaxRetries:   3,
		RetryBackoff: 2 * time.Millisecond,
	})
	_, err := client.Embed(context.Background(), EmbedRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	if base.calls != 1 {
		t.Fatalf("expected 1 call without retry, got %d", base.calls)
	}
}

func TestRetryClientRetriesOnHTTP429(t *testing.T) {
	base := &fakeClient{
		fn: func(call int) (*EmbedResponse, error) {
			if call == 1 {
				return nil, &HTTPStatusError{StatusCode: 429, StatusText: "Too Many Requests", Body: "rate limit"}
			}
			return &EmbedResponse{Embeddings: [][]float32{{1}}}, nil
		},
	}

	client := wrapWithRetry(base, Config{
		MaxRetries:   1,
		RetryBackoff: time.Millisecond,
	})
	resp, err := client.Embed(context.Background(), EmbedRequest{})
	if err != nil {
		t.Fatalf("expected success after retry, got error: %v", err)
	}
	if len(resp.Embeddings) != 1 {
		t.Fatalf("unexpected embedding count: %d", len(resp.Embeddings))
	}
	if base.calls != 2 {
		t.Fatalf("expected 2 calls, got %d", base.calls)
	}
}
