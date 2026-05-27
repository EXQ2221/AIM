package bot

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	botmodel "example.com/aim/chat-service/bot-internal/model"
	"example.com/aim/chat-service/internal/dal/model"
	"example.com/aim/chat-service/internal/repository"
	"example.com/aim/chat-service/internal/rpc"
	llm "example.com/aim/chat-service/llm-internal/client"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestServiceHandleMentionCreatesBotReply(t *testing.T) {
	messageRepo := &fakeMessageRepo{
		recent: []model.Message{
			{ID: 1, ConversationID: 10, SenderID: 10001, SenderType: model.SenderTypeUser, MessageType: model.MessageTypeText, Content: datatypes.JSON(`{"text":"hello"}`)},
		},
	}
	conversationRepo := &fakeConversationRepo{}
	memberRepo := &fakeMemberRepo{
		members: []model.ConversationMember{
			{ConversationID: 10, MemberType: model.MemberTypeUser, MemberID: 10001, Status: model.MemberStatusNormal},
			{ConversationID: 10, MemberType: model.MemberTypeUser, MemberID: 10002, Status: model.MemberStatusNormal},
			{ConversationID: 10, MemberType: model.MemberTypeBot, MemberID: 7, Role: model.MemberRoleBot, Status: model.MemberStatusNormal},
		},
	}
	botRepo := &fakeBotRepo{
		bots: map[uint64]*model.Bot{
			7: {
				ID:           7,
				Name:         "AIM",
				MentionName:  "aim",
				ModelName:    "db-model",
				SystemPrompt: "system prompt",
				Status:       model.BotStatusEnabled,
			},
		},
	}
	conversationBotRepo := newFakeConversationBotRepo()
	_ = conversationBotRepo.Create(context.Background(), &model.ConversationBot{
		ConversationID:  10,
		BotID:           7,
		Enabled:         true,
		PermissionScope: model.BotScopeConversationOnly,
	})
	aiCallLogRepo := &fakeAICallLogRepo{}
	replyPublisher := &fakeReplyPublisher{}
	llmClient := &fakeLLMClient{response: &llm.GenerateResponse{
		Content:          "bot reply",
		PromptTokens:     11,
		CompletionTokens: 7,
		TotalTokens:      18,
	}}
	service := NewService(llmClient, messageRepo, conversationRepo, aiCallLogRepo)
	service.SetDefaultModel("env-model")
	service.SetLimiter(NewLimiter(10, 1))
	service.SetMemberRepository(memberRepo)
	service.SetBotRepository(botRepo)
	service.SetConversationBotRepository(conversationBotRepo)
	service.SetReplyPublisher(replyPublisher)
	service.SetUserClient(&fakeBotUserClient{
		users: map[uint64]*rpc.UserInfo{
			10001: {UserID: 10001, Nickname: "Alice"},
		},
	})

	err := service.HandleMention(context.Background(), HandleMentionRequest{
		ConversationID:   10,
		RequestMessageID: 1,
		UserID:           10001,
		Content:          "@AIM summarize",
	})
	if err != nil {
		t.Fatalf("HandleMention returned error: %v", err)
	}

	if messageRepo.listConversationID != 10 || messageRepo.listLimit != 20 {
		t.Fatalf("recent messages were not queried correctly: conversation=%d limit=%d", messageRepo.listConversationID, messageRepo.listLimit)
	}
	if len(llmClient.requests) != 1 {
		t.Fatalf("expected one LLM request, got %d", len(llmClient.requests))
	}
	req := llmClient.requests[0]
	if req.Model != "db-model" {
		t.Fatalf("unexpected model: %s", req.Model)
	}
	if len(req.Messages) != 2 || req.Messages[0].Role != "system" || req.Messages[0].Content != "system prompt" {
		t.Fatalf("unexpected LLM messages: %+v", req.Messages)
	}
	if !strings.Contains(req.Messages[1].Content, "\u3010\u7528\u6237\u95ee\u9898\u3011summarize") {
		t.Fatalf("prompt did not include extracted question: %q", req.Messages[1].Content)
	}

	if len(messageRepo.created) != 1 {
		t.Fatalf("expected one created message, got %d", len(messageRepo.created))
	}
	created := messageRepo.created[0]
	if created.ConversationID != 10 ||
		created.SenderID != 7 ||
		created.SenderType != model.SenderTypeBot ||
		created.MessageType != model.MessageTypeBotReply ||
		model.ExtractTextMessageContent(created.Content) != "bot reply" ||
		created.Status != model.MessageStatusNormal {
		t.Fatalf("unexpected bot message: %+v", created)
	}
	if conversationRepo.lastConversationID != 10 || conversationRepo.lastMessageID != created.ID || conversationRepo.lastAt.IsZero() {
		t.Fatalf("last message was not updated correctly: %+v", conversationRepo)
	}
	if memberRepo.listConversationID != 10 {
		t.Fatalf("conversation members were not queried: %d", memberRepo.listConversationID)
	}
	if len(replyPublisher.events) != 1 {
		t.Fatalf("expected one bot reply event, got %d", len(replyPublisher.events))
	}
	event := replyPublisher.events[0]
	if event.Message.ID != int64(created.ID) ||
		event.Message.ConversationID != "c_test" ||
		event.Message.SenderID != 7 ||
		event.Message.SenderType != string(model.SenderTypeBot) ||
		event.Message.MessageType != string(model.MessageTypeBotReply) ||
		!strings.Contains(event.Message.Content, "bot reply") ||
		event.Message.Status != string(model.MessageStatusNormal) ||
		event.Message.CreatedAt == 0 ||
		len(event.RecipientUserIDs) != 2 ||
		event.RecipientUserIDs[0] != 10001 ||
		event.RecipientUserIDs[1] != 10002 {
		t.Fatalf("unexpected bot reply event: %+v", event)
	}

	if len(aiCallLogRepo.created) != 1 {
		t.Fatalf("expected one ai call log, got %d", len(aiCallLogRepo.created))
	}
	callLog := aiCallLogRepo.created[0]
	if callLog.UserID != 10001 ||
		callLog.BotID != 7 ||
		callLog.ConversationID != 10 ||
		callLog.RequestMessageID == nil ||
		*callLog.RequestMessageID != 1 ||
		callLog.ResponseMessageID == nil ||
		*callLog.ResponseMessageID != created.ID ||
		callLog.ModelName != "db-model" ||
		callLog.PromptTokens != 11 ||
		callLog.CompletionTokens != 7 ||
		callLog.TotalTokens != 18 ||
		callLog.Status != model.AICallStatusSuccess ||
		callLog.ErrorMessage != "" {
		t.Fatalf("unexpected success ai call log: %+v", callLog)
	}
}

func TestServiceHandleMentionReturnsNilWhenNoBotMatches(t *testing.T) {
	messageRepo := &fakeMessageRepo{}
	conversationRepo := &fakeConversationRepo{}
	aiCallLogRepo := &fakeAICallLogRepo{}
	llmClient := &fakeLLMClient{response: &llm.GenerateResponse{Content: "bot reply"}}
	service := NewService(llmClient, messageRepo, conversationRepo, aiCallLogRepo)
	service.SetDefaultModel("env-model")
	service.SetLimiter(NewLimiter(10, 1))
	service.SetMemberRepository(&fakeMemberRepo{
		members: []model.ConversationMember{
			{ConversationID: 10, MemberType: model.MemberTypeUser, MemberID: 10001, Status: model.MemberStatusNormal},
		},
	})
	service.SetBotRepository(&fakeBotRepo{})
	service.SetConversationBotRepository(newFakeConversationBotRepo())

	err := service.HandleMention(context.Background(), HandleMentionRequest{
		ConversationID:   10,
		RequestMessageID: 3,
		UserID:           10001,
		Content:          "@nobody hello",
	})
	if err != nil {
		t.Fatalf("expected nil error when no bot matches, got %v", err)
	}
	if len(llmClient.requests) != 0 {
		t.Fatalf("llm should not be called when no bot matches")
	}
	if len(messageRepo.created) != 0 {
		t.Fatalf("expected no bot message when no bot matches")
	}
	if len(aiCallLogRepo.created) != 0 {
		t.Fatalf("expected no ai call log when no bot matches")
	}
}

func TestServiceHandleMentionFiltersRecalledMessagesFromContext(t *testing.T) {
	messageRepo := &fakeMessageRepo{
		recent: []model.Message{
			{ID: 1, ConversationID: 10, SenderID: 10001, SenderType: model.SenderTypeUser, MessageType: model.MessageTypeText, Content: datatypes.JSON(`{"text":"normal text"}`), Status: model.MessageStatusNormal},
			{ID: 2, ConversationID: 10, SenderID: 10002, SenderType: model.SenderTypeUser, MessageType: model.MessageTypeText, Content: datatypes.JSON(`{"text":"should not be seen"}`), Status: model.MessageStatusRecalled},
		},
	}
	conversationRepo := &fakeConversationRepo{}
	aiCallLogRepo := &fakeAICallLogRepo{}
	llmClient := &fakeLLMClient{response: &llm.GenerateResponse{Content: "ok"}}
	service := NewService(llmClient, messageRepo, conversationRepo, aiCallLogRepo)
	service.SetDefaultModel("env-model")
	service.SetLimiter(NewLimiter(10, 1))
	service.SetMemberRepository(&fakeMemberRepo{
		members: []model.ConversationMember{
			{ConversationID: 10, MemberType: model.MemberTypeBot, MemberID: 7, Role: model.MemberRoleBot, Status: model.MemberStatusNormal},
		},
	})
	service.SetBotRepository(&fakeBotRepo{
		bots: map[uint64]*model.Bot{
			7: {ID: 7, MentionName: "aim", ModelName: "db-model", Status: model.BotStatusEnabled},
		},
	})
	conversationBotRepo := newFakeConversationBotRepo()
	_ = conversationBotRepo.Create(context.Background(), &model.ConversationBot{
		ConversationID:  10,
		BotID:           7,
		Enabled:         true,
		PermissionScope: model.BotScopeConversationOnly,
	})
	service.SetConversationBotRepository(conversationBotRepo)

	err := service.HandleMention(context.Background(), HandleMentionRequest{
		ConversationID: 10,
		UserID:         10001,
		Content:        "@aim summarize",
	})
	if err != nil {
		t.Fatalf("HandleMention returned error: %v", err)
	}
	if len(llmClient.requests) != 1 {
		t.Fatalf("expected one LLM request, got %d", len(llmClient.requests))
	}
	if strings.Contains(llmClient.requests[0].Messages[1].Content, "should not be seen") {
		t.Fatalf("recalled message leaked into bot prompt: %q", llmClient.requests[0].Messages[1].Content)
	}
}

func TestServiceHandleMentionNonVisionModelSendsTextOnly(t *testing.T) {
	messageRepo := &fakeMessageRepo{
		recent: []model.Message{
			{ID: 1, ConversationID: 10, SenderID: 10001, SenderType: model.SenderTypeUser, MessageType: model.MessageTypeImage, Content: datatypes.JSON(`{"url":"https://cdn.example.com/a.png","name":"a.png","mimeType":"image/png","text":"look"}`), Status: model.MessageStatusNormal},
		},
	}
	conversationRepo := &fakeConversationRepo{}
	aiCallLogRepo := &fakeAICallLogRepo{}
	llmClient := &fakeLLMClient{response: &llm.GenerateResponse{Content: "ok"}}
	service := NewService(llmClient, messageRepo, conversationRepo, aiCallLogRepo)
	service.SetDefaultModel("qwen-plus")
	service.SetLimiter(NewLimiter(10, 1))
	service.SetMemberRepository(&fakeMemberRepo{
		members: []model.ConversationMember{
			{ConversationID: 10, MemberType: model.MemberTypeBot, MemberID: 7, Role: model.MemberRoleBot, Status: model.MemberStatusNormal},
		},
	})
	service.SetBotRepository(&fakeBotRepo{
		bots: map[uint64]*model.Bot{
			7: {ID: 7, MentionName: "qwen", ModelName: "qwen-plus", Status: model.BotStatusEnabled},
		},
	})
	conversationBotRepo := newFakeConversationBotRepo()
	_ = conversationBotRepo.Create(context.Background(), &model.ConversationBot{
		ConversationID:  10,
		BotID:           7,
		Enabled:         true,
		PermissionScope: model.BotScopeConversationOnly,
	})
	service.SetConversationBotRepository(conversationBotRepo)

	err := service.HandleMention(context.Background(), HandleMentionRequest{
		ConversationID: 10,
		UserID:         10001,
		Content:        "@qwen test",
	})
	if err != nil {
		t.Fatalf("HandleMention returned error: %v", err)
	}
	if len(llmClient.requests) != 1 {
		t.Fatalf("expected one LLM request, got %d", len(llmClient.requests))
	}
	if len(llmClient.requests[0].Messages) < 2 {
		t.Fatalf("unexpected message count: %d", len(llmClient.requests[0].Messages))
	}
	if got := len(llmClient.requests[0].Messages[1].Parts); got != 1 {
		t.Fatalf("non-vision model should keep text-only parts, got %d", got)
	}
}

func TestServiceHandleMentionReturnsNilWhenAliasIsAmbiguous(t *testing.T) {
	messageRepo := &fakeMessageRepo{}
	conversationRepo := &fakeConversationRepo{}
	aiCallLogRepo := &fakeAICallLogRepo{}
	llmClient := &fakeLLMClient{response: &llm.GenerateResponse{Content: "bot reply"}}
	service := NewService(llmClient, messageRepo, conversationRepo, aiCallLogRepo)
	service.SetDefaultModel("env-model")
	service.SetLimiter(NewLimiter(10, 1))
	service.SetMemberRepository(&fakeMemberRepo{
		members: []model.ConversationMember{
			{ConversationID: 10, MemberType: model.MemberTypeBot, MemberID: 7, Role: model.MemberRoleBot, Status: model.MemberStatusNormal},
			{ConversationID: 10, MemberType: model.MemberTypeBot, MemberID: 8, Role: model.MemberRoleBot, Status: model.MemberStatusNormal},
		},
	})
	service.SetBotRepository(&fakeBotRepo{
		bots: map[uint64]*model.Bot{
			7: {ID: 7, MentionName: "aim", Aliases: "[\"helper\"]", ModelName: "m1", Status: model.BotStatusEnabled},
			8: {ID: 8, MentionName: "bot2", Aliases: "[\"helper\"]", ModelName: "m2", Status: model.BotStatusEnabled},
		},
	})
	conversationBotRepo := newFakeConversationBotRepo()
	_ = conversationBotRepo.Create(context.Background(), &model.ConversationBot{ConversationID: 10, BotID: 7, Enabled: true, PermissionScope: model.BotScopeConversationOnly})
	_ = conversationBotRepo.Create(context.Background(), &model.ConversationBot{ConversationID: 10, BotID: 8, Enabled: true, PermissionScope: model.BotScopeConversationOnly})
	service.SetConversationBotRepository(conversationBotRepo)

	err := service.HandleMention(context.Background(), HandleMentionRequest{
		ConversationID: 10,
		UserID:         10001,
		Content:        "@helper hello",
	})
	if err != nil {
		t.Fatalf("expected nil error when alias is ambiguous, got %v", err)
	}
	if len(llmClient.requests) != 0 {
		t.Fatalf("llm should not be called for ambiguous alias")
	}
}

func TestServiceHandleMentionReturnsLLMErrorWithoutCreatingMessage(t *testing.T) {
	messageRepo := &fakeMessageRepo{}
	conversationRepo := &fakeConversationRepo{}
	aiCallLogRepo := &fakeAICallLogRepo{}
	llmErr := errors.New("llm failed")
	service := NewService(&fakeLLMClient{err: llmErr}, messageRepo, conversationRepo, aiCallLogRepo)
	service.SetDefaultModel("env-model")
	service.SetLimiter(NewLimiter(10, 1))
	service.SetMemberRepository(&fakeMemberRepo{
		members: []model.ConversationMember{
			{ConversationID: 10, MemberType: model.MemberTypeBot, MemberID: 7, Role: model.MemberRoleBot, Status: model.MemberStatusNormal},
		},
	})
	service.SetBotRepository(&fakeBotRepo{
		bots: map[uint64]*model.Bot{
			7: {ID: 7, MentionName: "bot", ModelName: "", Status: model.BotStatusEnabled},
		},
	})
	conversationBotRepo := newFakeConversationBotRepo()
	_ = conversationBotRepo.Create(context.Background(), &model.ConversationBot{
		ConversationID:  10,
		BotID:           7,
		Enabled:         true,
		PermissionScope: model.BotScopeConversationOnly,
	})
	service.SetConversationBotRepository(conversationBotRepo)

	err := service.HandleMention(context.Background(), HandleMentionRequest{
		ConversationID:   10,
		RequestMessageID: 3,
		UserID:           10001,
		Content:          "@bot hello",
	})
	if !errors.Is(err, llmErr) {
		t.Fatalf("expected LLM error, got %v", err)
	}
	if len(messageRepo.created) != 0 {
		t.Fatalf("expected no bot message on LLM failure, got %d", len(messageRepo.created))
	}
	if conversationRepo.lastMessageID != 0 {
		t.Fatalf("conversation last message should not update on LLM failure")
	}
	if len(aiCallLogRepo.created) != 1 {
		t.Fatalf("expected one failed ai call log, got %d", len(aiCallLogRepo.created))
	}
	callLog := aiCallLogRepo.created[0]
	if callLog.UserID != 10001 ||
		callLog.BotID != 7 ||
		callLog.ConversationID != 10 ||
		callLog.RequestMessageID == nil ||
		*callLog.RequestMessageID != 3 ||
		callLog.ResponseMessageID != nil ||
		callLog.ModelName != "env-model" ||
		callLog.Status != model.AICallStatusFailed ||
		!strings.Contains(callLog.ErrorMessage, "llm failed") {
		t.Fatalf("unexpected failed ai call log: %+v", callLog)
	}
}

func TestServiceHandleMentionGenericBotMentionFallsBackToDefaultBot(t *testing.T) {
	messageRepo := &fakeMessageRepo{}
	conversationRepo := &fakeConversationRepo{}
	aiCallLogRepo := &fakeAICallLogRepo{}
	llmClient := &fakeLLMClient{response: &llm.GenerateResponse{Content: "fallback reply"}}
	service := NewService(llmClient, messageRepo, conversationRepo, aiCallLogRepo)
	service.SetDefaultModel("env-model")
	service.SetLimiter(NewLimiter(10, 1))
	service.SetMemberRepository(&fakeMemberRepo{
		members: []model.ConversationMember{
			{ConversationID: 10, MemberType: model.MemberTypeBot, MemberID: 7, Role: model.MemberRoleBot, Status: model.MemberStatusNormal},
		},
	})
	service.SetBotRepository(&fakeBotRepo{
		bots: map[uint64]*model.Bot{
			7: {ID: 7, MentionName: "ai", ModelName: "deepseek-v4-flash", Status: model.BotStatusEnabled},
		},
	})
	conversationBotRepo := newFakeConversationBotRepo()
	_ = conversationBotRepo.Create(context.Background(), &model.ConversationBot{
		ConversationID:  10,
		BotID:           7,
		Enabled:         true,
		PermissionScope: model.BotScopeConversationOnly,
	})
	service.SetConversationBotRepository(conversationBotRepo)

	err := service.HandleMention(context.Background(), HandleMentionRequest{
		ConversationID:   10,
		RequestMessageID: 3,
		UserID:           10001,
		Content:          "@bot hello",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(llmClient.requests) != 1 {
		t.Fatalf("expected one llm call, got %d", len(llmClient.requests))
	}
	if len(messageRepo.created) != 1 {
		t.Fatalf("expected one bot message, got %d", len(messageRepo.created))
	}
	if messageRepo.created[0].SenderID != 7 {
		t.Fatalf("expected bot sender 7, got %d", messageRepo.created[0].SenderID)
	}
}

func TestServiceHandleMentionSkipsWhenConversationConcurrencyLimitReached(t *testing.T) {
	messageRepo := &fakeMessageRepo{}
	conversationRepo := &fakeConversationRepo{}
	aiCallLogRepo := &fakeAICallLogRepo{}
	llmClient := &fakeLLMClient{response: &llm.GenerateResponse{Content: "bot reply"}}
	service := NewService(llmClient, messageRepo, conversationRepo, aiCallLogRepo)
	service.SetDefaultModel("env-model")
	limiter := NewLimiter(10, 1)
	release, err := limiter.TryAcquire(10)
	if err != nil {
		t.Fatalf("unexpected limiter acquire error: %v", err)
	}
	defer release()
	service.SetLimiter(limiter)
	service.SetMemberRepository(&fakeMemberRepo{
		members: []model.ConversationMember{
			{ConversationID: 10, MemberType: model.MemberTypeBot, MemberID: 7, Role: model.MemberRoleBot, Status: model.MemberStatusNormal},
		},
	})
	service.SetBotRepository(&fakeBotRepo{
		bots: map[uint64]*model.Bot{
			7: {ID: 7, MentionName: "aim", ModelName: "db-model", Status: model.BotStatusEnabled},
		},
	})
	conversationBotRepo := newFakeConversationBotRepo()
	_ = conversationBotRepo.Create(context.Background(), &model.ConversationBot{
		ConversationID:  10,
		BotID:           7,
		Enabled:         true,
		PermissionScope: model.BotScopeConversationOnly,
	})
	service.SetConversationBotRepository(conversationBotRepo)

	err = service.HandleMention(context.Background(), HandleMentionRequest{
		ConversationID:   10,
		RequestMessageID: 3,
		UserID:           10001,
		Content:          "@aim hello",
	})
	if !errors.Is(err, ErrConversationConcurrencyLimitReached) {
		t.Fatalf("expected ErrConversationConcurrencyLimitReached, got %v", err)
	}
	if len(llmClient.requests) != 0 {
		t.Fatalf("llm should not be called when conversation concurrency is exceeded")
	}
	if len(messageRepo.created) != 0 {
		t.Fatalf("bot reply should not be created when conversation concurrency is exceeded")
	}
	if len(aiCallLogRepo.created) != 1 {
		t.Fatalf("expected one failed ai call log, got %d", len(aiCallLogRepo.created))
	}
	callLog := aiCallLogRepo.created[0]
	if callLog.Status != model.AICallStatusFailed || callLog.ErrorMessage != ErrConversationConcurrencyLimitReached.Error() {
		t.Fatalf("unexpected failed ai call log: %+v", callLog)
	}
}

func TestServiceHandleMentionSkipsWhenGlobalConcurrencyLimitReached(t *testing.T) {
	messageRepo := &fakeMessageRepo{}
	conversationRepo := &fakeConversationRepo{}
	aiCallLogRepo := &fakeAICallLogRepo{}
	llmClient := &fakeLLMClient{response: &llm.GenerateResponse{Content: "bot reply"}}
	service := NewService(llmClient, messageRepo, conversationRepo, aiCallLogRepo)
	service.SetDefaultModel("env-model")
	limiter := NewLimiter(1, 2)
	release, err := limiter.TryAcquire(99)
	if err != nil {
		t.Fatalf("unexpected limiter acquire error: %v", err)
	}
	defer release()
	service.SetLimiter(limiter)
	service.SetMemberRepository(&fakeMemberRepo{
		members: []model.ConversationMember{
			{ConversationID: 10, MemberType: model.MemberTypeBot, MemberID: 7, Role: model.MemberRoleBot, Status: model.MemberStatusNormal},
		},
	})
	service.SetBotRepository(&fakeBotRepo{
		bots: map[uint64]*model.Bot{
			7: {ID: 7, MentionName: "aim", ModelName: "db-model", Status: model.BotStatusEnabled},
		},
	})
	conversationBotRepo := newFakeConversationBotRepo()
	_ = conversationBotRepo.Create(context.Background(), &model.ConversationBot{
		ConversationID:  10,
		BotID:           7,
		Enabled:         true,
		PermissionScope: model.BotScopeConversationOnly,
	})
	service.SetConversationBotRepository(conversationBotRepo)

	err = service.HandleMention(context.Background(), HandleMentionRequest{
		ConversationID:   10,
		RequestMessageID: 3,
		UserID:           10001,
		Content:          "@aim hello",
	})
	if !errors.Is(err, ErrGlobalConcurrencyLimitReached) {
		t.Fatalf("expected ErrGlobalConcurrencyLimitReached, got %v", err)
	}
	if len(llmClient.requests) != 0 {
		t.Fatalf("llm should not be called when global concurrency is exceeded")
	}
	if len(messageRepo.created) != 0 {
		t.Fatalf("bot reply should not be created when global concurrency is exceeded")
	}
	if len(aiCallLogRepo.created) != 1 {
		t.Fatalf("expected one failed ai call log, got %d", len(aiCallLogRepo.created))
	}
	callLog := aiCallLogRepo.created[0]
	if callLog.Status != model.AICallStatusFailed || callLog.ErrorMessage != ErrGlobalConcurrencyLimitReached.Error() {
		t.Fatalf("unexpected failed ai call log: %+v", callLog)
	}
}

type fakeLLMClient struct {
	requests []llm.GenerateRequest
	response *llm.GenerateResponse
	err      error
}

func (c *fakeLLMClient) Generate(ctx context.Context, req llm.GenerateRequest) (*llm.GenerateResponse, error) {
	c.requests = append(c.requests, req)
	if c.err != nil {
		return nil, c.err
	}
	return c.response, nil
}

type fakeMessageRepo struct {
	recent             []model.Message
	created            []*model.Message
	nextID             uint64
	listConversationID uint64
	listLimit          int
}

func (r *fakeMessageRepo) WithTx(tx *gorm.DB) repository.MessageRepository {
	return r
}

func (r *fakeMessageRepo) Create(ctx context.Context, message *model.Message) error {
	if r.nextID == 0 {
		r.nextID = 1
	}
	message.ID = r.nextID
	r.nextID++
	r.created = append(r.created, message)
	return nil
}

func (r *fakeMessageRepo) GetByID(ctx context.Context, id uint64) (*model.Message, error) {
	for _, message := range r.created {
		if message.ID == id {
			return message, nil
		}
	}
	for index := range r.recent {
		if r.recent[index].ID == id {
			return &r.recent[index], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeMessageRepo) GetByIDs(ctx context.Context, ids []uint64) ([]model.Message, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	idSet := make(map[uint64]struct{}, len(ids))
	for _, id := range ids {
		idSet[id] = struct{}{}
	}
	result := make([]model.Message, 0, len(ids))
	for _, message := range r.created {
		if _, ok := idSet[message.ID]; ok {
			result = append(result, *message)
		}
	}
	for index := range r.recent {
		message := r.recent[index]
		if _, ok := idSet[message.ID]; ok {
			result = append(result, message)
		}
	}
	return result, nil
}

func (r *fakeMessageRepo) UpdateStatus(ctx context.Context, id uint64, status model.MessageStatus) error {
	for _, message := range r.created {
		if message.ID == id {
			message.Status = status
			message.UpdatedAt = time.Now()
			return nil
		}
	}
	for index := range r.recent {
		if r.recent[index].ID == id {
			r.recent[index].Status = status
			r.recent[index].UpdatedAt = time.Now()
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (r *fakeMessageRepo) ListByConversationID(ctx context.Context, conversationID uint64, beforeID *uint64, limit int) ([]model.Message, error) {
	r.listConversationID = conversationID
	r.listLimit = limit
	return r.recent, nil
}

type fakeConversationRepo struct {
	lastConversationID uint64
	lastMessageID      uint64
	lastAt             time.Time
}

func (r *fakeConversationRepo) WithTx(tx *gorm.DB) repository.ConversationRepository {
	return r
}

func (r *fakeConversationRepo) Create(ctx context.Context, conversation *model.Conversation) error {
	return nil
}

func (r *fakeConversationRepo) GetByID(ctx context.Context, id uint64) (*model.Conversation, error) {
	return &model.Conversation{
		ID:             id,
		ConversationID: "c_test",
	}, nil
}

func (r *fakeConversationRepo) GetByConversationID(ctx context.Context, conversationID string) (*model.Conversation, error) {
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeConversationRepo) FindSingleByUsers(ctx context.Context, userID uint64, peerUserID uint64) (*model.Conversation, error) {
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeConversationRepo) ListByUserID(ctx context.Context, userID uint64) ([]repository.ConversationListRow, error) {
	return nil, nil
}

func (r *fakeConversationRepo) UpdateLastMessage(ctx context.Context, conversationID uint64, messageID uint64, at time.Time) error {
	r.lastConversationID = conversationID
	r.lastMessageID = messageID
	r.lastAt = at
	return nil
}

type fakeAICallLogRepo struct {
	created   []*model.AICallLog
	usedToday int64
}

func (r *fakeAICallLogRepo) WithTx(tx *gorm.DB) repository.AICallLogRepository {
	return r
}

func (r *fakeAICallLogRepo) Create(ctx context.Context, callLog *model.AICallLog) error {
	r.created = append(r.created, callLog)
	return nil
}

func (r *fakeAICallLogRepo) ListByConversationID(ctx context.Context, conversationID uint64, beforeID *uint64, limit int, botID *uint64, status string) ([]model.AICallLog, error) {
	return nil, nil
}

func (r *fakeAICallLogRepo) SumTotalTokensByConversationBetween(ctx context.Context, conversationID uint64, startAt time.Time, endAt time.Time) (int64, error) {
	return r.usedToday, nil
}

func (r *fakeAICallLogRepo) SumPlatformTotalTokensByConversationBetween(ctx context.Context, conversationID uint64, startAt time.Time, endAt time.Time) (int64, error) {
	return r.usedToday, nil
}

func (r *fakeAICallLogRepo) SumTotalTokensByConversationAndModelBetween(ctx context.Context, conversationID uint64, modelName string, startAt time.Time, endAt time.Time) (int64, error) {
	return r.usedToday, nil
}

func (r *fakeAICallLogRepo) SumTotalTokensByConversationAndProviderModelBetween(ctx context.Context, conversationID uint64, providerName string, modelName string, startAt time.Time, endAt time.Time) (int64, error) {
	return r.usedToday, nil
}

type fakeMemberRepo struct {
	members            []model.ConversationMember
	listConversationID uint64
}

func (r *fakeMemberRepo) WithTx(tx *gorm.DB) repository.MemberRepository {
	return r
}

func (r *fakeMemberRepo) Create(ctx context.Context, member *model.ConversationMember) error {
	return nil
}

func (r *fakeMemberRepo) Update(ctx context.Context, member *model.ConversationMember) error {
	return nil
}

func (r *fakeMemberRepo) UpdateLastReadMessageID(ctx context.Context, conversationID, userID, lastReadMessageID uint64) error {
	return nil
}

func (r *fakeMemberRepo) GetDB() *gorm.DB {
	return nil
}

func (r *fakeMemberRepo) GetUserMember(ctx context.Context, conversationID, userID uint64) (*model.ConversationMember, error) {
	for _, member := range r.members {
		if member.ConversationID == conversationID && member.MemberType == model.MemberTypeUser && member.MemberID == userID {
			memberCopy := member
			return &memberCopy, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeMemberRepo) IsUserMember(ctx context.Context, conversationID, userID uint64) (bool, error) {
	_, err := r.GetUserMember(ctx, conversationID, userID)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return false, err
}

func (r *fakeMemberRepo) ListUserMembers(ctx context.Context, conversationID uint64) ([]model.ConversationMember, error) {
	members := make([]model.ConversationMember, 0)
	for _, member := range r.members {
		if member.ConversationID == conversationID && member.MemberType == model.MemberTypeUser && member.Status == model.MemberStatusNormal {
			members = append(members, member)
		}
	}
	return members, nil
}

func (r *fakeMemberRepo) ListUserMemberIDs(ctx context.Context, conversationID uint64) ([]uint64, error) {
	r.listConversationID = conversationID
	memberIDs := make([]uint64, 0)
	for _, member := range r.members {
		if member.ConversationID == conversationID && member.MemberType == model.MemberTypeUser && member.Status == model.MemberStatusNormal {
			memberIDs = append(memberIDs, member.MemberID)
		}
	}
	return memberIDs, nil
}

func (r *fakeMemberRepo) GetBotMember(ctx context.Context, conversationID, botID uint64) (*model.ConversationMember, error) {
	for _, member := range r.members {
		if member.ConversationID == conversationID && member.MemberType == model.MemberTypeBot && member.MemberID == botID {
			memberCopy := member
			return &memberCopy, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeMemberRepo) IsBotMember(ctx context.Context, conversationID, botID uint64) (bool, error) {
	_, err := r.GetBotMember(ctx, conversationID, botID)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return false, err
}

func (r *fakeMemberRepo) ListBotMembers(ctx context.Context, conversationID uint64) ([]model.ConversationMember, error) {
	members := make([]model.ConversationMember, 0)
	for _, member := range r.members {
		if member.ConversationID == conversationID && member.MemberType == model.MemberTypeBot && member.Status == model.MemberStatusNormal {
			members = append(members, member)
		}
	}
	return members, nil
}

func (r *fakeMemberRepo) ListByConversationID(ctx context.Context, conversationID uint64) ([]model.ConversationMember, error) {
	r.listConversationID = conversationID
	return r.members, nil
}

type fakeReplyPublisher struct {
	events       []botmodel.BotReplyCreatedEvent
	streamEvents []botmodel.BotReplyStreamEvent
}

func (p *fakeReplyPublisher) PublishBotReplyCreated(ctx context.Context, event botmodel.BotReplyCreatedEvent) error {
	p.events = append(p.events, event)
	return nil
}

func (p *fakeReplyPublisher) PublishBotReplyStream(ctx context.Context, event botmodel.BotReplyStreamEvent) error {
	p.streamEvents = append(p.streamEvents, event)
	return nil
}

type fakeBotUserClient struct {
	users map[uint64]*rpc.UserInfo
}

func (c *fakeBotUserClient) GetUser(ctx context.Context, userID uint64) (*rpc.UserInfo, error) {
	user, ok := c.users[userID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return user, nil
}

func (c *fakeBotUserClient) CheckFriendRelation(ctx context.Context, userID uint64, friendUserID uint64) (bool, error) {
	return false, nil
}

type fakeRAGSearcher struct {
	chunks []RAGChunk
	err    error
	last   RAGSearchRequest
	called int
}

func (s *fakeRAGSearcher) SearchForConversation(ctx context.Context, req RAGSearchRequest) ([]RAGChunk, error) {
	s.called++
	s.last = req
	if s.err != nil {
		return nil, s.err
	}
	return s.chunks, nil
}

func TestServiceHandleMentionKnowledgeBaseOnlyUsesRAG(t *testing.T) {
	messageRepo := &fakeMessageRepo{
		recent: []model.Message{
			{ID: 1, ConversationID: 10, SenderID: 10001, SenderType: model.SenderTypeUser, MessageType: model.MessageTypeText, Content: datatypes.JSON(`{"text":"history message"}`), Status: model.MessageStatusNormal},
		},
	}
	conversationRepo := &fakeConversationRepo{}
	aiCallLogRepo := &fakeAICallLogRepo{}
	llmClient := &fakeLLMClient{response: &llm.GenerateResponse{Content: "ok"}}
	service := NewService(llmClient, messageRepo, conversationRepo, aiCallLogRepo)
	service.SetDefaultModel("env-model")
	service.SetLimiter(NewLimiter(10, 1))
	service.SetRAGTopK(3)
	searcher := &fakeRAGSearcher{
		chunks: []RAGChunk{
			{Index: 1, Content: "kb-1", Score: 0.92},
		},
	}
	service.SetRAGSearcher(searcher)
	service.SetMemberRepository(&fakeMemberRepo{
		members: []model.ConversationMember{
			{ConversationID: 10, MemberType: model.MemberTypeBot, MemberID: 7, Role: model.MemberRoleBot, Status: model.MemberStatusNormal},
		},
	})
	service.SetBotRepository(&fakeBotRepo{
		bots: map[uint64]*model.Bot{
			7: {ID: 7, MentionName: "aim", ModelName: "db-model", Status: model.BotStatusEnabled},
		},
	})
	conversationBotRepo := newFakeConversationBotRepo()
	_ = conversationBotRepo.Create(context.Background(), &model.ConversationBot{
		ConversationID:  10,
		BotID:           7,
		Enabled:         true,
		PermissionScope: model.BotScopeKnowledgeBaseOnly,
	})
	service.SetConversationBotRepository(conversationBotRepo)

	err := service.HandleMention(context.Background(), HandleMentionRequest{
		ConversationID: 10,
		UserID:         10001,
		Content:        "@aim summarize",
	})
	if err != nil {
		t.Fatalf("HandleMention returned error: %v", err)
	}
	if searcher.called != 1 {
		t.Fatalf("expected rag search once, got %d", searcher.called)
	}
	if searcher.last.TopK != 3 || searcher.last.ConversationID != 10 || searcher.last.Question != "summarize" {
		t.Fatalf("unexpected rag search request: %+v", searcher.last)
	}
	if len(llmClient.requests) != 1 {
		t.Fatalf("expected one llm call, got %d", len(llmClient.requests))
	}
	prompt := llmClient.requests[0].Messages[1].Content
	if strings.Contains(prompt, "\u3010\u7fa4\u804a\u4e0a\u4e0b\u6587\u3011") {
		t.Fatalf("knowledge-base-only prompt should not include conversation context: %q", prompt)
	}
	if !strings.Contains(prompt, "\u3010\u672c\u5730\u77e5\u8bc6\u5e93\u3011") || !strings.Contains(prompt, "kb-1") {
		t.Fatalf("knowledge-base-only prompt should include rag chunks: %q", prompt)
	}
}

func TestServiceHandleMentionKnowledgeBaseOnlyNoBindingReply(t *testing.T) {
	messageRepo := &fakeMessageRepo{}
	conversationRepo := &fakeConversationRepo{}
	aiCallLogRepo := &fakeAICallLogRepo{}
	llmClient := &fakeLLMClient{response: &llm.GenerateResponse{Content: "ok"}}
	service := NewService(llmClient, messageRepo, conversationRepo, aiCallLogRepo)
	service.SetDefaultModel("env-model")
	service.SetLimiter(NewLimiter(10, 1))
	searcher := &fakeRAGSearcher{chunks: nil}
	service.SetRAGSearcher(searcher)
	service.SetMemberRepository(&fakeMemberRepo{
		members: []model.ConversationMember{
			{ConversationID: 10, MemberType: model.MemberTypeBot, MemberID: 7, Role: model.MemberRoleBot, Status: model.MemberStatusNormal},
		},
	})
	service.SetBotRepository(&fakeBotRepo{
		bots: map[uint64]*model.Bot{
			7: {ID: 7, MentionName: "aim", ModelName: "db-model", Status: model.BotStatusEnabled},
		},
	})
	conversationBotRepo := newFakeConversationBotRepo()
	_ = conversationBotRepo.Create(context.Background(), &model.ConversationBot{
		ConversationID:  10,
		BotID:           7,
		Enabled:         true,
		PermissionScope: model.BotScopeKnowledgeBaseOnly,
	})
	service.SetConversationBotRepository(conversationBotRepo)

	err := service.HandleMention(context.Background(), HandleMentionRequest{
		ConversationID: 10,
		UserID:         10001,
		Content:        "@aim summarize",
	})
	if err != nil {
		t.Fatalf("HandleMention returned error: %v", err)
	}
	if len(llmClient.requests) != 0 {
		t.Fatalf("llm should not be called when kb-only has no rag chunks")
	}
	if len(messageRepo.created) != 1 {
		t.Fatalf("expected one fallback bot reply, got %d", len(messageRepo.created))
	}
	got := string(messageRepo.created[0].Content)
	if !strings.Contains(got, "\u5f53\u524d\u672a\u68c0\u7d22\u5230\u53ef\u7528\u7684\u77e5\u8bc6\u5e93\u8d44\u6599") {
		t.Fatalf("unexpected fallback reply: %q", got)
	}
}

func TestServiceHandleMentionConversationAndKBContinuesOnRAGError(t *testing.T) {
	messageRepo := &fakeMessageRepo{
		recent: []model.Message{
			{ID: 1, ConversationID: 10, SenderID: 10001, SenderType: model.SenderTypeUser, MessageType: model.MessageTypeText, Content: datatypes.JSON(`{"text":"history message"}`), Status: model.MessageStatusNormal},
		},
	}
	conversationRepo := &fakeConversationRepo{}
	aiCallLogRepo := &fakeAICallLogRepo{}
	llmClient := &fakeLLMClient{response: &llm.GenerateResponse{Content: "ok"}}
	service := NewService(llmClient, messageRepo, conversationRepo, aiCallLogRepo)
	service.SetDefaultModel("env-model")
	service.SetLimiter(NewLimiter(10, 1))
	searcher := &fakeRAGSearcher{err: errors.New("rag failed")}
	service.SetRAGSearcher(searcher)
	service.SetMemberRepository(&fakeMemberRepo{
		members: []model.ConversationMember{
			{ConversationID: 10, MemberType: model.MemberTypeBot, MemberID: 7, Role: model.MemberRoleBot, Status: model.MemberStatusNormal},
		},
	})
	service.SetBotRepository(&fakeBotRepo{
		bots: map[uint64]*model.Bot{
			7: {ID: 7, MentionName: "aim", ModelName: "db-model", Status: model.BotStatusEnabled},
		},
	})
	conversationBotRepo := newFakeConversationBotRepo()
	_ = conversationBotRepo.Create(context.Background(), &model.ConversationBot{
		ConversationID:  10,
		BotID:           7,
		Enabled:         true,
		PermissionScope: model.BotScopeConversationAndKB,
	})
	service.SetConversationBotRepository(conversationBotRepo)

	err := service.HandleMention(context.Background(), HandleMentionRequest{
		ConversationID: 10,
		UserID:         10001,
		Content:        "@aim summarize",
	})
	if err != nil {
		t.Fatalf("HandleMention returned error: %v", err)
	}
	if searcher.called != 1 {
		t.Fatalf("expected rag search once, got %d", searcher.called)
	}
	if len(llmClient.requests) != 1 {
		t.Fatalf("llm should continue when rag fails in mixed scope")
	}
	if len(messageRepo.created) != 1 {
		t.Fatalf("expected normal bot reply, got %d", len(messageRepo.created))
	}
}

func TestServiceHandleMentionSummaryIntentForcesConversationOnly(t *testing.T) {
	messageRepo := &fakeMessageRepo{
		recent: []model.Message{
			{ID: 1, ConversationID: 10, SenderID: 10001, SenderType: model.SenderTypeUser, MessageType: model.MessageTypeText, Content: datatypes.JSON(`{"text":"history message"}`), Status: model.MessageStatusNormal},
		},
	}
	conversationRepo := &fakeConversationRepo{}
	aiCallLogRepo := &fakeAICallLogRepo{}
	llmClient := &fakeLLMClient{response: &llm.GenerateResponse{Content: "ok"}}
	service := NewService(llmClient, messageRepo, conversationRepo, aiCallLogRepo)
	service.SetDefaultModel("env-model")
	service.SetLimiter(NewLimiter(10, 1))
	searcher := &fakeRAGSearcher{
		chunks: []RAGChunk{
			{Index: 1, Content: "kb-1", Score: 0.92},
		},
	}
	service.SetRAGSearcher(searcher)
	service.SetMemberRepository(&fakeMemberRepo{
		members: []model.ConversationMember{
			{ConversationID: 10, MemberType: model.MemberTypeBot, MemberID: 7, Role: model.MemberRoleBot, Status: model.MemberStatusNormal},
		},
	})
	service.SetBotRepository(&fakeBotRepo{
		bots: map[uint64]*model.Bot{
			7: {ID: 7, MentionName: "aim", ModelName: "db-model", Status: model.BotStatusEnabled},
		},
	})
	conversationBotRepo := newFakeConversationBotRepo()
	_ = conversationBotRepo.Create(context.Background(), &model.ConversationBot{
		ConversationID:  10,
		BotID:           7,
		Enabled:         true,
		PermissionScope: model.BotScopeConversationAndKB,
	})
	service.SetConversationBotRepository(conversationBotRepo)

	err := service.HandleMention(context.Background(), HandleMentionRequest{
		ConversationID: 10,
		UserID:         10001,
		Content:        "@aim 总结群聊消息",
	})
	if err != nil {
		t.Fatalf("HandleMention returned error: %v", err)
	}
	if searcher.called != 0 {
		t.Fatalf("expected rag search skipped by summary intent, got %d calls", searcher.called)
	}
	if len(llmClient.requests) != 1 {
		t.Fatalf("expected one llm call, got %d", len(llmClient.requests))
	}
	prompt := llmClient.requests[0].Messages[1].Content
	if strings.Contains(prompt, "【本地知识库】") {
		t.Fatalf("summary intent should not include knowledge base prompt: %q", prompt)
	}
}

func TestSupportsVisionModel(t *testing.T) {
	if !supportsVisionModel("qwen3.6-plus") {
		t.Fatalf("qwen3.6-plus should support vision")
	}
	if !supportsVisionModel("qwen3.5-plus") {
		t.Fatalf("qwen3.5-plus should support vision")
	}
	if supportsVisionModel("qwen-plus") {
		t.Fatalf("qwen-plus should not support vision")
	}
}
