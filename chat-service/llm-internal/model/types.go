package llmmodel

import (
	"context"
	"time"
)

const DefaultTimeout = 30 * time.Second

type ChatMessage struct {
	Role    string
	Content string
	Parts   []ChatMessagePart
}

type ChatMessagePart struct {
	Type     string
	Text     string
	ImageURL string
}

type GenerateRequest struct {
	Model    string
	Messages []ChatMessage
}

type StreamChunk struct {
	Content          string
	ReasoningContent string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

type GenerateResponse struct {
	Content          string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

type Client interface {
	Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error)
}

type StreamingClient interface {
	GenerateStream(ctx context.Context, req GenerateRequest, onChunk func(StreamChunk) error) (*GenerateResponse, error)
}

type Config struct {
	BaseURL            string
	APIKey             string
	Model              string
	Timeout            time.Duration
	InsecureSkipVerify bool
	EnableSearch       bool
	ForceSearch        bool
	SearchStrategy     string
	EnableThinking     *bool
}

type MultiConfig struct {
	DefaultProvider string
	Providers       map[string]Config
}
