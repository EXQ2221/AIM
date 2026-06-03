package llmprofile

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"example.com/aim/gateway/internal/rpc"
	chatpb "example.com/aim/gateway/kitex_gen/chat"
)

type Config struct {
	BaseURL            string
	APIKey             string
	Model              string
	Timeout            time.Duration
	InsecureSkipVerify bool
	Provider           string
	BotName            string
}

func ResolveFromConversationBotOrEnv(ctx context.Context, operatorID int64, conversationID string, botID *int64) (Config, error) {
	if botID == nil || *botID <= 0 || strings.TrimSpace(conversationID) == "" {
		return DefaultConfigFromEnv(), nil
	}
	client, err := rpc.ChatClient()
	if err != nil {
		return Config{}, err
	}
	resp, err := client.ListConversationBots(ctx, &chatpb.ListConversationBotsRequest{
		OperatorId:     operatorID,
		ConversationId: strings.TrimSpace(conversationID),
	})
	if err != nil {
		return Config{}, err
	}
	for _, item := range resp.GetBots() {
		if item == nil || item.BotId != *botID {
			continue
		}
		modelName := strings.TrimSpace(item.ModelName)
		if modelName == "" {
			break
		}
		cfg, cfgErr := providerConfigFromModelName(modelName)
		if cfgErr != nil {
			return Config{}, cfgErr
		}
		cfg.Model = modelName
		cfg.BotName = strings.TrimSpace(item.DisplayName)
		if cfg.BotName == "" {
			cfg.BotName = strings.TrimSpace(item.Name)
		}
		return cfg, nil
	}
	return Config{}, fmt.Errorf("bot %d not found in conversation %s", *botID, strings.TrimSpace(conversationID))
}

func DefaultConfigFromEnv() Config {
	return Config{
		BaseURL:            strings.TrimRight(getenvFirstNonEmpty("LLM_BASE_URL", "SUMMARY_LLM_BASE_URL", "CHUNKER_BASE_URL", "https://api.deepseek.com"), "/"),
		APIKey:             getenvFirstNonEmpty("LLM_API_KEY", "SUMMARY_LLM_API_KEY", "CHUNKER_API_KEY", "DEEPSEEK_API_KEY"),
		Model:              getenvFirstNonEmpty("LLM_MODEL", "SUMMARY_LLM_MODEL", "CHUNKER_MODEL", "deepseek-chat"),
		Timeout:            getenvDuration("LLM_TIMEOUT_SECONDS", 30*time.Second),
		InsecureSkipVerify: getenvBool("LLM_INSECURE_SKIP_VERIFY", getenvBool("SUMMARY_LLM_INSECURE_SKIP_VERIFY", false)),
		Provider:           "primary",
	}
}

func providerConfigFromModelName(modelName string) (Config, error) {
	provider := providerByModelName(modelName)
	if provider == "secondary" {
		baseURL := strings.TrimRight(getenvFirstNonEmpty("LLM2_BASE_URL"), "/")
		apiKey := getenvFirstNonEmpty("LLM2_API_KEY")
		if baseURL == "" || apiKey == "" {
			return Config{}, fmt.Errorf("secondary provider is not configured for model %s", modelName)
		}
		return Config{
			BaseURL:            baseURL,
			APIKey:             apiKey,
			Model:              strings.TrimSpace(modelName),
			Timeout:            getenvDuration("LLM2_TIMEOUT_SECONDS", 30*time.Second),
			InsecureSkipVerify: getenvBool("LLM2_INSECURE_SKIP_VERIFY", false),
			Provider:           "secondary",
		}, nil
	}

	baseURL := strings.TrimRight(getenvFirstNonEmpty("LLM_BASE_URL", "SUMMARY_LLM_BASE_URL", "CHUNKER_BASE_URL", "https://api.deepseek.com"), "/")
	apiKey := getenvFirstNonEmpty("LLM_API_KEY", "SUMMARY_LLM_API_KEY", "CHUNKER_API_KEY", "DEEPSEEK_API_KEY")
	if baseURL == "" || apiKey == "" {
		return Config{}, fmt.Errorf("primary provider is not configured for model %s", modelName)
	}
	return Config{
		BaseURL:            baseURL,
		APIKey:             apiKey,
		Model:              strings.TrimSpace(modelName),
		Timeout:            getenvDuration("LLM_TIMEOUT_SECONDS", 30*time.Second),
		InsecureSkipVerify: getenvBool("LLM_INSECURE_SKIP_VERIFY", getenvBool("SUMMARY_LLM_INSECURE_SKIP_VERIFY", false)),
		Provider:           "primary",
	}, nil
}

func providerByModelName(modelName string) string {
	name := strings.ToLower(strings.TrimSpace(modelName))
	if strings.HasPrefix(name, "qwen") || strings.Contains(name, "tongyi") {
		return "secondary"
	}
	return "primary"
}

func getenvFirstNonEmpty(keys ...string) string {
	for _, key := range keys {
		value := strings.TrimSpace(os.Getenv(key))
		if value != "" {
			return value
		}
	}
	return ""
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func getenvBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
