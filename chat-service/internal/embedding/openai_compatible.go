package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type HTTPStatusError struct {
	StatusCode int
	StatusText string
	Body       string
}

func (e *HTTPStatusError) Error() string {
	if e == nil {
		return "embedding request failed"
	}
	return fmt.Sprintf("embedding request failed: status=%d %s: %s", e.StatusCode, e.StatusText, e.Body)
}

type OpenAICompatibleClient struct {
	baseURL    string
	apiKey     string
	model      string
	dimension  int
	httpClient *http.Client
}

func NewOpenAICompatibleClient(cfg Config) (*OpenAICompatibleClient, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		return nil, errors.New("EMBEDDING_BASE_URL is required")
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("EMBEDDING_API_KEY is required")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, errors.New("EMBEDDING_MODEL is required")
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &OpenAICompatibleClient{
		baseURL:   baseURL,
		apiKey:    strings.TrimSpace(cfg.APIKey),
		model:     strings.TrimSpace(cfg.Model),
		dimension: cfg.Dimension,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

type openAIEmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type openAIEmbeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

func (c *OpenAICompatibleClient) Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error) {
	if c == nil {
		return nil, errors.New("embedding client is nil")
	}
	modelName := strings.TrimSpace(req.Model)
	if modelName == "" {
		modelName = c.model
	}
	if modelName == "" {
		return nil, errors.New("embedding model is required")
	}
	if len(req.Input) == 0 {
		return nil, errors.New("embedding input is required")
	}

	inputs := make([]string, 0, len(req.Input))
	for _, part := range req.Input {
		if part.Type != InputPartText {
			return nil, errors.New("openai_compatible embedding supports text input only")
		}
		text := strings.TrimSpace(part.Text)
		if text == "" {
			return nil, errors.New("embedding input text is empty")
		}
		inputs = append(inputs, text)
	}

	payload := openAIEmbeddingRequest{
		Model: modelName,
		Input: inputs,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("embedding request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read embedding response failed: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &HTTPStatusError{
			StatusCode: resp.StatusCode,
			StatusText: http.StatusText(resp.StatusCode),
			Body:       safeBodyExcerpt(respBody),
		}
	}

	var parsed openAIEmbeddingResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("parse embedding response failed: %w", err)
	}
	if len(parsed.Data) != len(inputs) {
		return nil, fmt.Errorf("embedding count mismatch: expected=%d got=%d", len(inputs), len(parsed.Data))
	}

	result := &EmbedResponse{
		Embeddings:   make([][]float32, 0, len(parsed.Data)),
		PromptTokens: parsed.Usage.PromptTokens,
		TotalTokens:  parsed.Usage.TotalTokens,
	}
	for _, item := range parsed.Data {
		if len(item.Embedding) != c.dimension {
			return nil, fmt.Errorf("embedding dimension mismatch: expected=%d got=%d", c.dimension, len(item.Embedding))
		}
		result.Embeddings = append(result.Embeddings, item.Embedding)
	}
	return result, nil
}

func safeBodyExcerpt(body []byte) string {
	const maxLen = 512
	text := strings.TrimSpace(string(body))
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")
	if len(text) > maxLen {
		return text[:maxLen] + "..."
	}
	return text
}

var _ Client = (*OpenAICompatibleClient)(nil)
