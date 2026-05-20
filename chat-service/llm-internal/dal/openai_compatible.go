package llmdal

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	llmconf "example.com/aim/chat-service/llm-internal/conf"
	llmmodel "example.com/aim/chat-service/llm-internal/model"
	"example.com/aim/shared/errno"
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
	baseURL        string
	apiKey         string
	model          string
	enableSearch   bool
	forceSearch    bool
	searchStrategy string
	enableThinking *bool
	httpClient     *http.Client
}

func NewOpenAICompatibleClient(cfg llmmodel.Config) (*OpenAICompatibleClient, error) {
	baseURL := strings.TrimSpace(cfg.BaseURL)
	apiKey := strings.TrimSpace(cfg.APIKey)
	model := strings.TrimSpace(cfg.Model)
	if baseURL == "" {
		return nil, errno.Required("LLM_BASE_URL")
	}
	if apiKey == "" {
		return nil, errno.Required("LLM_API_KEY")
	}
	if model == "" {
		return nil, errno.Required("LLM_MODEL")
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = llmmodel.DefaultTimeout
	}
	return &OpenAICompatibleClient{
		baseURL:        strings.TrimRight(baseURL, "/"),
		apiKey:         apiKey,
		model:          model,
		enableSearch:   cfg.EnableSearch,
		forceSearch:    cfg.ForceSearch,
		searchStrategy: strings.TrimSpace(cfg.SearchStrategy),
		enableThinking: cfg.EnableThinking,
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.InsecureSkipVerify}, //nolint:gosec
			},
		},
	}, nil
}

func NewOpenAICompatibleClientFromEnv() (*OpenAICompatibleClient, error) {
	cfg, err := llmconf.LoadConfigFromEnv()
	if err != nil {
		return nil, err
	}
	return NewOpenAICompatibleClient(cfg)
}

func (c *OpenAICompatibleClient) Generate(ctx context.Context, req llmmodel.GenerateRequest) (*llmmodel.GenerateResponse, error) {
	if c == nil {
		return nil, errno.Internal("llm client is nil")
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = c.model
	}
	if model == "" {
		return nil, errno.Required("model")
	}
	if len(req.Messages) == 0 {
		return nil, errno.Required("messages")
	}

	payload := chatCompletionRequest{
		Model:    model,
		Messages: make([]chatCompletionMessage, 0, len(req.Messages)),
		Stream:   false,
	}
	c.applyExtraBody(&payload)
	for _, message := range req.Messages {
		role := strings.TrimSpace(message.Role)
		if role == "" {
			return nil, errno.Required("message role")
		}
		trimmedContent := strings.TrimSpace(message.Content)
		if len(message.Parts) == 0 {
			payload.Messages = append(payload.Messages, chatCompletionMessage{
				Role:    role,
				Content: trimmedContent,
			})
			continue
		}

		parts := make([]chatCompletionMessagePart, 0, len(message.Parts))
		for _, part := range message.Parts {
			partType := strings.TrimSpace(part.Type)
			switch partType {
			case "text":
				text := strings.TrimSpace(part.Text)
				if text == "" {
					continue
				}
				parts = append(parts, chatCompletionMessagePart{
					Type: "text",
					Text: text,
				})
			case "image_url":
				imageURL := strings.TrimSpace(part.ImageURL)
				if imageURL == "" {
					continue
				}
				parts = append(parts, chatCompletionMessagePart{
					Type: "image_url",
					ImageURL: &chatCompletionImageURL{
						URL: imageURL,
					},
				})
			default:
				continue
			}
		}
		if len(parts) == 0 {
			payload.Messages = append(payload.Messages, chatCompletionMessage{
				Role:    role,
				Content: trimmedContent,
			})
			continue
		}
		payload.Messages = append(payload.Messages, chatCompletionMessage{
			Role:    role,
			Content: parts,
		})
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	log.Printf(
		"llm request start: model=%s stream=false enable_search=%t force_search=%t search_strategy=%s enable_thinking=%s",
		model, c.enableSearch, c.forceSearch, c.searchStrategy, formatOptionalBool(c.enableThinking),
	)
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
		return nil, errno.Internal("llm response choices is empty")
	}
	content := parsed.Choices[0].Message.Content
	return &llmmodel.GenerateResponse{
		Content:          content,
		PromptTokens:     parsed.Usage.PromptTokens,
		CompletionTokens: parsed.Usage.CompletionTokens,
		TotalTokens:      parsed.Usage.TotalTokens,
	}, nil
}

func (c *OpenAICompatibleClient) GenerateStream(ctx context.Context, req llmmodel.GenerateRequest, onChunk func(llmmodel.StreamChunk) error) (*llmmodel.GenerateResponse, error) {
	if c == nil {
		return nil, errno.Internal("llm client is nil")
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = c.model
	}
	if model == "" {
		return nil, errno.Required("model")
	}
	if len(req.Messages) == 0 {
		return nil, errno.Required("messages")
	}

	payload := chatCompletionRequest{
		Model:    model,
		Messages: make([]chatCompletionMessage, 0, len(req.Messages)),
		Stream:   true,
		StreamOptions: &chatCompletionStreamOptions{
			IncludeUsage: true,
		},
	}
	c.applyExtraBody(&payload)
	for _, message := range req.Messages {
		role := strings.TrimSpace(message.Role)
		if role == "" {
			return nil, errno.Required("message role")
		}
		trimmedContent := strings.TrimSpace(message.Content)
		if len(message.Parts) == 0 {
			payload.Messages = append(payload.Messages, chatCompletionMessage{
				Role:    role,
				Content: trimmedContent,
			})
			continue
		}

		parts := make([]chatCompletionMessagePart, 0, len(message.Parts))
		for _, part := range message.Parts {
			partType := strings.TrimSpace(part.Type)
			switch partType {
			case "text":
				text := strings.TrimSpace(part.Text)
				if text == "" {
					continue
				}
				parts = append(parts, chatCompletionMessagePart{
					Type: "text",
					Text: text,
				})
			case "image_url":
				imageURL := strings.TrimSpace(part.ImageURL)
				if imageURL == "" {
					continue
				}
				parts = append(parts, chatCompletionMessagePart{
					Type: "image_url",
					ImageURL: &chatCompletionImageURL{
						URL: imageURL,
					},
				})
			default:
				continue
			}
		}
		if len(parts) == 0 {
			payload.Messages = append(payload.Messages, chatCompletionMessage{
				Role:    role,
				Content: trimmedContent,
			})
			continue
		}
		payload.Messages = append(payload.Messages, chatCompletionMessage{
			Role:    role,
			Content: parts,
		})
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	log.Printf(
		"llm request start: model=%s stream=true enable_search=%t force_search=%t search_strategy=%s enable_thinking=%s",
		model, c.enableSearch, c.forceSearch, c.searchStrategy, formatOptionalBool(c.enableThinking),
	)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("llm request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		if readErr != nil {
			return nil, fmt.Errorf("read llm response failed: %w", readErr)
		}
		return nil, &HTTPStatusError{
			StatusCode: resp.StatusCode,
			StatusText: statusHint(resp.StatusCode),
			Body:       safeBodyExcerpt(respBody),
		}
	}

	var (
		contentBuilder strings.Builder
		usage          streamUsage
	)
	reader := bufio.NewReader(resp.Body)
	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return nil, fmt.Errorf("read llm stream failed: %w", readErr)
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
			if data == "[DONE]" {
				break
			}
			if data != "" {
				var chunk chatCompletionStreamChunk
				if err := json.Unmarshal([]byte(data), &chunk); err != nil {
					return nil, fmt.Errorf("parse llm stream chunk failed: %w", err)
				}
				if chunk.Usage != nil {
					usage = *chunk.Usage
				}
				var deltaText, reasoningText string
				if len(chunk.Choices) > 0 {
					deltaText = chunk.Choices[0].Delta.Content
					reasoningText = chunk.Choices[0].Delta.ReasoningContent
				}
				if deltaText != "" {
					contentBuilder.WriteString(deltaText)
				}
				if onChunk != nil && (deltaText != "" || reasoningText != "" || chunk.Usage != nil) {
					if err := onChunk(llmmodel.StreamChunk{
						Content:          deltaText,
						ReasoningContent: reasoningText,
						PromptTokens:     usage.PromptTokens,
						CompletionTokens: usage.CompletionTokens,
						TotalTokens:      usage.TotalTokens,
					}); err != nil {
						return nil, err
					}
				}
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
	}

	return &llmmodel.GenerateResponse{
		Content:          contentBuilder.String(),
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	}, nil
}

type chatCompletionRequest struct {
	Model          string                       `json:"model"`
	Messages       []chatCompletionMessage      `json:"messages"`
	Stream         bool                         `json:"stream"`
	StreamOptions  *chatCompletionStreamOptions `json:"stream_options,omitempty"`
	EnableThinking *bool                        `json:"enable_thinking,omitempty"`
	EnableSearch   *bool                        `json:"enable_search,omitempty"`
	SearchOptions  *chatCompletionSearchOptions `json:"search_options,omitempty"`
}

type chatCompletionSearchOptions struct {
	ForcedSearch   *bool  `json:"forced_search,omitempty"`
	SearchStrategy string `json:"search_strategy,omitempty"`
}

type chatCompletionStreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

type chatCompletionMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content,omitempty"`
}

type chatCompletionMessagePart struct {
	Type     string                  `json:"type"`
	Text     string                  `json:"text,omitempty"`
	ImageURL *chatCompletionImageURL `json:"image_url,omitempty"`
}

type chatCompletionImageURL struct {
	URL string `json:"url"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type chatCompletionStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *streamUsage `json:"usage,omitempty"`
}

type streamUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
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

func (c *OpenAICompatibleClient) applyExtraBody(payload *chatCompletionRequest) {
	if payload == nil || c == nil {
		return
	}
	payload.EnableThinking = c.enableThinking
	if c.enableSearch {
		enableSearch := true
		payload.EnableSearch = &enableSearch
		if c.forceSearch || c.searchStrategy != "" {
			options := &chatCompletionSearchOptions{
				SearchStrategy: c.searchStrategy,
			}
			if c.forceSearch {
				forced := true
				options.ForcedSearch = &forced
			}
			payload.SearchOptions = options
		}
	}
}

func formatOptionalBool(value *bool) string {
	if value == nil {
		return "unset"
	}
	if *value {
		return "true"
	}
	return "false"
}

var _ llmmodel.Client = (*OpenAICompatibleClient)(nil)
var _ llmmodel.StreamingClient = (*OpenAICompatibleClient)(nil)
