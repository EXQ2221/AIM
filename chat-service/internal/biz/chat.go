package biz

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"example.com/aim/chat-service/internal/bot"
	"example.com/aim/chat-service/internal/dal/model"
	"example.com/aim/chat-service/internal/repository"
	"example.com/aim/chat-service/internal/rpc"
	"gorm.io/gorm"
)

var (
	ErrBadRequest           = errors.New("bad_request: invalid request")
	ErrConversationNotFound = errors.New("not_found: conversation not found")
	ErrGroupNotFound        = errors.New("not_found: group not found")
	ErrNotMember            = errors.New("forbidden: user is not a member of this conversation")
	ErrOwnerCannotLeave     = errors.New("forbidden: owner cannot leave group before ownership transfer")
	ErrSingleSelfChat       = errors.New("bad_request: cannot create single conversation with yourself")
	ErrSingleTargetInvalid  = errors.New("forbidden: target user is not available")
	ErrSingleFriendRequired = errors.New("forbidden: single conversation requires active friendship")
	ErrMessageEmpty         = errors.New("bad_request: message content is empty")
	ErrMessageUnsupported   = errors.New("bad_request: only text message is supported")
	ErrMemberMuted          = errors.New("forbidden: member is muted")
	ErrGroupMutedAll        = errors.New("forbidden: group is muted for members")
)

type ChatService struct {
	ConversationRepo     repository.ConversationRepository
	GroupRepo            repository.GroupRepository
	MemberRepo           repository.MemberRepository
	BotRepo              repository.BotRepository
	ConversationBotRepo  repository.ConversationBotRepository
	MessageRepo          repository.MessageRepository
	AICallLogRepo        repository.AICallLogRepository
	TxManager            repository.TxManager
	UserClient           rpc.UserClient
	BotService           bot.MentionHandler
	BotMembershipService *bot.MembershipService
	BotTaskTimeout       time.Duration
}

func NewChatService(
	conversationRepo repository.ConversationRepository,
	groupRepo repository.GroupRepository,
	memberRepo repository.MemberRepository,
	messageRepo repository.MessageRepository,
	txManager repository.TxManager,
	userClient rpc.UserClient,
) *ChatService {
	return &ChatService{
		ConversationRepo: conversationRepo,
		GroupRepo:        groupRepo,
		MemberRepo:       memberRepo,
		MessageRepo:      messageRepo,
		TxManager:        txManager,
		UserClient:       userClient,
		BotTaskTimeout:   30 * time.Second,
	}
}

func (s *ChatService) SetBotService(botService bot.MentionHandler) {
	s.BotService = botService
}

func (s *ChatService) SetBotTaskTimeout(timeout time.Duration) {
	if timeout > 0 {
		s.BotTaskTimeout = timeout
	}
}

func (s *ChatService) SetAICallLogRepository(aiCallLogRepo repository.AICallLogRepository) {
	s.AICallLogRepo = aiCallLogRepo
}

func (s *ChatService) SetBotManagement(botRepo repository.BotRepository, conversationBotRepo repository.ConversationBotRepository, membershipService *bot.MembershipService) {
	s.BotRepo = botRepo
	s.ConversationBotRepo = conversationBotRepo
	s.BotMembershipService = membershipService
}

func (s *ChatService) CreateGroup(ctx context.Context, input CreateGroupInput) (*GroupView, error) {
	name := strings.TrimSpace(input.Name)
	if input.OperatorID == 0 || name == "" || len([]rune(name)) > 128 {
		return nil, ErrBadRequest
	}

	joinPolicy := normalizeJoinPolicy(input.JoinPolicy)
	now := time.Now()
	var conversation *model.Conversation
	var group *model.GroupInfo
	conversationID, err := newConversationID()
	if err != nil {
		return nil, err
	}

	err = s.TxManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		conversationRepo := s.ConversationRepo.WithTx(tx)
		groupRepo := s.GroupRepo.WithTx(tx)
		memberRepo := s.MemberRepo.WithTx(tx)

		conversation = &model.Conversation{
			ConversationID: conversationID,
			Type:           model.ConversationTypeGroup,
			Title:          name,
			Avatar:         input.Avatar,
			CreatedBy:      input.OperatorID,
		}
		if err := conversationRepo.Create(ctx, conversation); err != nil {
			return err
		}

		group = &model.GroupInfo{
			ConversationID: conversation.ID,
			Name:           name,
			Avatar:         input.Avatar,
			Announcement:   input.Announcement,
			OwnerID:        input.OperatorID,
			JoinPolicy:     joinPolicy,
			MuteAll:        false,
			MaxMembers:     500,
		}
		if err := groupRepo.Create(ctx, group); err != nil {
			return err
		}

		return memberRepo.Create(ctx, &model.ConversationMember{
			ConversationID: conversation.ID,
			MemberType:     model.MemberTypeUser,
			MemberID:       input.OperatorID,
			Role:           model.MemberRoleOwner,
			Status:         model.MemberStatusNormal,
			JoinedAt:       now,
		})
	})
	if err != nil {
		return nil, err
	}

	return groupToView(group, conversation.ConversationID), nil
}

func (s *ChatService) CreateSingleConversation(ctx context.Context, input CreateSingleConversationInput) (*ConversationView, error) {
	if input.OperatorID == 0 || input.TargetID == 0 {
		return nil, ErrBadRequest
	}
	if input.OperatorID == input.TargetID {
		return nil, ErrSingleSelfChat
	}

	if s.UserClient != nil {
		target, err := s.UserClient.GetUser(ctx, input.TargetID)
		if err != nil {
			return nil, err
		}
		if target == nil || target.UserID == 0 || !strings.EqualFold(target.Status, "NORMAL") {
			return nil, ErrSingleTargetInvalid
		}
	}

	conversation, err := s.ConversationRepo.FindSingleByUsers(ctx, input.OperatorID, input.TargetID)
	switch {
	case err == nil:
		return s.buildConversationView(ctx, input.OperatorID, *conversation)
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return nil, err
	}

	conversationID, err := newConversationID()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	var created *model.Conversation
	err = s.TxManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		conversationRepo := s.ConversationRepo.WithTx(tx)
		memberRepo := s.MemberRepo.WithTx(tx)

		existing, err := conversationRepo.FindSingleByUsers(ctx, input.OperatorID, input.TargetID)
		switch {
		case err == nil:
			created = existing
			return nil
		case !errors.Is(err, gorm.ErrRecordNotFound):
			return err
		}

		created = &model.Conversation{
			ConversationID: conversationID,
			Type:           model.ConversationTypeSingle,
			CreatedBy:      input.OperatorID,
		}
		if err := conversationRepo.Create(ctx, created); err != nil {
			return err
		}
		if err := memberRepo.Create(ctx, &model.ConversationMember{
			ConversationID: created.ID,
			MemberType:     model.MemberTypeUser,
			MemberID:       input.OperatorID,
			Role:           model.MemberRoleMember,
			Status:         model.MemberStatusNormal,
			JoinedAt:       now,
		}); err != nil {
			return err
		}
		return memberRepo.Create(ctx, &model.ConversationMember{
			ConversationID: created.ID,
			MemberType:     model.MemberTypeUser,
			MemberID:       input.TargetID,
			Role:           model.MemberRoleMember,
			Status:         model.MemberStatusNormal,
			JoinedAt:       now,
		})
	})
	if err != nil {
		return nil, err
	}

	return s.buildConversationView(ctx, input.OperatorID, *created)
}

func (s *ChatService) FindSingleConversationByUsers(ctx context.Context, operatorID, targetUserID uint64) (*ConversationView, error) {
	if operatorID == 0 || targetUserID == 0 {
		return nil, ErrBadRequest
	}
	if operatorID == targetUserID {
		return nil, ErrSingleSelfChat
	}

	// 先查已有会话
	conv, err := s.ConversationRepo.FindSingleByUsers(ctx, operatorID, targetUserID)
	switch {
	case err == nil:
		return s.buildConversationView(ctx, operatorID, *conv)
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return nil, err
	}

	// 不存在则自动创建（兼容好友通过非标准路径添加的场景）
	view, err := s.CreateSingleConversation(ctx, CreateSingleConversationInput{
		OperatorID: operatorID,
		TargetID:   targetUserID,
	})
	if err != nil {
		return nil, err
	}
	return view, nil
}

func (s *ChatService) ListConversations(ctx context.Context, userID uint64) ([]ConversationView, error) {
	if userID == 0 {
		return nil, ErrBadRequest
	}

	rows, err := s.ConversationRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make([]ConversationView, 0, len(rows))
	for _, row := range rows {
		var lastMessageAt *int64
		if row.LastMessageAt != nil {
			value := row.LastMessageAt.Unix()
			lastMessageAt = &value
		}
		view := ConversationView{
			ConversationID:        row.ConversationID,
			Type:                  row.Type,
			Title:                 row.Title,
			Avatar:                row.Avatar,
			LastMessageID:         row.LastMessageID,
			LastMessageAt:         lastMessageAt,
			LastMessageSenderID:   row.LastMessageSenderID,
			LastMessageSenderType: row.LastMessageSenderType,
			LastMessageContent:    row.LastMessageContent,
			Role:                  row.Role,
			IsPinned:              row.IsPinned,
			IsMuted:               row.IsMuted,
			UpdatedAt:             row.UpdatedAt.Unix(),
		}
		if err := s.decorateSingleConversation(ctx, userID, &view); err != nil {
			return nil, err
		}
		s.decorateLastMessageSender(ctx, &view)
		result = append(result, view)
	}
	return result, nil
}

func (s *ChatService) JoinGroup(ctx context.Context, operatorID uint64, conversationID string) error {
	conversation, err := s.requireGroupConversation(ctx, conversationID)
	if err != nil {
		return err
	}
	if operatorID == 0 {
		return ErrBadRequest
	}

	member, err := s.MemberRepo.GetUserMember(ctx, conversation.ID, operatorID)
	if err == nil {
		if member.Status != model.MemberStatusRemoved {
			return nil
		}
		member.Status = model.MemberStatusNormal
		member.Role = model.MemberRoleMember
		member.JoinedAt = time.Now()
		return s.MemberRepo.Update(ctx, member)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	return s.MemberRepo.Create(ctx, &model.ConversationMember{
		ConversationID: conversation.ID,
		MemberType:     model.MemberTypeUser,
		MemberID:       operatorID,
		Role:           model.MemberRoleMember,
		Status:         model.MemberStatusNormal,
		JoinedAt:       time.Now(),
	})
}

func (s *ChatService) InviteMember(ctx context.Context, input InviteMemberInput, conversationID string) error {
	if input.OperatorID == 0 || input.TargetUserID == 0 {
		return ErrBadRequest
	}
	conversation, err := s.requireGroupConversation(ctx, conversationID)
	if err != nil {
		return err
	}
	if _, err := s.requireMember(ctx, conversation.ID, input.OperatorID); err != nil {
		return err
	}

	existing, err := s.MemberRepo.GetUserMember(ctx, conversation.ID, input.TargetUserID)
	if err == nil && existing.Status != model.MemberStatusRemoved {
		return nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	now := time.Now()
	if existing != nil && existing.Status == model.MemberStatusRemoved {
		return s.MemberRepo.GetDB().WithContext(ctx).
			Exec("UPDATE conversation_members SET status=?, role=?, joined_at=? WHERE id=?",
				model.MemberStatusNormal, model.MemberRoleMember, now, existing.ID).Error
	}

	return s.MemberRepo.Create(ctx, &model.ConversationMember{
		ConversationID: conversation.ID,
		MemberType:     model.MemberTypeUser,
		MemberID:       input.TargetUserID,
		Role:           model.MemberRoleMember,
		Status:         model.MemberStatusNormal,
		JoinedAt:       now,
	})
}

func (s *ChatService) LeaveGroup(ctx context.Context, operatorID uint64, conversationID string) error {
	conversation, err := s.requireGroupConversation(ctx, conversationID)
	if err != nil {
		return err
	}
	member, err := s.requireMember(ctx, conversation.ID, operatorID)
	if err != nil {
		return err
	}
	if member.Role == model.MemberRoleOwner {
		return ErrOwnerCannotLeave
	}
	member.Status = model.MemberStatusRemoved
	return s.MemberRepo.Update(ctx, member)
}

func (s *ChatService) ListMembers(ctx context.Context, operatorID uint64, conversationID string) ([]MemberListView, error) {
	conversation, err := s.requireConversation(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if _, err := s.requireMember(ctx, conversation.ID, operatorID); err != nil {
		return nil, err
	}

	members, err := s.MemberRepo.ListByConversationID(ctx, conversation.ID)
	if err != nil {
		return nil, err
	}

	result := make([]MemberListView, 0, len(members))
	for _, member := range members {
		switch member.MemberType {
		case model.MemberTypeUser:
			view := MemberListView{
				UserID:     member.MemberID,
				Role:       string(member.Role),
				Status:     string(member.Status),
				JoinedAt:   member.JoinedAt.Unix(),
				MemberType: string(model.MemberTypeUser),
			}
			if s.UserClient != nil {
				user, uErr := s.UserClient.GetUser(ctx, member.MemberID)
				if uErr == nil && user != nil {
					view.Nickname = user.Nickname
					view.Avatar = user.Avatar
				}
			}
			result = append(result, view)

		case model.MemberTypeBot:
			if s.BotRepo == nil || s.ConversationBotRepo == nil {
				continue
			}
			botModel, bErr := s.BotRepo.GetByID(ctx, member.MemberID)
			if bErr != nil {
				if errors.Is(bErr, gorm.ErrRecordNotFound) {
					continue
				}
				return nil, bErr
			}

			enabled := botModel.Status == model.BotStatusEnabled
			view := MemberListView{
				UserID:          member.MemberID,
				Nickname:        botModel.Name,
				Avatar:          botModel.Avatar,
				Role:            string(member.Role),
				Status:          string(member.Status),
				JoinedAt:        member.JoinedAt.Unix(),
				MemberType:      string(model.MemberTypeBot),
				BotID:           botModel.ID,
				MentionName:     botModel.MentionName,
				Enabled:         &enabled,
				PermissionScope: string(model.BotScopeConversationOnly),
			}
			if aliases, parseErr := parseAliasesText(botModel.Aliases); parseErr == nil {
				view.Aliases = aliases
			}

			conversationBot, cbErr := s.ConversationBotRepo.GetByConversationAndBotID(ctx, conversation.ID, member.MemberID)
			if cbErr == nil && conversationBot != nil {
				view.Enabled = &conversationBot.Enabled
				view.PermissionScope = string(conversationBot.PermissionScope)
				if override := strings.TrimSpace(conversationBot.DisplayNameOverride); override != "" {
					view.Nickname = override
				}
				if override := strings.TrimSpace(conversationBot.MentionNameOverride); override != "" {
					view.MentionName = override
				}
				if strings.TrimSpace(conversationBot.AliasesOverride) != "" {
					if aliases, parseErr := parseAliasesText(conversationBot.AliasesOverride); parseErr == nil {
						view.Aliases = aliases
					}
				}
			}
			result = append(result, view)
		}
	}
	return result, nil
}

func (s *ChatService) ListMessages(ctx context.Context, operatorID uint64, conversationID string, beforeID *uint64, limit int) ([]MessageView, error) {
	conversation, err := s.requireConversation(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if _, err := s.requireMember(ctx, conversation.ID, operatorID); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}

	messages, err := s.MessageRepo.ListByConversationID(ctx, conversation.ID, beforeID, limit)
	if err != nil {
		return nil, err
	}

	result := make([]MessageView, 0, len(messages))
	for _, message := range messages {
		result = append(result, messageToView(&message, conversation.ConversationID))
	}
	return result, nil
}

func (s *ChatService) CreateMessage(ctx context.Context, operatorID uint64, conversationID string, content string, replyToID *uint64) (*MessageView, error) {
	content = strings.TrimSpace(content)
	conversation, err := s.requireConversation(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if operatorID == 0 {
		return nil, ErrBadRequest
	}
	if content == "" {
		return nil, ErrMessageEmpty
	}
	member, err := s.requireMember(ctx, conversation.ID, operatorID)
	if err != nil {
		return nil, err
	}
	if err := s.checkSendPermission(ctx, conversation, member); err != nil {
		return nil, err
	}

	now := time.Now()
	message := &model.Message{
		ConversationID: conversation.ID,
		SenderID:       operatorID,
		SenderType:     model.SenderTypeUser,
		MessageType:    model.MessageTypeText,
		Content:        content,
		ReplyToID:      replyToID,
		Status:         model.MessageStatusNormal,
		CreatedAt:      now,
	}

	err = s.TxManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		messageRepo := s.MessageRepo.WithTx(tx)
		conversationRepo := s.ConversationRepo.WithTx(tx)
		if err := messageRepo.Create(ctx, message); err != nil {
			return err
		}
		return conversationRepo.UpdateLastMessage(ctx, conversation.ID, message.ID, message.CreatedAt)
	})
	if err != nil {
		return nil, err
	}

	if s.BotService != nil && bot.ShouldTriggerBot(*message) {
		s.handleBotAsync(s.BotService, bot.HandleMentionRequest{
			ConversationID:   message.ConversationID,
			RequestMessageID: message.ID,
			UserID:           message.SenderID,
			Content:          message.Content,
		})
	}

	view := messageToView(message, conversation.ConversationID)
	return &view, nil
}

func (s *ChatService) handleBotAsync(botService bot.MentionHandler, req bot.HandleMentionRequest) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("bot async panic: %v", r)
			}
		}()

		timeout := s.BotTaskTimeout
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		if err := botService.HandleMention(ctx, req); err != nil {
			log.Printf("bot async failed: %v", err)
		}
	}()
}

func (s *ChatService) requireConversation(ctx context.Context, conversationID string) (*model.Conversation, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil, ErrBadRequest
	}
	conversation, err := s.ConversationRepo.GetByConversationID(ctx, conversationID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrConversationNotFound
		}
		return nil, err
	}
	return conversation, nil
}

func (s *ChatService) requireGroupConversation(ctx context.Context, conversationID string) (*model.Conversation, error) {
	conversation, err := s.requireConversation(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if conversation.Type != model.ConversationTypeGroup {
		return nil, ErrConversationNotFound
	}
	if _, err := s.GroupRepo.GetByConversationID(ctx, conversation.ID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGroupNotFound
		}
		return nil, err
	}
	return conversation, nil
}

func (s *ChatService) requireMember(ctx context.Context, conversationID, userID uint64) (*model.ConversationMember, error) {
	if conversationID == 0 || userID == 0 {
		return nil, ErrBadRequest
	}
	member, err := s.MemberRepo.GetUserMember(ctx, conversationID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotMember
		}
		return nil, err
	}
	if member.Status == model.MemberStatusRemoved {
		return nil, ErrNotMember
	}
	return member, nil
}

func (s *ChatService) checkSendPermission(ctx context.Context, conversation *model.Conversation, member *model.ConversationMember) error {
	if conversation == nil {
		return ErrConversationNotFound
	}
	if member == nil {
		return ErrNotMember
	}
	if member.Status == model.MemberStatusMuted {
		return ErrMemberMuted
	}
	if member.MuteUntil != nil && member.MuteUntil.After(time.Now()) {
		return ErrMemberMuted
	}
	if conversation.Type == model.ConversationTypeSingle {
		return s.checkSingleConversationFriendship(ctx, conversation, member.MemberID)
	}
	if conversation.Type != model.ConversationTypeGroup {
		return ErrConversationNotFound
	}

	group, err := s.GroupRepo.GetByConversationID(ctx, conversation.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrGroupNotFound
		}
		return err
	}
	if group.MuteAll && member.Role != model.MemberRoleOwner && member.Role != model.MemberRoleAdmin {
		return ErrGroupMutedAll
	}
	return nil
}

func (s *ChatService) checkSingleConversationFriendship(ctx context.Context, conversation *model.Conversation, senderID uint64) error {
	if conversation == nil {
		return ErrConversationNotFound
	}
	if senderID == 0 {
		return ErrBadRequest
	}
	members, err := s.MemberRepo.ListByConversationID(ctx, conversation.ID)
	if err != nil {
		return err
	}
	var peerID uint64
	for _, member := range members {
		if member.Status == model.MemberStatusRemoved {
			continue
		}
		if member.MemberType != model.MemberTypeUser {
			continue
		}
		if member.MemberID != senderID {
			peerID = member.MemberID
			break
		}
	}
	if peerID == 0 {
		return ErrSingleFriendRequired
	}
	if s.UserClient == nil {
		return ErrSingleFriendRequired
	}
	isFriend, err := s.UserClient.CheckFriendRelation(ctx, senderID, peerID)
	if err != nil {
		return err
	}
	if !isFriend {
		return ErrSingleFriendRequired
	}
	reverseFriend, err := s.UserClient.CheckFriendRelation(ctx, peerID, senderID)
	if err != nil {
		return err
	}
	if !reverseFriend {
		return ErrSingleFriendRequired
	}
	return nil
}

func (s *ChatService) buildConversationView(ctx context.Context, viewerID uint64, conversation model.Conversation) (*ConversationView, error) {
	var lastMessageAt *int64
	if conversation.LastMessageAt != nil {
		value := conversation.LastMessageAt.Unix()
		lastMessageAt = &value
	}

	member, err := s.requireMember(ctx, conversation.ID, viewerID)
	if err != nil {
		return nil, err
	}

	view := &ConversationView{
		ConversationID: conversation.ConversationID,
		Type:           string(conversation.Type),
		Title:          conversation.Title,
		Avatar:         conversation.Avatar,
		LastMessageID:  conversation.LastMessageID,
		LastMessageAt:  lastMessageAt,
		Role:           string(member.Role),
		IsPinned:       member.IsPinned,
		IsMuted:        member.IsMuted,
		UpdatedAt:      conversation.UpdatedAt.Unix(),
	}
	if err := s.decorateSingleConversation(ctx, viewerID, view); err != nil {
		return nil, err
	}
	return view, nil
}

func (s *ChatService) decorateLastMessageSender(ctx context.Context, view *ConversationView) {
	if view == nil || view.LastMessageSenderID == nil || *view.LastMessageSenderID == 0 {
		return
	}
	if strings.EqualFold(view.LastMessageSenderType, string(model.SenderTypeBot)) {
		if s.BotRepo != nil {
			botModel, err := s.BotRepo.GetByID(ctx, *view.LastMessageSenderID)
			if err == nil && botModel != nil {
				if name := strings.TrimSpace(botModel.Name); name != "" {
					view.LastMessageSenderName = name
					return
				}
				if mention := strings.TrimSpace(botModel.MentionName); mention != "" {
					view.LastMessageSenderName = mention
					return
				}
			}
		}
		view.LastMessageSenderName = "Bot"
		return
	}
	if s.UserClient == nil {
		return
	}
	user, err := s.UserClient.GetUser(ctx, *view.LastMessageSenderID)
	if err == nil && user != nil {
		view.LastMessageSenderName = user.Nickname
	}
}

func (s *ChatService) decorateSingleConversation(ctx context.Context, viewerID uint64, view *ConversationView) error {
	if view == nil || view.Type != string(model.ConversationTypeSingle) {
		return nil
	}
	conversation, err := s.ConversationRepo.GetByConversationID(ctx, view.ConversationID)
	if err != nil {
		return err
	}
	members, err := s.MemberRepo.ListByConversationID(ctx, conversation.ID)
	if err != nil {
		return err
	}
	var peerID uint64
	for _, member := range members {
		if member.MemberType != model.MemberTypeUser {
			continue
		}
		if member.MemberID != viewerID {
			peerID = member.MemberID
			break
		}
	}
	if peerID == 0 || s.UserClient == nil {
		return nil
	}
	peer, err := s.UserClient.GetUser(ctx, peerID)
	if err != nil || peer == nil {
		return err
	}
	view.Title = peer.Nickname
	view.Avatar = peer.Avatar
	return nil
}

func normalizeJoinPolicy(value string) model.GroupJoinPolicy {
	switch model.GroupJoinPolicy(strings.ToUpper(strings.TrimSpace(value))) {
	case model.GroupJoinFree:
		return model.GroupJoinFree
	case model.GroupJoinApproval:
		return model.GroupJoinApproval
	default:
		return model.GroupJoinInviteOnly
	}
}

func groupToView(group *model.GroupInfo, conversationID string) *GroupView {
	if group == nil {
		return nil
	}
	return &GroupView{
		ConversationID: conversationID,
		Type:           string(model.ConversationTypeGroup),
		Name:           group.Name,
		Avatar:         group.Avatar,
		Announcement:   group.Announcement,
		OwnerID:        group.OwnerID,
		JoinPolicy:     string(group.JoinPolicy),
		CreatedAt:      group.CreatedAt.Unix(),
	}
}

func messageToView(message *model.Message, conversationID string) MessageView {
	return MessageView{
		ID:             message.ID,
		ConversationID: conversationID,
		SenderID:       message.SenderID,
		SenderType:     string(message.SenderType),
		MessageType:    string(message.MessageType),
		Content:        message.Content,
		ReplyToID:      message.ReplyToID,
		Status:         string(message.Status),
		CreatedAt:      message.CreatedAt.Unix(),
	}
}
