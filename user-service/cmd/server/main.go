package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"example.com/aim/user-service/internal/biz"
	"example.com/aim/user-service/internal/dal/postgres"
	"example.com/aim/user-service/internal/handler"
	"example.com/aim/user-service/internal/realtime"
	"example.com/aim/user-service/internal/repository"
	"example.com/aim/user-service/internal/rpc"
	"example.com/aim/user-service/kitex_gen/user/userservice"
	"github.com/cloudwego/kitex/server"
	redisv9 "github.com/redis/go-redis/v9"
)

func main() {
	ctx := context.Background()
	rpcAddr := ":" + getenv("PORT", "9001")
	healthAddr := ":" + getenv("HEALTH_PORT", "19001")
	postgresDSN := mustGetenv("USER_POSTGRES_DSN")

	db, err := postgres.Init(postgresDSN)
	if err != nil {
		log.Fatal(err)
	}

	service := biz.NewUserService(
		repository.NewUserRepository(db),
		repository.NewFriendGroupRepository(db),
		repository.NewFriendRelationRepository(db),
		repository.NewFriendRequestRepository(db),
		repository.NewTxManager(db),
	)
	chatClient, err := rpc.NewChatClient(getenv("CHAT_SERVICE_ADDR", "127.0.0.1:9003"))
	if err != nil {
		log.Fatal(err)
	}
	service.SetChatClient(chatClient)
	redisClient := newRedisClient(ctx, strings.TrimSpace(os.Getenv("REDIS_ADDR")))
	if redisClient != nil {
		defer redisClient.Close()
		service.SetFriendEventPublisher(realtime.NewRedisFriendSyncPublisher(redisClient))
	}
	if err := service.SeedDemoUser(ctx); err != nil {
		log.Fatal(err)
	}

	go serveHealth(healthAddr)

	addr, err := net.ResolveTCPAddr("tcp", rpcAddr)
	if err != nil {
		log.Fatal(err)
	}

	svr := userservice.NewServer(
		handler.NewUserServiceImpl(service),
		server.WithServiceAddr(addr),
	)

	log.Printf("user-service kitex listening on %s", rpcAddr)
	if err := svr.Run(); err != nil {
		log.Fatal(err)
	}
}

func serveHealth(addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	log.Printf("user-service health listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func newRedisClient(ctx context.Context, addr string) *redisv9.Client {
	if addr == "" {
		return nil
	}
	client := redisv9.NewClient(&redisv9.Options{Addr: addr})
	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("user-service redis unavailable, friend sync publisher disabled: %v", err)
		return nil
	}
	return client
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func mustGetenv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("missing required environment variable %s", key)
	}
	return value
}
