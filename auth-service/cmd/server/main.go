package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"example.com/aim/auth-service/internal/biz"
	pgstore "example.com/aim/auth-service/internal/dal/postgres"
	redisstore "example.com/aim/auth-service/internal/dal/redis"
	"example.com/aim/auth-service/internal/handler"
	"example.com/aim/auth-service/internal/repository"
	"example.com/aim/auth-service/internal/rpc"
	"example.com/aim/auth-service/kitex_gen/auth/authservice"
	"github.com/cloudwego/kitex/server"
)

func main() {
	rpcAddr := ":" + getenv("PORT", "9002")
	healthAddr := ":" + getenv("HEALTH_PORT", "19002")
	postgresDSN := mustGetenv("AUTH_POSTGRES_DSN")
	redisAddr := getenv("REDIS_ADDR", "127.0.0.1:6379")
	userServiceAddr := getenv("USER_SERVICE_ADDR", "127.0.0.1:9001")
	jwtSecret := mustGetenv("JWT_SECRET")

	db, err := pgstore.Init(postgresDSN)
	if err != nil {
		log.Fatal(err)
	}

	redisClient, err := redisstore.Init(redisAddr)
	if err != nil {
		log.Fatal(err)
	}

	userClient, err := rpc.NewUserClient(userServiceAddr)
	if err != nil {
		log.Fatal(err)
	}

	service := biz.NewAuthService(
		repository.NewSessionRepository(db),
		repository.NewRefreshTokenRepository(db),
		repository.NewSecurityEventRepository(db),
		repository.NewTxManager(db),
		repository.NewAuthCache(redisClient),
		userClient,
		jwtSecret,
		15*time.Minute,
		7*24*time.Hour,
	)

	go serveHealth(healthAddr)

	addr, err := net.ResolveTCPAddr("tcp", rpcAddr)
	if err != nil {
		log.Fatal(err)
	}

	svr := authservice.NewServer(
		handler.NewAuthServiceImpl(service),
		server.WithServiceAddr(addr),
	)

	log.Printf("auth-service kitex listening on %s", rpcAddr)
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

	log.Printf("auth-service health listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
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
