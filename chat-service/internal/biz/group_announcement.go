package biz

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"example.com/aim/chat-service/internal/dal/model"
	"gorm.io/gorm"
)

func (s *ChatService) UpdateGroupAnnouncement(ctx context.Context, input UpdateGroupAnnouncementInput) (*ConversationEventView, error) {
	if input.OperatorID == 0 {
		return nil, ErrBadRequest
	}

	announcement := strings.TrimSpace(input.Announcement)
	if utf8.RuneCountInString(announcement) > 2000 {
		return nil, ErrBadRequest
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
		return nil, ErrGroupAdminRequired
	}

	var event *ConversationEventView
	err = s.TxManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		memberRepo := s.MemberRepo.WithTx(tx)
		groupRepo := s.GroupRepo.WithTx(tx)
		messageRepo := s.MessageRepo.WithTx(tx)
		conversationRepo := s.ConversationRepo.WithTx(tx)

		currentOperator, group, loadErr := s.loadOperatorAndGroupTx(ctx, memberRepo, groupRepo, conversation.ID, input.OperatorID)
		if loadErr != nil {
			return loadErr
		}
		if currentOperator.Role != model.MemberRoleOwner && currentOperator.Role != model.MemberRoleAdmin {
			return ErrGroupAdminRequired
		}
		if group.Announcement == announcement {
			return nil
		}

		now := time.Now()
		group.Announcement = announcement
		group.AnnouncementUpdatedBy = &input.OperatorID
		group.AnnouncementUpdatedAt = &now
		if err := groupRepo.Update(ctx, group); err != nil {
			return err
		}

		systemMessage, recipientUserIDs, eventErr := s.createSystemMessageTx(
			ctx,
			conversation,
			conversationRepo,
			memberRepo,
			messageRepo,
			s.notificationRepoWithTx(tx),
			model.SystemEventAnnouncementUpdated,
			input.OperatorID,
			nil,
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
