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
	groupJoinRequestRepo := repository.NewGroupJoinRequestRepository(db)
	userMemoryRepo := repository.NewUserMemoryRepository(db)
	userMemorySettingRepo := repository.NewUserMemorySettingRepository(db)
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
	chatService.SetGroupJoinRequestRepository(groupJoinRequestRepo)
	chatService.SetUserMemoryRepository(userMemoryRepo)
	chatService.SetUserMemorySettingRepository(userMemorySettingRepo)
	chatService.SetBotTaskTimeout(botconf.BotTaskTimeoutFromEnv())
	chatService.SetBotManagement(
		botRepo,
		conversationBotRepo,
		botrepo.NewMembershipService(repository.NewTxManager(db), conversationRepo, memberRepo, botRepo, conversationBotRepo),
	)
	if botService := newBotServiceFromEnv(messageRepo, conversationRepo, memberRepo, botRepo, conversationBotRepo, userMemoryRepo, userMemorySettingRepo, ragClient, aiCallLogRepo, redisClient, userClient); botService != nil {
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
	userMemoryRepo repository.UserMemoryRepository,
	userMemorySettingRepo repository.UserMemorySettingRepository,
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
		if botModel.CreatedBy > 0 {
			customBaseURL := strings.TrimSpace(botModel.APIBaseURL)
			customAPIKey := strings.TrimSpace(botModel.APIKeyEncrypted)
			customModel := strings.TrimSpace(botModel.ModelName)
			if customBaseURL == "" || customAPIKey == "" || customModel == "" {
				return nil, "", fmt.Errorf("custom bot llm config is incomplete")
			}
			customClient, customErr := llm.NewOpenAICompatibleClient(llm.Config{
				BaseURL:            customBaseURL,
				APIKey:             customAPIKey,
				Model:              customModel,
				Timeout:            botconf.BotLLMTimeoutFromEnv(),
				InsecureSkipVerify: customBotLLMInsecureSkipVerifyFromEnv(),
			})
			if customErr != nil {
				return nil, "", fmt.Errorf("init custom bot llm client failed: %w", customErr)
			}
			return customClient, "custom", nil
		}

		provider := providerByModelName(strings.TrimSpace(botModel.ModelName))
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
	botService.SetRAGClient(ragClient)
	if ragSearcher := newBotRAGSearcher(ragClient); ragSearcher != nil {
		botService.SetRAGSearcher(ragSearcher)
	}
	botService.SetMemberRepository(memberRepo)
	botService.SetBotRepository(botRepo)
	botService.SetConversationBotRepository(conversationBotRepo)
	botService.SetUserClient(userClient)
	botService.SetUserMemoryRepository(userMemoryRepo)
	botService.SetUserMemorySettingRepository(userMemorySettingRepo)
	botService.SetMemoryExtractTimeout(botconf.MemoryExtractTimeoutFromEnv())
	botService.SetMemoryCandidateLimit(botconf.MemoryCandidateLimitFromEnv())
	botService.SetMemoryMaxRecall(botconf.MemoryMaxRecallFromEnv())
	if botconf.MemoryEnabledFromEnv() {
		memoryProvider := botconf.MemoryProviderFromEnv()
		memoryModel := botconf.MemoryModelFromEnv()
		memoryClient, _, providerName, selectErr := registry.Client(memoryProvider)
		if selectErr != nil {
			log.Printf("memory extractor degraded: provider=%s err=%v", memoryProvider, selectErr)
		} else {
			botService.SetUserMemoryExtractor(&bot.LLMUserMemoryExtractor{
				Client: memoryClient,
				Model:  memoryModel,
			})
			log.Printf("memory extractor enabled: provider=%s model=%s", providerName, memoryModel)
		}
	} else {
		log.Printf("memory extractor disabled by MEMORY_ENABLED=false")
	}
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

func customBotLLMInsecureSkipVerifyFromEnv() bool {
	return parseBoolFromEnv(
		"BOT_CUSTOM_LLM_INSECURE_SKIP_VERIFY",
		parseBoolFromEnv("LLM_INSECURE_SKIP_VERIFY", false),
	)
}

func newBotRAGSearcher(ragClient ragservice.Client) bot.RAGSearcher {
	if ragClient == nil {
		return nil
	}
	reranker := newRerankerFromEnv()
	return botdal.NewConversationRAGSearcher(
		ragClient,
		reranker,
		botconf.RerankRecallTopKFromEnv(),
		botconf.RerankScoreThresholdFromEnv(),
	)
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
	seconds := intFromEnv("RAG_SEARCH_TIMEOUT_SECONDS", 80)
	if seconds <= 0 {
		seconds = 40
	}
	return time.Duration(seconds) * time.Second
}

func newRerankerFromEnv() botdal.TextReranker {
	if !botconf.RerankEnabledFromEnv() {
		log.Printf("rerank disabled by RERANK_ENABLED=false")
		return nil
	}
	apiKey := botconf.RerankAPIKeyFromEnv()
	if strings.TrimSpace(apiKey) == "" {
		log.Printf("rerank degraded: api key is empty")
		return nil
	}
	insecureSkipVerify := parseBoolFromEnv("RERANK_INSECURE_SKIP_VERIFY", false)
	client, err := botdal.NewDashScopeCompatibleReranker(
		botconf.RerankBaseURLFromEnv(),
		apiKey,
		botconf.RerankModelFromEnv(),
		botconf.RerankTimeoutFromEnv(),
		insecureSkipVerify,
	)
	if err != nil {
		log.Printf("rerank degraded: %v", err)
		return nil
	}
	log.Printf(
		"rerank enabled: model=%s base_url=%s timeout=%s threshold=%.3f recall_top_k=%d",
		botconf.RerankModelFromEnv(),
		botconf.RerankBaseURLFromEnv(),
		botconf.RerankTimeoutFromEnv().String(),
		botconf.RerankScoreThresholdFromEnv(),
		botconf.RerankRecallTopKFromEnv(),
	)
	return client
}

func builtInBotConfigsFromEnv() []pgstore.BuiltInBotConfig {
	return []pgstore.BuiltInBotConfig{
		{
			ID:              uint64(intFromEnv("BOT_ID", 100000)),
			Name:            "DeepSeek Flash",
			MentionName:     "dsflash",
			Aliases:         []string{"deepseek"},
			Description:     "\u5e73\u53f0\u5185\u7f6e DeepSeek Flash\uff08\u901f\u5ea6\u4f18\u5148\uff09\u3002",
			ModelName:       "deepseek-v4-flash",
			SupportedModels: []string{"deepseek-v4-flash"},
			SystemPrompt:    bot.DefaultSystemPrompt,
		},
		{
			ID:              uint64(intFromEnv("BOT2_ID", 100001)),
			Name:            "DeepSeek Pro",
			MentionName:     "dspro",
			Aliases:         []string{"deepseek-pro"},
			Description:     "\u5e73\u53f0\u5185\u7f6e DeepSeek Pro\uff08\u6548\u679c\u4f18\u5148\uff09\u3002",
			ModelName:       "deepseek-v4-pro",
			SupportedModels: []string{"deepseek-v4-pro"},
			SystemPrompt:    bot.DefaultSystemPrompt,
		},
		{
			ID:              uint64(intFromEnv("BOT3_ID", 100002)),
			Name:            "\u901a\u4e49 Turbo",
			MentionName:     "qwenturbo",
			Aliases:         []string{"tongyi-turbo"},
			Description:     "\u5e73\u53f0\u5185\u7f6e\u901a\u4e49 Turbo\uff08\u901f\u5ea6\u4f18\u5148\uff09\u3002",
			ModelName:       "qwen-turbo",
			SupportedModels: []string{"qwen-turbo"},
			SystemPrompt:    bot.DefaultSystemPrompt,
		},
		{
			ID:              uint64(intFromEnv("BOT4_ID", 100003)),
			Name:            "\u901a\u4e49 Plus",
			MentionName:     "qwenplus",
			Aliases:         []string{"tongyi-plus"},
			Description:     "\u5e73\u53f0\u5185\u7f6e\u901a\u4e49 Plus\uff08\u5747\u8861\uff09\u3002",
			ModelName:       "qwen-plus",
			SupportedModels: []string{"qwen-plus"},
			SystemPrompt:    bot.DefaultSystemPrompt,
		},
		{
			ID:              uint64(intFromEnv("BOT5_ID", 100004)),
			Name:            "\u901a\u4e49 Max",
			MentionName:     "qwenmax",
			Aliases:         []string{"tongyi-max"},
			Description:     "\u5e73\u53f0\u5185\u7f6e\u901a\u4e49 Max\uff08\u6548\u679c\u4f18\u5148\uff09\u3002",
			ModelName:       "qwen-max",
			SupportedModels: []string{"qwen-max"},
			SystemPrompt:    bot.DefaultSystemPrompt,
		},
		{
			ID:              uint64(intFromEnv("BOT6_ID", 100005)),
			Name:            "\u901a\u4e49 3.6 Plus",
			MentionName:     "qwen36",
			Aliases:         []string{"tongyi-36"},
			Description:     "\u5e73\u53f0\u5185\u7f6e\u901a\u4e49 3.6 Plus\uff08\u8bfb\u56fe/\u591a\u6a21\u6001\u589e\u5f3a\uff09\u3002",
			ModelName:       "qwen3.6-plus",
			SupportedModels: []string{"qwen3.6-plus"},
			SystemPrompt:    bot.DefaultSystemPrompt,
		},
		{
			ID:              uint64(intFromEnv("BOT7_ID", 100006)),
			Name:            "\u901a\u4e49 3.5 Plus",
			MentionName:     "qwen35",
			Aliases:         []string{"tongyi-35", "qwen"},
			Description:     "\u5e73\u53f0\u5185\u7f6e\u901a\u4e49 3.5 Plus\u3002",
			ModelName:       "qwen3.5-plus",
			SupportedModels: []string{"qwen3.5-plus"},
			SystemPrompt:    bot.DefaultSystemPrompt,
		},
	}
}

func providerByModelName(modelName string) string {
	name := strings.ToLower(strings.TrimSpace(modelName))
	if strings.HasPrefix(name, "qwen") || strings.Contains(name, "tongyi") {
		return "secondary"
	}
	return "primary"
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

func parseBoolFromEnv(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		log.Printf("invalid %s=%q, using %t", key, value, fallback)
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
