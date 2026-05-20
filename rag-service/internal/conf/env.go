package ragconf

import (
	"os"
	"strconv"
	"strings"
	"time"

	ragmodel "example.com/aim/rag-service/internal/dal/model"
	"example.com/aim/rag-service/internal/errx"
)

func LoadConfigFromEnv() (ragmodel.Config, error) {
	modelName := strings.TrimSpace(os.Getenv("EMBEDDING_MODEL"))
	cfg := ragmodel.Config{
		Provider:     ragmodel.ProviderOpenAICompatible,
		BaseURL:      strings.TrimSpace(os.Getenv("EMBEDDING_BASE_URL")),
		APIKey:       strings.TrimSpace(os.Getenv("EMBEDDING_API_KEY")),
		Model:        modelName,
		Dimension:    defaultDimensionForModel(modelName),
		Timeout:      ragmodel.DefaultTimeout,
		MaxRetries:   ragmodel.DefaultMaxRetries,
		RetryBackoff: ragmodel.DefaultRetryBackoff,
	}

	if cfg.BaseURL == "" {
		return ragmodel.Config{}, errx.Required("EMBEDDING_BASE_URL")
	}
	if cfg.APIKey == "" {
		return ragmodel.Config{}, errx.Required("EMBEDDING_API_KEY")
	}
	if cfg.Model == "" {
		return ragmodel.Config{}, errx.Required("EMBEDDING_MODEL")
	}

	if isTextEmbeddingModel(cfg.Model) {
		cfg.Provider = ragmodel.ProviderOpenAICompatible
	} else if isDashScopeMultimodalModel(cfg.Model) {
		cfg.Provider = ragmodel.ProviderDashScopeMultimodal
	} else if provider := strings.TrimSpace(os.Getenv("EMBEDDING_PROVIDER")); provider != "" {
		cfg.Provider = ragmodel.Provider(strings.ToLower(provider))
	}

	if dimText := strings.TrimSpace(os.Getenv("EMBEDDING_DIMENSION")); dimText != "" {
		value, err := strconv.Atoi(dimText)
		if err != nil || value <= 0 {
			return ragmodel.Config{}, errx.MustPositiveInteger("EMBEDDING_DIMENSION")
		}
		cfg.Dimension = value
	}

	if timeoutText := strings.TrimSpace(os.Getenv("EMBEDDING_TIMEOUT_SECONDS")); timeoutText != "" {
		value, err := strconv.Atoi(timeoutText)
		if err != nil || value <= 0 {
			return ragmodel.Config{}, errx.MustPositiveInteger("EMBEDDING_TIMEOUT_SECONDS")
		}
		cfg.Timeout = time.Duration(value) * time.Second
	}
	if retryText := strings.TrimSpace(os.Getenv("EMBEDDING_MAX_RETRIES")); retryText != "" {
		value, err := strconv.Atoi(retryText)
		if err != nil || value < 0 {
			return ragmodel.Config{}, errx.MustNonNegativeInteger("EMBEDDING_MAX_RETRIES")
		}
		cfg.MaxRetries = value
	}
	if backoffText := strings.TrimSpace(os.Getenv("EMBEDDING_RETRY_BACKOFF_MS")); backoffText != "" {
		value, err := strconv.Atoi(backoffText)
		if err != nil || value <= 0 {
			return ragmodel.Config{}, errx.MustPositiveInteger("EMBEDDING_RETRY_BACKOFF_MS")
		}
		cfg.RetryBackoff = time.Duration(value) * time.Millisecond
	}
	if insecureText := strings.TrimSpace(os.Getenv("EMBEDDING_INSECURE_SKIP_VERIFY")); insecureText != "" {
		value, err := strconv.ParseBool(insecureText)
		if err != nil {
			return ragmodel.Config{}, errx.MustBeBool("EMBEDDING_INSECURE_SKIP_VERIFY")
		}
		cfg.InsecureSkipVerify = value
	}

	return cfg, nil
}

func LoadSplitterConfigFromEnv() (ragmodel.SplitterConfig, error) {
	cfg := ragmodel.SplitterConfig{
		ChunkSize:    1000,
		ChunkOverlap: 150,
	}

	if value := strings.TrimSpace(os.Getenv("RAG_CHUNK_SIZE")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			return ragmodel.SplitterConfig{}, errx.MustPositiveInteger("RAG_CHUNK_SIZE")
		}
		cfg.ChunkSize = parsed
	}

	if value := strings.TrimSpace(os.Getenv("RAG_CHUNK_OVERLAP")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			return ragmodel.SplitterConfig{}, errx.MustNonNegativeInteger("RAG_CHUNK_OVERLAP")
		}
		cfg.ChunkOverlap = parsed
	}
	if cfg.ChunkOverlap >= cfg.ChunkSize {
		return ragmodel.SplitterConfig{}, errx.MustBeSmaller("RAG_CHUNK_OVERLAP", "RAG_CHUNK_SIZE")
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

func isTextEmbeddingModel(modelName string) bool {
	name := strings.ToLower(strings.TrimSpace(modelName))
	return strings.HasPrefix(name, "text-embedding-")
}

func defaultDimensionForModel(modelName string) int {
	name := strings.ToLower(strings.TrimSpace(modelName))
	switch name {
	case "text-embedding-v4":
		return 1024
	case "text-embedding-v3":
		return 1024
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
