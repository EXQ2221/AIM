package model

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

type DocumentType string

const (
	DocumentTypePlainText    DocumentType = "PLAIN_TEXT"
	DocumentTypeMarkdown     DocumentType = "MARKDOWN"
	DocumentTypeScript       DocumentType = "SCRIPT"
	DocumentTypeQuestionBank DocumentType = "QUESTION_BANK"
)

type ChunkMetadata struct {
	DocumentType DocumentType `json:"documentType"`
	SectionTitle string       `json:"sectionTitle,omitempty"`
	HeadingPath  []string     `json:"headingPath,omitempty"`
	QuestionNo   int          `json:"questionNo,omitempty"`
	QuestionText string       `json:"questionText,omitempty"`
	PageStart    int          `json:"pageStart,omitempty"`
	PageEnd      int          `json:"pageEnd,omitempty"`
	CharStart    int          `json:"charStart,omitempty"`
	CharEnd      int          `json:"charEnd,omitempty"`
	Sentences    []SentenceSpan `json:"sentences,omitempty"`
}

type SentenceSpan struct {
	SentenceIndex int    `json:"sentenceIndex"`
	Text          string `json:"text"`
	PageStart     int    `json:"pageStart,omitempty"`
	PageEnd       int    `json:"pageEnd,omitempty"`
	CharStart     int    `json:"charStart,omitempty"`
	CharEnd       int    `json:"charEnd,omitempty"`
}

type Chunk struct {
	Index    int
	Content  string
	Metadata ChunkMetadata
}
