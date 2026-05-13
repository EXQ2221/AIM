package embedding

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultTimeout = 30 * time.Second

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
	InsecureSkipVerify bool
}

func LoadConfigFromEnv() (Config, error) {
	modelName := strings.TrimSpace(os.Getenv("EMBEDDING_MODEL"))
	cfg := Config{
		Provider:  ProviderOpenAICompatible,
		BaseURL:   strings.TrimSpace(os.Getenv("EMBEDDING_BASE_URL")),
		APIKey:    strings.TrimSpace(os.Getenv("EMBEDDING_API_KEY")),
		Model:     modelName,
		Dimension: defaultDimensionForModel(modelName),
		Timeout:   defaultTimeout,
	}

	if cfg.BaseURL == "" {
		return Config{}, errors.New("EMBEDDING_BASE_URL is required")
	}
	if cfg.APIKey == "" {
		return Config{}, errors.New("EMBEDDING_API_KEY is required")
	}
	if cfg.Model == "" {
		return Config{}, errors.New("EMBEDDING_MODEL is required")
	}

	if provider := strings.TrimSpace(os.Getenv("EMBEDDING_PROVIDER")); provider != "" {
		cfg.Provider = Provider(strings.ToLower(provider))
	} else if isDashScopeMultimodalModel(cfg.Model) {
		// If user configures a DashScope multimodal embedding model explicitly, default to DashScope mode.
		cfg.Provider = ProviderDashScopeMultimodal
	}

	if dimText := strings.TrimSpace(os.Getenv("EMBEDDING_DIMENSION")); dimText != "" {
		value, err := strconv.Atoi(dimText)
		if err != nil || value <= 0 {
			return Config{}, errors.New("EMBEDDING_DIMENSION must be a positive integer")
		}
		cfg.Dimension = value
	}

	if timeoutText := strings.TrimSpace(os.Getenv("EMBEDDING_TIMEOUT_SECONDS")); timeoutText != "" {
		value, err := strconv.Atoi(timeoutText)
		if err != nil || value <= 0 {
			return Config{}, errors.New("EMBEDDING_TIMEOUT_SECONDS must be a positive integer")
		}
		cfg.Timeout = time.Duration(value) * time.Second
	}
	if insecureText := strings.TrimSpace(os.Getenv("EMBEDDING_INSECURE_SKIP_VERIFY")); insecureText != "" {
		value, err := strconv.ParseBool(insecureText)
		if err != nil {
			return Config{}, errors.New("EMBEDDING_INSECURE_SKIP_VERIFY must be true or false")
		}
		cfg.InsecureSkipVerify = value
	}

	return cfg, nil
}

func isDashScopeMultimodalModel(modelName string) bool {
	name := strings.ToLower(strings.TrimSpace(modelName))
	if name == "" {
		return false
	}
	if strings.Contains(name, "embedding-vision") {
		return true
	}
	if strings.Contains(name, "vl-embedding") {
		return true
	}
	return name == "multimodal-embedding-v1"
}

func defaultDimensionForModel(modelName string) int {
	name := strings.ToLower(strings.TrimSpace(modelName))
	switch name {
	case "qwen3-vl-embedding":
		return 2560
	case "qwen2.5-vl-embedding":
		return 1024
	case "tongyi-embedding-vision-plus":
		return 1152
	case "tongyi-embedding-vision-flash":
		return 768
	case "tongyi-embedding-vision-plus-2026-03-06":
		return 1152
	case "tongyi-embedding-vision-flash-2026-03-06":
		return 768
	case "multimodal-embedding-v1":
		return 1024
	default:
		return 1536
	}
}

func NewClient(cfg Config) (Client, error) {
	switch cfg.Provider {
	case ProviderOpenAICompatible:
		return NewOpenAICompatibleClient(cfg)
	case ProviderDashScopeMultimodal:
		return NewDashScopeMultimodalClient(cfg)
	default:
		return nil, errors.New("unsupported EMBEDDING_PROVIDER")
	}
}
