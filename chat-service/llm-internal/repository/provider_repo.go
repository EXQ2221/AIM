package llmrepo

import (
	"strings"

	llmmodel "example.com/aim/chat-service/llm-internal/model"
)

type ProviderRepository interface {
	ListProviders() map[string]llmmodel.Config
	GetProvider(name string) (llmmodel.Config, bool)
}

type MemoryProviderRepository struct {
	providers map[string]llmmodel.Config
}

func NewMemoryProviderRepository(providers map[string]llmmodel.Config) *MemoryProviderRepository {
	items := make(map[string]llmmodel.Config, len(providers))
	for name, cfg := range providers {
		normalized := strings.TrimSpace(name)
		if normalized == "" {
			continue
		}
		items[normalized] = cfg
	}
	return &MemoryProviderRepository{providers: items}
}

func (r *MemoryProviderRepository) ListProviders() map[string]llmmodel.Config {
	if r == nil {
		return nil
	}
	items := make(map[string]llmmodel.Config, len(r.providers))
	for name, cfg := range r.providers {
		items[name] = cfg
	}
	return items
}

func (r *MemoryProviderRepository) GetProvider(name string) (llmmodel.Config, bool) {
	if r == nil {
		return llmmodel.Config{}, false
	}
	cfg, ok := r.providers[strings.TrimSpace(name)]
	return cfg, ok
}
