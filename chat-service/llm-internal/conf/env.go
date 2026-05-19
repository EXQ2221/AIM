package llmconf

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	llmmodel "example.com/aim/chat-service/llm-internal/model"
)

func LoadConfigFromEnv() (llmmodel.Config, error) {
	cfg := llmmodel.Config{
		BaseURL: strings.TrimSpace(os.Getenv("LLM_BASE_URL")),
		APIKey:  strings.TrimSpace(os.Getenv("LLM_API_KEY")),
		Model:   strings.TrimSpace(os.Getenv("LLM_MODEL")),
		Timeout: llmmodel.DefaultTimeout,
	}
	if cfg.BaseURL == "" {
		return llmmodel.Config{}, errors.New("LLM_BASE_URL is required")
	}
	if cfg.APIKey == "" {
		return llmmodel.Config{}, errors.New("LLM_API_KEY is required")
	}
	if cfg.Model == "" {
		return llmmodel.Config{}, errors.New("LLM_MODEL is required")
	}

	timeoutValue := strings.TrimSpace(os.Getenv("LLM_TIMEOUT_SECONDS"))
	if timeoutValue != "" {
		seconds, err := strconv.Atoi(timeoutValue)
		if err != nil || seconds <= 0 {
			return llmmodel.Config{}, errors.New("LLM_TIMEOUT_SECONDS must be a positive integer")
		}
		cfg.Timeout = time.Duration(seconds) * time.Second
	}
	if insecureText := strings.TrimSpace(os.Getenv("LLM_INSECURE_SKIP_VERIFY")); insecureText != "" {
		value, err := strconv.ParseBool(insecureText)
		if err != nil {
			return llmmodel.Config{}, errors.New("LLM_INSECURE_SKIP_VERIFY must be true or false")
		}
		cfg.InsecureSkipVerify = value
	}
	if searchText := strings.TrimSpace(os.Getenv("LLM_ENABLE_SEARCH")); searchText != "" {
		value, err := strconv.ParseBool(searchText)
		if err != nil {
			return llmmodel.Config{}, errors.New("LLM_ENABLE_SEARCH must be true or false")
		}
		cfg.EnableSearch = value
	}
	if forceSearchText := strings.TrimSpace(os.Getenv("LLM_FORCE_SEARCH")); forceSearchText != "" {
		value, err := strconv.ParseBool(forceSearchText)
		if err != nil {
			return llmmodel.Config{}, errors.New("LLM_FORCE_SEARCH must be true or false")
		}
		cfg.ForceSearch = value
	}
	cfg.SearchStrategy = strings.TrimSpace(os.Getenv("LLM_SEARCH_STRATEGY"))
	if thinkingText := strings.TrimSpace(os.Getenv("LLM_ENABLE_THINKING")); thinkingText != "" {
		value, err := strconv.ParseBool(thinkingText)
		if err != nil {
			return llmmodel.Config{}, errors.New("LLM_ENABLE_THINKING must be true or false")
		}
		cfg.EnableThinking = &value
	}
	return cfg, nil
}

func LoadMultiConfigFromEnv() (llmmodel.MultiConfig, error) {
	primary, err := LoadConfigFromEnv()
	if err != nil {
		return llmmodel.MultiConfig{}, err
	}
	providers := map[string]llmmodel.Config{
		"primary": primary,
	}

	secondaryBaseURL := strings.TrimSpace(os.Getenv("LLM2_BASE_URL"))
	secondaryAPIKey := strings.TrimSpace(os.Getenv("LLM2_API_KEY"))
	secondaryModel := strings.TrimSpace(os.Getenv("LLM2_MODEL"))
	if secondaryBaseURL != "" || secondaryAPIKey != "" || secondaryModel != "" {
		if secondaryBaseURL == "" || secondaryAPIKey == "" || secondaryModel == "" {
			return llmmodel.MultiConfig{}, errors.New("LLM2_BASE_URL, LLM2_API_KEY and LLM2_MODEL must be set together")
		}
		secondary := llmmodel.Config{
			BaseURL:      secondaryBaseURL,
			APIKey:       secondaryAPIKey,
			Model:        secondaryModel,
			Timeout:      llmmodel.DefaultTimeout,
			EnableSearch: true,
			ForceSearch:  true,
		}
		timeoutValue := strings.TrimSpace(os.Getenv("LLM2_TIMEOUT_SECONDS"))
		if timeoutValue != "" {
			seconds, parseErr := strconv.Atoi(timeoutValue)
			if parseErr != nil || seconds <= 0 {
				return llmmodel.MultiConfig{}, errors.New("LLM2_TIMEOUT_SECONDS must be a positive integer")
			}
			secondary.Timeout = time.Duration(seconds) * time.Second
		}
		if insecureText := strings.TrimSpace(os.Getenv("LLM2_INSECURE_SKIP_VERIFY")); insecureText != "" {
			value, parseErr := strconv.ParseBool(insecureText)
			if parseErr != nil {
				return llmmodel.MultiConfig{}, errors.New("LLM2_INSECURE_SKIP_VERIFY must be true or false")
			}
			secondary.InsecureSkipVerify = value
		}
		if searchText := strings.TrimSpace(os.Getenv("LLM2_ENABLE_SEARCH")); searchText != "" {
			value, parseErr := strconv.ParseBool(searchText)
			if parseErr != nil {
				return llmmodel.MultiConfig{}, errors.New("LLM2_ENABLE_SEARCH must be true or false")
			}
			secondary.EnableSearch = value
		}
		if forceSearchText := strings.TrimSpace(os.Getenv("LLM2_FORCE_SEARCH")); forceSearchText != "" {
			value, parseErr := strconv.ParseBool(forceSearchText)
			if parseErr != nil {
				return llmmodel.MultiConfig{}, errors.New("LLM2_FORCE_SEARCH must be true or false")
			}
			secondary.ForceSearch = value
		}
		secondary.SearchStrategy = strings.TrimSpace(os.Getenv("LLM2_SEARCH_STRATEGY"))
		if thinkingText := strings.TrimSpace(os.Getenv("LLM2_ENABLE_THINKING")); thinkingText != "" {
			value, parseErr := strconv.ParseBool(thinkingText)
			if parseErr != nil {
				return llmmodel.MultiConfig{}, errors.New("LLM2_ENABLE_THINKING must be true or false")
			}
			secondary.EnableThinking = &value
		}
		providers["secondary"] = secondary
	}

	defaultProvider := strings.TrimSpace(os.Getenv("LLM_PROVIDER"))
	if defaultProvider == "" {
		defaultProvider = "primary"
	}
	if _, ok := providers[defaultProvider]; !ok {
		return llmmodel.MultiConfig{}, errors.New("LLM_PROVIDER must reference an existing provider (primary or secondary)")
	}

	return llmmodel.MultiConfig{
		DefaultProvider: defaultProvider,
		Providers:       providers,
	}, nil
}
