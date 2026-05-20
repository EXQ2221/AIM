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
