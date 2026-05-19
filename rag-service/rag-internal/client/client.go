package embedding

import (
	ragconf "example.com/aim/rag-service/rag-internal/conf"
	ragdal "example.com/aim/rag-service/rag-internal/dal"
	raghandler "example.com/aim/rag-service/rag-internal/handler"
	ragmodel "example.com/aim/rag-service/rag-internal/model"
	ragrepo "example.com/aim/rag-service/rag-internal/repository"
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

type OpenAICompatibleClient = ragdal.OpenAICompatibleClient
type DashScopeMultimodalClient = ragdal.DashScopeMultimodalClient
type HTTPStatusError = ragdal.HTTPStatusError

func NewOpenAICompatibleClient(cfg Config) (*OpenAICompatibleClient, error) {
	return ragdal.NewOpenAICompatibleClient(cfg)
}

func NewDashScopeMultimodalClient(cfg Config) (*DashScopeMultimodalClient, error) {
	return ragdal.NewDashScopeMultimodalClient(cfg)
}

func NewClient(cfg Config) (Client, error) {
	provider := raghandler.NormalizeProvider(cfg.Provider)
	repo := ragrepo.NewMemoryProviderRepository(map[ragmodel.Provider]ragmodel.Config{
		provider: cfg,
	})

	selected, ok := repo.GetProvider(provider)
	if !ok {
		return nil, errUnsupportedProvider
	}

	switch provider {
	case ragmodel.ProviderOpenAICompatible:
		base, err := ragdal.NewOpenAICompatibleClient(selected)
		if err != nil {
			return nil, err
		}
		return wrapWithRetry(base, selected), nil
	case ragmodel.ProviderDashScopeMultimodal:
		base, err := ragdal.NewDashScopeMultimodalClient(selected)
		if err != nil {
			return nil, err
		}
		return wrapWithRetry(base, selected), nil
	default:
		return nil, errUnsupportedProvider
	}
}

var errUnsupportedProvider = ragdal.ErrUnsupportedProvider
