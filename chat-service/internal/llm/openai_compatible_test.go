package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLoadConfigFromEnvMissingRequired(t *testing.T) {
	t.Setenv("LLM_BASE_URL", "")
	t.Setenv("LLM_API_KEY", "")
	t.Setenv("LLM_MODEL", "")
	t.Setenv("LLM_TIMEOUT_SECONDS", "")

	if _, err := LoadConfigFromEnv(); err == nil {
		t.Fatal("expected missing env error")
	}
}

func TestOpenAICompatibleGenerateSuccess(t *testing.T) {
	var seenPath string
	var seenAuth string
	var seenPayload chatCompletionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		seenAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&seenPayload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices":[{"message":{"role":"assistant","content":"hello from AIM"}}],
			"usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}
		}`))
	}))
	defer server.Close()

	client, err := NewOpenAICompatibleClient(Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "deepseek-chat",
		Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	resp, err := client.Generate(context.Background(), GenerateRequest{
		Messages: []ChatMessage{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if seenPath != "/chat/completions" {
		t.Fatalf("unexpected path: %s", seenPath)
	}
	if seenAuth != "Bearer test-key" {
		t.Fatalf("unexpected auth header: %s", seenAuth)
	}
	if seenPayload.Model != "deepseek-chat" || seenPayload.Stream {
		t.Fatalf("unexpected payload: %+v", seenPayload)
	}
	if resp.Content != "hello from AIM" || resp.PromptTokens != 3 || resp.CompletionTokens != 4 || resp.TotalTokens != 7 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestOpenAICompatibleGenerateWithImageParts(t *testing.T) {
	var seenPayload chatCompletionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&seenPayload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices":[{"message":{"role":"assistant","content":"ok"}}],
			"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}
		}`))
	}))
	defer server.Close()

	client, err := NewOpenAICompatibleClient(Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "qwen3.6-plus",
		Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	_, err = client.Generate(context.Background(), GenerateRequest{
		Messages: []ChatMessage{
			{
				Role: "user",
				Parts: []ChatMessagePart{
					{Type: "text", Text: "describe image"},
					{Type: "image_url", ImageURL: "https://cdn.example.com/a.png"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if len(seenPayload.Messages) != 1 {
		t.Fatalf("unexpected message count: %d", len(seenPayload.Messages))
	}
	contentBytes, err := json.Marshal(seenPayload.Messages[0].Content)
	if err != nil {
		t.Fatalf("marshal content: %v", err)
	}
	var contentParts []chatCompletionMessagePart
	if err := json.Unmarshal(contentBytes, &contentParts); err != nil {
		t.Fatalf("unmarshal content parts: %v", err)
	}
	if len(contentParts) != 2 {
		t.Fatalf("unexpected multicontent count: %d", len(contentParts))
	}
	if contentParts[0].Type != "text" || contentParts[0].Text != "describe image" {
		t.Fatalf("unexpected first part: %+v", contentParts[0])
	}
	if contentParts[1].Type != "image_url" || contentParts[1].ImageURL == nil || contentParts[1].ImageURL.URL != "https://cdn.example.com/a.png" {
		t.Fatalf("unexpected second part: %+v", contentParts[1])
	}
}

func TestOpenAICompatibleGenerateNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad key", http.StatusUnauthorized)
	}))
	defer server.Close()

	client, err := NewOpenAICompatibleClient(Config{BaseURL: server.URL, APIKey: "test-key", Model: "deepseek-chat"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	_, err = client.Generate(context.Background(), GenerateRequest{Messages: []ChatMessage{{Role: "user", Content: "hello"}}})
	if err == nil || !strings.Contains(err.Error(), "status=401") {
		t.Fatalf("expected status error, got %v", err)
	}
	if strings.Contains(err.Error(), "test-key") {
		t.Fatalf("error leaked api key: %v", err)
	}
}

func TestOpenAICompatibleGenerateEmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer server.Close()

	client, err := NewOpenAICompatibleClient(Config{BaseURL: server.URL, APIKey: "test-key", Model: "deepseek-chat"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	_, err = client.Generate(context.Background(), GenerateRequest{Messages: []ChatMessage{{Role: "user", Content: "hello"}}})
	if err == nil || !strings.Contains(err.Error(), "choices") {
		t.Fatalf("expected empty choices error, got %v", err)
	}
}
