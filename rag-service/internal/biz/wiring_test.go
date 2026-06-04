package rag

import "testing"

func TestRagTopKFromEnvDefaultAndClamp(t *testing.T) {
	t.Setenv("RAG_TOP_K", "")
	if got := ragTopKFromEnv(); got != 8 {
		t.Fatalf("ragTopKFromEnv() = %d, want 8", got)
	}

	t.Setenv("RAG_TOP_K", "20")
	if got := ragTopKFromEnv(); got != 10 {
		t.Fatalf("ragTopKFromEnv() clamp = %d, want 10", got)
	}
}
