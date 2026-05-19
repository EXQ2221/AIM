package llmdal

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	llmmodel "example.com/aim/chat-service/llm-internal/model"
)

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

	client, err := NewOpenAICompatibleClient(llmmodel.Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "deepseek-chat",
		Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	resp, err := client.Generate(context.Background(), llmmodel.GenerateRequest{
		Messages: []llmmodel.ChatMessage{{Role: "user", Content: "hello"}},
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
	if seenPayload.EnableSearch != nil || seenPayload.EnableThinking != nil || seenPayload.SearchOptions != nil {
		t.Fatalf("unexpected search/thinking fields for default client: %+v", seenPayload)
	}
	if resp.Content != "hello from AIM" || resp.PromptTokens != 3 || resp.CompletionTokens != 4 || resp.TotalTokens != 7 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestOpenAICompatibleGenerateEnableSearch(t *testing.T) {
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

	client, err := NewOpenAICompatibleClient(llmmodel.Config{
		BaseURL:      server.URL,
		APIKey:       "test-key",
		Model:        "qwen3.5-plus",
		Timeout:      time.Second,
		EnableSearch: true,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	_, err = client.Generate(context.Background(), llmmodel.GenerateRequest{
		Messages: []llmmodel.ChatMessage{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if seenPayload.EnableSearch == nil || !*seenPayload.EnableSearch {
		t.Fatalf("expected enable_search=true, got %+v", seenPayload)
	}
}

func TestOpenAICompatibleGenerateEnableThinking(t *testing.T) {
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

	thinking := false
	client, err := NewOpenAICompatibleClient(llmmodel.Config{
		BaseURL:        server.URL,
		APIKey:         "test-key",
		Model:          "qwen3.6-plus",
		Timeout:        time.Second,
		EnableSearch:   true,
		ForceSearch:    true,
		SearchStrategy: "max",
		EnableThinking: &thinking,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	_, err = client.Generate(context.Background(), llmmodel.GenerateRequest{
		Messages: []llmmodel.ChatMessage{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if seenPayload.EnableSearch == nil || !*seenPayload.EnableSearch {
		t.Fatalf("expected enable_search=true, got %+v", seenPayload)
	}
	if seenPayload.EnableThinking == nil || *seenPayload.EnableThinking {
		t.Fatalf("expected enable_thinking=false, got %+v", seenPayload)
	}
	if seenPayload.SearchOptions == nil || seenPayload.SearchOptions.ForcedSearch == nil || !*seenPayload.SearchOptions.ForcedSearch {
		t.Fatalf("expected search_options.forced_search=true, got %+v", seenPayload)
	}
	if seenPayload.SearchOptions.SearchStrategy != "max" {
		t.Fatalf("expected search_strategy=max, got %+v", seenPayload.SearchOptions)
	}
}

func TestOpenAICompatibleGenerateStream(t *testing.T) {
	var seenPayload chatCompletionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&seenPayload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"你\"}}]}\n\n")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"好\"}}]}\n\n")
		_, _ = io.WriteString(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":5,\"completion_tokens\":2,\"total_tokens\":7}}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	client, err := NewOpenAICompatibleClient(llmmodel.Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "qwen-plus",
		Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	chunks := make([]llmmodel.StreamChunk, 0, 3)
	resp, err := client.GenerateStream(context.Background(), llmmodel.GenerateRequest{
		Messages: []llmmodel.ChatMessage{{Role: "user", Content: "hi"}},
	}, func(chunk llmmodel.StreamChunk) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("GenerateStream returned error: %v", err)
	}
	if !seenPayload.Stream {
		t.Fatalf("expected stream=true in payload")
	}
	if seenPayload.StreamOptions == nil || !seenPayload.StreamOptions.IncludeUsage {
		t.Fatalf("expected stream_options.include_usage=true, got %+v", seenPayload.StreamOptions)
	}
	if resp.Content != "你好" || resp.TotalTokens != 7 {
		t.Fatalf("unexpected stream response: %+v", resp)
	}
	if len(chunks) < 2 || chunks[0].Content != "你" || chunks[1].Content != "好" {
		t.Fatalf("unexpected chunks: %+v", chunks)
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

	client, err := NewOpenAICompatibleClient(llmmodel.Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "qwen3.6-plus",
		Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	_, err = client.Generate(context.Background(), llmmodel.GenerateRequest{
		Messages: []llmmodel.ChatMessage{
			{
				Role: "user",
				Parts: []llmmodel.ChatMessagePart{
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

	client, err := NewOpenAICompatibleClient(llmmodel.Config{BaseURL: server.URL, APIKey: "test-key", Model: "deepseek-chat"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	_, err = client.Generate(context.Background(), llmmodel.GenerateRequest{Messages: []llmmodel.ChatMessage{{Role: "user", Content: "hello"}}})
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

	client, err := NewOpenAICompatibleClient(llmmodel.Config{BaseURL: server.URL, APIKey: "test-key", Model: "deepseek-chat"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	_, err = client.Generate(context.Background(), llmmodel.GenerateRequest{Messages: []llmmodel.ChatMessage{{Role: "user", Content: "hello"}}})
	if err == nil || !strings.Contains(err.Error(), "choices") {
		t.Fatalf("expected empty choices error, got %v", err)
	}
}
