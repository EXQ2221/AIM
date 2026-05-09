package llm

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultTimeout = 30 * time.Second

type ChatMessage struct {
	Role    string
	Content string
}

type GenerateRequest struct {
	Model    string
	Messages []ChatMessage
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

type Config struct {
	BaseURL string
	APIKey  string
	Model   string
	Timeout time.Duration
}

func LoadConfigFromEnv() (Config, error) {
	cfg := Config{
		BaseURL: strings.TrimSpace(os.Getenv("LLM_BASE_URL")),
		APIKey:  strings.TrimSpace(os.Getenv("LLM_API_KEY")),
		Model:   strings.TrimSpace(os.Getenv("LLM_MODEL")),
		Timeout: defaultTimeout,
	}
	if cfg.BaseURL == "" {
		return Config{}, errors.New("LLM_BASE_URL is required")
	}
	if cfg.APIKey == "" {
		return Config{}, errors.New("LLM_API_KEY is required")
	}
	if cfg.Model == "" {
		return Config{}, errors.New("LLM_MODEL is required")
	}

	timeoutValue := strings.TrimSpace(os.Getenv("LLM_TIMEOUT_SECONDS"))
	if timeoutValue != "" {
		seconds, err := strconv.Atoi(timeoutValue)
		if err != nil || seconds <= 0 {
			return Config{}, errors.New("LLM_TIMEOUT_SECONDS must be a positive integer")
		}
		cfg.Timeout = time.Duration(seconds) * time.Second
	}
	return cfg, nil
}
