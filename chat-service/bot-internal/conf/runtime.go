package botconf

import (
	"os"
	"strconv"
	"strings"
	"time"
)

func BotMaxConcurrencyFromEnv() int {
	return intFromEnv("BOT_MAX_CONCURRENCY", 10)
}

func BotMaxConversationConcurrencyFromEnv() int {
	return intFromEnv("BOT_MAX_CONVERSATION_CONCURRENCY", 1)
}

func BotTaskTimeoutFromEnv() time.Duration {
	seconds := intFromEnv("BOT_TASK_TIMEOUT_SECONDS", 120)
	if seconds <= 0 {
		seconds = 30
	}
	return time.Duration(seconds) * time.Second
}

func BotLLMTimeoutFromEnv() time.Duration {
	seconds := intFromEnv("BOT_LLM_TIMEOUT_SECONDS", 30)
	if seconds <= 0 {
		seconds = 30
	}
	return time.Duration(seconds) * time.Second
}

func BotContextMessagesFromEnv() int {
	return intFromEnv("BOT_CONTEXT_MESSAGES", 20)
}

func MemoryEnabledFromEnv() bool {
	value := strings.TrimSpace(os.Getenv("MEMORY_ENABLED"))
	if value == "" {
		return true
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return true
	}
	return parsed
}

func MemoryProviderFromEnv() string {
	value := strings.TrimSpace(os.Getenv("MEMORY_LLM_PROVIDER"))
	if value == "" {
		return "primary"
	}
	return value
}

func MemoryModelFromEnv() string {
	value := strings.TrimSpace(os.Getenv("MEMORY_MODEL"))
	if value == "" {
		return "deepseek-v4-flash"
	}
	return value
}

func MemoryExtractTimeoutFromEnv() time.Duration {
	seconds := intFromEnv("MEMORY_EXTRACT_TIMEOUT_SECONDS", 6)
	if seconds <= 0 {
		seconds = 6
	}
	return time.Duration(seconds) * time.Second
}

func MemoryCandidateLimitFromEnv() int {
	limit := intFromEnv("MEMORY_CANDIDATE_LIMIT", 20)
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return limit
}

func MemoryMaxRecallFromEnv() int {
	limit := intFromEnv("MEMORY_MAX_RECALL", 5)
	if limit <= 0 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}
	return limit
}

func RerankEnabledFromEnv() bool {
	value := strings.TrimSpace(os.Getenv("RERANK_ENABLED"))
	if value == "" {
		return true
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return true
	}
	return parsed
}

func RerankBaseURLFromEnv() string {
	value := strings.TrimSpace(os.Getenv("RERANK_BASE_URL"))
	if value != "" {
		return value
	}
	return "https://dashscope.aliyuncs.com/compatible-api/v1"
}

func RerankAPIKeyFromEnv() string {
	for _, key := range []string{"RERANK_API_KEY", "EMBEDDING_API_KEY", "DASHSCOPE_API_KEY", "LLM2_API_KEY"} {
		value := strings.TrimSpace(os.Getenv(key))
		if value != "" {
			return value
		}
	}
	return ""
}

func RerankModelFromEnv() string {
	value := strings.TrimSpace(os.Getenv("RERANK_MODEL"))
	if value != "" {
		return value
	}
	return "qwen3-rerank"
}

func RerankTimeoutFromEnv() time.Duration {
	seconds := intFromEnv("RERANK_TIMEOUT_SECONDS", 15)
	if seconds <= 0 {
		seconds = 15
	}
	return time.Duration(seconds) * time.Second
}

func RerankScoreThresholdFromEnv() float64 {
	value := strings.TrimSpace(os.Getenv("RERANK_SCORE_THRESHOLD"))
	if value == "" {
		return 0.25
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0.25
	}
	if parsed < 0 {
		return 0
	}
	if parsed > 1 {
		return 1
	}
	return parsed
}

func RerankRecallTopKFromEnv() int {
	limit := intFromEnv("RERANK_RECALL_TOP_K", 30)
	if limit <= 0 {
		limit = 30
	}
	if limit > 500 {
		limit = 500
	}
	return limit
}

func intFromEnv(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
