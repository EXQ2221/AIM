package llm

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

type Registry struct {
	defaultProvider string
	clients         map[string]Client
	configs         map[string]Config
}

func NewRegistry(multiCfg MultiConfig) (*Registry, error) {
	if len(multiCfg.Providers) == 0 {
		return nil, errors.New("no llm providers configured")
	}

	clients := make(map[string]Client, len(multiCfg.Providers))
	configs := make(map[string]Config, len(multiCfg.Providers))
	for name, cfg := range multiCfg.Providers {
		normalized := strings.TrimSpace(name)
		if normalized == "" {
			return nil, errors.New("provider name cannot be empty")
		}
		client, err := NewOpenAICompatibleClient(cfg)
		if err != nil {
			return nil, fmt.Errorf("init llm provider %q failed: %w", normalized, err)
		}
		clients[normalized] = client
		configs[normalized] = cfg
	}

	defaultProvider := strings.TrimSpace(multiCfg.DefaultProvider)
	if defaultProvider == "" {
		defaultProvider = "primary"
	}
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

func (r *Registry) Client(provider string) (Client, Config, string, error) {
	if r == nil {
		return nil, Config{}, "", errors.New("llm registry is nil")
	}
	name := strings.TrimSpace(provider)
	if name == "" {
		name = r.defaultProvider
	}
	client, ok := r.clients[name]
	if !ok {
		return nil, Config{}, "", fmt.Errorf("llm provider %q not found", name)
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
