package rag

import (
	"os"
	"strconv"
	"time"

	"example.com/aim/rag-service/internal/observability"
	embedding "example.com/aim/rag-service/internal/provider"
	"example.com/aim/rag-service/internal/repository"
	"go.uber.org/zap"
)

func NewServiceFromEnv(
	ragRepo repository.RAGRepository,
	conversationRepo repository.ConversationRepository,
	memberRepo repository.MemberRepository,
	notificationRepo repository.NotificationRepository,
) *RAGService {
	logger := observability.L()

	cfg, err := embedding.LoadConfigFromEnv()
	if err != nil {
		logger.Warn("rag service disabled: invalid embedding config", zap.Error(err))
		return nil
	}
	embedClient, err := embedding.NewClient(cfg)
	if err != nil {
		logger.Warn("rag service disabled: create embedding client failed", zap.Error(err))
		return nil
	}
	splitterCfg, err := LoadSplitterConfigFromEnv()
	if err != nil {
		logger.Warn("rag service disabled: invalid splitter config", zap.Error(err))
		return nil
	}
	processor := NewDocumentProcessor(embedClient, ragRepo, splitterCfg)
	service := NewRAGService(ragRepo, conversationRepo, memberRepo, notificationRepo, embedClient, processor)
	service.DefaultTopK = ragTopKFromEnv()
	service.SearchTimeout = ragSearchTimeoutFromEnv()
	service.ProcessTimeout = ragProcessTimeoutFromEnv()
	logger.Info(
		"rag service enabled",
		zap.String("provider", string(cfg.Provider)),
		zap.String("model", cfg.Model),
		zap.Int64("embedding_timeout_ms", cfg.Timeout.Milliseconds()),
		zap.Int("embedding_max_retries", cfg.MaxRetries),
		zap.Int64("embedding_retry_backoff_ms", cfg.RetryBackoff.Milliseconds()),
		zap.Int("chunk_size", splitterCfg.ChunkSize),
		zap.Int("chunk_overlap", splitterCfg.ChunkOverlap),
		zap.Int("top_k", service.DefaultTopK),
		zap.Int64("document_process_timeout_ms", service.ProcessTimeout.Milliseconds()),
	)
	return service
}

func ragTopKFromEnv() int {
	value := intFromEnv("RAG_TOP_K", 8)
	if value < 1 {
		return 1
	}
	if value > 10 {
		return 10
	}
	return value
}

func ragSearchTimeoutFromEnv() time.Duration {
	seconds := intFromEnv("RAG_SEARCH_TIMEOUT_SECONDS", 80)
	if seconds <= 0 {
		seconds = 40
	}
	return time.Duration(seconds) * time.Second
}

func ragProcessTimeoutFromEnv() time.Duration {
	seconds := intFromEnv("RAG_DOCUMENT_PROCESS_TIMEOUT_SECONDS", 900)
	if seconds <= 0 {
		seconds = int(defaultDocumentProcessTimeout.Seconds())
	}
	return time.Duration(seconds) * time.Second
}

func intFromEnv(key string, fallback int) int {
	value := getenv(key, "")
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		observability.L().Warn(
			"invalid env fallback applied",
			zap.String("key", key),
			zap.String("value", value),
			zap.Int("fallback", fallback),
		)
		return fallback
	}
	return parsed
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
