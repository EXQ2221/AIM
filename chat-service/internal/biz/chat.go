package biz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	bot "example.com/aim/chat-service/bot-internal/biz"
	botrepo "example.com/aim/chat-service/bot-internal/repository"
	"example.com/aim/chat-service/internal/dal/model"
	"example.com/aim/chat-service/internal/repository"
	"example.com/aim/chat-service/internal/rpc"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var (
	ErrBadRequest           = errors.New("bad_request: invalid request")
	ErrConversationNotFound = errors.New("not_found: conversation not found")
	ErrGroupNotFound        = errors.New("not_found: group not found")
	ErrMessageNotFound      = errors.New("not_found: message not found")
	ErrNotMember            = errors.New("forbidden: user is not a member of this conversation")
	ErrOwnerCannotLeave     = errors.New("forbidden: owner cannot leave group before ownership transfer")
	ErrSingleSelfChat       = errors.New("bad_request: cannot create single conversation with yourself")
	ErrSingleTargetInvalid  = errors.New("forbidden: target user is not available")
	ErrSingleFriendRequired = errors.New("forbidden: single conversation requires active friendship")
	ErrMessageEmpty         = errors.New("bad_request: message content is empty")
	ErrMessageUnsupported   = errors.New("bad_request: unsupported message type")
	ErrMessageInvalid       = errors.New("bad_request: invalid message content")
	ErrReplyTargetInvalid   = errors.New("bad_request: invalid reply target")
	ErrMessageRecallDenied  = errors.New("forbidden: only the sender can recall this message")
	ErrGroupAdminRequired   = errors.New("forbidden: only owner or admin can perform this action")
	ErrMemberMuted          = errors.New("forbidden: member is muted")
	ErrGroupMutedAll        = errors.New("forbidden: group is muted for members")
)

const replyPreviewLimit = 80

type ChatService struct {
	ConversationRepo     repository.ConversationRepository
	GroupRepo            repository.GroupRepository
	MemberRepo           repository.MemberRepository
	BotRepo              repository.BotRepository
	ConversationBotRepo  repository.ConversationBotRepository
	MessageRepo          repository.MessageRepository
	AICallLogRepo        repository.AICallLogRepository
	NotificationRepo     repository.NotificationRepository
	TxManager            repository.TxManager
	UserClient           rpc.UserClient
	BotService           bot.MentionHandler
	BotMembershipService botrepo.MembershipManager
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

func (s *ChatService) SetNotificationRepository(notificationRepo repository.NotificationRepository) {
	s.NotificationRepo = notificationRepo
}

func (s *ChatService) SetBotManagement(botRepo repository.BotRepository, conversationBotRepo repository.ConversationBotRepository, membershipService botrepo.MembershipManager) {
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

func (s *ChatService) GetGroupInfo(ctx context.Context, operatorID uint64, conversationID string) (*GroupView, error) {
	if operatorID == 0 {
		return nil, ErrBadRequest
	}

	conversation, err := s.requireGroupConversation(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if _, err := s.requireMember(ctx, conversation.ID, operatorID); err != nil {
		return nil, err
	}

	group, err := s.GroupRepo.GetByConversationID(ctx, conversation.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGroupNotFound
		}
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

	// 鍏堟煡宸叉湁浼氳瘽
	conv, err := s.ConversationRepo.FindSingleByUsers(ctx, operatorID, targetUserID)
	switch {
	case err == nil:
		return s.buildConversationView(ctx, operatorID, *conv)
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return nil, err
	}

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
			LastMessageContent:    conversationLastMessageContent(row),
			MuteAll:               row.MuteAll,
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

func (s *ChatService) JoinGroup(ctx context.Context, operatorID uint64, conversationID string) (*ConversationEventView, error) {
	conversation, err := s.requireGroupConversation(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if operatorID == 0 {
		return nil, ErrBadRequest
	}

	var event *ConversationEventView
	err = s.TxManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		memberRepo := s.MemberRepo.WithTx(tx)
		messageRepo := s.MessageRepo.WithTx(tx)
		conversationRepo := s.ConversationRepo.WithTx(tx)

		member, getErr := memberRepo.GetUserMember(ctx, conversation.ID, operatorID)
		if getErr == nil {
			if member.Status != model.MemberStatusRemoved {
				return nil
			}
			member.Status = model.MemberStatusNormal
			member.Role = model.MemberRoleMember
			member.JoinedAt = time.Now()
			if updateErr := memberRepo.Update(ctx, member); updateErr != nil {
				return updateErr
			}
		} else if !errors.Is(getErr, gorm.ErrRecordNotFound) {
			return getErr
		} else {
			if createErr := memberRepo.Create(ctx, &model.ConversationMember{
				ConversationID: conversation.ID,
				MemberType:     model.MemberTypeUser,
				MemberID:       operatorID,
				Role:           model.MemberRoleMember,
				Status:         model.MemberStatusNormal,
				JoinedAt:       time.Now(),
			}); createErr != nil {
				return createErr
			}
		}

		systemMessage, recipientUserIDs, eventErr := s.createSystemMessageTx(
			ctx,
			conversation,
			conversationRepo,
			memberRepo,
			messageRepo,
			s.notificationRepoWithTx(tx),
			model.SystemEventMemberJoined,
			operatorID,
			[]uint64{operatorID},
		)
		if eventErr != nil {
			return eventErr
		}
		view := messageToView(systemMessage, conversation.ConversationID)
		event = &ConversationEventView{Message: &view, RecipientUserIDs: recipientUserIDs}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return event, nil
}

func (s *ChatService) InviteMember(ctx context.Context, input InviteMemberInput, conversationID string) (*ConversationEventView, error) {
	if input.OperatorID == 0 || input.TargetUserID == 0 {
		return nil, ErrBadRequest
	}
	conversation, err := s.requireGroupConversation(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if _, err := s.requireMember(ctx, conversation.ID, input.OperatorID); err != nil {
		return nil, err
	}

	var event *ConversationEventView
	err = s.TxManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		memberRepo := s.MemberRepo.WithTx(tx)
		messageRepo := s.MessageRepo.WithTx(tx)
		conversationRepo := s.ConversationRepo.WithTx(tx)

		existing, getErr := memberRepo.GetUserMember(ctx, conversation.ID, input.TargetUserID)
		if getErr == nil {
			if existing.Status != model.MemberStatusRemoved {
				return nil
			}
			existing.Status = model.MemberStatusNormal
			existing.Role = model.MemberRoleMember
			existing.JoinedAt = time.Now()
			if updateErr := memberRepo.Update(ctx, existing); updateErr != nil {
				return updateErr
			}
		} else if !errors.Is(getErr, gorm.ErrRecordNotFound) {
			return getErr
		} else {
			if createErr := memberRepo.Create(ctx, &model.ConversationMember{
				ConversationID: conversation.ID,
				MemberType:     model.MemberTypeUser,
				MemberID:       input.TargetUserID,
				Role:           model.MemberRoleMember,
				Status:         model.MemberStatusNormal,
				JoinedAt:       time.Now(),
			}); createErr != nil {
				return createErr
			}
		}

		systemMessage, recipientUserIDs, eventErr := s.createSystemMessageTx(
			ctx,
			conversation,
			conversationRepo,
			memberRepo,
			messageRepo,
			s.notificationRepoWithTx(tx),
			model.SystemEventMemberInvited,
			input.OperatorID,
			[]uint64{input.TargetUserID},
		)
		if eventErr != nil {
			return eventErr
		}
		view := messageToView(systemMessage, conversation.ConversationID)
		event = &ConversationEventView{Message: &view, RecipientUserIDs: recipientUserIDs}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return event, nil
}

func (s *ChatService) LeaveGroup(ctx context.Context, operatorID uint64, conversationID string) (*ConversationEventView, error) {
	conversation, err := s.requireGroupConversation(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	member, err := s.requireMember(ctx, conversation.ID, operatorID)
	if err != nil {
		return nil, err
	}
	if member.Role == model.MemberRoleOwner {
		return nil, ErrOwnerCannotLeave
	}

	var event *ConversationEventView
	err = s.TxManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		memberRepo := s.MemberRepo.WithTx(tx)
		messageRepo := s.MessageRepo.WithTx(tx)
		conversationRepo := s.ConversationRepo.WithTx(tx)

		leavingMember, getErr := memberRepo.GetUserMember(ctx, conversation.ID, operatorID)
		if getErr != nil {
			return getErr
		}
		leavingMember.Status = model.MemberStatusRemoved
		if updateErr := memberRepo.Update(ctx, leavingMember); updateErr != nil {
			return updateErr
		}

		systemMessage, recipientUserIDs, eventErr := s.createSystemMessageTx(
			ctx,
			conversation,
			conversationRepo,
			memberRepo,
			messageRepo,
			s.notificationRepoWithTx(tx),
			model.SystemEventMemberLeft,
			operatorID,
			[]uint64{operatorID},
		)
		if eventErr != nil {
			return eventErr
		}
		view := messageToView(systemMessage, conversation.ConversationID)
		event = &ConversationEventView{Message: &view, RecipientUserIDs: recipientUserIDs}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return event, nil
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
			if member.MuteUntil != nil {
				value := member.MuteUntil.Unix()
				view.MuteUntil = &value
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
		view := messageToView(&message, conversation.ConversationID)
		result = append(result, view)
	}
	if err := s.decorateMessageReadState(ctx, conversation, operatorID, result); err != nil {
		return nil, err
	}
	if err := s.decorateMessageReplies(ctx, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *ChatService) MarkConversationRead(ctx context.Context, input MarkConversationReadInput) error {
	if input.OperatorID == 0 || strings.TrimSpace(input.ConversationID) == "" || input.LastReadMessageID == 0 {
		return ErrBadRequest
	}

	conversation, err := s.requireConversation(ctx, input.ConversationID)
	if err != nil {
		return err
	}

	member, err := s.requireMember(ctx, conversation.ID, input.OperatorID)
	if err != nil {
		return err
	}
	if member.MemberType != model.MemberTypeUser {
		return ErrBadRequest
	}

	message, err := s.MessageRepo.GetByID(ctx, input.LastReadMessageID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrMessageNotFound
		}
		return err
	}
	if message.ConversationID != conversation.ID {
		return ErrBadRequest
	}
	if member.LastReadMessageID != nil && *member.LastReadMessageID >= input.LastReadMessageID {
		return nil
	}

	return s.MemberRepo.UpdateLastReadMessageID(ctx, conversation.ID, input.OperatorID, input.LastReadMessageID)
}

func (s *ChatService) RecallMessage(ctx context.Context, input RecallMessageInput) (*MessageRecalledEventView, error) {
	if input.OperatorID == 0 || strings.TrimSpace(input.ConversationID) == "" || input.MessageID == 0 {
		return nil, ErrBadRequest
	}

	conversation, err := s.requireConversation(ctx, input.ConversationID)
	if err != nil {
		return nil, err
	}
	if _, err := s.requireMember(ctx, conversation.ID, input.OperatorID); err != nil {
		return nil, err
	}

	message, err := s.MessageRepo.GetByID(ctx, input.MessageID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMessageNotFound
		}
		return nil, err
	}
	if message.ConversationID != conversation.ID {
		return nil, ErrBadRequest
	}
	if message.SenderType != model.SenderTypeUser || message.SenderID != input.OperatorID {
		return nil, ErrMessageRecallDenied
	}
	if message.Status == model.MessageStatusRecalled {
		recipientUserIDs, err := s.MemberRepo.ListUserMemberIDs(ctx, conversation.ID)
		if err != nil {
			return nil, err
		}
		return &MessageRecalledEventView{
			MessageID:        message.ID,
			ConversationID:   conversation.ConversationID,
			RecipientUserIDs: recipientUserIDs,
		}, nil
	}
	if message.Status != model.MessageStatusNormal {
		return nil, ErrBadRequest
	}

	err = s.TxManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		messageRepo := s.MessageRepo.WithTx(tx)
		return messageRepo.UpdateStatus(ctx, message.ID, model.MessageStatusRecalled)
	})
	if err != nil {
		return nil, err
	}

	recipientUserIDs, err := s.MemberRepo.ListUserMemberIDs(ctx, conversation.ID)
	if err != nil {
		return nil, err
	}
	return &MessageRecalledEventView{
		MessageID:        message.ID,
		ConversationID:   conversation.ConversationID,
		RecipientUserIDs: recipientUserIDs,
	}, nil
}

func (s *ChatService) CreateMessage(ctx context.Context, operatorID uint64, conversationID string, content string, replyToID *uint64, messageType string) (*MessageView, error) {
	conversation, err := s.requireConversation(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if operatorID == 0 {
		return nil, ErrBadRequest
	}
	member, err := s.requireMember(ctx, conversation.ID, operatorID)
	if err != nil {
		return nil, err
	}
	if err := s.checkSendPermission(ctx, conversation, member); err != nil {
		return nil, err
	}
	normalizedType, normalizedContent, err := normalizeMessagePayload(messageType, content)
	if err != nil {
		return nil, err
	}
	var replyPreview *ReplyPreviewView
	if replyToID != nil {
		replyMessage, err := s.MessageRepo.GetByID(ctx, *replyToID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrReplyTargetInvalid
			}
			return nil, err
		}
		if replyMessage.ConversationID != conversation.ID {
			return nil, ErrReplyTargetInvalid
		}
		replyPreview = buildReplyPreviewView(replyMessage)
	}

	now := time.Now()
	message := &model.Message{
		ConversationID: conversation.ID,
		SenderID:       operatorID,
		SenderType:     model.SenderTypeUser,
		MessageType:    normalizedType,
		Content:        normalizedContent,
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
			Content:          model.ExtractTextMessageContent(message.Content),
		})
	}

	view := messageToView(message, conversation.ConversationID)
	view.ReplyTo = replyPreview
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
	if conversation.Type == model.ConversationTypeGroup {
		group, err := s.GroupRepo.GetByConversationID(ctx, conversation.ID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrGroupNotFound
			}
			return nil, err
		}
		value := group.MuteAll
		view.MuteAll = &value
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
	var announcementUpdatedAt *int64
	if group.AnnouncementUpdatedAt != nil {
		value := group.AnnouncementUpdatedAt.Unix()
		announcementUpdatedAt = &value
	}
	return &GroupView{
		ConversationID:        conversationID,
		Type:                  string(model.ConversationTypeGroup),
		Name:                  group.Name,
		Avatar:                group.Avatar,
		Announcement:          group.Announcement,
		AnnouncementUpdatedBy: group.AnnouncementUpdatedBy,
		AnnouncementUpdatedAt: announcementUpdatedAt,
		OwnerID:               group.OwnerID,
		JoinPolicy:            string(group.JoinPolicy),
		CreatedAt:             group.CreatedAt.Unix(),
	}
}

func normalizeMessagePayload(messageType string, content string) (model.MessageType, datatypes.JSON, error) {
	normalizedType := model.MessageType(strings.ToUpper(strings.TrimSpace(messageType)))
	if normalizedType == "" {
		normalizedType = model.MessageTypeText
	}

	switch normalizedType {
	case model.MessageTypeText:
		normalizedContent, err := model.NormalizeTextMessageContent(content)
		if err != nil {
			return "", nil, ErrMessageInvalid
		}
		if model.ExtractTextMessageContent(normalizedContent) == "" {
			return "", nil, ErrMessageEmpty
		}
		return normalizedType, normalizedContent, nil
	case model.MessageTypeImage:
		var payload model.ImageMessageContent
		if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &payload); err != nil {
			return "", nil, ErrMessageInvalid
		}
		payload.URL = strings.TrimSpace(payload.URL)
		payload.Name = strings.TrimSpace(payload.Name)
		payload.MimeType = strings.TrimSpace(payload.MimeType)
		payload.Text = strings.TrimSpace(payload.Text)
		if payload.URL == "" || payload.Name == "" || payload.MimeType == "" {
			return "", nil, ErrMessageInvalid
		}
		normalizedContent, err := json.Marshal(payload)
		if err != nil {
			return "", nil, ErrMessageInvalid
		}
		return normalizedType, datatypes.JSON(normalizedContent), nil
	case model.MessageTypeFile:
		var payload model.FileMessageContent
		if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &payload); err != nil {
			return "", nil, ErrMessageInvalid
		}
		payload.URL = strings.TrimSpace(payload.URL)
		payload.Name = strings.TrimSpace(payload.Name)
		payload.MimeType = strings.TrimSpace(payload.MimeType)
		if payload.URL == "" || payload.Name == "" || payload.MimeType == "" || payload.Size <= 0 {
			return "", nil, ErrMessageInvalid
		}
		normalizedContent, err := json.Marshal(payload)
		if err != nil {
			return "", nil, ErrMessageInvalid
		}
		return normalizedType, datatypes.JSON(normalizedContent), nil
	case model.MessageTypeVoice:
		var payload model.VoiceMessageContent
		if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &payload); err != nil {
			return "", nil, ErrMessageInvalid
		}
		payload.URL = strings.TrimSpace(payload.URL)
		payload.Name = strings.TrimSpace(payload.Name)
		payload.MimeType = strings.TrimSpace(payload.MimeType)
		if payload.URL == "" || payload.Name == "" || payload.MimeType == "" || payload.DurationMS <= 0 {
			return "", nil, ErrMessageInvalid
		}
		normalizedContent, err := json.Marshal(payload)
		if err != nil {
			return "", nil, ErrMessageInvalid
		}
		return normalizedType, datatypes.JSON(normalizedContent), nil
	default:
		return "", nil, ErrMessageUnsupported
	}
}

func messageToView(message *model.Message, conversationID string) MessageView {
	return MessageView{
		ID:             message.ID,
		ConversationID: conversationID,
		SenderID:       message.SenderID,
		SenderType:     string(message.SenderType),
		MessageType:    string(message.MessageType),
		Content:        visibleMessageContent(message),
		ReplyToID:      message.ReplyToID,
		Status:         string(message.Status),
		CreatedAt:      message.CreatedAt.Unix(),
	}
}

func buildReplyPreviewView(message *model.Message) *ReplyPreviewView {
	if message == nil {
		return nil
	}
	return &ReplyPreviewView{
		MessageID:      message.ID,
		SenderID:       message.SenderID,
		SenderType:     string(message.SenderType),
		MessageType:    string(message.MessageType),
		ContentPreview: model.ReplyContentPreviewWithStatus(message.Status, message.MessageType, message.Content, replyPreviewLimit),
	}
}

func visibleMessageContent(message *model.Message) string {
	if message == nil {
		return ""
	}
	if message.Status == model.MessageStatusRecalled {
		return ""
	}
	return string(message.Content)
}

func conversationLastMessageContent(row repository.ConversationListRow) string {
	if strings.EqualFold(row.LastMessageStatus, string(model.MessageStatusRecalled)) {
		return model.RecalledMessagePlaceholder
	}
	return string(row.LastMessageContent)
}

func (s *ChatService) decorateMessageReplies(ctx context.Context, messages []MessageView) error {
	if len(messages) == 0 {
		return nil
	}

	replyIDs := make([]uint64, 0)
	seen := make(map[uint64]struct{})
	for _, message := range messages {
		if message.ReplyToID == nil || *message.ReplyToID == 0 {
			continue
		}
		if _, ok := seen[*message.ReplyToID]; ok {
			continue
		}
		seen[*message.ReplyToID] = struct{}{}
		replyIDs = append(replyIDs, *message.ReplyToID)
	}
	if len(replyIDs) == 0 {
		return nil
	}

	replyMessages, err := s.MessageRepo.GetByIDs(ctx, replyIDs)
	if err != nil {
		return err
	}

	replyMap := make(map[uint64]*ReplyPreviewView, len(replyMessages))
	for index := range replyMessages {
		replyMessage := replyMessages[index]
		replyMap[replyMessage.ID] = buildReplyPreviewView(&replyMessage)
	}

	for index := range messages {
		replyToID := messages[index].ReplyToID
		if replyToID == nil || *replyToID == 0 {
			continue
		}
		messages[index].ReplyTo = replyMap[*replyToID]
	}
	return nil
}

func (s *ChatService) createSystemMessageTx(
	ctx context.Context,
	conversation *model.Conversation,
	conversationRepo repository.ConversationRepository,
	memberRepo repository.MemberRepository,
	messageRepo repository.MessageRepository,
	notificationRepo repository.NotificationRepository,
	eventType string,
	actorUserID uint64,
	targetUserIDs []uint64,
) (*model.Message, []uint64, error) {
	if conversation == nil {
		return nil, nil, ErrConversationNotFound
	}

	text := s.renderSystemMessageTextV2(ctx, eventType, actorUserID, targetUserIDs)
	payload, err := json.Marshal(model.SystemMessageContent{
		EventType:     eventType,
		ActorUserID:   actorUserID,
		TargetUserIDs: append([]uint64(nil), targetUserIDs...),
		Text:          text,
	})
	if err != nil {
		return nil, nil, err
	}

	now := time.Now()
	message := &model.Message{
		ConversationID: conversation.ID,
		SenderID:       0,
		SenderType:     model.SenderTypeSystem,
		MessageType:    model.MessageTypeSystem,
		Content:        datatypes.JSON(payload),
		Status:         model.MessageStatusNormal,
		CreatedAt:      now,
	}
	if err := messageRepo.Create(ctx, message); err != nil {
		return nil, nil, err
	}
	if err := conversationRepo.UpdateLastMessage(ctx, conversation.ID, message.ID, message.CreatedAt); err != nil {
		return nil, nil, err
	}
	if err := s.createEventNotificationsTx(ctx, notificationRepo, conversation, eventType, actorUserID, targetUserIDs, message.ID); err != nil {
		return nil, nil, err
	}

	recipientUserIDs, err := memberRepo.ListUserMemberIDs(ctx, conversation.ID)
	if err != nil {
		return nil, nil, err
	}
	return message, recipientUserIDs, nil
}

func (s *ChatService) renderSystemMessageText(ctx context.Context, eventType string, actorUserID uint64, targetUserIDs []uint64) string {
	actorName := s.lookupUserDisplayName(ctx, actorUserID)
	targetNames := make([]string, 0, len(targetUserIDs))
	for _, targetUserID := range targetUserIDs {
		targetNames = append(targetNames, s.lookupUserDisplayName(ctx, targetUserID))
	}
	targetText := strings.Join(targetNames, "、")

	switch eventType {
	case model.SystemEventMemberJoined:
		return fmt.Sprintf("%s joined the group", actorName)
	case model.SystemEventMemberLeft:
		return fmt.Sprintf("%s left the group", actorName)
	case model.SystemEventMemberInvited:
		if targetText == "" {
			return fmt.Sprintf("%s invited a new member", actorName)
		}
		return fmt.Sprintf("%s invited %s to the group", actorName, targetText)
	case model.SystemEventMemberRemoved:
		if targetText == "" {
			return fmt.Sprintf("%s removed a member", actorName)
		}
		return fmt.Sprintf("%s removed %s from the group", actorName, targetText)
	case model.SystemEventMemberMuted:
		if targetText == "" {
			return fmt.Sprintf("%s muted a member", actorName)
		}
		return fmt.Sprintf("%s muted %s", actorName, targetText)
	case model.SystemEventMemberUnmuted:
		if targetText == "" {
			return fmt.Sprintf("%s unmuted a member", actorName)
		}
		return fmt.Sprintf("%s unmuted %s", actorName, targetText)
	case model.SystemEventGroupMuted:
		return fmt.Sprintf("%s enabled mute all", actorName)
	case model.SystemEventGroupUnmuted:
		return fmt.Sprintf("%s disabled mute all", actorName)
	case model.SystemEventAdminAdded:
		if targetText == "" {
			return fmt.Sprintf("%s set a new admin", actorName)
		}
		return fmt.Sprintf("%s set %s as admin", actorName, targetText)
	case model.SystemEventAdminRemoved:
		if targetText == "" {
			return fmt.Sprintf("%s removed an admin", actorName)
		}
		return fmt.Sprintf("%s removed admin role from %s", actorName, targetText)
	case model.SystemEventOwnerTransferred:
		if targetText == "" {
			return fmt.Sprintf("%s transferred ownership", actorName)
		}
		return fmt.Sprintf("%s transferred ownership to %s", actorName, targetText)
	default:
		return "Group system message"
	}
}

func (s *ChatService) renderSystemMessageTextV2(ctx context.Context, eventType string, actorUserID uint64, targetUserIDs []uint64) string {
	actorName := s.lookupUserDisplayName(ctx, actorUserID)
	targetNames := make([]string, 0, len(targetUserIDs))
	for _, targetUserID := range targetUserIDs {
		targetNames = append(targetNames, s.lookupUserDisplayName(ctx, targetUserID))
	}
	targetText := strings.Join(targetNames, "、")

	switch eventType {
	case model.SystemEventMemberJoined:
		return fmt.Sprintf("%s 加入了群聊", actorName)
	case model.SystemEventMemberLeft:
		return fmt.Sprintf("%s 退出了群聊", actorName)
	case model.SystemEventMemberInvited:
		if targetText == "" {
			return fmt.Sprintf("%s 邀请了新成员加入群聊", actorName)
		}
		return fmt.Sprintf("%s 邀请 %s 加入了群聊", actorName, targetText)
	case model.SystemEventMemberRemoved:
		if targetText == "" {
			return fmt.Sprintf("%s 移出了一名成员", actorName)
		}
		return fmt.Sprintf("%s 将 %s 移出了群聊", actorName, targetText)
	case model.SystemEventMemberMuted:
		if targetText == "" {
			return fmt.Sprintf("%s 禁言了一名成员", actorName)
		}
		return fmt.Sprintf("%s 禁言了 %s", actorName, targetText)
	case model.SystemEventMemberUnmuted:
		if targetText == "" {
			return fmt.Sprintf("%s 解除了成员禁言", actorName)
		}
		return fmt.Sprintf("%s 解除了 %s 的禁言", actorName, targetText)
	case model.SystemEventGroupMuted:
		return fmt.Sprintf("%s 开启了全员禁言", actorName)
	case model.SystemEventGroupUnmuted:
		return fmt.Sprintf("%s 关闭了全员禁言", actorName)
	case model.SystemEventAdminAdded:
		if targetText == "" {
			return fmt.Sprintf("%s 设置了新管理员", actorName)
		}
		return fmt.Sprintf("%s 将 %s 设为管理员", actorName, targetText)
	case model.SystemEventAdminRemoved:
		if targetText == "" {
			return fmt.Sprintf("%s 取消了一名管理员", actorName)
		}
		return fmt.Sprintf("%s 取消了 %s 的管理员身份", actorName, targetText)
	case model.SystemEventOwnerTransferred:
		if targetText == "" {
			return fmt.Sprintf("%s 转让了群主", actorName)
		}
		return fmt.Sprintf("%s 将群主转让给 %s", actorName, targetText)
	case model.SystemEventAnnouncementUpdated:
		return fmt.Sprintf("%s 更新了群公告", actorName)
	default:
		return "群系统消息"
	}
}

func (s *ChatService) lookupUserDisplayName(ctx context.Context, userID uint64) string {
	if userID == 0 {
		return "System"
	}
	if s.UserClient != nil {
		user, err := s.UserClient.GetUser(ctx, userID)
		if err == nil && user != nil {
			if nickname := strings.TrimSpace(user.Nickname); nickname != "" {
				return nickname
			}
		}
	}
	return fmt.Sprintf("User %d", userID)
}

func (s *ChatService) decorateMessageReadState(ctx context.Context, conversation *model.Conversation, operatorID uint64, messages []MessageView) error {
	if conversation == nil || len(messages) == 0 {
		return nil
	}

	userMembers, err := s.MemberRepo.ListUserMembers(ctx, conversation.ID)
	if err != nil {
		return err
	}

	switch conversation.Type {
	case model.ConversationTypeSingle:
		var peerLastReadMessageID *uint64
		for _, member := range userMembers {
			if member.MemberID == operatorID {
				continue
			}
			peerLastReadMessageID = member.LastReadMessageID
			break
		}
		for index := range messages {
			readByPeer := peerLastReadMessageID != nil && *peerLastReadMessageID >= messages[index].ID
			messages[index].ReadByPeer = &readByPeer
		}
	case model.ConversationTypeGroup:
		for index := range messages {
			var readCount int32
			for _, member := range userMembers {
				if member.LastReadMessageID != nil && *member.LastReadMessageID >= messages[index].ID {
					readCount++
				}
			}
			messages[index].ReadCount = &readCount
		}
	}

	return nil
}
