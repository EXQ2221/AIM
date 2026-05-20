package rag

import "testing"

func TestSplitTextSmallContent(t *testing.T) {
	chunks, err := SplitText("hello world", SplitterConfig{ChunkSize: 1000, ChunkOverlap: 150})
	if err != nil {
		t.Fatalf("split failed: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Index != 0 {
		t.Fatalf("expected index=0, got %d", chunks[0].Index)
	}
	if chunks[0].Content != "hello world" {
		t.Fatalf("unexpected chunk content: %q", chunks[0].Content)
	}
}

func TestSplitTextOverlap(t *testing.T) {
	input := "abcde\n\nfghij\n\nklmno\n\npqrst"
	chunks, err := SplitText(input, SplitterConfig{ChunkSize: 10, ChunkOverlap: 3})
	if err != nil {
		t.Fatalf("split failed: %v", err)
	}
	if len(chunks) != 4 {
		t.Fatalf("expected 4 chunks, got %d", len(chunks))
	}
	if chunks[0].Content != "abcde" {
		t.Fatalf("unexpected first chunk: %q", chunks[0].Content)
	}
	if chunks[1].Content != "fghij" {
		t.Fatalf("unexpected second chunk: %q", chunks[1].Content)
	}
}

func TestSplitTextRejectInvalidOverlap(t *testing.T) {
	_, err := SplitText("abc", SplitterConfig{ChunkSize: 0, ChunkOverlap: 3})
	if err == nil {
		t.Fatal("expected chunk size validation error")
	}
}
