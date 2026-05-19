package ragmodel

import (
	"context"
	"time"
)

const (
	DefaultTimeout      = 60 * time.Second
	DefaultMaxRetries   = 2
	DefaultRetryBackoff = 1200 * time.Millisecond
)

type Provider string

const (
	ProviderOpenAICompatible    Provider = "openai_compatible"
	ProviderDashScopeMultimodal Provider = "dashscope_multimodal"
)

type InputPartType string

const (
	InputPartText  InputPartType = "text"
	InputPartImage InputPartType = "image"
	InputPartVideo InputPartType = "video"
)

type InputPart struct {
	Type  InputPartType
	Text  string
	Image string
	Video string
}

type EmbedRequest struct {
	Model string
	Input []InputPart
}

type EmbedResponse struct {
	Embeddings   [][]float32
	PromptTokens int
	TotalTokens  int
}

type Client interface {
	Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error)
}

type Config struct {
	Provider           Provider
	BaseURL            string
	APIKey             string
	Model              string
	Dimension          int
	Timeout            time.Duration
	MaxRetries         int
	RetryBackoff       time.Duration
	InsecureSkipVerify bool
}

type SplitterConfig struct {
	ChunkSize    int
	ChunkOverlap int
}

type Chunk struct {
	Index   int
	Content string
}
