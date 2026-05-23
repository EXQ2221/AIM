package biz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	bot "example.com/aim/chat-service/bot-internal/biz"
	botrepo "example.com/aim/chat-service/bot-internal/repository"
	"example.com/aim/chat-service/internal/dal/model"
	"gorm.io/gorm"
)

var (
	ErrBotManagementUnavailable  = errors.New("internal: bot management is unavailable")
	ErrAdminRequired             = errors.New("forbidden: owner/admin is required")
	ErrBotMentionNameInvalid     = errors.New("bad_request: mentionName must be 2-32 chars and must not include @")
	ErrBotAliasInvalid           = errors.New("bad_request: aliases must be 2-32 chars and must not include @")
	ErrBotModelInvalid           = errors.New("bad_request: model_name_override is not supported by this bot")
	ErrBotReservedName           = errors.New("bad_request: reserved mention name is not allowed")
	ErrBotConflict               = errors.New("bad_request: bot mentionName or aliases conflict in conversation")
	ErrBotPermissionScopeInvalid = errors.New("bad_request: permission_scope must be CONVERSATION_ONLY / KNOWLEDGE_BASE_ONLY / CONVERSATION_AND_KB")
	ErrBotOwnerRequired          = errors.New("forbidden: only bot owner can add this custom bot")
)

var reservedBotNames = map[string]struct{}{
	"all":      {},
	"here":     {},
	"everyone": {},
	"system":   {},
}

func (s *ChatService) ListBots(ctx context.Context, operatorID uint64) ([]BotView, error) {
	if operatorID == 0 {
		return nil, ErrBadRequest
	}
	if s.BotRepo == nil {
		return nil, ErrBotManagementUnavailable
	}

	bots, err := s.BotRepo.ListEnabledByOwner(ctx, operatorID)
	if err != nil {
		return nil, err
	}

	result := make([]BotView, 0, len(bots))
	for _, botModel := range bots {
		view, err := buildAvailableBotView(botModel)
		if err != nil {
			return nil, err
		}
		result = append(result, view)
	}
	return result, nil
}

func (s *ChatService) ListConversationBots(ctx context.Context, operatorID uint64, conversationID string) ([]BotView, error) {
	if s.BotRepo == nil || s.ConversationBotRepo == nil {
		return nil, ErrBotManagementUnavailable
	}
	conversation, err := s.requireGroupConversation(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if _, err := s.requireMember(ctx, conversation.ID, operatorID); err != nil {
		return nil, err
	}

	conversationBots, err := s.ConversationBotRepo.ListByConversationID(ctx, conversation.ID)
	if err != nil {
		return nil, err
	}

	result := make([]BotView, 0, len(conversationBots))
	for _, conversationBot := range conversationBots {
		if !conversationBot.Enabled {
			continue
		}
		member, err := s.MemberRepo.GetBotMember(ctx, conversation.ID, conversationBot.BotID)
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
		view, err := buildConversationBotView(*botModel, conversationBot, *member)
		if err != nil {
			return nil, err
		}
		result = append(result, view)
	}
	return result, nil
}

func (s *ChatService) AddConversationBot(ctx context.Context, input AddConversationBotInput) (*BotView, error) {
	if input.OperatorID == 0 || input.BotID == 0 {
		return nil, ErrBadRequest
	}
	if s.BotRepo == nil || s.ConversationBotRepo == nil || s.BotMembershipService == nil {
		return nil, ErrBotManagementUnavailable
	}

	conversation, err := s.requireGroupConversation(ctx, input.ConversationID)
	if err != nil {
		return nil, err
	}
	member, err := s.requireMember(ctx, conversation.ID, input.OperatorID)
	if err != nil {
		return nil, err
	}
	if member.Role != model.MemberRoleOwner && member.Role != model.MemberRoleAdmin {
		return nil, ErrAdminRequired
	}

	permissionScope := strings.TrimSpace(input.PermissionScope)
	if permissionScope == "" {
		permissionScope = string(model.BotScopeConversationOnly)
	}
	if permissionScope != string(model.BotScopeConversationOnly) &&
		permissionScope != string(model.BotScopeKnowledgeBaseOnly) &&
		permissionScope != string(model.BotScopeConversationAndKB) {
		return nil, ErrBotPermissionScopeInvalid
	}

	botModel, err := s.BotRepo.GetByID(ctx, input.BotID)
	if err != nil {
		return nil, err
	}
	if botModel.Status != model.BotStatusEnabled {
		return nil, botrepo.ErrBotNotFound
	}
	if botModel.CreatedBy != 0 && botModel.CreatedBy != input.OperatorID {
		return nil, ErrBotOwnerRequired
	}

	if err := validateBaseBotTokens(*botModel); err != nil {
		return nil, err
	}
	if err := validateOverrideNames(input.MentionNameOverride, input.AliasesOverride); err != nil {
		return nil, err
	}
	modelNameOverride, err := validateModelNameOverride(*botModel, input.ModelNameOverride)
	if err != nil {
		return nil, err
	}
	if err := s.ensureConversationBotConflictFree(ctx, conversation.ID, input.BotID, *botModel, input); err != nil {
		return nil, err
	}

	aliasesOverride := ""
	if len(input.AliasesOverride) > 0 {
		aliasesOverride, err = marshalAliases(input.AliasesOverride)
		if err != nil {
			return nil, err
		}
	}

	if err := s.BotMembershipService.AddBotToConversationWithConfig(ctx, conversation.ID, input.BotID, botrepo.ConversationBotConfig{
		ModelNameOverride:   modelNameOverride,
		DisplayNameOverride: strings.TrimSpace(input.DisplayNameOverride),
		MentionNameOverride: normalizeOptionalName(input.MentionNameOverride),
		AliasesOverride:     aliasesOverride,
		PermissionScope:     model.BotPermissionScope(permissionScope),
	}); err != nil {
		return nil, err
	}

	conversationBot, err := s.ConversationBotRepo.GetByConversationAndBotID(ctx, conversation.ID, input.BotID)
	if err != nil {
		return nil, err
	}
	member, err = s.MemberRepo.GetBotMember(ctx, conversation.ID, input.BotID)
	if err != nil {
		return nil, err
	}

	view, err := buildConversationBotView(*botModel, *conversationBot, *member)
	if err != nil {
		return nil, err
	}
	return &view, nil
}

func (s *ChatService) CreateCustomBot(ctx context.Context, input CreateCustomBotInput) (*BotView, error) {
	if input.OperatorID == 0 {
		return nil, ErrBadRequest
	}
	if s.BotRepo == nil {
		return nil, ErrBotManagementUnavailable
	}

	name := strings.TrimSpace(input.Name)
	mentionName := normalizeOptionalName(input.MentionName)
	baseURL := normalizeOpenAIBaseURL(input.APIBaseURL)
	apiKey := strings.TrimSpace(input.APIKey)
	modelName := strings.TrimSpace(input.ModelName)
	if name == "" || mentionName == "" || baseURL == "" || apiKey == "" || modelName == "" {
		return nil, ErrBadRequest
	}
	if err := validateMentionToken(mentionName, ErrBotMentionNameInvalid); err != nil {
		return nil, err
	}
	if err := validateOverrideNames("", input.Aliases); err != nil {
		return nil, err
	}

	aliasesText, err := marshalAliases(input.Aliases)
	if err != nil {
		return nil, err
	}
	supportedModels := input.SupportedModels
	if len(supportedModels) == 0 {
		supportedModels = []string{modelName}
	}
	supportedModelsText, err := marshalAliases(supportedModels)
	if err != nil {
		return nil, err
	}

	botModel := &model.Bot{
		Name:            name,
		MentionName:     mentionName,
		Aliases:         aliasesText,
		Description:     strings.TrimSpace(input.Description),
		ModelName:       modelName,
		SupportedModels: supportedModelsText,
		APIBaseURL:      baseURL,
		APIKeyEncrypted: apiKey,
		SystemPrompt:    strings.TrimSpace(input.SystemPrompt),
		CreatedBy:       input.OperatorID,
		Status:          model.BotStatusEnabled,
	}
	if botModel.SystemPrompt == "" {
		botModel.SystemPrompt = bot.DefaultSystemPrompt
	}
	if err := s.BotRepo.Create(ctx, botModel); err != nil {
		return nil, fmt.Errorf("create custom bot failed: %w", err)
	}
	view, err := buildAvailableBotView(*botModel)
	if err != nil {
		return nil, err
	}
	return &view, nil
}

func normalizeOpenAIBaseURL(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	if strings.HasSuffix(lower, "/chat/completions") {
		return strings.TrimSpace(value[:len(value)-len("/chat/completions")])
	}
	return value
}

func (s *ChatService) RemoveConversationBot(ctx context.Context, operatorID uint64, conversationID string, botID uint64) error {
	if operatorID == 0 || botID == 0 {
		return ErrBadRequest
	}
	if s.BotMembershipService == nil {
		return ErrBotManagementUnavailable
	}

	conversation, err := s.requireGroupConversation(ctx, conversationID)
	if err != nil {
		return err
	}
	member, err := s.requireMember(ctx, conversation.ID, operatorID)
	if err != nil {
		return err
	}
	if member.Role != model.MemberRoleOwner && member.Role != model.MemberRoleAdmin {
		return ErrAdminRequired
	}
	return s.BotMembershipService.RemoveBotFromConversation(ctx, conversation.ID, botID)
}

func buildConversationBotView(botModel model.Bot, conversationBot model.ConversationBot, member model.ConversationMember) (BotView, error) {
	aliases, err := parseAliasesText(botModel.Aliases)
	if err != nil {
		return BotView{}, err
	}
	if strings.TrimSpace(conversationBot.AliasesOverride) != "" {
		aliases, err = parseAliasesText(conversationBot.AliasesOverride)
		if err != nil {
			return BotView{}, err
		}
	}

	displayName := botModel.Name
	if override := strings.TrimSpace(conversationBot.DisplayNameOverride); override != "" {
		displayName = override
	}
	mentionName := botModel.MentionName
	if override := strings.TrimSpace(conversationBot.MentionNameOverride); override != "" {
		mentionName = override
	}
	supportedModels, err := bot.ParseSupportedModels(botModel.SupportedModels, strings.TrimSpace(botModel.ModelName))
	if err != nil {
		return BotView{}, err
	}

	return BotView{
		BotID:           botModel.ID,
		MemberType:      string(model.MemberTypeBot),
		MemberID:        member.MemberID,
		Name:            botModel.Name,
		DisplayName:     displayName,
		MentionName:     mentionName,
		Aliases:         aliases,
		Avatar:          botModel.Avatar,
		Description:     botModel.Description,
		Enabled:         conversationBot.Enabled,
		PermissionScope: string(conversationBot.PermissionScope),
		MemberStatus:    string(member.Status),
		ModelName:       bot.EffectiveModelName(botModel, conversationBot, ""),
		SupportedModels: supportedModels,
	}, nil
}

func buildAvailableBotView(botModel model.Bot) (BotView, error) {
	aliases, err := parseAliasesText(botModel.Aliases)
	if err != nil {
		return BotView{}, err
	}
	supportedModels, err := bot.ParseSupportedModels(botModel.SupportedModels, strings.TrimSpace(botModel.ModelName))
	if err != nil {
		return BotView{}, err
	}
	return BotView{
		BotID:           botModel.ID,
		MemberType:      string(model.MemberTypeBot),
		MemberID:        botModel.ID,
		Name:            botModel.Name,
		DisplayName:     botModel.Name,
		MentionName:     botModel.MentionName,
		Aliases:         aliases,
		Avatar:          botModel.Avatar,
		Description:     botModel.Description,
		Enabled:         botModel.Status == model.BotStatusEnabled,
		PermissionScope: string(model.BotScopeConversationOnly),
		ModelName:       strings.TrimSpace(botModel.ModelName),
		SupportedModels: supportedModels,
	}, nil
}

func validateOverrideNames(mentionName string, aliases []string) error {
	if mentionName != "" {
		if err := validateMentionToken(mentionName, ErrBotMentionNameInvalid); err != nil {
			return err
		}
	}
	for _, alias := range aliases {
		if err := validateMentionToken(alias, ErrBotAliasInvalid); err != nil {
			return err
		}
	}
	return nil
}

func validateBaseBotTokens(botModel model.Bot) error {
	if err := validateMentionToken(botModel.MentionName, ErrBotMentionNameInvalid); err != nil {
		return err
	}
	aliases, err := parseAliasesText(botModel.Aliases)
	if err != nil {
		return err
	}
	for _, alias := range aliases {
		if err := validateMentionToken(alias, ErrBotAliasInvalid); err != nil {
			return err
		}
	}
	return nil
}

func validateMentionToken(value string, invalidErr error) error {
	value = normalizeOptionalName(value)
	if len([]rune(value)) < 2 || len([]rune(value)) > 32 || strings.Contains(value, "@") {
		return invalidErr
	}
	if _, found := reservedBotNames[value]; found {
		return ErrBotReservedName
	}
	return nil
}

func normalizeOptionalName(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func parseAliasesText(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var aliases []string
	if err := json.Unmarshal([]byte(raw), &aliases); err != nil {
		return nil, err
	}
	result := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		normalized := normalizeOptionalName(alias)
		if normalized == "" {
			continue
		}
		result = append(result, normalized)
	}
	return result, nil
}

func marshalAliases(aliases []string) (string, error) {
	normalized := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		value := normalizeOptionalName(alias)
		if value == "" {
			continue
		}
		normalized = append(normalized, value)
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func validateModelNameOverride(botModel model.Bot, override string) (string, error) {
	override = strings.TrimSpace(override)
	supportedModels, err := bot.ParseSupportedModels(botModel.SupportedModels, strings.TrimSpace(botModel.ModelName))
	if err != nil {
		return "", err
	}
	if override == "" {
		return "", nil
	}
	for _, supportedModel := range supportedModels {
		if supportedModel == override {
			return override, nil
		}
	}
	return "", ErrBotModelInvalid
}

func (s *ChatService) ensureConversationBotConflictFree(ctx context.Context, conversationID, botID uint64, botModel model.Bot, input AddConversationBotInput) error {
	conversationBots, err := s.ConversationBotRepo.ListByConversationID(ctx, conversationID)
	if err != nil {
		return err
	}

	targetTokens, err := targetConversationTokens(botModel, input)
	if err != nil {
		return err
	}

	for _, conversationBot := range conversationBots {
		if !conversationBot.Enabled || conversationBot.BotID == botID {
			continue
		}
		member, err := s.MemberRepo.GetBotMember(ctx, conversationID, conversationBot.BotID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return err
		}
		if member.Status != model.MemberStatusNormal {
			continue
		}

		existingBot, err := s.BotRepo.GetByID(ctx, conversationBot.BotID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return err
		}
		existingTokens, err := existingConversationTokens(*existingBot, conversationBot)
		if err != nil {
			return err
		}
		for token := range targetTokens {
			if _, found := existingTokens[token]; found {
				return ErrBotConflict
			}
		}
	}
	return nil
}

func targetConversationTokens(botModel model.Bot, input AddConversationBotInput) (map[string]struct{}, error) {
	tokens := make(map[string]struct{})
	mentionName := normalizeOptionalName(input.MentionNameOverride)
	if mentionName == "" {
		mentionName = normalizeOptionalName(botModel.MentionName)
	}
	if mentionName != "" {
		tokens[mentionName] = struct{}{}
	}

	var aliases []string
	var err error
	if len(input.AliasesOverride) > 0 {
		aliases = make([]string, 0, len(input.AliasesOverride))
		for _, alias := range input.AliasesOverride {
			aliases = append(aliases, normalizeOptionalName(alias))
		}
	} else {
		aliases, err = parseAliasesText(botModel.Aliases)
		if err != nil {
			return nil, err
		}
	}
	for _, alias := range aliases {
		if alias == "" {
			continue
		}
		tokens[alias] = struct{}{}
	}
	return tokens, nil
}

func existingConversationTokens(botModel model.Bot, conversationBot model.ConversationBot) (map[string]struct{}, error) {
	tokens := make(map[string]struct{})
	mentionName := normalizeOptionalName(botModel.MentionName)
	if override := normalizeOptionalName(conversationBot.MentionNameOverride); override != "" {
		mentionName = override
	}
	if mentionName != "" {
		tokens[mentionName] = struct{}{}
	}

	aliases, err := parseAliasesText(botModel.Aliases)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(conversationBot.AliasesOverride) != "" {
		aliases, err = parseAliasesText(conversationBot.AliasesOverride)
		if err != nil {
			return nil, err
		}
	}
	for _, alias := range aliases {
		if alias == "" {
			continue
		}
		tokens[alias] = struct{}{}
	}
	return tokens, nil
}
