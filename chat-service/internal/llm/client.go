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
	BaseURL            string
	APIKey             string
	Model              string
	Timeout            time.Duration
	InsecureSkipVerify bool
}

type MultiConfig struct {
	DefaultProvider string
	Providers       map[string]Config
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
	if insecureText := strings.TrimSpace(os.Getenv("LLM_INSECURE_SKIP_VERIFY")); insecureText != "" {
		value, err := strconv.ParseBool(insecureText)
		if err != nil {
			return Config{}, errors.New("LLM_INSECURE_SKIP_VERIFY must be true or false")
		}
		cfg.InsecureSkipVerify = value
	}
	return cfg, nil
}

func LoadMultiConfigFromEnv() (MultiConfig, error) {
	primary, err := LoadConfigFromEnv()
	if err != nil {
		return MultiConfig{}, err
	}
	providers := map[string]Config{
		"primary": primary,
	}

	secondaryBaseURL := strings.TrimSpace(os.Getenv("LLM2_BASE_URL"))
	secondaryAPIKey := strings.TrimSpace(os.Getenv("LLM2_API_KEY"))
	secondaryModel := strings.TrimSpace(os.Getenv("LLM2_MODEL"))
	if secondaryBaseURL != "" || secondaryAPIKey != "" || secondaryModel != "" {
		if secondaryBaseURL == "" || secondaryAPIKey == "" || secondaryModel == "" {
			return MultiConfig{}, errors.New("LLM2_BASE_URL, LLM2_API_KEY and LLM2_MODEL must be set together")
		}
		secondary := Config{
			BaseURL: secondaryBaseURL,
			APIKey:  secondaryAPIKey,
			Model:   secondaryModel,
			Timeout: defaultTimeout,
		}
		timeoutValue := strings.TrimSpace(os.Getenv("LLM2_TIMEOUT_SECONDS"))
		if timeoutValue != "" {
			seconds, parseErr := strconv.Atoi(timeoutValue)
			if parseErr != nil || seconds <= 0 {
				return MultiConfig{}, errors.New("LLM2_TIMEOUT_SECONDS must be a positive integer")
			}
			secondary.Timeout = time.Duration(seconds) * time.Second
		}
		if insecureText := strings.TrimSpace(os.Getenv("LLM2_INSECURE_SKIP_VERIFY")); insecureText != "" {
			value, parseErr := strconv.ParseBool(insecureText)
			if parseErr != nil {
				return MultiConfig{}, errors.New("LLM2_INSECURE_SKIP_VERIFY must be true or false")
			}
			secondary.InsecureSkipVerify = value
		}
		providers["secondary"] = secondary
	}

	defaultProvider := strings.TrimSpace(os.Getenv("LLM_PROVIDER"))
	if defaultProvider == "" {
		defaultProvider = "primary"
	}
	if _, ok := providers[defaultProvider]; !ok {
		return MultiConfig{}, errors.New("LLM_PROVIDER must reference an existing provider (primary or secondary)")
	}

	return MultiConfig{
		DefaultProvider: defaultProvider,
		Providers:       providers,
	}, nil
}
