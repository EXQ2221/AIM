package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"example.com/aim/chat-service/internal/biz"
	"example.com/aim/chat-service/internal/bot"
	mysqlstore "example.com/aim/chat-service/internal/dal/mysql"
	"example.com/aim/chat-service/internal/handler"
	"example.com/aim/chat-service/internal/llm"
	"example.com/aim/chat-service/internal/repository"
	"example.com/aim/chat-service/internal/rpc"
	"example.com/aim/chat-service/kitex_gen/chat/chatservice"
	"github.com/cloudwego/kitex/server"
	redisv9 "github.com/redis/go-redis/v9"
)

func main() {
	rpcAddr := ":" + getenv("PORT", "9003")
	healthAddr := ":" + getenv("HEALTH_PORT", "19003")
	mysqlDSN := mustGetenv("MYSQL_DSN")
	userServiceAddr := getenv("USER_SERVICE_ADDR", "127.0.0.1:9001")

	db, err := mysqlstore.Init(mysqlDSN)
	if err != nil {
		log.Fatal(err)
	}
	if err := mysqlstore.EnsureBuiltInBot(db, builtInBotConfigFromEnv()); err != nil {
		log.Fatal(err)
	}

	userClient, err := rpc.NewUserClient(userServiceAddr)
	if err != nil {
		log.Fatal(err)
	}

	conversationRepo := repository.NewConversationRepository(db)
	groupRepo := repository.NewGroupRepository(db)
	memberRepo := repository.NewMemberRepository(db)
	botRepo := repository.NewBotRepository(db)
	conversationBotRepo := repository.NewConversationBotRepository(db)
	messageRepo := repository.NewMessageRepository(db)
	aiCallLogRepo := repository.NewAICallLogRepository(db)
	redisClient := newRedisClient(context.Background(), strings.TrimSpace(os.Getenv("REDIS_ADDR")))
	if redisClient != nil {
		defer redisClient.Close()
	}

	chatService := biz.NewChatService(
		conversationRepo,
		groupRepo,
		memberRepo,
		messageRepo,
		repository.NewTxManager(db),
		userClient,
	)
	chatService.SetAICallLogRepository(aiCallLogRepo)
	chatService.SetBotTaskTimeout(botTaskTimeoutFromEnv())
	chatService.SetBotManagement(
		botRepo,
		conversationBotRepo,
		bot.NewMembershipService(repository.NewTxManager(db), conversationRepo, memberRepo, botRepo, conversationBotRepo),
	)
	if botService := newBotServiceFromEnv(messageRepo, conversationRepo, memberRepo, botRepo, conversationBotRepo, aiCallLogRepo, redisClient, userClient); botService != nil {
		chatService.SetBotService(botService)
	}

	go serveHealth(healthAddr)

	addr, err := net.ResolveTCPAddr("tcp", rpcAddr)
	if err != nil {
		log.Fatal(err)
	}

	svr := chatservice.NewServer(
		handler.NewChatServiceImpl(chatService),
		server.WithServiceAddr(addr),
	)

	log.Printf("chat-service kitex listening on %s", rpcAddr)
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

	log.Printf("chat-service health listening on %s", addr)
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
		log.Printf("chat-service redis unavailable, bot reply publisher disabled: %v", err)
		return nil
	}
	return client
}

func newBotServiceFromEnv(
	messageRepo repository.MessageRepository,
	conversationRepo repository.ConversationRepository,
	memberRepo repository.MemberRepository,
	botRepo repository.BotRepository,
	conversationBotRepo repository.ConversationBotRepository,
	aiCallLogRepo repository.AICallLogRepository,
	redisClient *redisv9.Client,
	userClient rpc.UserClient,
) *bot.Service {
	llmConfig, err := llm.LoadConfigFromEnv()
	if err != nil {
		log.Printf("bot service disabled: %v", err)
		return nil
	}
	llmClient, err := llm.NewOpenAICompatibleClient(llmConfig)
	if err != nil {
		log.Printf("bot service disabled: %v", err)
		return nil
	}
	botService := bot.NewService(llmClient, messageRepo, conversationRepo, aiCallLogRepo)
	botService.SetDefaultModel(llmConfig.Model)
	botService.SetLimiter(bot.NewLimiter(botMaxConcurrencyFromEnv(), botMaxConversationConcurrencyFromEnv()))
	botService.SetMemberRepository(memberRepo)
	botService.SetBotRepository(botRepo)
	botService.SetConversationBotRepository(conversationBotRepo)
	botService.SetUserClient(userClient)
	if redisClient != nil {
		botService.SetReplyPublisher(bot.NewRedisReplyPublisher(redisClient))
	}
	return botService
}

func builtInBotConfigFromEnv() mysqlstore.BuiltInBotConfig {
	supportedModels := []string{"deepseek-v4-flash", "deepseek-v4-pro"}
	modelName := supportedModels[0]
	if envModel := strings.TrimSpace(os.Getenv("LLM_MODEL")); envModel != "" {
		for _, supportedModel := range supportedModels {
			if supportedModel == envModel {
				modelName = envModel
				break
			}
		}
	}
	botID := uint64(intFromEnv("BOT_ID", 100000))
	return mysqlstore.BuiltInBotConfig{
		ID:              botID,
		Name:            "DeepSeek",
		MentionName:     "ai",
		Aliases:         []string{"deepseek"},
		Description:     "平台内置 AI 助手，可在群里切换 Flash / Pro 模型。",
		ModelName:       modelName,
		SupportedModels: supportedModels,
		SystemPrompt:    bot.DefaultSystemPrompt,
	}
}

func botMaxConcurrencyFromEnv() int {
	return intFromEnv("BOT_MAX_CONCURRENCY", 10)
}

func botMaxConversationConcurrencyFromEnv() int {
	return intFromEnv("BOT_MAX_CONVERSATION_CONCURRENCY", 1)
}

func botTaskTimeoutFromEnv() time.Duration {
	seconds := intFromEnv("BOT_TASK_TIMEOUT_SECONDS", 30)
	if seconds <= 0 {
		seconds = 30
	}
	return time.Duration(seconds) * time.Second
}

func intFromEnv(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		log.Printf("invalid %s=%q, using %d", key, value, fallback)
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

func mustGetenv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("missing required environment variable %s", key)
	}
	return value
}
