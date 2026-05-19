package rag

import (
	"os"
	"strconv"
	"time"

	"example.com/aim/rag-service/internal/observability"
	"example.com/aim/rag-service/internal/repository"
	embedding "example.com/aim/rag-service/rag-internal/client"
	"go.uber.org/zap"
)

func NewServiceFromEnv(
	ragRepo repository.RAGRepository,
	conversationRepo repository.ConversationRepository,
	memberRepo repository.MemberRepository,
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
	service := NewRAGService(ragRepo, conversationRepo, memberRepo, embedClient, processor)
	service.DefaultTopK = ragTopKFromEnv()
	service.SearchTimeout = ragSearchTimeoutFromEnv()
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
	)
	return service
}

func ragTopKFromEnv() int {
	value := intFromEnv("RAG_TOP_K", 5)
	if value < 1 {
		return 1
	}
	if value > 10 {
		return 10
	}
	return value
}

func ragSearchTimeoutFromEnv() time.Duration {
	seconds := intFromEnv("RAG_SEARCH_TIMEOUT_SECONDS", 40)
	if seconds <= 0 {
		seconds = 40
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
