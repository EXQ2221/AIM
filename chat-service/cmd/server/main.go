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

	bot "example.com/aim/chat-service/bot-internal/biz"
	botclient "example.com/aim/chat-service/bot-internal/client"
	botconf "example.com/aim/chat-service/bot-internal/conf"
	botdal "example.com/aim/chat-service/bot-internal/dal"
	botrepo "example.com/aim/chat-service/bot-internal/repository"
	"example.com/aim/chat-service/internal/biz"
	"example.com/aim/chat-service/internal/dal/model"
	pgstore "example.com/aim/chat-service/internal/dal/postgres"
	"example.com/aim/chat-service/internal/handler"
	"example.com/aim/chat-service/internal/repository"
	"example.com/aim/chat-service/internal/rpc"
	"example.com/aim/chat-service/kitex_gen/chat/chatservice"
	"example.com/aim/chat-service/kitex_gen/rag/ragservice"
	llm "example.com/aim/chat-service/llm-internal/client"
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
	ragClient, err := rpc.NewRAGClient(getenv("RAG_SERVICE_ADDR", "127.0.0.1:9004"))
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
	notificationRepo := repository.NewNotificationRepository(db)
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
	chatService.SetNotificationRepository(notificationRepo)
	chatService.SetBotTaskTimeout(botconf.BotTaskTimeoutFromEnv())
	chatService.SetBotManagement(
		botRepo,
		conversationBotRepo,
		botrepo.NewMembershipService(repository.NewTxManager(db), conversationRepo, memberRepo, botRepo, conversationBotRepo),
	)
	if botService := newBotServiceFromEnv(messageRepo, conversationRepo, memberRepo, botRepo, conversationBotRepo, ragClient, aiCallLogRepo, redisClient, userClient); botService != nil {
		chatService.SetBotService(botService)
	}

	go serveHealth(healthAddr)

	addr, err := net.ResolveTCPAddr("tcp", rpcAddr)
	if err != nil {
		log.Fatal(err)
	}

	svr := chatservice.NewServer(
		handler.NewChatServiceImpl(chatService, ragClient),
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
	ragClient ragservice.Client,
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
		mentionName := strings.ToLower(strings.TrimSpace(botModel.MentionName))
		if mentionName == "qwen" {
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
	botService.SetLimiter(bot.NewLimiter(botconf.BotMaxConcurrencyFromEnv(), botconf.BotMaxConversationConcurrencyFromEnv()))
	botService.SetLLMTimeout(botconf.BotLLMTimeoutFromEnv())
	botService.SetContextMessages(botconf.BotContextMessagesFromEnv())
	botService.SetRAGTopK(ragTopKFromEnv())
	if ragSearcher := newBotRAGSearcher(ragClient); ragSearcher != nil {
		botService.SetRAGSearcher(ragSearcher)
	}
	botService.SetMemberRepository(memberRepo)
	botService.SetBotRepository(botRepo)
	botService.SetConversationBotRepository(conversationBotRepo)
	botService.SetUserClient(userClient)
	if redisClient != nil {
		botService.SetReplyPublisher(botclient.NewRedisReplyPublisher(redisClient))
	}
	log.Printf(
		"bot service enabled: provider=%s, model=%s, context_messages=%d, llm_timeout=%s, providers=%s",
		providerName,
		llmConfig.Model,
		botconf.BotContextMessagesFromEnv(),
		botconf.BotLLMTimeoutFromEnv().String(),
		strings.Join(registry.ProviderNames(), ","),
	)
	return botService
}

func newBotRAGSearcher(ragClient ragservice.Client) bot.RAGSearcher {
	if ragClient == nil {
		return nil
	}
	return botdal.NewConversationRAGSearcher(ragClient)
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

	qwenSupportedModels := []string{"qwen-turbo", "qwen-plus", "qwen-max", "qwen3.6-plus", "qwen3.5-plus"}
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
			Description:     "\u5e73\u53f0\u5185\u7f6e AI \u52a9\u624b\uff08\u6587\u672c\u63a8\u7406\u4f18\u5148\uff09\u3002",
			ModelName:       deepSeekModelName,
			SupportedModels: deepSeekSupportedModels,
			SystemPrompt:    bot.DefaultSystemPrompt,
		},
		{
			ID:              uint64(intFromEnv("BOT2_ID", 100001)),
			Name:            "\u901a\u4e49\u5343\u95ee",
			MentionName:     "qwen",
			Aliases:         []string{"tongyi", "qw"},
			Description:     "\u5e73\u53f0\u5185\u7f6e\u901a\u4e49\u5343\u95ee\u52a9\u624b\uff1aqwen-turbo\uff08\u901f\u5ea6\u5feb\uff09\u3001qwen-plus\uff08\u5747\u8861\uff09\u3001qwen-max\uff08\u6548\u679c\u6700\u597d\uff09\u3001qwen3.6-plus\uff08\u652f\u6301\u8bfb\u56fe\uff09\u3001qwen3.5-plus\uff08\u652f\u6301\u8bfb\u56fe\uff09\u3002",
			ModelName:       qwenModelName,
			SupportedModels: qwenSupportedModels,
			SystemPrompt:    bot.DefaultSystemPrompt,
		},
	}
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
