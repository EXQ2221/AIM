package llm

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
		return "llm request failed"
	}
	return fmt.Sprintf("llm request failed: status=%d %s: %s", e.StatusCode, e.StatusText, e.Body)
}

type OpenAICompatibleClient struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewOpenAICompatibleClient(cfg Config) (*OpenAICompatibleClient, error) {
	baseURL := strings.TrimSpace(cfg.BaseURL)
	apiKey := strings.TrimSpace(cfg.APIKey)
	model := strings.TrimSpace(cfg.Model)
	if baseURL == "" {
		return nil, errors.New("LLM_BASE_URL is required")
	}
	if apiKey == "" {
		return nil, errors.New("LLM_API_KEY is required")
	}
	if model == "" {
		return nil, errors.New("LLM_MODEL is required")
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &OpenAICompatibleClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

func NewOpenAICompatibleClientFromEnv() (*OpenAICompatibleClient, error) {
	cfg, err := LoadConfigFromEnv()
	if err != nil {
		return nil, err
	}
	return NewOpenAICompatibleClient(cfg)
}

func (c *OpenAICompatibleClient) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	if c == nil {
		return nil, errors.New("llm client is nil")
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = c.model
	}
	if model == "" {
		return nil, errors.New("model is required")
	}
	if len(req.Messages) == 0 {
		return nil, errors.New("messages are required")
	}

	payload := chatCompletionRequest{
		Model:    model,
		Messages: make([]chatCompletionMessage, 0, len(req.Messages)),
		Stream:   false,
	}
	for _, message := range req.Messages {
		role := strings.TrimSpace(message.Role)
		if role == "" {
			return nil, errors.New("message role is required")
		}
		payload.Messages = append(payload.Messages, chatCompletionMessage{
			Role:    role,
			Content: message.Content,
		})
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("llm request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return nil, fmt.Errorf("read llm response failed: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &HTTPStatusError{
			StatusCode: resp.StatusCode,
			StatusText: statusHint(resp.StatusCode),
			Body:       safeBodyExcerpt(respBody),
		}
	}

	var parsed chatCompletionResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("parse llm response failed: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return nil, errors.New("llm response choices is empty")
	}
	content := parsed.Choices[0].Message.Content
	return &GenerateResponse{
		Content:          content,
		PromptTokens:     parsed.Usage.PromptTokens,
		CompletionTokens: parsed.Usage.CompletionTokens,
		TotalTokens:      parsed.Usage.TotalTokens,
	}, nil
}

type chatCompletionRequest struct {
	Model    string                  `json:"model"`
	Messages []chatCompletionMessage `json:"messages"`
	Stream   bool                    `json:"stream"`
}

type chatCompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatCompletionMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func statusHint(statusCode int) string {
	switch statusCode {
	case http.StatusBadRequest:
		return "bad request"
	case http.StatusUnauthorized:
		return "authentication failed"
	case http.StatusPaymentRequired:
		return "insufficient balance"
	case http.StatusUnprocessableEntity:
		return "invalid parameters"
	case http.StatusTooManyRequests:
		return "rate limit reached"
	case http.StatusInternalServerError:
		return "server error"
	case http.StatusServiceUnavailable:
		return "service unavailable"
	default:
		return http.StatusText(statusCode)
	}
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
