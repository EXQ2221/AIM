package llmbiz

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	llmdal "example.com/aim/chat-service/llm-internal/dal"
	llmhandler "example.com/aim/chat-service/llm-internal/handler"
	llmmodel "example.com/aim/chat-service/llm-internal/model"
	llmrepo "example.com/aim/chat-service/llm-internal/repository"
)

type Registry struct {
	defaultProvider string
	clients         map[string]llmmodel.Client
	configs         map[string]llmmodel.Config
}

func NewRegistry(multiCfg llmmodel.MultiConfig) (*Registry, error) {
	if len(multiCfg.Providers) == 0 {
		return nil, errors.New("no llm providers configured")
	}

	providerRepo := llmrepo.NewMemoryProviderRepository(multiCfg.Providers)
	providers := providerRepo.ListProviders()
	clients := make(map[string]llmmodel.Client, len(providers))
	configs := make(map[string]llmmodel.Config, len(providers))
	for name, cfg := range providers {
		normalized := strings.TrimSpace(name)
		if normalized == "" {
			return nil, errors.New("provider name cannot be empty")
		}
		client, err := llmdal.NewOpenAICompatibleClient(cfg)
		if err != nil {
			return nil, fmt.Errorf("init llm provider %q failed: %w", normalized, err)
		}
		clients[normalized] = client
		configs[normalized] = cfg
	}

	defaultProvider := llmhandler.NormalizeProviderName(multiCfg.DefaultProvider, "primary")
	if _, ok := clients[defaultProvider]; !ok {
		return nil, fmt.Errorf("default provider %q is not configured", defaultProvider)
	}

	return &Registry{
		defaultProvider: defaultProvider,
		clients:         clients,
		configs:         configs,
	}, nil
}

func (r *Registry) DefaultProvider() string {
	if r == nil {
		return ""
	}
	return r.defaultProvider
}

func (r *Registry) Client(provider string) (llmmodel.Client, llmmodel.Config, string, error) {
	if r == nil {
		return nil, llmmodel.Config{}, "", errors.New("llm registry is nil")
	}
	name := llmhandler.NormalizeProviderName(provider, r.defaultProvider)
	client, ok := r.clients[name]
	if !ok {
		return nil, llmmodel.Config{}, "", fmt.Errorf("llm provider %q not found", name)
	}
	return client, r.configs[name], name, nil
}

func (r *Registry) ProviderNames() []string {
	if r == nil {
		return nil
	}
	names := make([]string, 0, len(r.clients))
	for name := range r.clients {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
