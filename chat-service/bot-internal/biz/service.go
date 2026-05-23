package bot

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	botclient "example.com/aim/chat-service/bot-internal/client"
	botmodel "example.com/aim/chat-service/bot-internal/model"
	"example.com/aim/chat-service/internal/dal/model"
	"example.com/aim/chat-service/internal/repository"
	"example.com/aim/chat-service/internal/rpc"
	llm "example.com/aim/chat-service/llm-internal/client"
	"example.com/aim/shared/errno"
	"gorm.io/gorm"
)

type HandleMentionRequest struct {
	ConversationID   uint64
	RequestMessageID uint64
	UserID           uint64
	Content          string
}

type MentionHandler interface {
	HandleMention(ctx context.Context, req HandleMentionRequest) error
}

type Service struct {
	LLM                  llm.Client
	LLMSelector          func(bot model.Bot) (llm.Client, string, error)
	LLMTimeout           time.Duration
	DefaultModel         string
	ContextMessages      int
	DailyTokenLimit      int64
	Limiter              *Limiter
	MessageRepo          repository.MessageRepository
	ConversationRepo     repository.ConversationRepository
	MemberRepo           repository.MemberRepository
	BotRepo              repository.BotRepository
	ConversationBotRepo  repository.ConversationBotRepository
	AICallLogRepo        repository.AICallLogRepository
	ReplyPublisher       botclient.ReplyPublisher
	UserClient           rpc.UserClient
	RAGSearcher          RAGSearcher
	RAGTopK              int
	UserMemoryRepo       repository.UserMemoryRepository
	UserMemoryExtractor  UserMemoryExtractor
	MemoryExtractTimeout time.Duration
	MemoryCandidateLimit int
	MemoryMaxRecall      int
}

type RAGChunk struct {
	Index   int
	Content string
	Score   float64
}

type RAGSearchRequest struct {
	UserID         uint64
	ConversationID uint64
	Question       string
	TopK           int
}

type RAGSearcher interface {
	SearchForConversation(ctx context.Context, req RAGSearchRequest) ([]RAGChunk, error)
}

type llmStreamingClient interface {
	GenerateStream(ctx context.Context, req llm.GenerateRequest, onChunk func(llm.StreamChunk) error) (*llm.GenerateResponse, error)
}

type botReplyStreamMeta struct {
	info             botmodel.BotReplyStreamInfo
	recipientUserIDs []int64
}

var summaryCountDirectivePattern = regexp.MustCompile(`(?i)\[AIM_SUMMARY_COUNT=(\d{1,4})\]`)

func NewService(
	llmClient llm.Client,
	messageRepo repository.MessageRepository,
	conversationRepo repository.ConversationRepository,
	aiCallLogRepo repository.AICallLogRepository,
) *Service {
	return &Service{
		LLM:                  llmClient,
		ContextMessages:      20,
		DailyTokenLimit:      1_000_000,
		MemoryExtractTimeout: 6 * time.Second,
		MemoryCandidateLimit: 20,
		MemoryMaxRecall:      5,
		MessageRepo:          messageRepo,
		ConversationRepo:     conversationRepo,
		AICallLogRepo:        aiCallLogRepo,
	}
}

func (s *Service) SetDefaultModel(modelName string) {
	s.DefaultModel = strings.TrimSpace(modelName)
}

func (s *Service) SetLLMTimeout(timeout time.Duration) {
	if timeout > 0 {
		s.LLMTimeout = timeout
	}
}

func (s *Service) SetLimiter(limiter *Limiter) {
	s.Limiter = limiter
}

func (s *Service) SetLLMSelector(selector func(bot model.Bot) (llm.Client, string, error)) {
	s.LLMSelector = selector
}

func (s *Service) SetDailyTokenLimit(limit int64) {
	if limit > 0 {
		s.DailyTokenLimit = limit
	}
}

func (s *Service) SetContextMessages(limit int) {
	if limit > 0 {
		s.ContextMessages = limit
	}
}

func (s *Service) SetMemberRepository(memberRepo repository.MemberRepository) {
	s.MemberRepo = memberRepo
}

func (s *Service) SetBotRepository(botRepo repository.BotRepository) {
	s.BotRepo = botRepo
}

func (s *Service) SetConversationBotRepository(conversationBotRepo repository.ConversationBotRepository) {
	s.ConversationBotRepo = conversationBotRepo
}

func (s *Service) SetReplyPublisher(replyPublisher botclient.ReplyPublisher) {
	s.ReplyPublisher = replyPublisher
}

func (s *Service) SetUserClient(userClient rpc.UserClient) {
	s.UserClient = userClient
}

func (s *Service) SetRAGSearcher(searcher RAGSearcher) {
	s.RAGSearcher = searcher
}

func (s *Service) SetRAGTopK(topK int) {
	if topK <= 0 {
		return
	}
	if topK > 10 {
		topK = 10
	}
	s.RAGTopK = topK
}

func (s *Service) SetUserMemoryRepository(repo repository.UserMemoryRepository) {
	s.UserMemoryRepo = repo
}

func (s *Service) SetUserMemoryExtractor(extractor UserMemoryExtractor) {
	s.UserMemoryExtractor = extractor
}

func (s *Service) SetMemoryExtractTimeout(timeout time.Duration) {
	if timeout > 0 {
		s.MemoryExtractTimeout = timeout
	}
}

func (s *Service) SetMemoryCandidateLimit(limit int) {
	if limit > 0 {
		s.MemoryCandidateLimit = limit
	}
}

func (s *Service) SetMemoryMaxRecall(limit int) {
	if limit > 0 {
		s.MemoryMaxRecall = limit
	}
}

func (s *Service) HandleMention(ctx context.Context, req HandleMentionRequest) error {
	if s == nil {
		return errno.Internal("bot service is nil")
	}
	if s.LLM == nil {
		return errno.Required("llm client")
	}
	if s.MessageRepo == nil {
		return errno.Required("message repository")
	}
	if s.ConversationRepo == nil {
		return errno.Required("conversation repository")
	}
	if s.MemberRepo == nil {
		return errno.Required("member repository")
	}
	if s.BotRepo == nil {
		return errno.Required("bot repository")
	}
	if s.ConversationBotRepo == nil {
		return errno.Required("conversation bot repository")
	}
	if s.AICallLogRepo == nil {
		return errno.Required("ai call log repository")
	}
	if req.ConversationID == 0 {
		return errno.Required("conversation id")
	}

	resolved, err := s.resolveTargetBot(ctx, req.ConversationID, req.Content)
	if err != nil {
		return err
	}
	if resolved == nil {
		log.Printf("bot mention unresolved: conversation=%d request_message=%d user=%d content=%q", req.ConversationID, req.RequestMessageID, req.UserID, req.Content)
		return nil
	}
	release, err := s.acquireConcurrencySlot(req.ConversationID, req, resolved.Bot)
	if err != nil {
		return err
	}
	defer release()

	modelName := EffectiveModelName(resolved.Bot, resolved.ConversationBot, s.DefaultModel)
	if modelName == "" {
		start := time.Now()
		err := errno.Required("model")
		latencyMS := time.Since(start).Milliseconds()
		if logErr := s.createFailedLog(ctx, req, resolved.Bot, modelName, err, latencyMS); logErr != nil {
			return fmt.Errorf("record failed ai call log: %w; bot error: %v", logErr, err)
		}
		return err
	}

	contextLimit := s.ContextMessages
	if contextLimit <= 0 {
		contextLimit = 20
	}
	question := ExtractQuestion(req.Content)
	question, summaryContextLimit := extractSummaryContextLimit(question)
	if summaryContextLimit > 0 {
		contextLimit = summaryContextLimit
	}
	recentMessages, err := s.MessageRepo.ListByConversationID(ctx, req.ConversationID, nil, contextLimit)
	if err != nil {
		return err
	}
	recentMessages = filterBotVisibleMessages(recentMessages)
	userDisplayNames := s.resolveUserDisplayNames(ctx, recentMessages, req.UserID)
	promptContent := rebuildMentionContent(req.Content, question)
	scope := resolved.ConversationBot.PermissionScope
	if scope == "" {
		scope = model.BotScopeConversationOnly
	}
	if shouldForceConversationOnlyByIntent(question) {
		scope = model.BotScopeConversationOnly
	}
	ragTopK := s.RAGTopK
	if ragTopK <= 0 {
		ragTopK = 5
	}
	ragChunks, ragErr := s.searchRAGChunks(ctx, scope, req.UserID, req.ConversationID, question, ragTopK)
	if ragErr != nil {
		log.Printf("rag search failed: conversation=%d bot=%d scope=%s err=%v", req.ConversationID, resolved.Bot.ID, scope, ragErr)
		if scope == model.BotScopeKnowledgeBaseOnly {
			if _, replyErr := s.createBotReplyMessage(ctx, req, resolved.Bot, "\u77e5\u8bc6\u5e93\u68c0\u7d22\u5931\u8d25\uff0c\u8bf7\u7a0d\u540e\u518d\u8bd5"); replyErr != nil {
				return replyErr
			}
			_ = s.createFailedLog(ctx, req, resolved.Bot, modelName, ragErr, 0)
			return nil
		}
	}
	longTermMemories := s.collectLongTermMemories(ctx, req, question)
	prompt := BuildPromptWithRAGAndMemory(recentMessages, promptContent, contextLimit, userDisplayNames, req.UserID, scope, ragChunks, longTermMemories)
	if scope == model.BotScopeKnowledgeBaseOnly && len(ragChunks) == 0 {
		if _, replyErr := s.createBotReplyMessage(ctx, req, resolved.Bot, "\u5f53\u524d\u672a\u68c0\u7d22\u5230\u53ef\u7528\u7684\u77e5\u8bc6\u5e93\u8d44\u6599\uff0c\u65e0\u6cd5\u57fa\u4e8e\u77e5\u8bc6\u5e93\u56de\u7b54\u3002"); replyErr != nil {
			return replyErr
		}
		return nil
	}
	systemPrompt := strings.TrimSpace(resolved.Bot.SystemPrompt)
	if systemPrompt == "" {
		systemPrompt = DefaultSystemPrompt
	}

	if err := s.checkDailyTokenLimit(ctx, req, resolved.Bot); err != nil {
		if logErr := s.createFailedLog(ctx, req, resolved.Bot, modelName, err, 0); logErr != nil {
			return fmt.Errorf("record failed ai call log: %w; bot error: %v", logErr, err)
		}
		return err
	}

	start := time.Now()
	llmClient := s.LLM
	if s.LLMSelector != nil {
		selectedClient, _, selectErr := s.LLMSelector(resolved.Bot)
		if selectErr != nil {
			if logErr := s.createFailedLog(ctx, req, resolved.Bot, modelName, selectErr, 0); logErr != nil {
				return fmt.Errorf("record failed ai call log: %w; llm select error: %v", logErr, selectErr)
			}
			return selectErr
		}
		if selectedClient != nil {
			llmClient = selectedClient
		}
	}

	parts, err := buildUserPromptParts(ctx, prompt, recentMessages, supportsVisionModel(modelName))
	if err != nil {
		if logErr := s.createFailedLog(ctx, req, resolved.Bot, modelName, err, 0); logErr != nil {
			return fmt.Errorf("record failed ai call log: %w; build prompt parts error: %v", logErr, err)
		}
		if _, replyErr := s.createBotReplyMessage(ctx, req, resolved.Bot, err.Error()); replyErr != nil {
			return fmt.Errorf("create bot failure reply error: %w; build prompt parts error: %v", replyErr, err)
		}
		return nil
	}

	llmCtx := ctx
	cancel := func() {}
	if s.LLMTimeout > 0 {
		llmCtx, cancel = context.WithTimeout(ctx, s.LLMTimeout)
	}
	defer cancel()

	llmReq := llm.GenerateRequest{
		Model: modelName,
		Messages: []llm.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{
				Role:    "user",
				Content: prompt,
				Parts:   parts,
			},
		},
	}
	var resp *llm.GenerateResponse
	llmStart := time.Now()
	streamer, supportsStreaming := llmClient.(llmStreamingClient)
	if supportsStreaming {
		var (
			contentBuilder strings.Builder
			streamState    *botReplyStreamMeta
			firstChunkAt   time.Time
			chunkCount     int
		)
		streamMeta, metaErr := s.buildStreamMeta(ctx, req, resolved.Bot)
		if metaErr != nil {
			log.Printf("bot stream meta unavailable: conversation=%d bot=%d err=%v", req.ConversationID, resolved.Bot.ID, metaErr)
		} else {
			streamState = streamMeta
			// Backend-confirmed trigger signal: tell clients the bot task has started.
			s.publishBotReplyStream(ctx, *streamState)
		}
		resp, err = streamer.GenerateStream(llmCtx, llmReq, func(chunk llm.StreamChunk) error {
			chunkCount++
			if firstChunkAt.IsZero() && (chunk.Content != "" || chunk.ReasoningContent != "") {
				firstChunkAt = time.Now()
			}
			if chunk.Content != "" {
				contentBuilder.WriteString(chunk.Content)
				if streamState != nil {
					streamState.info.Content = contentBuilder.String()
					streamState.info.Done = false
					s.publishBotReplyStream(ctx, *streamState)
				}
			}
			return nil
		})
		log.Printf(
			"bot stream timing: conversation=%d bot=%d model=%s first_chunk_ms=%d llm_total_ms=%d chunks=%d",
			req.ConversationID,
			resolved.Bot.ID,
			modelName,
			durationMillis(llmStart, firstChunkAt),
			time.Since(llmStart).Milliseconds(),
			chunkCount,
		)
		if err == nil && resp != nil {
			if resp.Content == "" {
				resp.Content = contentBuilder.String()
			}
			if streamState != nil && strings.TrimSpace(resp.Content) != "" {
				streamState.info.Content = resp.Content
				streamState.info.Done = true
				s.publishBotReplyStream(ctx, *streamState)
			}
		}
	} else {
		resp, err = llmClient.Generate(llmCtx, llmReq)
		log.Printf(
			"bot llm timing: conversation=%d bot=%d model=%s llm_total_ms=%d mode=non_stream",
			req.ConversationID,
			resolved.Bot.ID,
			modelName,
			time.Since(llmStart).Milliseconds(),
		)
	}
	latencyMS := time.Since(start).Milliseconds()
	if err != nil {
		if logErr := s.createFailedLog(ctx, req, resolved.Bot, modelName, err, latencyMS); logErr != nil {
			return fmt.Errorf("record failed ai call log: %w; llm error: %v", logErr, err)
		}
		var statusErr *llm.HTTPStatusError
		if errors.As(err, &statusErr) {
			if _, replyErr := s.createBotReplyMessage(ctx, req, resolved.Bot, "\u6a21\u578b\u8c03\u7528\u5931\u8d25\uff0c\u8bf7\u7a0d\u540e\u518d\u8bd5\u3002"); replyErr != nil {
				return fmt.Errorf("create bot failure reply error: %w; llm error: %v", replyErr, err)
			}
			return nil
		}
		return err
	}
	if resp == nil {
		err := errno.Internal("llm response is nil")
		if logErr := s.createFailedLog(ctx, req, resolved.Bot, modelName, err, latencyMS); logErr != nil {
			return fmt.Errorf("record failed ai call log: %w; bot error: %v", logErr, err)
		}
		return err
	}

	botMessage, err := s.createBotReplyMessage(ctx, req, resolved.Bot, resp.Content)
	if err != nil {
		if logErr := s.createFailedLog(ctx, req, resolved.Bot, modelName, err, latencyMS); logErr != nil {
			return fmt.Errorf("record failed ai call log: %w; create bot reply error: %v", logErr, err)
		}
		return err
	}
	return s.createSuccessLog(ctx, req, resolved.Bot, modelName, botMessage.ID, resp, latencyMS)
}

func (s *Service) searchRAGChunks(ctx context.Context, scope model.BotPermissionScope, userID uint64, conversationID uint64, question string, topK int) ([]RAGChunk, error) {
	if scope == model.BotScopeConversationOnly {
		return nil, nil
	}
	if strings.TrimSpace(question) == "" {
		return nil, nil
	}
	if s.RAGSearcher == nil {
		return nil, nil
	}
	chunks, err := s.RAGSearcher.SearchForConversation(ctx, RAGSearchRequest{
		UserID:         userID,
		ConversationID: conversationID,
		Question:       question,
		TopK:           topK,
	})
	if err != nil {
		return nil, err
	}
	return chunks, nil
}

func shouldForceConversationOnlyByIntent(question string) bool {
	normalized := strings.ToLower(strings.TrimSpace(question))
	if normalized == "" {
		return false
	}
	return strings.Contains(normalized, "总结群聊") ||
		strings.Contains(normalized, "总结最近消息") ||
		strings.Contains(normalized, "总结一下群聊") ||
		strings.Contains(normalized, "总结下群聊") ||
		strings.Contains(normalized, "群聊总结")
}

func extractSummaryContextLimit(question string) (string, int) {
	normalized := strings.TrimSpace(question)
	if normalized == "" {
		return "", 0
	}
	matches := summaryCountDirectivePattern.FindStringSubmatch(normalized)
	if len(matches) < 2 {
		return normalized, 0
	}
	value, err := strconv.Atoi(matches[1])
	if err != nil {
		return strings.TrimSpace(summaryCountDirectivePattern.ReplaceAllString(normalized, "")), 0
	}
	if value < 20 {
		value = 20
	}
	if value > 500 {
		value = 500
	}
	clean := strings.TrimSpace(summaryCountDirectivePattern.ReplaceAllString(normalized, ""))
	return clean, value
}

func rebuildMentionContent(rawContent string, question string) string {
	trimmed := strings.TrimSpace(rawContent)
	token, ok := leadingMentionToken(trimmed)
	if !ok {
		return strings.TrimSpace(question)
	}
	question = strings.TrimSpace(question)
	if question == "" {
		return "@" + token
	}
	return "@" + token + " " + question
}

func (s *Service) resolveTargetBot(ctx context.Context, conversationID uint64, content string) (*resolvedBot, error) {
	token, ok := leadingMentionToken(content)
	if !ok {
		return nil, nil
	}

	conversationBots, err := s.ConversationBotRepo.ListByConversationID(ctx, conversationID)
	if err != nil {
		return nil, err
	}

	matches := make([]resolvedBot, 0, 1)
	enabledBots := make([]resolvedBot, 0, len(conversationBots))
	for _, conversationBot := range conversationBots {
		if !conversationBot.Enabled {
			continue
		}

		member, err := s.MemberRepo.GetBotMember(ctx, conversationID, conversationBot.BotID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return nil, err
		}
		if member.Status != model.MemberStatusNormal {
			continue
		}

		botModel, err := s.BotRepo.GetByID(ctx, conversationBot.BotID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return nil, err
		}
		if botModel.Status != model.BotStatusEnabled {
			continue
		}
		current := resolvedBot{
			Bot:             *botModel,
			ConversationBot: conversationBot,
		}
		enabledBots = append(enabledBots, current)

		match, err := s.matchesToken(token, *botModel, conversationBot)
		if err != nil {
			return nil, err
		}
		if match {
			matches = append(matches, current)
		}
	}

	if len(matches) == 0 {
		if isGenericBotMentionToken(token) {
			return pickDefaultBotForGenericMention(enabledBots), nil
		}
		return nil, nil
	}
	if len(matches) > 1 {
		log.Printf("bot mention token %q matched multiple bots in conversation %d; skipping", token, conversationID)
		return nil, nil
	}
	return &matches[0], nil
}

func isGenericBotMentionToken(token string) bool {
	token = strings.ToLower(strings.TrimSpace(token))
	switch token {
	case "bot", "assistant":
		return true
	default:
		return false
	}
}

func pickDefaultBotForGenericMention(bots []resolvedBot) *resolvedBot {
	if len(bots) == 0 {
		return nil
	}
	for index := range bots {
		mention := strings.ToLower(strings.TrimSpace(bots[index].ConversationBot.MentionNameOverride))
		if mention == "" {
			mention = strings.ToLower(strings.TrimSpace(bots[index].Bot.MentionName))
		}
		if mention == "ai" {
			result := bots[index]
			return &result
		}
	}
	result := bots[0]
	return &result
}

func (s *Service) matchesToken(token string, botModel model.Bot, conversationBot model.ConversationBot) (bool, error) {
	if token == effectiveMentionName(botModel, conversationBot) {
		return true, nil
	}

	aliases, err := effectiveAliases(botModel, conversationBot)
	if err != nil {
		return false, err
	}
	for _, alias := range aliases {
		if token == alias {
			return true, nil
		}
	}
	return false, nil
}

func (s *Service) publishBotReplyCreated(ctx context.Context, req HandleMentionRequest, message *model.Message) error {
	if s.MemberRepo == nil || s.ReplyPublisher == nil || message == nil {
		return nil
	}
	conversation, err := s.ConversationRepo.GetByID(ctx, req.ConversationID)
	if err != nil {
		return err
	}
	memberIDs, err := s.MemberRepo.ListUserMemberIDs(ctx, req.ConversationID)
	if err != nil {
		return err
	}

	recipientUserIDs := make([]int64, 0, len(memberIDs))
	for _, memberID := range memberIDs {
		recipientUserIDs = append(recipientUserIDs, int64(memberID))
	}
	var replyToID *int64
	if message.ReplyToID != nil {
		value := int64(*message.ReplyToID)
		replyToID = &value
	}

	return s.ReplyPublisher.PublishBotReplyCreated(ctx, botmodel.BotReplyCreatedEvent{
		Message: botmodel.BotReplyMessageInfo{
			ID:             int64(message.ID),
			ConversationID: conversation.ConversationID,
			SenderID:       int64(message.SenderID),
			SenderType:     string(message.SenderType),
			MessageType:    string(message.MessageType),
			Content:        string(message.Content),
			ReplyToID:      replyToID,
			Status:         string(message.Status),
			CreatedAt:      message.CreatedAt.Unix(),
		},
		RecipientUserIDs: recipientUserIDs,
	})
}

func (s *Service) buildStreamMeta(ctx context.Context, req HandleMentionRequest, botModel model.Bot) (*botReplyStreamMeta, error) {
	if s.MemberRepo == nil || s.ReplyPublisher == nil {
		return nil, nil
	}
	conversation, err := s.ConversationRepo.GetByID(ctx, req.ConversationID)
	if err != nil {
		return nil, err
	}
	memberIDs, err := s.MemberRepo.ListUserMemberIDs(ctx, req.ConversationID)
	if err != nil {
		return nil, err
	}
	recipientUserIDs := make([]int64, 0, len(memberIDs))
	for _, memberID := range memberIDs {
		recipientUserIDs = append(recipientUserIDs, int64(memberID))
	}
	return &botReplyStreamMeta{
		info: botmodel.BotReplyStreamInfo{
			ConversationID: conversation.ConversationID,
			SenderID:       int64(botModel.ID),
			SenderType:     string(model.SenderTypeBot),
			MessageType:    string(model.MessageTypeBotReply),
			Content:        "",
			Done:           false,
		},
		recipientUserIDs: recipientUserIDs,
	}, nil
}

func (s *Service) publishBotReplyStream(ctx context.Context, stream botReplyStreamMeta) {
	if s.ReplyPublisher == nil || stream.info.ConversationID == "" || len(stream.recipientUserIDs) == 0 {
		return
	}
	event := botmodel.BotReplyStreamEvent{
		Stream:           stream.info,
		RecipientUserIDs: stream.recipientUserIDs,
	}
	if err := s.ReplyPublisher.PublishBotReplyStream(ctx, event); err != nil {
		log.Printf("publish bot reply stream event failed: %v", err)
	}
}

func (s *Service) acquireConcurrencySlot(conversationID uint64, req HandleMentionRequest, botModel model.Bot) (func(), error) {
	if s.Limiter == nil {
		return func() {}, nil
	}

	release, err := s.Limiter.TryAcquire(conversationID)
	if err == nil {
		return release, nil
	}

	log.Printf("bot concurrency limit reached: conversation=%d bot=%d err=%v", conversationID, botModel.ID, err)
	if logErr := s.createFailedLog(context.Background(), req, botModel, "", err, 0); logErr != nil {
		return nil, fmt.Errorf("record failed ai call log: %w; concurrency error: %v", logErr, err)
	}
	return nil, err
}

func (s *Service) createSuccessLog(ctx context.Context, req HandleMentionRequest, botModel model.Bot, modelName string, responseMessageID uint64, resp *llm.GenerateResponse, latencyMS int64) error {
	requestMessageID := nullableID(req.RequestMessageID)
	return s.AICallLogRepo.Create(ctx, &model.AICallLog{
		UserID:            req.UserID,
		BotID:             botModel.ID,
		ConversationID:    req.ConversationID,
		RequestMessageID:  requestMessageID,
		ResponseMessageID: &responseMessageID,
		ModelName:         modelName,
		PromptTokens:      resp.PromptTokens,
		CompletionTokens:  resp.CompletionTokens,
		TotalTokens:       resp.TotalTokens,
		LatencyMS:         latencyMS,
		Status:            model.AICallStatusSuccess,
	})
}

func (s *Service) createFailedLog(ctx context.Context, req HandleMentionRequest, botModel model.Bot, modelName string, cause error, latencyMS int64) error {
	requestMessageID := nullableID(req.RequestMessageID)
	errorMessage := ""
	if cause != nil {
		errorMessage = cause.Error()
	}
	if modelName == "" {
		modelName = strings.TrimSpace(botModel.ModelName)
	}
	if modelName == "" {
		modelName = strings.TrimSpace(s.DefaultModel)
	}
	return s.AICallLogRepo.Create(ctx, &model.AICallLog{
		UserID:           req.UserID,
		BotID:            botModel.ID,
		ConversationID:   req.ConversationID,
		RequestMessageID: requestMessageID,
		ModelName:        modelName,
		LatencyMS:        latencyMS,
		Status:           model.AICallStatusFailed,
		ErrorMessage:     errorMessage,
	})
}

func (s *Service) createBotReplyMessage(ctx context.Context, req HandleMentionRequest, botModel model.Bot, content string) (*model.Message, error) {
	normalizedContent, err := model.NormalizeTextMessageContent(content)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	botMessage := &model.Message{
		ConversationID: req.ConversationID,
		SenderID:       botModel.ID,
		SenderType:     model.SenderTypeBot,
		MessageType:    model.MessageTypeBotReply,
		Content:        normalizedContent,
		Status:         model.MessageStatusNormal,
		CreatedAt:      now,
	}
	if err := s.MessageRepo.Create(ctx, botMessage); err != nil {
		return nil, err
	}
	if err := s.ConversationRepo.UpdateLastMessage(ctx, req.ConversationID, botMessage.ID, botMessage.CreatedAt); err != nil {
		return nil, err
	}
	if err := s.publishBotReplyCreated(ctx, req, botMessage); err != nil {
		log.Printf("publish bot reply created event failed: %v", err)
	}
	return botMessage, nil
}

type resolvedBot struct {
	Bot             model.Bot
	ConversationBot model.ConversationBot
}

func nullableID(id uint64) *uint64 {
	if id == 0 {
		return nil
	}
	return &id
}

func durationMillis(start time.Time, end time.Time) int64 {
	if start.IsZero() || end.IsZero() {
		return -1
	}
	return end.Sub(start).Milliseconds()
}

func (s *Service) checkDailyTokenLimit(ctx context.Context, req HandleMentionRequest, botModel model.Bot) error {
	if s.AICallLogRepo == nil || s.DailyTokenLimit <= 0 {
		return nil
	}
	// User-owned bots (CreatedBy > 0) should not consume platform quota.
	if botModel.CreatedBy > 0 {
		return nil
	}
	startAt, endAt := dayWindow()
	total, err := s.AICallLogRepo.SumTotalTokensByConversationBetween(ctx, req.ConversationID, startAt, endAt)
	if err != nil {
		return err
	}
	if total >= s.DailyTokenLimit {
		return errno.Forbidden("daily ai token limit reached")
	}
	return nil
}

func dayWindow() (time.Time, time.Time) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return start, start.Add(24 * time.Hour)
}

func (s *Service) resolveUserDisplayNames(ctx context.Context, recentMessages []model.Message, currentUserID uint64) map[uint64]string {
	userIDs := make(map[uint64]struct{})
	if currentUserID > 0 {
		userIDs[currentUserID] = struct{}{}
	}
	for _, msg := range recentMessages {
		if msg.SenderType == model.SenderTypeUser && msg.SenderID > 0 {
			userIDs[msg.SenderID] = struct{}{}
		}
	}
	if len(userIDs) == 0 {
		return nil
	}

	displayNames := make(map[uint64]string, len(userIDs))
	if s.UserClient == nil {
		return displayNames
	}
	for userID := range userIDs {
		user, err := s.UserClient.GetUser(ctx, userID)
		if err != nil || user == nil {
			continue
		}
		name := strings.TrimSpace(user.Nickname)
		if name == "" {
			continue
		}
		displayNames[userID] = name
	}
	return displayNames
}

func buildUserPromptParts(ctx context.Context, prompt string, recentMessages []model.Message, enableImages bool) ([]llm.ChatMessagePart, error) {
	parts := []llm.ChatMessagePart{
		{
			Type: "text",
			Text: prompt,
		},
	}
	if !enableImages {
		return parts, nil
	}

	for _, msg := range recentMessages {
		if msg.MessageType != model.MessageTypeImage {
			continue
		}
		var image model.ImageMessageContent
		if err := json.Unmarshal([]byte(strings.TrimSpace(string(msg.Content))), &image); err != nil {
			continue
		}
		imageURL := strings.TrimSpace(image.URL)
		if imageURL == "" {
			continue
		}
		normalizedURL, err := normalizeImageURLForLLM(ctx, imageURL)
		if err != nil {
			return nil, err
		}
		parts = append(parts, llm.ChatMessagePart{
			Type:     "image_url",
			ImageURL: normalizedURL,
		})
	}
	return parts, nil
}

func supportsVisionModel(modelName string) bool {
	switch strings.ToLower(strings.TrimSpace(modelName)) {
	case "qwen3.6-plus", "qwen3.5-plus":
		return true
	default:
		return false
	}
}

func filterBotVisibleMessages(messages []model.Message) []model.Message {
	if len(messages) == 0 {
		return messages
	}
	filtered := make([]model.Message, 0, len(messages))
	for _, msg := range messages {
		if msg.Status != model.MessageStatusNormal {
			continue
		}
		filtered = append(filtered, msg)
	}
	return filtered
}

func normalizeImageURLForLLM(ctx context.Context, raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", errno.Required("image URL")
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "data:") {
		if strings.Contains(lower, ";base64,") {
			return value, nil
		}
		return "", errno.BadRequest("invalid base64 image format, expected data:*;base64, prefix")
	}
	if strings.HasPrefix(lower, "file://") {
		return "", errno.BadRequest("file:// is not supported in OpenAI-compatible mode")
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return "", errno.BadRequest("invalid image URL format, expected http(s) URL or data:base64")
	}
	if parsed.Scheme == "" && strings.HasPrefix(value, "/") {
		baseURL := strings.TrimSpace(os.Getenv("BOT_MEDIA_BASE_URL"))
		if baseURL == "" {
			baseURL = "http://gateway:8080"
		}
		target := strings.TrimRight(baseURL, "/") + value
		return fetchAndEncodeImageToDataURL(ctx, target)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errno.BadRequest("unsupported image URL scheme, expected http(s) URL or data:base64")
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return "", errno.BadRequest("image URL missing hostname")
	}
	if isPrivateHost(host) {
		return fetchAndEncodeImageToDataURL(ctx, value)
	}
	return value, nil
}

func isPrivateHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
	}
	return false
}

func fetchAndEncodeImageToDataURL(ctx context.Context, sourceURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return "", fmt.Errorf("invalid image URL: %w", err)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("cannot access image URL: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("image URL request failed (HTTP %d)", resp.StatusCode)
	}

	const maxImageBytes = 8 * 1024 * 1024
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxImageBytes+1))
	if err != nil {
		return "", fmt.Errorf("failed to read image body: %w", err)
	}
	if len(body) == 0 {
		return "", errno.BadRequest("image body is empty")
	}
	if len(body) > maxImageBytes {
		return "", errno.BadRequest("image too large (over 8MB)")
	}

	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		parsed, parseErr := url.Parse(sourceURL)
		if parseErr == nil {
			ext := strings.ToLower(filepath.Ext(parsed.Path))
			if guessed := mime.TypeByExtension(ext); guessed != "" {
				contentType = guessed
			}
		}
	}
	if contentType == "" {
		contentType = "image/jpeg"
	}
	contentType = strings.Split(contentType, ";")[0]
	encoded := base64.StdEncoding.EncodeToString(body)
	return "data:" + contentType + ";base64," + encoded, nil
}

var _ MentionHandler = (*Service)(nil)
