package embedding

import (
	"context"
	"errors"
	"testing"

	ragmodel "example.com/aim/rag-service/internal/dal/model"
)

type callCounterClient struct {
	calls int
	err   error
}

func (c *callCounterClient) Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error) {
	c.calls++
	if c.err != nil {
		return nil, c.err
	}
	return &EmbedResponse{Embeddings: [][]float32{{1}}}, nil
}

func TestHybridClientUsesPrimaryForTextOnly(t *testing.T) {
	primary := &callCounterClient{}
	fallback := &callCounterClient{}
	client := &hybridClient{primary: primary, fallback: fallback}

	_, err := client.Embed(context.Background(), EmbedRequest{
		Input: []InputPart{{Type: ragmodel.InputPartText, Text: "hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if primary.calls != 1 {
		t.Fatalf("expected primary to be called once, got %d", primary.calls)
	}
	if fallback.calls != 0 {
		t.Fatalf("expected fallback not to be called, got %d", fallback.calls)
	}
}

func TestHybridClientUsesFallbackForImageInput(t *testing.T) {
	primary := &callCounterClient{err: errors.New("should not be used")}
	fallback := &callCounterClient{}
	client := &hybridClient{primary: primary, fallback: fallback}

	_, err := client.Embed(context.Background(), EmbedRequest{
		Input: []InputPart{{Type: ragmodel.InputPartImage, Image: "https://example.com/a.png"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if primary.calls != 0 {
		t.Fatalf("expected primary not to be called, got %d", primary.calls)
	}
	if fallback.calls != 1 {
		t.Fatalf("expected fallback to be called once, got %d", fallback.calls)
	}
}
