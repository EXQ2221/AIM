package embedding

import "testing"

func TestIsTextEmbeddingModel(t *testing.T) {
	if !isTextEmbeddingModel("text-embedding-v4") {
		t.Fatal("expected text-embedding-v4 to be detected as text embedding model")
	}
	if isTextEmbeddingModel("qwen3-vl-embedding") {
		t.Fatal("expected qwen3-vl-embedding to not be detected as text embedding model")
	}
}

func TestDefaultDimensionForModel(t *testing.T) {
	if got := defaultDimensionForModel("text-embedding-v4"); got != 1024 {
		t.Fatalf("expected 1024, got %d", got)
	}
	if got := defaultDimensionForModel("qwen3-vl-embedding"); got != 2560 {
		t.Fatalf("expected 2560, got %d", got)
	}
}
