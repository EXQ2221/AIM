package repository

import (
	"strings"

	ragmodel "example.com/aim/rag-service/internal/dal/model"
)

type ProviderRepository interface {
	GetProvider(name ragmodel.Provider) (ragmodel.Config, bool)
}

type MemoryProviderRepository struct {
	providers map[ragmodel.Provider]ragmodel.Config
}

func NewMemoryProviderRepository(configs map[ragmodel.Provider]ragmodel.Config) *MemoryProviderRepository {
	items := make(map[ragmodel.Provider]ragmodel.Config, len(configs))
	for name, cfg := range configs {
		normalized := ragmodel.Provider(strings.ToLower(strings.TrimSpace(string(name))))
		if normalized == "" {
			continue
		}
		items[normalized] = cfg
	}
	return &MemoryProviderRepository{providers: items}
}

func (r *MemoryProviderRepository) GetProvider(name ragmodel.Provider) (ragmodel.Config, bool) {
	if r == nil {
		return ragmodel.Config{}, false
	}
	cfg, ok := r.providers[ragmodel.Provider(strings.ToLower(strings.TrimSpace(string(name))))]
	return cfg, ok
}
