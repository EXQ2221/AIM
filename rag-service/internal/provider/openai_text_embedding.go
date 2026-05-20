package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"example.com/aim/shared/errno"
	ragmodel "example.com/aim/rag-service/internal/dal/model"
	"example.com/aim/rag-service/internal/errx"
)

type OpenAITextEmbeddingClient struct {
	baseURL    string
	apiKey     string
	model      string
	dimension  int
	httpClient *http.Client
}

func NewOpenAITextEmbeddingClient(cfg ragmodel.Config) (*OpenAITextEmbeddingClient, error) {
	baseURL := normalizeOpenAICompatibleBaseURL(strings.TrimSpace(cfg.BaseURL))
	if baseURL == "" {
		return nil, errx.Required("EMBEDDING_BASE_URL")
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errx.Required("EMBEDDING_API_KEY")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, errx.Required("EMBEDDING_MODEL")
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = ragmodel.DefaultTimeout
	}
	return &OpenAITextEmbeddingClient{
		baseURL:   baseURL,
		apiKey:    strings.TrimSpace(cfg.APIKey),
		model:     strings.TrimSpace(cfg.Model),
		dimension: cfg.Dimension,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

type openAITextEmbeddingRequest struct {
	Model      string `json:"model"`
	Input      any    `json:"input"`
	Dimensions int    `json:"dimensions,omitempty"`
}

func (c *OpenAITextEmbeddingClient) Embed(ctx context.Context, req ragmodel.EmbedRequest) (*ragmodel.EmbedResponse, error) {
	if c == nil {
		return nil, errx.NilDependency("embedding client")
	}
	modelName := strings.TrimSpace(req.Model)
	if modelName == "" {
		modelName = c.model
	}
	if modelName == "" {
		return nil, errx.Required("embedding model")
	}
	if len(req.Input) == 0 {
		return nil, errx.Required("embedding input")
	}

	inputs := make([]string, 0, len(req.Input))
	for _, part := range req.Input {
		if part.Type != ragmodel.InputPartText {
			return nil, errno.BadRequest("openai text embedding supports text input only")
		}
		text := strings.TrimSpace(part.Text)
		if text == "" {
			return nil, errx.EmptyInput("embedding input text")
		}
		inputs = append(inputs, text)
	}

	payload := openAITextEmbeddingRequest{
		Model: modelName,
		Input: inputs,
	}
	if c.dimension > 0 {
		payload.Dimensions = c.dimension
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := c.baseURL + "/embeddings"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
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

	result := &ragmodel.EmbedResponse{
		Embeddings:   make([][]float32, 0, len(parsed.Data)),
		PromptTokens: parsed.Usage.PromptTokens,
		TotalTokens:  parsed.Usage.TotalTokens,
	}
	for _, item := range parsed.Data {
		if len(item.Embedding) == 0 {
			return nil, errno.Internal("embedding result is empty")
		}
		if c.dimension > 0 && len(item.Embedding) != c.dimension {
			return nil, fmt.Errorf("embedding dimension mismatch: expected=%d got=%d", c.dimension, len(item.Embedding))
		}
		result.Embeddings = append(result.Embeddings, item.Embedding)
	}
	return result, nil
}

var _ ragmodel.Client = (*OpenAITextEmbeddingClient)(nil)
