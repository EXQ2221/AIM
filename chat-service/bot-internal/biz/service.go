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
	"example.com/aim/chat-service/kitex_gen/rag/ragservice"
	llm "example.com/aim/chat-service/llm-internal/client"
	"example.com/aim/shared/errno"
	"gorm.io/gorm"
)

type HandleMentionRequest struct {
	ConversationID   uint64
	RequestMessageID uint64
	UserID           uint64
	Content          string
	ReplyToID        *uint64
	ReplyToPreview   string
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
	RAGClient            ragservice.Client
	RAGTopK              int
	UserMemoryRepo       repository.UserMemoryRepository
	UserMemorySettingRepo repository.UserMemorySettingRepository
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
var implicitReferenceQuestionPattern = regexp.MustCompile(`(?i)(你怎么看|怎么看|怎么想|觉得呢|有何看法|你的看法|你的意见)`)

const (
	botPersonaPragmatic = "你是实用派助手：直接给可落地方案，优先步骤、命令、风险与验收点，表达简洁，不绕弯子。"
	botPersonaDeep      = "你是深度派助手：先解释为什么，再讲原理、边界条件与长期影响，最后给可执行建议。"
)

func NewService(
	llmClient llm.Client,
	messageRepo repository.MessageRepository,
	conversationRepo repository.ConversationRepository,
	aiCallLogRepo repository.AICallLogRepository,
) *Service {
	return &Service{
		LLM:                  llmClient,
		ContextMessages:      20,
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

func (s *Service) SetRAGClient(client ragservice.Client) {
	s.RAGClient = client
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

func (s *Service) SetUserMemorySettingRepository(repo repository.UserMemorySettingRepository) {
	s.UserMemorySettingRepo = repo
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

	providerName := resolveProviderNameForBilling(resolved.Bot)
	modelName := EffectiveModelName(resolved.Bot, resolved.ConversationBot, s.DefaultModel)
	if modelName == "" {
		start := time.Now()
		err := errno.Required("model")
		latencyMS := time.Since(start).Milliseconds()
		if logErr := s.createFailedLog(ctx, req, resolved.Bot, providerName, modelName, err, latencyMS); logErr != nil {
			return fmt.Errorf("record failed ai call log: %w; bot error: %v", logErr, err)
		}
		return err
	}
	llmClient, providerName, err := s.resolveLLMClientAndProvider(resolved.Bot, providerName, modelName)
	if err != nil {
		if logErr := s.createFailedLog(ctx, req, resolved.Bot, providerName, modelName, err, 0); logErr != nil {
			return fmt.Errorf("record failed ai call log: %w; llm select error: %v", logErr, err)
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
	recentMessages = reverseMessagesInPlace(recentMessages)
	userDisplayNames := s.resolveUserDisplayNames(ctx, recentMessages, req.UserID)
	promptContent := rebuildMentionContent(req.Content, question)
	promptContent = attachReplyContext(promptContent, req.ReplyToPreview)
	scope := resolved.ConversationBot.PermissionScope
	if scope == "" {
		scope = model.BotScopeConversationOnly
	}
	var workflowStreamMeta *botReplyStreamMeta
	if scope != model.BotScopeConversationOnly && s.RAGClient != nil {
		streamMeta, streamErr := s.buildStreamMeta(ctx, req, resolved.Bot)
		if streamErr != nil {
			log.Printf("knowledge workflow stream meta unavailable: conversation=%d bot=%d err=%v", req.ConversationID, resolved.Bot.ID, streamErr)
		} else if streamMeta != nil {
			workflowStreamMeta = streamMeta
		}
	}
	if scope != model.BotScopeConversationOnly && s.RAGClient != nil {
		workflow, workflowErr := s.runKnowledgeWorkflow(ctx, knowledgeWorkflowRequest{
			LLMClient:        llmClient,
			ModelName:        modelName,
			ResolvedBot:      *resolved,
			MentionRequest:   req,
			Question:         question,
			PromptContent:    promptContent,
			KnowledgeScope:   scope,
			UserDisplayNames: userDisplayNames,
			RecentMessages:   recentMessages,
			StreamMeta:       workflowStreamMeta,
		})
		if workflowErr != nil {
			log.Printf("knowledge workflow failed: conversation=%d bot=%d scope=%s question=%q err=%v", req.ConversationID, resolved.Bot.ID, scope, question, workflowErr)
			if scope == model.BotScopeKnowledgeBaseOnly {
				if _, replyErr := s.createBotReplyMessage(ctx, req, resolved.Bot, "知识库查询失败，请稍后再试"); replyErr != nil {
					return replyErr
				}
				_ = s.createFailedLog(ctx, req, resolved.Bot, providerName, modelName, workflowErr, 0)
				return nil
			}
		} else if workflow != nil {
			if scope == model.BotScopeKnowledgeBaseOnly || workflow.DirectAnswer {
				latencyMS := workflow.LatencyMS
				botMessage, replyErr := s.createBotReplyMessage(ctx, req, resolved.Bot, workflow.Answer)
				if replyErr != nil {
					if logErr := s.createFailedLog(ctx, req, resolved.Bot, providerName, modelName, replyErr, latencyMS); logErr != nil {
						return fmt.Errorf("record failed ai call log: %w; create bot reply error: %v", logErr, replyErr)
					}
					return replyErr
				}
				if workflow.GenerateResp != nil {
					return s.createSuccessLog(ctx, req, resolved.Bot, providerName, modelName, botMessage.ID, workflow.GenerateResp, latencyMS)
				}
				return nil
			}
			if strings.TrimSpace(workflow.KnowledgeContext) != "" {
				ragChunks := []RAGChunk{
					{Index: 1, Content: workflow.KnowledgeContext, Score: 1},
				}
				longTermMemories := s.collectLongTermMemories(ctx, req, question)
				topicSummary := summarizeRecentTopicV2(recentMessages, req.RequestMessageID, 5)
				prompt := BuildPromptWithRAGAndMemory(recentMessages, promptContent, contextLimit, userDisplayNames, req.UserID, scope, ragChunks, topicSummary, longTermMemories)
				systemPrompt := strings.TrimSpace(resolved.Bot.SystemPrompt)
				if systemPrompt == "" {
					systemPrompt = DefaultSystemPrompt
				}
				systemPrompt = mergePersonaIntoSystemPrompt(systemPrompt, resolved.Bot)
				return s.generateBotReply(ctx, req, resolved.Bot, llmClient, providerName, modelName, prompt, systemPrompt, recentMessages)
			}
		}
	}
	ragTopK := s.RAGTopK
	if ragTopK <= 0 {
		ragTopK = 5
	}
	ragQueries := buildRAGQueries(question, recentMessages, req.RequestMessageID)
	var (
		ragChunks []RAGChunk
		ragErr    error
	)
	for index, ragQuery := range ragQueries {
		chunks, searchErr := s.searchRAGChunks(ctx, scope, req.UserID, req.ConversationID, ragQuery, ragTopK)
		if searchErr != nil {
			ragErr = searchErr
			log.Printf("rag search failed: conversation=%d bot=%d scope=%s query=%q err=%v", req.ConversationID, resolved.Bot.ID, scope, ragQuery, searchErr)
			continue
		}
		if len(chunks) == 0 {
			continue
		}
		ragChunks = chunks
		if index > 0 {
			log.Printf("rag query fallback hit: conversation=%d bot=%d scope=%s query=%q chunks=%d", req.ConversationID, resolved.Bot.ID, scope, ragQuery, len(chunks))
		}
		ragErr = nil
		break
	}
	if ragErr != nil && scope == model.BotScopeKnowledgeBaseOnly {
		if _, replyErr := s.createBotReplyMessage(ctx, req, resolved.Bot, "\u77e5\u8bc6\u5e93\u68c0\u7d22\u5931\u8d25\uff0c\u8bf7\u7a0d\u540e\u518d\u8bd5"); replyErr != nil {
			return replyErr
		}
		_ = s.createFailedLog(ctx, req, resolved.Bot, providerName, modelName, ragErr, 0)
		return nil
	}
	longTermMemories := s.collectLongTermMemories(ctx, req, question)
	topicSummary := summarizeRecentTopicV2(recentMessages, req.RequestMessageID, 5)
	prompt := BuildPromptWithRAGAndMemory(recentMessages, promptContent, contextLimit, userDisplayNames, req.UserID, scope, ragChunks, topicSummary, longTermMemories)
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
	systemPrompt = mergePersonaIntoSystemPrompt(systemPrompt, resolved.Bot)

	if err := s.checkDailyTokenLimit(ctx, req, resolved.Bot, providerName, modelName); err != nil {
		if logErr := s.createFailedLog(ctx, req, resolved.Bot, providerName, modelName, err, 0); logErr != nil {
			return fmt.Errorf("record failed ai call log: %w; bot error: %v", logErr, err)
		}
		return err
	}

	start := time.Now()
	_ = start
	return s.generateBotReply(ctx, req, resolved.Bot, llmClient, providerName, modelName, prompt, systemPrompt, recentMessages)
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

func attachReplyContext(content string, replyPreview string) string {
	content = strings.TrimSpace(content)
	replyPreview = strings.TrimSpace(replyPreview)
	if replyPreview == "" {
		return content
	}
	if content == "" {
		return "【回复目标】" + replyPreview
	}
	return content + "\n【回复目标】" + replyPreview
}

func attachImplicitReferenceContext(content string, question string, recentMessages []model.Message, requestMessageID uint64) string {
	content = strings.TrimSpace(content)
	question = strings.TrimSpace(question)
	if question == "" || !implicitReferenceQuestionPattern.MatchString(question) {
		return content
	}
	if strings.Contains(content, "【回复目标】") || strings.Contains(content, "【隐式指代参考】") {
		return content
	}
	ref := findImplicitReferencePreview(recentMessages, requestMessageID)
	if ref == "" {
		return content
	}
	if content == "" {
		return "【隐式指代参考】" + ref
	}
	return content + "\n【隐式指代参考】" + ref
}

func findImplicitReferencePreview(recentMessages []model.Message, requestMessageID uint64) string {
	if len(recentMessages) == 0 {
		return ""
	}
	currentIndex := len(recentMessages) - 1
	if requestMessageID > 0 {
		for i := len(recentMessages) - 1; i >= 0; i-- {
			if recentMessages[i].ID == requestMessageID {
				currentIndex = i
				break
			}
		}
	}
	for i := currentIndex - 1; i >= 0; i-- {
		msg := recentMessages[i]
		if msg.Status != model.MessageStatusNormal {
			continue
		}
		if msg.SenderType == model.SenderTypeSystem {
			continue
		}
		preview := strings.TrimSpace(model.MessagePreview(msg.MessageType, msg.Content))
		if preview == "" || preview == "[图片]" || preview == "[文件]" || preview == "[语音]" {
			continue
		}
		return preview
	}
	return ""
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
	if logErr := s.createFailedLog(context.Background(), req, botModel, "", "", err, 0); logErr != nil {
		return nil, fmt.Errorf("record failed ai call log: %w; concurrency error: %v", logErr, err)
	}
	return nil, err
}

func (s *Service) createSuccessLog(ctx context.Context, req HandleMentionRequest, botModel model.Bot, providerName string, modelName string, responseMessageID uint64, resp *llm.GenerateResponse, latencyMS int64) error {
	requestMessageID := nullableID(req.RequestMessageID)
	return s.AICallLogRepo.Create(ctx, &model.AICallLog{
		UserID:            req.UserID,
		BotID:             botModel.ID,
		ConversationID:    req.ConversationID,
		RequestMessageID:  requestMessageID,
		ResponseMessageID: &responseMessageID,
		ProviderName:      strings.TrimSpace(providerName),
		ModelName:         modelName,
		PromptTokens:      resp.PromptTokens,
		CompletionTokens:  resp.CompletionTokens,
		TotalTokens:       resp.TotalTokens,
		LatencyMS:         latencyMS,
		Status:            model.AICallStatusSuccess,
	})
}

func (s *Service) createFailedLog(ctx context.Context, req HandleMentionRequest, botModel model.Bot, providerName string, modelName string, cause error, latencyMS int64) error {
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
		ProviderName:     strings.TrimSpace(providerName),
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

func (s *Service) checkDailyTokenLimit(ctx context.Context, req HandleMentionRequest, botModel model.Bot, providerName string, modelName string) error {
	if s.AICallLogRepo == nil {
		return nil
	}
	// User-owned bots (CreatedBy > 0) should not consume platform quota.
	if botModel.CreatedBy > 0 {
		return nil
	}
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		modelName = strings.TrimSpace(botModel.ModelName)
	}
	if modelName == "" {
		return nil
	}
	providerName = strings.ToLower(strings.TrimSpace(providerName))
	dailyLimit := dailyTokenLimitByProviderModel(providerName, modelName)
	if dailyLimit <= 0 {
		return nil
	}
	startAt, endAt := dayWindow()
	total, err := s.AICallLogRepo.SumTotalTokensByConversationAndProviderModelBetween(ctx, req.ConversationID, providerName, modelName, startAt, endAt)
	if err != nil {
		return err
	}
	if total >= dailyLimit {
		return errno.Forbidden("已到达限额，请等待或者切换Bot")
	}
	return nil
}

func dailyTokenLimitByProviderModel(providerName string, modelName string) int64 {
	name := strings.ToLower(strings.TrimSpace(modelName))
	if name == "" {
		return 0
	}
	if supportsVisionModel(name) {
		return 25_000
	}
	return 50_000
}

func resolveProviderNameForBilling(botModel model.Bot) string {
	if botModel.CreatedBy > 0 {
		return "custom"
	}
	modelName := strings.ToLower(strings.TrimSpace(botModel.ModelName))
	if strings.HasPrefix(modelName, "qwen") || strings.Contains(modelName, "tongyi") {
		return "secondary"
	}
	return "primary"
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

func reverseMessagesInPlace(messages []model.Message) []model.Message {
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages
}

func findRecentFallbackQuestions(recentMessages []model.Message, requestMessageID uint64, maxCount int) []string {
	if len(recentMessages) == 0 {
		return nil
	}
	if maxCount <= 0 {
		maxCount = 1
	}
	currentIndex := len(recentMessages) - 1
	if requestMessageID > 0 {
		for i := len(recentMessages) - 1; i >= 0; i-- {
			if recentMessages[i].ID == requestMessageID {
				currentIndex = i
				break
			}
		}
	}
	result := make([]string, 0, maxCount)
	seen := make(map[string]struct{}, maxCount)
	for i := currentIndex - 1; i >= 0; i-- {
		msg := recentMessages[i]
		if msg.SenderType != model.SenderTypeUser || msg.Status != model.MessageStatusNormal {
			continue
		}
		raw := strings.TrimSpace(model.ExtractTextMessageContent(msg.Content))
		if raw == "" {
			continue
		}
		question := strings.TrimSpace(ExtractQuestion(raw))
		if question == "" {
			continue
		}
		if _, exists := seen[question]; exists {
			continue
		}
		seen[question] = struct{}{}
		result = append(result, question)
		if len(result) >= maxCount {
			break
		}
	}
	return result
}

func buildRAGQueries(currentQuestion string, recentMessages []model.Message, requestMessageID uint64) []string {
	currentQuestion = strings.TrimSpace(currentQuestion)
	fallbackQuestions := findRecentFallbackQuestions(recentMessages, requestMessageID, 3)
	if isWeakRAGQuestion(currentQuestion) {
		queries := make([]string, 0, len(fallbackQuestions)+1)
		queries = append(queries, fallbackQuestions...)
		if currentQuestion != "" {
			queries = append(queries, currentQuestion)
		}
		return deduplicateQuestions(queries)
	}
	if currentQuestion == "" {
		return nil
	}
	// For explicit questions, keep retrieval anchored to the current question only.
	return []string{currentQuestion}
}

func isWeakRAGQuestion(question string) bool {
	normalized := strings.TrimSpace(strings.ToLower(question))
	if normalized == "" {
		return true
	}
	weak := []string{
		"你怎么看", "怎么看", "你觉得呢", "觉得呢", "怎么想", "为什么这样做",
		"what do you think", "thoughts", "why so",
	}
	for _, token := range weak {
		if strings.Contains(normalized, token) {
			return true
		}
	}
	if len([]rune(normalized)) <= 8 {
		return true
	}
	return false
}

func deduplicateQuestions(questions []string) []string {
	result := make([]string, 0, len(questions))
	seen := make(map[string]struct{}, len(questions))
	for _, item := range questions {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func summarizeRecentTopicV2(recentMessages []model.Message, requestMessageID uint64, blockSize int) string {
	if len(recentMessages) == 0 {
		return ""
	}
	if blockSize <= 0 {
		blockSize = 5
	}
	currentIndex := len(recentMessages) - 1
	if requestMessageID > 0 {
		for i := len(recentMessages) - 1; i >= 0; i-- {
			if recentMessages[i].ID == requestMessageID {
				currentIndex = i
				break
			}
		}
	}
	start := currentIndex - blockSize + 1
	if start < 0 {
		start = 0
	}

	parts := make([]string, 0, blockSize)
	historyTexts := make([]string, 0, blockSize)
	latestPreview := ""

	for i := start; i <= currentIndex && i < len(recentMessages); i++ {
		msg := recentMessages[i]
		if msg.Status != model.MessageStatusNormal || msg.SenderType == model.SenderTypeSystem {
			continue
		}
		preview := strings.TrimSpace(model.MessagePreview(msg.MessageType, msg.Content))
		if preview == "" {
			continue
		}
		if i == currentIndex {
			latestPreview = preview
		} else {
			historyTexts = append(historyTexts, preview)
		}
		name := "BOT"
		if msg.SenderType == model.SenderTypeUser {
			name = "用户"
		}
		parts = append(parts, fmt.Sprintf("%s提到%s", name, preview))
	}
	if len(parts) == 0 {
		return ""
	}

	base := strings.Join(parts, "；")
	if latestPreview != "" && isTopicShiftedV2(latestPreview, historyTexts) {
		return fmt.Sprintf("最新话题优先：%s。前序讨论：%s", latestPreview, base)
	}
	return base
}

func isTopicShiftedV2(latest string, history []string) bool {
	latestTokens := topicTokensV2(latest)
	if len(latestTokens) == 0 || len(history) == 0 {
		return false
	}
	historySet := make(map[string]struct{}, 64)
	for _, item := range history {
		for _, token := range topicTokensV2(item) {
			historySet[token] = struct{}{}
		}
	}
	matchCount := 0
	for _, token := range latestTokens {
		if _, ok := historySet[token]; ok {
			matchCount++
		}
	}
	overlapRatio := float64(matchCount) / float64(len(latestTokens))
	return overlapRatio < 0.2
}

func topicTokensV2(text string) []string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return nil
	}
	raw := strings.FieldsFunc(text, func(r rune) bool {
		if r >= 'a' && r <= 'z' {
			return false
		}
		if r >= '0' && r <= '9' {
			return false
		}
		if r >= 0x4e00 && r <= 0x9fff {
			return false
		}
		return true
	})
	stop := map[string]struct{}{
		"的": {}, "了": {}, "吗": {}, "呢": {}, "吧": {}, "是": {}, "有": {}, "和": {}, "或": {}, "就": {}, "都": {},
		"你": {}, "我": {}, "他": {}, "她": {}, "它": {}, "我们": {}, "你们": {},
		"怎么看": {}, "怎么想": {}, "怎么样": {}, "对吗": {}, "觉得": {}, "看法": {}, "意见": {},
	}
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		if _, ok := stop[item]; ok {
			continue
		}
		if len([]rune(item)) <= 1 {
			continue
		}
		result = append(result, item)
	}
	return result
}

func summarizeRecentTopic(recentMessages []model.Message, requestMessageID uint64, blockSize int) string {
	if len(recentMessages) == 0 {
		return ""
	}
	if blockSize <= 0 {
		blockSize = 5
	}
	currentIndex := len(recentMessages) - 1
	if requestMessageID > 0 {
		for i := len(recentMessages) - 1; i >= 0; i-- {
			if recentMessages[i].ID == requestMessageID {
				currentIndex = i
				break
			}
		}
	}
	start := currentIndex - blockSize + 1
	if start < 0 {
		start = 0
	}
	parts := make([]string, 0, blockSize)
	for i := start; i <= currentIndex && i < len(recentMessages); i++ {
		msg := recentMessages[i]
		if msg.Status != model.MessageStatusNormal || msg.SenderType == model.SenderTypeSystem {
			continue
		}
		preview := strings.TrimSpace(model.MessagePreview(msg.MessageType, msg.Content))
		if preview == "" {
			continue
		}
		name := "BOT"
		if msg.SenderType == model.SenderTypeUser {
			name = "用户"
		}
		parts = append(parts, fmt.Sprintf("%s提到%s", name, preview))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "；")
}

func mergePersonaIntoSystemPrompt(systemPrompt string, botModel model.Bot) string {
	systemPrompt = strings.TrimSpace(systemPrompt)
	if systemPrompt == "" {
		systemPrompt = DefaultSystemPrompt
	}
	mention := strings.ToLower(strings.TrimSpace(botModel.MentionName))
	name := strings.ToLower(strings.TrimSpace(botModel.Name))
	persona := ""
	switch {
	case mention == "ai" || strings.Contains(name, "deepseek"):
		persona = botPersonaPragmatic
	case mention == "qwen" || strings.Contains(name, "通义") || strings.Contains(name, "qwen"):
		persona = botPersonaDeep
	default:
		return systemPrompt
	}
	return systemPrompt + "\n" + persona
}

func logLLMRequestDebug(req HandleMentionRequest, botModel model.Bot, llmReq llm.GenerateRequest) {
	flag := strings.ToLower(strings.TrimSpace(os.Getenv("BOT_DEBUG_LLM_REQUEST")))
	if flag != "1" && flag != "true" && flag != "yes" {
		return
	}
	payload, err := json.MarshalIndent(llmReq, "", "  ")
	if err != nil {
		log.Printf("bot llm request debug marshal failed: conversation=%d bot=%d request=%d err=%v", req.ConversationID, botModel.ID, req.RequestMessageID, err)
		return
	}
	log.Printf(
		"bot llm request debug: conversation=%d bot=%d request=%d user=%d payload=%s",
		req.ConversationID,
		botModel.ID,
		req.RequestMessageID,
		req.UserID,
		string(payload),
	)
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
