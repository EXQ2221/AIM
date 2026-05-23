package handler

import (
	"strconv"
	"strings"

	"example.com/aim/gateway/internal/model/dto"
	notificationx "example.com/aim/gateway/internal/notification"
	chatpb "example.com/aim/gateway/kitex_gen/chat"
	"github.com/gin-gonic/gin"
)

func conversationIDParam(ctx *gin.Context) (string, bool) {
	value := strings.TrimSpace(ctx.Param("conversationId"))
	if value == "" {
		writeError(ctx, 400, "invalid conversationId")
		return "", false
	}
	return value, true
}

func targetUserIDParam(ctx *gin.Context) (int64, bool) {
	value := strings.TrimSpace(ctx.Param("targetUserId"))
	targetUserID, err := strconv.ParseInt(value, 10, 64)
	if err != nil || targetUserID <= 0 {
		writeError(ctx, 400, "invalid targetUserId")
		return 0, false
	}
	return targetUserID, true
}

func messageIDParam(ctx *gin.Context) (int64, bool) {
	value := strings.TrimSpace(ctx.Param("messageId"))
	messageID, err := strconv.ParseInt(value, 10, 64)
	if err != nil || messageID <= 0 {
		writeError(ctx, 400, "invalid messageId")
		return 0, false
	}
	return messageID, true
}

func toGroupModel(group *chatpb.GroupInfo) dto.GroupInfo {
	if group == nil {
		return dto.GroupInfo{}
	}
	info := dto.GroupInfo{
		ConversationID: group.ConversationId,
		Type:           group.Type,
		Name:           group.Name,
		Avatar:         group.Avatar,
		Announcement:   group.Announcement,
		OwnerID:        group.OwnerId,
		JoinPolicy:     group.JoinPolicy,
		CreatedAt:      group.CreatedAt,
	}
	if group.AnnouncementUpdatedBy != nil {
		value := *group.AnnouncementUpdatedBy
		info.AnnouncementUpdatedBy = &value
	}
	if group.AnnouncementUpdatedAt != nil {
		value := *group.AnnouncementUpdatedAt
		info.AnnouncementUpdatedAt = &value
	}
	return info
}

func toConversationModel(conversation *chatpb.ConversationInfo) dto.ConversationInfo {
	if conversation == nil {
		return dto.ConversationInfo{}
	}
	return dto.ConversationInfo{
		ConversationID:        conversation.ConversationId,
		Type:                  conversation.Type,
		Title:                 conversation.Title,
		Avatar:                conversation.Avatar,
		LastMessageID:         conversation.LastMessageId,
		LastMessageAt:         conversation.LastMessageAt,
		LastMessageSenderID:   conversation.LastMessageSenderId,
		LastMessageSenderName: conversation.LastMessageSenderName,
		LastMessageContent:    conversation.LastMessageContent,
		MuteAll:               conversation.MuteAll,
		Role:                  conversation.Role,
		IsPinned:              conversation.IsPinned,
		IsMuted:               conversation.IsMuted,
		UpdatedAt:             conversation.UpdatedAt,
	}
}

func toNotificationModel(item *chatpb.NotificationInfo) dto.NotificationInfo {
	if item == nil {
		return dto.NotificationInfo{}
	}
	category, summary, detail := notificationx.Normalize(item.Type, item.Title, item.Content)
	return dto.NotificationInfo{
		ID:               item.Id,
		Type:             item.Type,
		Category:         category,
		Title:            item.Title,
		Summary:          summary,
		Content:          item.Content,
		Detail:           detail,
		ConversationID:   item.ConversationId,
		RelatedMessageID: item.RelatedMessageId,
		IsRead:           item.IsRead,
		CreatedAt:        item.CreatedAt,
	}
}

func toMessageModel(message *chatpb.MessageInfo) dto.MessageInfo {
	if message == nil {
		return dto.MessageInfo{}
	}
	var replyTo *dto.ReplyPreviewInfo
	if message.ReplyTo != nil {
		replyTo = &dto.ReplyPreviewInfo{
			MessageID:      message.ReplyTo.MessageId,
			SenderID:       message.ReplyTo.SenderId,
			SenderType:     message.ReplyTo.SenderType,
			MessageType:    message.ReplyTo.MessageType,
			ContentPreview: message.ReplyTo.ContentPreview,
		}
	}
	return dto.MessageInfo{
		ID:             message.Id,
		ConversationID: message.ConversationId,
		SenderID:       message.SenderId,
		SenderType:     message.SenderType,
		MessageType:    message.MessageType,
		Content:        message.Content,
		ReplyToID:      message.ReplyToId,
		ReplyTo:        replyTo,
		Status:         message.Status,
		CreatedAt:      message.CreatedAt,
		ReadByPeer:     message.ReadByPeer,
		ReadCount:      message.ReadCount,
	}
}

func toBotModel(item *chatpb.BotInfo) dto.BotInfo {
	if item == nil {
		return dto.BotInfo{}
	}
	return dto.BotInfo{
		BotID:           item.BotId,
		MemberType:      item.MemberType,
		MemberID:        item.MemberId,
		Name:            item.Name,
		DisplayName:     item.DisplayName,
		MentionName:     item.MentionName,
		Aliases:         item.Aliases,
		Avatar:          item.Avatar,
		Description:     item.Description,
		Enabled:         item.Enabled,
		PermissionScope: item.PermissionScope,
		MemberStatus:    item.MemberStatus,
		ModelName:       item.ModelName,
		SupportedModels: item.SupportedModels,
	}
}

func toAICallLogModel(item *chatpb.AICallLogInfo) dto.AICallLogInfo {
	if item == nil {
		return dto.AICallLogInfo{}
	}
	return dto.AICallLogInfo{
		ID:                item.Id,
		ConversationID:    item.ConversationId,
		UserID:            item.UserId,
		BotID:             item.BotId,
		BotName:           item.BotName,
		RequestMessageID:  item.RequestMessageId,
		ResponseMessageID: item.ResponseMessageId,
		ModelName:         item.ModelName,
		PromptTokens:      item.PromptTokens,
		CompletionTokens:  item.CompletionTokens,
		TotalTokens:       item.TotalTokens,
		LatencyMS:         item.LatencyMs,
		Status:            item.Status,
		ErrorMessage:      item.ErrorMessage,
		CreatedAt:         item.CreatedAt,
	}
}

func toUserMemoryModel(item *chatpb.UserMemoryInfo) dto.UserMemoryInfo {
	if item == nil {
		return dto.UserMemoryInfo{}
	}
	info := dto.UserMemoryInfo{
		ID:                   item.Id,
		UserID:               item.UserId,
		Content:              item.Content,
		SourceConversationID: item.SourceConversationId,
		LastUsedAt:           item.LastUsedAt,
		CreatedAt:            item.CreatedAt,
		UpdatedAt:            item.UpdatedAt,
	}
	if item.SourceMessageId != nil {
		value := *item.SourceMessageId
		info.SourceMessageID = &value
	}
	return info
}
