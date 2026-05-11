package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"example.com/aim/chat-service/internal/biz"
	"example.com/aim/chat-service/internal/bot"
	"example.com/aim/chat-service/internal/dal/model"
	pgstore "example.com/aim/chat-service/internal/dal/postgres"
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
	postgresDSN := mustGetenv("CHAT_POSTGRES_DSN")
	userServiceAddr := getenv("USER_SERVICE_ADDR", "127.0.0.1:9001")

	db, err := pgstore.Init(postgresDSN)
	if err != nil {
		log.Fatal(err)
	}
	for _, cfg := range builtInBotConfigsFromEnv() {
		if err := pgstore.EnsureBuiltInBot(db, cfg); err != nil {
			log.Fatal(err)
		}
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
	multiCfg, err := llm.LoadMultiConfigFromEnv()
	if err != nil {
		log.Printf("bot service disabled: %v", err)
		return nil
	}
	registry, err := llm.NewRegistry(multiCfg)
	if err != nil {
		log.Printf("bot service disabled: %v", err)
		return nil
	}
	llmClient, llmConfig, providerName, err := registry.Client("")
	if err != nil {
		log.Printf("bot service disabled: %v", err)
		return nil
	}

	botService := bot.NewService(llmClient, messageRepo, conversationRepo, aiCallLogRepo)
	botService.SetDefaultModel(llmConfig.Model)
	botService.SetLLMSelector(func(botModel model.Bot) (llm.Client, string, error) {
		provider := "primary"
		if strings.EqualFold(strings.TrimSpace(botModel.MentionName), "qwen") {
			provider = "secondary"
		}
		client, _, providerName, selectErr := registry.Client(provider)
		if selectErr != nil {
			if provider == "secondary" {
				return nil, "", fmt.Errorf("qwen provider not configured: %w", selectErr)
			}
			return nil, "", selectErr
		}
		return client, providerName, nil
	})
	botService.SetLimiter(bot.NewLimiter(botMaxConcurrencyFromEnv(), botMaxConversationConcurrencyFromEnv()))
	botService.SetContextMessages(botContextMessagesFromEnv())
	botService.SetMemberRepository(memberRepo)
	botService.SetBotRepository(botRepo)
	botService.SetConversationBotRepository(conversationBotRepo)
	botService.SetUserClient(userClient)
	if redisClient != nil {
		botService.SetReplyPublisher(bot.NewRedisReplyPublisher(redisClient))
	}
	log.Printf(
		"bot service enabled: provider=%s, model=%s, context_messages=%d, providers=%s",
		providerName,
		llmConfig.Model,
		botContextMessagesFromEnv(),
		strings.Join(registry.ProviderNames(), ","),
	)
	return botService
}

func builtInBotConfigsFromEnv() []pgstore.BuiltInBotConfig {
	deepSeekSupportedModels := []string{"deepseek-v4-flash", "deepseek-v4-pro"}
	deepSeekModelName := deepSeekSupportedModels[0]
	if envModel := strings.TrimSpace(os.Getenv("LLM_MODEL")); envModel != "" {
		for _, supportedModel := range deepSeekSupportedModels {
			if supportedModel == envModel {
				deepSeekModelName = envModel
				break
			}
		}
	}

	qwenSupportedModels := []string{"qwen-turbo", "qwen-plus", "qwen-max", "qwen3.6-plus"}
	qwenModelName := qwenSupportedModels[0]
	if envModel := strings.TrimSpace(os.Getenv("LLM2_MODEL")); envModel != "" {
		for _, supportedModel := range qwenSupportedModels {
			if supportedModel == envModel {
				qwenModelName = envModel
				break
			}
		}
	}

	return []pgstore.BuiltInBotConfig{
		{
			ID:              uint64(intFromEnv("BOT_ID", 100000)),
			Name:            "DeepSeek",
			MentionName:     "ai",
			Aliases:         []string{"deepseek"},
			Description:     "平台内置 AI 助手（文本推理优先）。",
			ModelName:       deepSeekModelName,
			SupportedModels: deepSeekSupportedModels,
			SystemPrompt:    bot.DefaultSystemPrompt,
		},
		{
			ID:              uint64(intFromEnv("BOT2_ID", 100001)),
			Name:            "千问",
			MentionName:     "qwen",
			Aliases:         []string{"tongyi", "qw"},
			Description:     "平台内置通义千问助手：qwen-turbo（速度快）、qwen-plus（均衡）、qwen-max（效果最好）、qwen3.6-plus（支持读图）。",
			ModelName:       qwenModelName,
			SupportedModels: qwenSupportedModels,
			SystemPrompt:    bot.DefaultSystemPrompt,
		},
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

func botContextMessagesFromEnv() int {
	return intFromEnv("BOT_CONTEXT_MESSAGES", 20)
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
