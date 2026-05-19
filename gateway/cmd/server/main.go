package main

import (
	"context"
	"os"
	"strings"

	"example.com/aim/gateway/internal/handler"
	"example.com/aim/gateway/internal/observability"
	"example.com/aim/gateway/internal/router"
	"example.com/aim/gateway/internal/rpc"
	redisv9 "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	if err := observability.InitLogger("gateway"); err != nil {
		panic(err)
	}
	defer observability.Sync()
	logger := observability.L()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := rpc.InitAuthClient(getenv("AUTH_SERVICE_ADDR", "127.0.0.1:9002")); err != nil {
		logger.Fatal("init auth client failed", zap.Error(err))
	}
	if err := rpc.InitUserClient(getenv("USER_SERVICE_ADDR", "127.0.0.1:9001")); err != nil {
		logger.Fatal("init user client failed", zap.Error(err))
	}
	if err := rpc.InitChatClient(getenv("CHAT_SERVICE_ADDR", "127.0.0.1:9003")); err != nil {
		logger.Fatal("init chat client failed", zap.Error(err))
	}
	if err := rpc.InitRAGClient(getenv("RAG_SERVICE_ADDR", "127.0.0.1:9004")); err != nil {
		logger.Fatal("init rag client failed", zap.Error(err))
	}
	if redisAddr := strings.TrimSpace(os.Getenv("REDIS_ADDR")); redisAddr != "" {
		redisClient := redisv9.NewClient(&redisv9.Options{Addr: redisAddr})
		if err := redisClient.Ping(ctx).Err(); err != nil {
			logger.Warn("gateway redis unavailable, realtime subscribers disabled", zap.Error(err))
		} else {
			defer redisClient.Close()
			handler.StartBotReplySubscriber(ctx, redisClient)
			handler.StartBotReplyStreamSubscriber(ctx, redisClient)
			handler.StartFriendSyncSubscriber(ctx, redisClient)
		}
	}

	addr := ":" + getenv("PORT", "8080")
	logger.Info("gateway listening", zap.String("addr", addr))

	if err := router.New().Run(addr); err != nil {
		logger.Fatal("gateway run failed", zap.Error(err))
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
