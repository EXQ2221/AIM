package main

import (
	"net"
	"net/http"
	"os"

	pgstore "example.com/aim/rag-service/internal/dal/postgres"
	"example.com/aim/rag-service/internal/handler"
	"example.com/aim/rag-service/internal/observability"
	"example.com/aim/rag-service/internal/repository"
	"example.com/aim/rag-service/kitex_gen/rag/ragservice"
	ragbiz "example.com/aim/rag-service/rag-internal/biz"
	"github.com/cloudwego/kitex/server"
	"go.uber.org/zap"
)

func main() {
	if err := observability.InitLogger("rag-service"); err != nil {
		panic(err)
	}
	defer observability.Sync()
	logger := observability.L()

	rpcAddr := ":" + getenv("PORT", "9004")
	healthAddr := ":" + getenv("HEALTH_PORT", "19004")
	postgresDSN := mustGetenv("RAG_POSTGRES_DSN")

	db, err := pgstore.Init(postgresDSN)
	if err != nil {
		logger.Fatal("postgres init failed", zap.Error(err))
	}

	ragRepo := repository.NewRAGRepository(db)
	conversationRepo := repository.NewConversationRepository(db)
	memberRepo := repository.NewMemberRepository(db)

	ragService := ragbiz.NewServiceFromEnv(ragRepo, conversationRepo, memberRepo)
	if ragService == nil {
		logger.Fatal("rag service disabled: embedding config is invalid")
	}

	go serveHealth(healthAddr)

	addr, err := net.ResolveTCPAddr("tcp", rpcAddr)
	if err != nil {
		logger.Fatal("resolve listen address failed", zap.Error(err), zap.String("addr", rpcAddr))
	}

	svr := ragservice.NewServer(
		handler.NewRAGServiceImpl(ragService),
		server.WithServiceAddr(addr),
	)

	logger.Info("rag-service kitex listening", zap.String("addr", rpcAddr))
	if err := svr.Run(); err != nil {
		logger.Fatal("rag-service run failed", zap.Error(err))
	}
}

func serveHealth(addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	observability.L().Info("rag-service health listening", zap.String("addr", addr))
	if err := http.ListenAndServe(addr, mux); err != nil {
		observability.L().Fatal("rag-service health server failed", zap.Error(err))
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
		observability.L().Fatal("missing required environment variable", zap.String("key", key))
	}
	return value
}
