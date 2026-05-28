package biz

import (
	"context"
	"strings"

	"example.com/aim/chat-service/internal/dal/model"
	"example.com/aim/chat-service/internal/repository"
	"gorm.io/gorm"
)

func (s *ChatService) ListNotifications(ctx context.Context, userID uint64, unreadOnly bool, limit int) (NotificationListView, error) {
	if userID == 0 {
		return NotificationListView{}, ErrBadRequest
	}
	if s.NotificationRepo == nil {
		return NotificationListView{}, nil
	}

	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	rows, err := s.NotificationRepo.ListByUserID(ctx, userID, unreadOnly, limit)
	if err != nil {
		return NotificationListView{}, err
	}
	unreadCount, err := s.NotificationRepo.CountUnreadByUserID(ctx, userID)
	if err != nil {
		return NotificationListView{}, err
	}

	items := make([]NotificationView, 0, len(rows))
	for index := range rows {
		items = append(items, notificationToView(&rows[index]))
	}
	return NotificationListView{
		Notifications: items,
		UnreadCount:   unreadCount,
	}, nil
}

func (s *ChatService) MarkNotificationRead(ctx context.Context, userID, notificationID uint64) error {
	if userID == 0 || notificationID == 0 {
		return ErrBadRequest
	}
	if s.NotificationRepo == nil {
		return nil
	}
	return s.NotificationRepo.MarkRead(ctx, userID, notificationID)
}

func (s *ChatService) MarkAllNotificationsRead(ctx context.Context, userID uint64) error {
	if userID == 0 {
		return ErrBadRequest
	}
	if s.NotificationRepo == nil {
		return nil
	}
	return s.NotificationRepo.MarkAllRead(ctx, userID)
}

func (s *ChatService) CreateNotification(ctx context.Context, input CreateNotificationInput) (NotificationView, int64, error) {
	if input.OperatorID == 0 || input.UserID == 0 || input.OperatorID != input.UserID {
		return NotificationView{}, 0, ErrBadRequest
	}
	if s.NotificationRepo == nil {
		return NotificationView{}, 0, ErrBadRequest
	}

	notificationType := strings.TrimSpace(input.Type)
	title := strings.TrimSpace(input.Title)
	content := strings.TrimSpace(input.Content)
	if notificationType == "" || title == "" {
		return NotificationView{}, 0, ErrBadRequest
	}

	item := &model.Notification{
		UserID:           input.UserID,
		Type:             model.NotificationType(notificationType),
		Title:            title,
		Content:          content,
		ConversationRef:  strings.TrimSpace(input.ConversationID),
		RelatedMessageID: input.RelatedMessageID,
		IsRead:           false,
	}
	if err := s.NotificationRepo.Create(ctx, item); err != nil {
		return NotificationView{}, 0, err
	}
	unreadCount, err := s.NotificationRepo.CountUnreadByUserID(ctx, input.UserID)
	if err != nil {
		return NotificationView{}, 0, err
	}
	return notificationToView(item), unreadCount, nil
}

func (s *ChatService) createEventNotificationsTx(
	ctx context.Context,
	notificationRepo repository.NotificationRepository,
	conversation *model.Conversation,
	eventType string,
	actorUserID uint64,
	targetUserIDs []uint64,
	relatedMessageID uint64,
) error {
	if notificationRepo == nil || conversation == nil {
		return nil
	}

	recipients := notificationRecipients(eventType, actorUserID, targetUserIDs)
	if len(recipients) == 0 {
		return nil
	}

	for _, userID := range recipients {
		title, content := s.renderNotificationCopy(ctx, conversation, eventType, actorUserID, targetUserIDs, userID)
		item := &model.Notification{
			UserID:           userID,
			Type:             model.NotificationTypeGroupEvent,
			Title:            title,
			Content:          content,
			ConversationRef:  conversation.ConversationID,
			RelatedMessageID: &relatedMessageID,
			IsRead:           false,
		}
		if err := notificationRepo.Create(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

func notificationToView(row *model.Notification) NotificationView {
	if row == nil {
		return NotificationView{}
	}
	return NotificationView{
		ID:               row.ID,
		Type:             string(row.Type),
		Title:            row.Title,
		Content:          row.Content,
		ConversationID:   row.ConversationRef,
		RelatedMessageID: row.RelatedMessageID,
		IsRead:           row.IsRead,
		CreatedAt:        row.CreatedAt.Unix(),
	}
}

func notificationRecipients(eventType string, actorUserID uint64, targetUserIDs []uint64) []uint64 {
	switch eventType {
	case model.SystemEventMemberInvited,
		model.SystemEventMemberRemoved,
		model.SystemEventMemberMuted,
		model.SystemEventMemberUnmuted,
		model.SystemEventAdminAdded,
		model.SystemEventAdminRemoved,
		model.SystemEventOwnerTransferred,
		model.SystemEventGroupDisbanded:
	default:
		return nil
	}

	seen := make(map[uint64]struct{}, len(targetUserIDs))
	result := make([]uint64, 0, len(targetUserIDs))
	for _, userID := range targetUserIDs {
		if userID == 0 || userID == actorUserID {
			continue
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		result = append(result, userID)
	}
	return result
}

func (s *ChatService) renderNotificationCopy(
	ctx context.Context,
	conversation *model.Conversation,
	eventType string,
	actorUserID uint64,
	targetUserIDs []uint64,
	recipientUserID uint64,
) (string, string) {
	title := notificationTitle(eventType)
	actorName := s.lookupUserDisplayName(ctx, actorUserID)
	groupName := notificationGroupName(conversation)
	targetText := s.notificationTargetText(ctx, targetUserIDs)

	switch eventType {
	case model.SystemEventMemberInvited:
		if containsUint64(targetUserIDs, recipientUserID) {
			return title, actorName + " 邀请你加入群聊「" + groupName + "」。"
		}
		return title, actorName + " 邀请 " + targetText + " 加入群聊「" + groupName + "」。"
	case model.SystemEventMemberRemoved:
		if containsUint64(targetUserIDs, recipientUserID) {
			return title, "你已被 " + actorName + " 移出群聊「" + groupName + "」。"
		}
		return title, actorName + " 将 " + targetText + " 移出群聊「" + groupName + "」。"
	case model.SystemEventMemberMuted:
		if containsUint64(targetUserIDs, recipientUserID) {
			return title, "你在群聊「" + groupName + "」中已被 " + actorName + " 禁言。"
		}
		return title, actorName + " 禁言了 " + targetText + "。"
	case model.SystemEventMemberUnmuted:
		if containsUint64(targetUserIDs, recipientUserID) {
			return title, "你在群聊「" + groupName + "」中的禁言已被 " + actorName + " 解除。"
		}
		return title, actorName + " 解除了 " + targetText + " 的禁言。"
	case model.SystemEventAdminAdded:
		if containsUint64(targetUserIDs, recipientUserID) {
			return title, "你已成为群聊「" + groupName + "」的管理员。"
		}
		return title, actorName + " 将 " + targetText + " 设为管理员。"
	case model.SystemEventAdminRemoved:
		if containsUint64(targetUserIDs, recipientUserID) {
			return title, "你已不再是群聊「" + groupName + "」的管理员。"
		}
		return title, actorName + " 取消了 " + targetText + " 的管理员身份。"
	case model.SystemEventOwnerTransferred:
		if containsUint64(targetUserIDs, recipientUserID) {
			return title, actorName + " 已将群聊「" + groupName + "」的群主转让给你。"
		}
		return title, actorName + " 已将群主转让给 " + targetText + "。"
	case model.SystemEventGroupDisbanded:
		return title, actorName + " 已解散群聊「" + groupName + "」。"
	default:
		content := strings.TrimSpace(s.renderSystemMessageTextV2(ctx, eventType, actorUserID, targetUserIDs))
		if content == "" {
			content = "群聊「" + groupName + "」有新的通知。"
		}
		return title, content
	}
}

func (s *ChatService) notificationTargetText(ctx context.Context, targetUserIDs []uint64) string {
	targetNames := make([]string, 0, len(targetUserIDs))
	for _, targetUserID := range targetUserIDs {
		if targetUserID == 0 {
			continue
		}
		targetNames = append(targetNames, s.lookupUserDisplayName(ctx, targetUserID))
	}
	if len(targetNames) == 0 {
		return "成员"
	}
	return strings.Join(targetNames, "、")
}

func notificationGroupName(conversation *model.Conversation) string {
	if conversation == nil {
		return "未命名群聊"
	}
	if title := strings.TrimSpace(conversation.Title); title != "" {
		return title
	}
	if conversationID := strings.TrimSpace(conversation.ConversationID); conversationID != "" {
		return conversationID
	}
	return "未命名群聊"
}

func containsUint64(values []uint64, target uint64) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func notificationTitle(eventType string) string {
	switch eventType {
	case model.SystemEventMemberInvited:
		return "群邀请"
	case model.SystemEventMemberRemoved:
		return "群成员变动"
	case model.SystemEventMemberMuted:
		return "禁言通知"
	case model.SystemEventMemberUnmuted:
		return "解除禁言"
	case model.SystemEventAdminAdded:
		return "管理员变更"
	case model.SystemEventAdminRemoved:
		return "管理员变更"
	case model.SystemEventOwnerTransferred:
		return "群主变更"
	case model.SystemEventGroupDisbanded:
		return "群聊解散"
	default:
		return "群通知"
	}
}

func (s *ChatService) notificationRepoWithTx(tx *gorm.DB) repository.NotificationRepository {
	if s == nil || s.NotificationRepo == nil {
		return nil
	}
	return s.NotificationRepo.WithTx(tx)
}

func (s *ChatService) groupJoinRequestRepoWithTx(tx *gorm.DB) repository.GroupJoinRequestRepository {
	if s == nil || s.GroupJoinRequestRepo == nil {
		return nil
	}
	return s.GroupJoinRequestRepo.WithTx(tx)
}
