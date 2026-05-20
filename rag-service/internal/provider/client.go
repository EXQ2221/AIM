package embedding

import (
	"example.com/aim/rag-service/internal/conf"
	ragmodel "example.com/aim/rag-service/internal/dal/model"
	ragrepo "example.com/aim/rag-service/internal/repository"

	"strings"
)

type Provider = ragmodel.Provider

const (
	ProviderOpenAICompatible    = ragmodel.ProviderOpenAICompatible
	ProviderDashScopeMultimodal = ragmodel.ProviderDashScopeMultimodal
)

type InputPartType = ragmodel.InputPartType

const (
	InputPartText  = ragmodel.InputPartText
	InputPartImage = ragmodel.InputPartImage
	InputPartVideo = ragmodel.InputPartVideo
)

type InputPart = ragmodel.InputPart

type EmbedRequest = ragmodel.EmbedRequest

type EmbedResponse = ragmodel.EmbedResponse

type Client = ragmodel.Client

type Config = ragmodel.Config

func LoadConfigFromEnv() (Config, error) {
	return ragconf.LoadConfigFromEnv()
}

func NewClient(cfg Config) (Client, error) {
	provider := normalizeProvider(cfg.Provider)
	repo := ragrepo.NewMemoryProviderRepository(map[ragmodel.Provider]ragmodel.Config{
		provider: cfg,
	})

	selected, ok := repo.GetProvider(provider)
	if !ok {
		return nil, errUnsupportedProvider
	}

	switch provider {
	case ragmodel.ProviderOpenAICompatible:
		if isTextEmbeddingModel(selected.Model) {
			base, err := NewOpenAITextEmbeddingClient(selected)
			if err != nil {
				return nil, err
			}
			primary := wrapWithRetry(base, selected)
			if fallbackCfg, ok := multimodalFallbackConfig(selected); ok {
				fallbackBase, fallbackErr := NewDashScopeMultimodalClient(fallbackCfg)
				if fallbackErr == nil {
					fallback := wrapWithRetry(fallbackBase, fallbackCfg)
					return &hybridClient{primary: primary, fallback: fallback}, nil
				}
			}
			return primary, nil
		}
		base, err := NewOpenAICompatibleClient(selected)
		if err != nil {
			return nil, err
		}
		return wrapWithRetry(base, selected), nil
	case ragmodel.ProviderDashScopeMultimodal:
		base, err := NewDashScopeMultimodalClient(selected)
		if err != nil {
			return nil, err
		}
		return wrapWithRetry(base, selected), nil
	default:
		return nil, errUnsupportedProvider
	}
}

var errUnsupportedProvider = ErrUnsupportedProvider

func normalizeProvider(value ragmodel.Provider) ragmodel.Provider {
	return ragmodel.Provider(strings.ToLower(strings.TrimSpace(string(value))))
}

func isTextEmbeddingModel(modelName string) bool {
	name := strings.ToLower(strings.TrimSpace(modelName))
	return strings.HasPrefix(name, "text-embedding-")
}

func multimodalFallbackConfig(selected ragmodel.Config) (ragmodel.Config, bool) {
	fallbackCfg := selected
	fallbackCfg.Provider = ragmodel.ProviderDashScopeMultimodal
	fallbackCfg.Model = defaultMultimodalFallbackModel(selected.Dimension)
	if fallbackCfg.Dimension <= 0 {
		fallbackCfg.Dimension = defaultDimensionForModel(fallbackCfg.Model)
	}
	return fallbackCfg, true
}

func defaultMultimodalFallbackModel(dimension int) string {
	switch dimension {
	case 2048, 1536, 1024, 768, 512, 256:
		return "qwen3-vl-embedding"
	default:
		return "qwen3-vl-embedding"
	}
}

func defaultDimensionForModel(modelName string) int {
	switch strings.ToLower(strings.TrimSpace(modelName)) {
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
