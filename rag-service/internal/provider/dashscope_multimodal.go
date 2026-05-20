package embedding

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"example.com/aim/shared/errno"
	ragmodel "example.com/aim/rag-service/internal/dal/model"
	"example.com/aim/rag-service/internal/errx"
)

type DashScopeMultimodalClient struct {
	baseURL    string
	apiKey     string
	model      string
	dimension  int
	httpClient *http.Client
}

const dashScopeMaxContentsPerRequest = 10

func NewDashScopeMultimodalClient(cfg ragmodel.Config) (*DashScopeMultimodalClient, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		return nil, errx.Required("EMBEDDING_BASE_URL")
	}
	baseURL = normalizeDashScopeBaseURL(baseURL)
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
	return &DashScopeMultimodalClient{
		baseURL:   baseURL,
		apiKey:    strings.TrimSpace(cfg.APIKey),
		model:     strings.TrimSpace(cfg.Model),
		dimension: cfg.Dimension,
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.InsecureSkipVerify}, //nolint:gosec
			},
		},
	}, nil
}

type dashScopeMultimodalRequest struct {
	Model      string                   `json:"model"`
	Input      dashScopeMultimodalInput `json:"input"`
	Parameters map[string]interface{}   `json:"parameters,omitempty"`
}

type dashScopeMultimodalInput struct {
	Contents []map[string]string `json:"contents"`
}

type dashScopeMultimodalResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	Output    struct {
		Embeddings []struct {
			Embedding []float32 `json:"embedding"`
			Type      string    `json:"type"`
		} `json:"embeddings"`
	} `json:"output"`
	Usage struct {
		InputTokens int `json:"input_tokens"`
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

func (c *DashScopeMultimodalClient) Embed(ctx context.Context, req ragmodel.EmbedRequest) (*ragmodel.EmbedResponse, error) {
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

	contents := make([]map[string]string, 0, len(req.Input))
	for _, part := range req.Input {
		switch part.Type {
		case ragmodel.InputPartText:
			text := strings.TrimSpace(part.Text)
			if text == "" {
				return nil, errx.EmptyInput("embedding input text")
			}
			contents = append(contents, map[string]string{"text": text})
		case ragmodel.InputPartImage:
			imageURL := strings.TrimSpace(part.Image)
			if imageURL == "" {
				return nil, errx.EmptyInput("embedding image url")
			}
			contents = append(contents, map[string]string{"image": imageURL})
		case ragmodel.InputPartVideo:
			videoURL := strings.TrimSpace(part.Video)
			if videoURL == "" {
				return nil, errx.EmptyInput("embedding video url")
			}
			contents = append(contents, map[string]string{"video": videoURL})
		default:
			return nil, errno.BadRequest("unsupported dashscope embedding input type")
		}
	}
	if len(contents) == 0 {
		return nil, errx.EmptyInput("embedding input")
	}

	result := &ragmodel.EmbedResponse{
		Embeddings: make([][]float32, 0, len(contents)),
	}

	for start := 0; start < len(contents); start += dashScopeMaxContentsPerRequest {
		end := start + dashScopeMaxContentsPerRequest
		if end > len(contents) {
			end = len(contents)
		}
		partial, err := c.embedBatch(ctx, modelName, contents[start:end])
		if err != nil {
			return nil, err
		}
		result.Embeddings = append(result.Embeddings, partial.Embeddings...)
		result.PromptTokens += partial.PromptTokens
		result.TotalTokens += partial.TotalTokens
	}
	return result, nil
}

func (c *DashScopeMultimodalClient) embedBatch(
	ctx context.Context,
	modelName string,
	contents []map[string]string,
) (*ragmodel.EmbedResponse, error) {
	payload := dashScopeMultimodalRequest{
		Model: modelName,
		Input: dashScopeMultimodalInput{
			Contents: contents,
		},
	}
	if c.dimension > 0 && supportsDimensionParameter(modelName) {
		payload.Parameters = map[string]interface{}{"dimension": c.dimension}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := c.baseURL + "/api/v1/services/embeddings/multimodal-embedding/multimodal-embedding"
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

	var parsed dashScopeMultimodalResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("parse embedding response failed: %w", err)
	}
	if strings.TrimSpace(parsed.Code) != "" {
		return nil, fmt.Errorf(
			"dashscope embedding failed: code=%s message=%s request_id=%s",
			parsed.Code,
			strings.TrimSpace(parsed.Message),
			strings.TrimSpace(parsed.RequestID),
		)
	}
	if len(parsed.Output.Embeddings) != len(contents) {
		return nil, fmt.Errorf("embedding count mismatch: expected=%d got=%d", len(contents), len(parsed.Output.Embeddings))
	}

	result := &ragmodel.EmbedResponse{
		Embeddings:   make([][]float32, 0, len(parsed.Output.Embeddings)),
		PromptTokens: parsed.Usage.InputTokens,
		TotalTokens:  parsed.Usage.TotalTokens,
	}
	for _, item := range parsed.Output.Embeddings {
		if len(item.Embedding) == 0 {
			return nil, errno.Internal("dashscope embedding result is empty")
		}
		if c.dimension > 0 && len(item.Embedding) != c.dimension {
			return nil, fmt.Errorf("embedding dimension mismatch: expected=%d got=%d", c.dimension, len(item.Embedding))
		}
		result.Embeddings = append(result.Embeddings, item.Embedding)
	}
	return result, nil
}

func normalizeDashScopeBaseURL(baseURL string) string {
	lower := strings.ToLower(baseURL)
	if strings.Contains(lower, "/compatible-mode/v1") {
		return strings.TrimRight(strings.Replace(baseURL, "/compatible-mode/v1", "", 1), "/")
	}
	return baseURL
}

func supportsDimensionParameter(modelName string) bool {
	name := strings.ToLower(strings.TrimSpace(modelName))
	switch name {
	case "qwen3-vl-embedding", "qwen2.5-vl-embedding", "tongyi-embedding-vision-plus-2026-03-06", "tongyi-embedding-vision-flash-2026-03-06":
		return true
	default:
		return false
	}
}

var _ ragmodel.Client = (*DashScopeMultimodalClient)(nil)
