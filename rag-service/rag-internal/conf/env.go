package ragconf

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	ragmodel "example.com/aim/rag-service/rag-internal/model"
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
		return ragmodel.Config{}, errors.New("EMBEDDING_BASE_URL is required")
	}
	if cfg.APIKey == "" {
		return ragmodel.Config{}, errors.New("EMBEDDING_API_KEY is required")
	}
	if cfg.Model == "" {
		return ragmodel.Config{}, errors.New("EMBEDDING_MODEL is required")
	}

	if provider := strings.TrimSpace(os.Getenv("EMBEDDING_PROVIDER")); provider != "" {
		cfg.Provider = ragmodel.Provider(strings.ToLower(provider))
	} else if isDashScopeMultimodalModel(cfg.Model) {
		cfg.Provider = ragmodel.ProviderDashScopeMultimodal
	}

	if dimText := strings.TrimSpace(os.Getenv("EMBEDDING_DIMENSION")); dimText != "" {
		value, err := strconv.Atoi(dimText)
		if err != nil || value <= 0 {
			return ragmodel.Config{}, errors.New("EMBEDDING_DIMENSION must be a positive integer")
		}
		cfg.Dimension = value
	}

	if timeoutText := strings.TrimSpace(os.Getenv("EMBEDDING_TIMEOUT_SECONDS")); timeoutText != "" {
		value, err := strconv.Atoi(timeoutText)
		if err != nil || value <= 0 {
			return ragmodel.Config{}, errors.New("EMBEDDING_TIMEOUT_SECONDS must be a positive integer")
		}
		cfg.Timeout = time.Duration(value) * time.Second
	}
	if retryText := strings.TrimSpace(os.Getenv("EMBEDDING_MAX_RETRIES")); retryText != "" {
		value, err := strconv.Atoi(retryText)
		if err != nil || value < 0 {
			return ragmodel.Config{}, errors.New("EMBEDDING_MAX_RETRIES must be a non-negative integer")
		}
		cfg.MaxRetries = value
	}
	if backoffText := strings.TrimSpace(os.Getenv("EMBEDDING_RETRY_BACKOFF_MS")); backoffText != "" {
		value, err := strconv.Atoi(backoffText)
		if err != nil || value <= 0 {
			return ragmodel.Config{}, errors.New("EMBEDDING_RETRY_BACKOFF_MS must be a positive integer")
		}
		cfg.RetryBackoff = time.Duration(value) * time.Millisecond
	}
	if insecureText := strings.TrimSpace(os.Getenv("EMBEDDING_INSECURE_SKIP_VERIFY")); insecureText != "" {
		value, err := strconv.ParseBool(insecureText)
		if err != nil {
			return ragmodel.Config{}, errors.New("EMBEDDING_INSECURE_SKIP_VERIFY must be true or false")
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
			return ragmodel.SplitterConfig{}, errors.New("RAG_CHUNK_SIZE must be a positive integer")
		}
		cfg.ChunkSize = parsed
	}

	if value := strings.TrimSpace(os.Getenv("RAG_CHUNK_OVERLAP")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			return ragmodel.SplitterConfig{}, errors.New("RAG_CHUNK_OVERLAP must be a non-negative integer")
		}
		cfg.ChunkOverlap = parsed
	}
	if cfg.ChunkOverlap >= cfg.ChunkSize {
		return ragmodel.SplitterConfig{}, errors.New("RAG_CHUNK_OVERLAP must be smaller than RAG_CHUNK_SIZE")
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
