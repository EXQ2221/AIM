package main

import (
	"context"
	"log"
	"os"
	"strings"

	"example.com/aim/gateway/internal/handler"
	"example.com/aim/gateway/internal/router"
	"example.com/aim/gateway/internal/rpc"
	redisv9 "github.com/redis/go-redis/v9"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := rpc.InitAuthClient(getenv("AUTH_SERVICE_ADDR", "127.0.0.1:9002")); err != nil {
		log.Fatal(err)
	}
	if err := rpc.InitUserClient(getenv("USER_SERVICE_ADDR", "127.0.0.1:9001")); err != nil {
		log.Fatal(err)
	}
	if err := rpc.InitChatClient(getenv("CHAT_SERVICE_ADDR", "127.0.0.1:9003")); err != nil {
		log.Fatal(err)
	}
	if redisAddr := strings.TrimSpace(os.Getenv("REDIS_ADDR")); redisAddr != "" {
		redisClient := redisv9.NewClient(&redisv9.Options{Addr: redisAddr})
		if err := redisClient.Ping(ctx).Err(); err != nil {
			log.Printf("gateway redis unavailable, bot reply subscriber disabled: %v", err)
		} else {
			defer redisClient.Close()
			handler.StartBotReplySubscriber(ctx, redisClient)
			handler.StartFriendSyncSubscriber(ctx, redisClient)
		}
	}

	addr := ":" + getenv("PORT", "8080")
	log.Printf("gateway listening on %s", addr)

	if err := router.New().Run(addr); err != nil {
		log.Fatal(err)
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
