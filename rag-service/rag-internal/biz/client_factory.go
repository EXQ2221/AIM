package rag

import (
	"errors"

	ragdal "example.com/aim/rag-service/rag-internal/dal"
	raghandler "example.com/aim/rag-service/rag-internal/handler"
	ragmodel "example.com/aim/rag-service/rag-internal/model"
	ragrepo "example.com/aim/rag-service/rag-internal/repository"
)

func NewClient(cfg ragmodel.Config) (ragmodel.Client, error) {
	provider := raghandler.NormalizeProvider(cfg.Provider)
	repo := ragrepo.NewMemoryProviderRepository(map[ragmodel.Provider]ragmodel.Config{
		provider: cfg,
	})

	selected, ok := repo.GetProvider(provider)
	if !ok {
		return nil, errors.New("unsupported EMBEDDING_PROVIDER")
	}

	switch provider {
	case ragmodel.ProviderOpenAICompatible:
		return ragdal.NewOpenAICompatibleClient(selected)
	case ragmodel.ProviderDashScopeMultimodal:
		return ragdal.NewDashScopeMultimodalClient(selected)
	default:
		return nil, errors.New("unsupported EMBEDDING_PROVIDER")
	}
}
