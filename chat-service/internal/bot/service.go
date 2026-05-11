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
	"strings"
	"time"

	"example.com/aim/chat-service/internal/dal/model"
	"example.com/aim/chat-service/internal/llm"
	"example.com/aim/chat-service/internal/repository"
	"example.com/aim/chat-service/internal/rpc"
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
	LLM                 llm.Client
	LLMSelector         func(bot model.Bot) (llm.Client, string, error)
	DefaultModel        string
	ContextMessages     int
	DailyTokenLimit     int64
	Limiter             *Limiter
	MessageRepo         repository.MessageRepository
	ConversationRepo    repository.ConversationRepository
	MemberRepo          repository.MemberRepository
	BotRepo             repository.BotRepository
	ConversationBotRepo repository.ConversationBotRepository
	AICallLogRepo       repository.AICallLogRepository
	ReplyPublisher      ReplyPublisher
	UserClient          rpc.UserClient
}

func NewService(
	llmClient llm.Client,
	messageRepo repository.MessageRepository,
	conversationRepo repository.ConversationRepository,
	aiCallLogRepo repository.AICallLogRepository,
) *Service {
	return &Service{
		LLM:              llmClient,
		ContextMessages:  20,
		DailyTokenLimit:  1_000_000,
		MessageRepo:      messageRepo,
		ConversationRepo: conversationRepo,
		AICallLogRepo:    aiCallLogRepo,
	}
}

func (s *Service) SetDefaultModel(modelName string) {
	s.DefaultModel = strings.TrimSpace(modelName)
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

func (s *Service) SetReplyPublisher(replyPublisher ReplyPublisher) {
	s.ReplyPublisher = replyPublisher
}

func (s *Service) SetUserClient(userClient rpc.UserClient) {
	s.UserClient = userClient
}

func (s *Service) HandleMention(ctx context.Context, req HandleMentionRequest) error {
	if s == nil {
		return errors.New("bot service is nil")
	}
	if s.LLM == nil {
		return errors.New("llm client is required")
	}
	if s.MessageRepo == nil {
		return errors.New("message repository is required")
	}
	if s.ConversationRepo == nil {
		return errors.New("conversation repository is required")
	}
	if s.MemberRepo == nil {
		return errors.New("member repository is required")
	}
	if s.BotRepo == nil {
		return errors.New("bot repository is required")
	}
	if s.ConversationBotRepo == nil {
		return errors.New("conversation bot repository is required")
	}
	if s.AICallLogRepo == nil {
		return errors.New("ai call log repository is required")
	}
	if req.ConversationID == 0 {
		return errors.New("conversation id is required")
	}

	resolved, err := s.resolveTargetBot(ctx, req.ConversationID, req.Content)
	if err != nil {
		return err
	}
	if resolved == nil {
		return nil
	}
	if resolved.ConversationBot.PermissionScope != model.BotScopeConversationOnly {
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
		err := errors.New("model is required")
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
	recentMessages, err := s.MessageRepo.ListByConversationID(ctx, req.ConversationID, nil, contextLimit)
	if err != nil {
		return err
	}
	recentMessages = filterBotVisibleMessages(recentMessages)
	userDisplayNames := s.resolveUserDisplayNames(ctx, recentMessages, req.UserID)
	prompt := BuildPrompt(recentMessages, req.Content, contextLimit, userDisplayNames, req.UserID)
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

	resp, err := llmClient.Generate(ctx, llm.GenerateRequest{
		Model: modelName,
		Messages: []llm.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{
				Role:    "user",
				Content: prompt,
				Parts:   parts,
			},
		},
	})
	latencyMS := time.Since(start).Milliseconds()
	if err != nil {
		if logErr := s.createFailedLog(ctx, req, resolved.Bot, modelName, err, latencyMS); logErr != nil {
			return fmt.Errorf("record failed ai call log: %w; llm error: %v", logErr, err)
		}
		var statusErr *llm.HTTPStatusError
		if errors.As(err, &statusErr) {
			if _, replyErr := s.createBotReplyMessage(ctx, req, resolved.Bot, "璋冪敤閿欒锛岃绋嶅悗鍐嶈瘯"); replyErr != nil {
				return fmt.Errorf("create bot failure reply error: %w; llm error: %v", replyErr, err)
			}
			return nil
		}
		return err
	}
	if resp == nil {
		err := errors.New("llm response is nil")
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
		if botModel.CreatedBy != 0 {
			continue
		}

		match, err := s.matchesToken(token, *botModel, conversationBot)
		if err != nil {
			return nil, err
		}
		if match {
			matches = append(matches, resolvedBot{
				Bot:             *botModel,
				ConversationBot: conversationBot,
			})
		}
	}

	if len(matches) == 0 {
		return nil, nil
	}
	if len(matches) > 1 {
		log.Printf("bot mention token %q matched multiple bots in conversation %d; skipping", token, conversationID)
		return nil, nil
	}
	return &matches[0], nil
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

	return s.ReplyPublisher.PublishBotReplyCreated(ctx, BotReplyCreatedEvent{
		Message: BotReplyMessageInfo{
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
		return errors.New("forbidden: daily ai token limit reached")
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
	return strings.EqualFold(strings.TrimSpace(modelName), "qwen3.6-plus")
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
		return "", errors.New("image URL is empty")
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "data:") {
		if strings.Contains(lower, ";base64,") {
			return value, nil
		}
		return "", errors.New("invalid base64 image format, expected data:*;base64, prefix")
	}
	if strings.HasPrefix(lower, "file://") {
		return "", errors.New("file:// is not supported in OpenAI-compatible mode")
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return "", errors.New("invalid image URL format, expected http(s) URL or data:base64")
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
		return "", errors.New("unsupported image URL scheme, expected http(s) URL or data:base64")
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return "", errors.New("image URL missing hostname")
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
		return "", errors.New("image body is empty")
	}
	if len(body) > maxImageBytes {
		return "", errors.New("image too large (over 8MB)")
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

