package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"example.com/aim/gateway/internal/knowledgeimport"
	"example.com/aim/gateway/internal/middleware"
	"example.com/aim/gateway/internal/model"
	notificationx "example.com/aim/gateway/internal/notification"
	"example.com/aim/gateway/internal/observability"
	"example.com/aim/gateway/internal/rpc"
	gatewayws "example.com/aim/gateway/internal/websocket"
	chatpb "example.com/aim/gateway/kitex_gen/chat"
	"example.com/aim/gateway/kitex_gen/chat/chatservice"
	ragpb "example.com/aim/gateway/kitex_gen/rag"
	"example.com/aim/gateway/kitex_gen/rag/ragservice"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	maxKnowledgeDocumentUploadBytes int64 = 20 << 20
)

var (
	knowledgeImportParseTimeout      = getenvDuration("KNOWLEDGE_IMPORT_PARSE_TIMEOUT", 5*time.Minute)
	knowledgeImportRAGAddTimeout     = getenvDuration("KNOWLEDGE_IMPORT_RAG_ADD_TIMEOUT", 90*time.Second)
	knowledgeImportWatchTimeout      = getenvDuration("KNOWLEDGE_IMPORT_WATCH_TIMEOUT", 5*time.Minute)
	knowledgeImportPollInterval      = getenvDuration("KNOWLEDGE_IMPORT_POLL_INTERVAL", 2*time.Second)
	knowledgeImportPollRPCDeadline   = getenvDuration("KNOWLEDGE_IMPORT_POLL_RPC_TIMEOUT", 5*time.Second)
	knowledgeImportNotifyRPCDeadline = getenvDuration("KNOWLEDGE_IMPORT_NOTIFY_RPC_TIMEOUT", 5*time.Second)
)

func getenvDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if d, err := time.ParseDuration(value); err == nil && d > 0 {
		return d
	}
	return fallback
}

func CreateGroup(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	var req model.CreateGroupRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(ctx, 400, "name is required")
		return
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.CreateGroup(ctx.Request.Context(), &chatpb.CreateGroupRequest{
		OperatorId:   authCtx.UserID,
		Name:         req.Name,
		Avatar:       req.Avatar,
		Announcement: req.Announcement,
		JoinPolicy:   req.JoinPolicy,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	writeJSON(ctx, 200, model.APIResponse{
		Code:    0,
		Message: "success",
		Data:    toGroupModel(resp.Group),
	})
}

func GetGroupInfo(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	conversationID, ok := conversationIDParam(ctx)
	if !ok {
		return
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.GetGroupInfo(ctx.Request.Context(), &chatpb.GetGroupInfoRequest{
		OperatorId:     authCtx.UserID,
		ConversationId: conversationID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	writeJSON(ctx, 200, model.APIResponse{
		Code:    0,
		Message: "success",
		Data:    toGroupModel(resp.Group),
	})
}

func ListConversations(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.ListConversations(ctx.Request.Context(), &chatpb.ListConversationsRequest{
		UserId: authCtx.UserID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	conversations := make([]model.ConversationInfo, 0, len(resp.Conversations))
	for _, conversation := range resp.Conversations {
		conversations = append(conversations, toConversationModel(conversation))
	}

	writeJSON(ctx, 200, model.APIResponse{
		Code:    0,
		Message: "success",
		Data:    conversations,
	})
}

func ListNotifications(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	unreadOnly := false
	if value := strings.TrimSpace(ctx.Query("unreadOnly")); value != "" {
		unreadOnly = strings.EqualFold(value, "true") || value == "1"
	}
	var limit *int32
	if value := strings.TrimSpace(ctx.Query("limit")); value != "" {
		parsed, parseErr := strconv.ParseInt(value, 10, 32)
		if parseErr != nil || parsed <= 0 {
			writeError(ctx, 400, "invalid limit")
			return
		}
		limitValue := int32(parsed)
		limit = &limitValue
	}

	resp, err := client.ListNotifications(ctx.Request.Context(), &chatpb.ListNotificationsRequest{
		OperatorId: authCtx.UserID,
		UnreadOnly: &unreadOnly,
		Limit:      limit,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	items := make([]model.NotificationInfo, 0, len(resp.Notifications))
	for _, item := range resp.Notifications {
		items = append(items, toNotificationModel(item))
	}
	writeJSON(ctx, 200, model.APIResponse{
		Code:    0,
		Message: "success",
		Data: model.NotificationListResponse{
			Notifications: items,
			UnreadCount:   resp.UnreadCount,
		},
	})
}

func MarkNotificationRead(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	value := strings.TrimSpace(ctx.Param("notificationId"))
	notificationID, err := strconv.ParseInt(value, 10, 64)
	if err != nil || notificationID <= 0 {
		writeError(ctx, 400, "invalid notificationId")
		return
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}
	resp, err := client.MarkNotificationRead(ctx.Request.Context(), &chatpb.MarkNotificationReadRequest{
		OperatorId:     authCtx.UserID,
		NotificationId: notificationID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if !resp.Success {
		writeError(ctx, statusFromMessage(resp.Message), presentableMessage(resp.Message))
		return
	}
	writeJSON(ctx, 200, model.APIResponse{Code: 0, Message: "success"})
}

func MarkAllNotificationsRead(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}
	resp, err := client.MarkAllNotificationsRead(ctx.Request.Context(), &chatpb.MarkAllNotificationsReadRequest{
		OperatorId: authCtx.UserID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if !resp.Success {
		writeError(ctx, statusFromMessage(resp.Message), presentableMessage(resp.Message))
		return
	}
	writeJSON(ctx, 200, model.APIResponse{Code: 0, Message: "success"})
}

func FindSingleConversation(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	targetUserIDStr := ctx.Query("targetUserId")
	if targetUserIDStr == "" {
		writeError(ctx, 400, "missing targetUserId")
		return
	}
	targetUserID, err := strconv.ParseInt(targetUserIDStr, 10, 64)
	if err != nil {
		writeError(ctx, 400, "invalid targetUserId")
		return
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.FindSingleByUsers(ctx.Request.Context(), &chatpb.FindSingleByUsersRequest{
		OperatorId:   authCtx.UserID,
		TargetUserId: targetUserID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	var data interface{} = nil
	if resp.Conversation != nil {
		data = toConversationModel(resp.Conversation)
	}

	writeJSON(ctx, 200, model.APIResponse{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

func JoinGroup(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	conversationID, ok := conversationIDParam(ctx)
	if !ok {
		return
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.JoinGroup(ctx.Request.Context(), &chatpb.JoinGroupRequest{
		OperatorId:     authCtx.UserID,
		ConversationId: conversationID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if !resp.Success {
		writeError(ctx, statusFromMessage(resp.Message), presentableMessage(resp.Message))
		return
	}
	broadcastConversationEvent(ctx.Request.Context(), client, resp)

	writeJSON(ctx, 200, model.APIResponse{Code: 0, Message: "success"})
}

func InviteMember(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	conversationID, ok := conversationIDParam(ctx)
	if !ok {
		return
	}

	var req model.InviteMemberRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}
	if req.TargetUserID <= 0 {
		writeError(ctx, 400, "targetUserId is required")
		return
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.InviteMember(ctx.Request.Context(), &chatpb.InviteMemberRequest{
		OperatorId:     authCtx.UserID,
		ConversationId: conversationID,
		TargetUserId:   req.TargetUserID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if !resp.Success {
		writeError(ctx, statusFromMessage(resp.Message), presentableMessage(resp.Message))
		return
	}
	broadcastConversationEvent(ctx.Request.Context(), client, resp)

	writeJSON(ctx, 200, model.APIResponse{Code: 0, Message: "success"})
}

func LeaveGroup(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	conversationID, ok := conversationIDParam(ctx)
	if !ok {
		return
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.LeaveGroup(ctx.Request.Context(), &chatpb.LeaveGroupRequest{
		OperatorId:     authCtx.UserID,
		ConversationId: conversationID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if !resp.Success {
		writeError(ctx, statusFromMessage(resp.Message), presentableMessage(resp.Message))
		return
	}
	broadcastConversationEvent(ctx.Request.Context(), client, resp)

	writeJSON(ctx, 200, model.APIResponse{Code: 0, Message: "success"})
}

func TransferOwner(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	conversationID, ok := conversationIDParam(ctx)
	if !ok {
		return
	}

	var req model.TransferOwnerRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}
	if req.TargetUserID <= 0 {
		writeError(ctx, 400, "targetUserId is required")
		return
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.TransferOwner(ctx.Request.Context(), &chatpb.TransferOwnerRequest{
		OperatorId:     authCtx.UserID,
		ConversationId: conversationID,
		TargetUserId:   req.TargetUserID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if !resp.Success {
		writeError(ctx, statusFromMessage(resp.Message), presentableMessage(resp.Message))
		return
	}
	broadcastConversationEvent(ctx.Request.Context(), client, resp)

	writeJSON(ctx, 200, model.APIResponse{Code: 0, Message: "success"})
}

func SetAdmin(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	conversationID, ok := conversationIDParam(ctx)
	if !ok {
		return
	}

	var req model.SetAdminRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}
	if req.TargetUserID <= 0 {
		writeError(ctx, 400, "targetUserId is required")
		return
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.SetAdmin(ctx.Request.Context(), &chatpb.SetAdminRequest{
		OperatorId:     authCtx.UserID,
		ConversationId: conversationID,
		TargetUserId:   req.TargetUserID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if !resp.Success {
		writeError(ctx, statusFromMessage(resp.Message), presentableMessage(resp.Message))
		return
	}
	broadcastConversationEvent(ctx.Request.Context(), client, resp)

	writeJSON(ctx, 200, model.APIResponse{Code: 0, Message: "success"})
}

func RemoveAdmin(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	conversationID, ok := conversationIDParam(ctx)
	if !ok {
		return
	}
	targetUserID, ok := targetUserIDParam(ctx)
	if !ok {
		return
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.RemoveAdmin(ctx.Request.Context(), &chatpb.RemoveAdminRequest{
		OperatorId:     authCtx.UserID,
		ConversationId: conversationID,
		TargetUserId:   targetUserID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if !resp.Success {
		writeError(ctx, statusFromMessage(resp.Message), presentableMessage(resp.Message))
		return
	}
	broadcastConversationEvent(ctx.Request.Context(), client, resp)

	writeJSON(ctx, 200, model.APIResponse{Code: 0, Message: "success"})
}

func MuteMember(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	conversationID, ok := conversationIDParam(ctx)
	if !ok {
		return
	}
	targetUserID, ok := targetUserIDParam(ctx)
	if !ok {
		return
	}

	var req model.MuteMemberRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}
	if req.MuteUntil <= 0 {
		writeError(ctx, 400, "muteUntil is required")
		return
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.MuteMember(ctx.Request.Context(), &chatpb.MuteMemberRequest{
		OperatorId:     authCtx.UserID,
		ConversationId: conversationID,
		TargetUserId:   targetUserID,
		MuteUntil:      req.MuteUntil,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if !resp.Success {
		writeError(ctx, statusFromMessage(resp.Message), presentableMessage(resp.Message))
		return
	}
	broadcastConversationEvent(ctx.Request.Context(), client, resp)

	writeJSON(ctx, 200, model.APIResponse{Code: 0, Message: "success"})
}

func UnmuteMember(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	conversationID, ok := conversationIDParam(ctx)
	if !ok {
		return
	}
	targetUserID, ok := targetUserIDParam(ctx)
	if !ok {
		return
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.UnmuteMember(ctx.Request.Context(), &chatpb.UnmuteMemberRequest{
		OperatorId:     authCtx.UserID,
		ConversationId: conversationID,
		TargetUserId:   targetUserID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if !resp.Success {
		writeError(ctx, statusFromMessage(resp.Message), presentableMessage(resp.Message))
		return
	}
	broadcastConversationEvent(ctx.Request.Context(), client, resp)

	writeJSON(ctx, 200, model.APIResponse{Code: 0, Message: "success"})
}

func RemoveMember(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	conversationID, ok := conversationIDParam(ctx)
	if !ok {
		return
	}
	targetUserID, ok := targetUserIDParam(ctx)
	if !ok {
		return
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.RemoveMember(ctx.Request.Context(), &chatpb.RemoveMemberRequest{
		OperatorId:     authCtx.UserID,
		ConversationId: conversationID,
		TargetUserId:   targetUserID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if !resp.Success {
		writeError(ctx, statusFromMessage(resp.Message), presentableMessage(resp.Message))
		return
	}
	broadcastConversationEvent(ctx.Request.Context(), client, resp)

	writeJSON(ctx, 200, model.APIResponse{Code: 0, Message: "success"})
}

func EnableGroupMuteAll(ctx *gin.Context) {
	setGroupMuteAll(ctx, true)
}

func DisableGroupMuteAll(ctx *gin.Context) {
	setGroupMuteAll(ctx, false)
}

func UpdateGroupAnnouncement(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	conversationID, ok := conversationIDParam(ctx)
	if !ok {
		return
	}

	var req model.UpdateGroupAnnouncementRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.UpdateGroupAnnouncement(ctx.Request.Context(), &chatpb.UpdateGroupAnnouncementRequest{
		OperatorId:     authCtx.UserID,
		ConversationId: conversationID,
		Announcement:   req.Announcement,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if !resp.Success {
		writeError(ctx, statusFromMessage(resp.Message), presentableMessage(resp.Message))
		return
	}
	broadcastConversationEvent(ctx.Request.Context(), client, resp)

	writeJSON(ctx, 200, model.APIResponse{Code: 0, Message: "success"})
}

func ListMembers(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	conversationID, ok := conversationIDParam(ctx)
	if !ok {
		return
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.ListMembers(ctx.Request.Context(), &chatpb.ListMembersRequest{
		OperatorId:     authCtx.UserID,
		ConversationId: conversationID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	members := make([]model.MemberInfo, 0, len(resp.Members))
	for _, member := range resp.Members {
		info := model.MemberInfo{
			UserID:     member.UserId,
			Nickname:   member.Nickname,
			Avatar:     member.Avatar,
			Role:       member.Role,
			Status:     member.Status,
			JoinedAt:   member.JoinedAt,
			MemberType: member.MemberType,
		}
		if member.BotId != nil {
			v := int64(*member.BotId)
			info.BotID = &v
		}
		if member.MentionName != nil {
			info.MentionName = member.MentionName
		}
		if member.Aliases != nil {
			info.Aliases = member.Aliases
		}
		if member.Enabled != nil {
			info.Enabled = member.Enabled
		}
		if member.PermissionScope != nil {
			info.PermissionScope = member.PermissionScope
		}
		if member.MuteUntil != nil {
			info.MuteUntil = member.MuteUntil
		}
		members = append(members, info)
	}

	writeJSON(ctx, 200, model.APIResponse{
		Code:    0,
		Message: "success",
		Data:    members,
	})
}

func ListMessages(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	conversationID, ok := conversationIDParam(ctx)
	if !ok {
		return
	}

	var beforeID *int64
	if raw := strings.TrimSpace(ctx.Query("beforeId")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			writeError(ctx, 400, "invalid beforeId")
			return
		}
		beforeID = &parsed
	}

	limit := int32(30)
	if raw := strings.TrimSpace(ctx.Query("limit")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 32)
		if err != nil || parsed <= 0 {
			writeError(ctx, 400, "invalid limit")
			return
		}
		limit = int32(parsed)
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.ListMessages(ctx.Request.Context(), &chatpb.ListMessagesRequest{
		OperatorId:     authCtx.UserID,
		ConversationId: conversationID,
		BeforeId:       beforeID,
		Limit:          limit,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	messages := make([]model.MessageInfo, 0, len(resp.Messages))
	for _, message := range resp.Messages {
		messages = append(messages, toMessageModel(message))
	}

	writeJSON(ctx, 200, model.APIResponse{
		Code:    0,
		Message: "success",
		Data:    messages,
	})
}

func MarkConversationRead(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	conversationID, ok := conversationIDParam(ctx)
	if !ok {
		return
	}

	var req model.MarkConversationReadRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}
	if req.LastReadMessageID <= 0 {
		writeError(ctx, 400, "lastReadMessageId is required")
		return
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.MarkConversationRead(ctx.Request.Context(), &chatpb.MarkConversationReadRequest{
		OperatorId:        authCtx.UserID,
		ConversationId:    conversationID,
		LastReadMessageId: req.LastReadMessageID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if !resp.Success {
		writeError(ctx, statusFromMessage(resp.Message), presentableMessage(resp.Message))
		return
	}

	writeJSON(ctx, 200, model.APIResponse{Code: 0, Message: "success"})
}

func RecallMessage(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	conversationID, ok := conversationIDParam(ctx)
	if !ok {
		return
	}
	messageID, ok := messageIDParam(ctx)
	if !ok {
		return
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.RecallMessage(ctx.Request.Context(), &chatpb.RecallMessageRequest{
		OperatorId:     authCtx.UserID,
		ConversationId: conversationID,
		MessageId:      messageID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if !resp.Success {
		writeError(ctx, statusFromMessage(resp.Message), presentableMessage(resp.Message))
		return
	}

	broadcastMessageRecalledEvent(resp)
	writeJSON(ctx, 200, model.APIResponse{Code: 0, Message: "success"})
}

func ListBots(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.ListBots(ctx.Request.Context(), &chatpb.ListBotsRequest{
		OperatorId: authCtx.UserID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	bots := make([]model.BotInfo, 0, len(resp.Bots))
	for _, item := range resp.Bots {
		bots = append(bots, toBotModel(item))
	}
	writeJSON(ctx, 200, model.APIResponse{Code: 0, Message: "success", Data: bots})
}

func CreateCustomBot(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}
	var req model.CreateCustomBotRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.CreateCustomBot(ctx.Request.Context(), &chatpb.CreateCustomBotRequest{
		OperatorId:      authCtx.UserID,
		Name:            req.Name,
		MentionName:     req.MentionName,
		Aliases:         req.Aliases,
		Description:     req.Description,
		ApiBaseUrl:      req.APIBaseURL,
		ApiKey:          req.APIKey,
		ModelName:       req.ModelName,
		SupportedModels: req.SupportedModels,
		SystemPrompt:    &req.SystemPrompt,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	writeJSON(ctx, 200, model.APIResponse{Code: 0, Message: "success", Data: toBotModel(resp.Bot)})
}

func ListConversationBots(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}
	conversationID, ok := conversationIDParam(ctx)
	if !ok {
		return
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.ListConversationBots(ctx.Request.Context(), &chatpb.ListConversationBotsRequest{
		OperatorId:     authCtx.UserID,
		ConversationId: conversationID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	bots := make([]model.BotInfo, 0, len(resp.Bots))
	for _, item := range resp.Bots {
		bots = append(bots, toBotModel(item))
	}
	writeJSON(ctx, 200, model.APIResponse{Code: 0, Message: "success", Data: bots})
}

func AddConversationBot(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}
	conversationID, ok := conversationIDParam(ctx)
	if !ok {
		return
	}

	var req model.AddConversationBotRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}
	if req.BotID <= 0 {
		writeError(ctx, 400, "botId is required")
		return
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.AddConversationBot(ctx.Request.Context(), &chatpb.AddConversationBotRequest{
		OperatorId:          authCtx.UserID,
		ConversationId:      conversationID,
		BotId:               req.BotID,
		DisplayNameOverride: req.DisplayNameOverride,
		MentionNameOverride: req.MentionNameOverride,
		AliasesOverride:     req.AliasesOverride,
		PermissionScope:     req.PermissionScope,
		ModelNameOverride:   req.ModelNameOverride,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	writeJSON(ctx, 200, model.APIResponse{Code: 0, Message: "success", Data: toBotModel(resp.Bot)})
}

func RemoveConversationBot(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}
	conversationID, ok := conversationIDParam(ctx)
	if !ok {
		return
	}
	botIDValue := strings.TrimSpace(ctx.Param("botId"))
	botID, err := strconv.ParseInt(botIDValue, 10, 64)
	if err != nil || botID <= 0 {
		writeError(ctx, 400, "invalid botId")
		return
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.RemoveConversationBot(ctx.Request.Context(), &chatpb.RemoveConversationBotRequest{
		OperatorId:     authCtx.UserID,
		ConversationId: conversationID,
		BotId:          botID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if !resp.Success {
		writeError(ctx, statusFromMessage(resp.Message), presentableMessage(resp.Message))
		return
	}
	writeJSON(ctx, 200, model.APIResponse{Code: 0, Message: "success"})
}

func ListAICallLogs(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}
	conversationID, ok := conversationIDParam(ctx)
	if !ok {
		return
	}

	var beforeID *int64
	if raw := strings.TrimSpace(ctx.Query("beforeId")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			writeError(ctx, 400, "invalid beforeId")
			return
		}
		beforeID = &parsed
	}

	limit := int32(30)
	if raw := strings.TrimSpace(ctx.Query("limit")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 32)
		if err != nil || parsed <= 0 {
			writeError(ctx, 400, "invalid limit")
			return
		}
		limit = int32(parsed)
	}

	var botID *int64
	if raw := strings.TrimSpace(ctx.Query("botId")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			writeError(ctx, 400, "invalid botId")
			return
		}
		botID = &parsed
	}
	status := strings.TrimSpace(ctx.Query("status"))
	var statusValue *string
	if status != "" {
		statusValue = &status
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.ListAICallLogs(ctx.Request.Context(), &chatpb.ListAICallLogsRequest{
		OperatorId:     authCtx.UserID,
		ConversationId: conversationID,
		BeforeId:       beforeID,
		Limit:          limit,
		BotId:          botID,
		Status:         statusValue,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	logs := make([]model.AICallLogInfo, 0, len(resp.Logs))
	for _, item := range resp.Logs {
		logs = append(logs, toAICallLogModel(item))
	}
	quota := model.AICallLogQuotaInfo{}
	if resp.Quota != nil {
		quota = model.AICallLogQuotaInfo{
			DailyTotalTokens: resp.Quota.DailyTotalTokens,
			DailyTokenLimit:  resp.Quota.DailyTokenLimit,
			RemainingTokens:  resp.Quota.RemainingTokens,
		}
	}
	writeJSON(ctx, 200, model.APIResponse{
		Code:    0,
		Message: "success",
		Data: model.AICallLogListResponse{
			Logs:  logs,
			Quota: quota,
		},
	})
}

func CreateKnowledgeBase(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	var req model.CreateKnowledgeBaseRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(ctx, 400, "name is required")
		return
	}

	client, err := rpc.RAGClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}
	resp, err := client.CreateKnowledgeBase(ctx.Request.Context(), &ragpb.CreateKnowledgeBaseRequest{
		OperatorId:  authCtx.UserID,
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if resp.KnowledgeBase == nil {
		writeError(ctx, 500, "knowledge base creation failed")
		return
	}
	writeJSON(ctx, 200, model.APIResponse{
		Code:    0,
		Message: "success",
		Data: model.KnowledgeBaseInfo{
			KnowledgeBaseID: resp.KnowledgeBase.KnowledgeBaseId,
			Name:            resp.KnowledgeBase.Name,
			Description:     resp.KnowledgeBase.Description,
			Status:          resp.KnowledgeBase.Status,
		},
	})
}

func ListKnowledgeBases(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	client, err := rpc.RAGClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.ListKnowledgeBases(ctx.Request.Context(), &ragpb.ListKnowledgeBasesRequest{
		OperatorId: authCtx.UserID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	items := make([]model.KnowledgeBaseInfo, 0, len(resp.KnowledgeBases))
	for _, item := range resp.KnowledgeBases {
		items = append(items, model.KnowledgeBaseInfo{
			KnowledgeBaseID: item.KnowledgeBaseId,
			Name:            item.Name,
			Description:     item.Description,
			Status:          item.Status,
		})
	}

	writeJSON(ctx, 200, model.APIResponse{
		Code:    0,
		Message: "success",
		Data:    items,
	})
}

func AddKnowledgeDocumentText(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	kbIDValue := strings.TrimSpace(ctx.Param("knowledgeBaseId"))
	kbID, err := strconv.ParseInt(kbIDValue, 10, 64)
	if err != nil || kbID <= 0 {
		writeError(ctx, 400, "invalid knowledgeBaseId")
		return
	}

	var req model.AddKnowledgeDocumentTextRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		writeError(ctx, 400, "title is required")
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		writeError(ctx, 400, "content is required")
		return
	}
	kbName := fmt.Sprintf("知识库 %d", kbID)
	if ragClient, ragErr := rpc.RAGClient(); ragErr == nil {
		kbName = knowledgeBaseDisplayNameV2(ctx.Request.Context(), ragClient, authCtx.UserID, kbID)
	}
	startedAt := time.Now()

	client, err := rpc.RAGClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}
	resp, err := client.AddKnowledgeDocumentText(ctx.Request.Context(), &ragpb.AddKnowledgeDocumentTextRequest{
		OperatorId:      authCtx.UserID,
		KnowledgeBaseId: kbID,
		Title:           req.Title,
		SourceType:      req.SourceType,
		Content:         req.Content,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if resp.Document == nil {
		writeError(ctx, 500, "document creation failed")
		return
	}
	if status := strings.ToUpper(strings.TrimSpace(resp.Document.Status)); status == "PENDING" || status == "PROCESSING" {
		pushKnowledgeImportStatusNotification(
			authCtx.UserID,
			"知识库文档导入已提交",
			fmt.Sprintf("知识库「%s」的文档「%s」已提交，正在后台处理。", kbName, strings.TrimSpace(resp.Document.Title)),
		)
		watchKnowledgeDocumentImport(
			authCtx.UserID,
			kbID,
			kbName,
			resp.Document.DocumentId,
			strings.TrimSpace(resp.Document.Title),
			startedAt,
		)
	}
	writeJSON(ctx, 200, model.APIResponse{
		Code:    0,
		Message: "success",
		Data: model.KnowledgeDocumentInfo{
			DocumentID:      resp.Document.DocumentId,
			KnowledgeBaseID: resp.Document.KnowledgeBaseId,
			Title:           resp.Document.Title,
			SourceType:      resp.Document.SourceType,
			Status:          resp.Document.Status,
			ErrorMessage:    resp.Document.ErrorMessage,
			CreatedAt:       resp.Document.CreatedAt,
		},
	})
}

func DeleteKnowledgeDocument(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	kbIDValue := strings.TrimSpace(ctx.Param("knowledgeBaseId"))
	kbID, err := strconv.ParseInt(kbIDValue, 10, 64)
	if err != nil || kbID <= 0 {
		writeError(ctx, 400, "invalid knowledgeBaseId")
		return
	}

	documentIDValue := strings.TrimSpace(ctx.Param("documentId"))
	documentID, err := strconv.ParseInt(documentIDValue, 10, 64)
	if err != nil || documentID <= 0 {
		writeError(ctx, 400, "invalid documentId")
		return
	}

	client, err := rpc.RAGClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}
	resp, err := client.DeleteKnowledgeDocument(ctx.Request.Context(), &ragpb.DeleteKnowledgeDocumentRequest{
		OperatorId:      authCtx.UserID,
		KnowledgeBaseId: kbID,
		DocumentId:      documentID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if !resp.Success {
		writeError(ctx, statusFromMessage(resp.Message), presentableMessage(resp.Message))
		return
	}

	writeJSON(ctx, 200, model.APIResponse{Code: 0, Message: "success"})
}

func AddKnowledgeDocumentFile(ctx *gin.Context) {
	logger := observability.L()
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	kbIDValue := strings.TrimSpace(ctx.Param("knowledgeBaseId"))
	kbID, err := strconv.ParseInt(kbIDValue, 10, 64)
	if err != nil || kbID <= 0 {
		writeError(ctx, 400, "invalid knowledgeBaseId")
		return
	}
	logger.Info("knowledge_import.start",
		zap.Int64("user_id", authCtx.UserID),
		zap.Int64("kb_id", kbID),
	)

	fileHeader, reqErr := readUploadFile(ctx, "file", maxKnowledgeDocumentUploadBytes)
	if reqErr != nil {
		writeError(ctx, reqErr.status, reqErr.message)
		return
	}

	src, openErr := fileHeader.Open()
	if openErr != nil {
		writeError(ctx, 500, openErr.Error())
		return
	}
	defer src.Close()

	data, readErr := io.ReadAll(io.LimitReader(src, maxKnowledgeDocumentUploadBytes+1))
	if readErr != nil {
		logger.Warn("knowledge_import.read_failed",
			zap.Int64("user_id", authCtx.UserID),
			zap.Int64("kb_id", kbID),
			zap.String("filename", fileHeader.Filename),
			zap.Error(readErr),
		)
		writeError(ctx, 500, readErr.Error())
		return
	}
	if int64(len(data)) > maxKnowledgeDocumentUploadBytes {
		logger.Warn("knowledge_import.rejected_too_large",
			zap.Int64("user_id", authCtx.UserID),
			zap.Int64("kb_id", kbID),
			zap.String("filename", fileHeader.Filename),
			zap.Int("size_bytes", len(data)),
			zap.Int64("limit_bytes", maxKnowledgeDocumentUploadBytes),
		)
		writeError(ctx, 413, "file is too large")
		return
	}

	title := strings.TrimSpace(ctx.PostForm("title"))
	if title == "" {
		title = knowledgeFileTitle(fileHeader.Filename)
	}
	kbName := fmt.Sprintf("知识库 %d", kbID)
	if client, err := rpc.RAGClient(); err == nil {
		kbName = knowledgeBaseDisplayNameV2(ctx.Request.Context(), client, authCtx.UserID, kbID)
	}

	startedAt := time.Now()
	pushKnowledgeImportStatusNotification(
		authCtx.UserID,
		"知识库文件导入已提交",
		fmt.Sprintf("知识库「%s」的文件「%s」已提交，正在后台解析并入库。", kbName, title),
	)
	go runKnowledgeDocumentFileImport(knowledgeDocumentFileImportTask{
		UserID:            authCtx.UserID,
		KnowledgeBaseID:   kbID,
		KnowledgeBaseName: kbName,
		Filename:          fileHeader.Filename,
		ContentType:       fileHeader.Header.Get("Content-Type"),
		Title:             title,
		Data:              data,
		StartedAt:         startedAt,
	})

	logger.Info("knowledge_import.accepted",
		zap.Int64("user_id", authCtx.UserID),
		zap.Int64("kb_id", kbID),
		zap.String("filename", fileHeader.Filename),
		zap.Int("size_bytes", len(data)),
	)
	writeJSON(ctx, 202, model.APIResponse{
		Code:    0,
		Message: "accepted",
		Data: model.KnowledgeDocumentInfo{
			DocumentID:      0,
			KnowledgeBaseID: kbID,
			Title:           title,
			SourceType:      "",
			Status:          "PENDING",
			ErrorMessage:    "",
			CreatedAt:       startedAt.Unix(),
		},
	})
}

type knowledgeDocumentFileImportTask struct {
	UserID            int64
	KnowledgeBaseID   int64
	KnowledgeBaseName string
	Filename          string
	ContentType       string
	Title             string
	Data              []byte
	StartedAt         time.Time
}

func runKnowledgeDocumentFileImport(task knowledgeDocumentFileImportTask) {
	logger := observability.L()
	parseStart := time.Now()
	parseCtx, cancelParse := context.WithTimeout(context.Background(), knowledgeImportParseTimeout)
	parsed, parseErr := knowledgeimport.ParseViaService(
		parseCtx,
		task.Filename,
		task.ContentType,
		task.Data,
		task.Title,
	)
	cancelParse()
	if parseErr != nil {
		logger.Warn("knowledge_import.parse_failed",
			zap.Int64("user_id", task.UserID),
			zap.Int64("kb_id", task.KnowledgeBaseID),
			zap.String("filename", task.Filename),
			zap.Int("size_bytes", len(task.Data)),
			zap.Int64("parse_ms", time.Since(parseStart).Milliseconds()),
			zap.Error(parseErr),
		)
		pushKnowledgeImportFailureNotification(task, parseErr.Error())
		return
	}
	logger.Info("knowledge_import.parse_ok",
		zap.Int64("user_id", task.UserID),
		zap.Int64("kb_id", task.KnowledgeBaseID),
		zap.String("filename", task.Filename),
		zap.Int("size_bytes", len(task.Data)),
		zap.String("file_type", parsed.FileType),
		zap.String("source_type", parsed.SourceType),
		zap.Int("image_count", parsed.ImageCount),
		zap.Bool("used_vision", parsed.UsedVision),
		zap.Int64("parse_ms", time.Since(parseStart).Milliseconds()),
		zap.Int("text_chars", len([]rune(parsed.Content))),
	)

	title := task.Title
	if title == "" {
		title = parsed.Title
	}
	client, err := rpc.RAGClient()
	if err != nil {
		pushKnowledgeImportFailureNotification(task, err.Error())
		return
	}
	ragCtx, cancelRAG := context.WithTimeout(context.Background(), knowledgeImportRAGAddTimeout)
	resp, err := client.AddKnowledgeDocumentText(ragCtx, &ragpb.AddKnowledgeDocumentTextRequest{
		OperatorId:      task.UserID,
		KnowledgeBaseId: task.KnowledgeBaseID,
		Title:           title,
		SourceType:      parsed.SourceType,
		Content:         marshalDocumentImportPayload(parsed),
	})
	cancelRAG()
	if err != nil {
		logger.Warn("knowledge_import.rag_add_failed",
			zap.Int64("user_id", task.UserID),
			zap.Int64("kb_id", task.KnowledgeBaseID),
			zap.String("filename", task.Filename),
			zap.String("file_type", parsed.FileType),
			zap.Error(err),
		)
		pushKnowledgeImportFailureNotification(task, presentableMessage(err.Error()))
		return
	}
	if resp == nil || resp.Document == nil {
		logger.Warn("knowledge_import.rag_add_empty_response",
			zap.Int64("user_id", task.UserID),
			zap.Int64("kb_id", task.KnowledgeBaseID),
			zap.String("filename", task.Filename),
		)
		pushKnowledgeImportFailureNotification(task, "document creation failed")
		return
	}
	logger.Info("knowledge_import.rag_submitted",
		zap.Int64("user_id", task.UserID),
		zap.Int64("kb_id", task.KnowledgeBaseID),
		zap.String("filename", task.Filename),
		zap.String("file_type", parsed.FileType),
		zap.Int64("document_id", resp.Document.DocumentId),
		zap.String("status", resp.Document.Status),
		zap.Int64("total_ms", time.Since(task.StartedAt).Milliseconds()),
	)
	watchKnowledgeDocumentImport(task.UserID, task.KnowledgeBaseID, task.KnowledgeBaseName, resp.Document.DocumentId, resp.Document.Title, task.StartedAt)
}

func marshalDocumentImportPayload(parsed *knowledgeimport.ParsedDocument) string {
	if parsed == nil {
		return ""
	}
	type payloadChunk struct {
		Index        int    `json:"index"`
		ChunkType    string `json:"chunkType,omitempty"`
		SectionTitle string `json:"sectionTitle,omitempty"`
		Content      string `json:"content"`
	}
	type payload struct {
		Version int            `json:"version"`
		Content string         `json:"content"`
		Chunks  []payloadChunk `json:"chunks,omitempty"`
	}

	body := payload{
		Version: 1,
		Content: parsed.Content,
	}
	if len(parsed.Chunks) > 0 {
		body.Chunks = make([]payloadChunk, 0, len(parsed.Chunks))
		for _, item := range parsed.Chunks {
			content := strings.TrimSpace(item.Content)
			if content == "" {
				continue
			}
			body.Chunks = append(body.Chunks, payloadChunk{
				Index:        item.Index,
				ChunkType:    strings.TrimSpace(item.ChunkType),
				SectionTitle: strings.TrimSpace(item.SectionTitle),
				Content:      content,
			})
		}
	}
	data, err := json.Marshal(body)
	if err != nil {
		return parsed.Content
	}
	return string(data)
}

func ListKnowledgeDocuments(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	kbIDValue := strings.TrimSpace(ctx.Param("knowledgeBaseId"))
	kbID, err := strconv.ParseInt(kbIDValue, 10, 64)
	if err != nil || kbID <= 0 {
		writeError(ctx, 400, "invalid knowledgeBaseId")
		return
	}

	client, err := rpc.RAGClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}
	resp, err := client.ListKnowledgeDocuments(ctx.Request.Context(), &ragpb.ListKnowledgeDocumentsRequest{
		OperatorId:      authCtx.UserID,
		KnowledgeBaseId: kbID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	docs := make([]model.KnowledgeDocumentInfo, 0, len(resp.Documents))
	for _, item := range resp.Documents {
		docs = append(docs, model.KnowledgeDocumentInfo{
			DocumentID:      item.DocumentId,
			KnowledgeBaseID: item.KnowledgeBaseId,
			Title:           item.Title,
			SourceType:      item.SourceType,
			Status:          item.Status,
			ErrorMessage:    item.ErrorMessage,
			CreatedAt:       item.CreatedAt,
		})
	}
	writeJSON(ctx, 200, model.APIResponse{
		Code:    0,
		Message: "success",
		Data:    docs,
	})
}

func knowledgeBaseDisplayName(ctx context.Context, client ragservice.Client, userID, knowledgeBaseID int64) string {
	if client == nil {
		return fmt.Sprintf("知识库 %d", knowledgeBaseID)
	}
	resp, err := client.ListKnowledgeBases(ctx, &ragpb.ListKnowledgeBasesRequest{
		OperatorId: userID,
	})
	if err != nil || resp == nil {
		return fmt.Sprintf("知识库 %d", knowledgeBaseID)
	}
	for _, item := range resp.KnowledgeBases {
		if item != nil && item.KnowledgeBaseId == knowledgeBaseID {
			name := strings.TrimSpace(item.Name)
			if name != "" {
				return name
			}
			break
		}
	}
	return fmt.Sprintf("知识库 %d", knowledgeBaseID)
}

func watchKnowledgeDocumentImport(userID, knowledgeBaseID int64, knowledgeBaseName string, documentID int64, documentTitle string, startedAt time.Time) {
	if userID <= 0 || knowledgeBaseID <= 0 || documentID <= 0 {
		return
	}

	go func() {
		timeout := time.NewTimer(knowledgeImportWatchTimeout)
		defer timeout.Stop()
		ticker := time.NewTicker(knowledgeImportPollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-timeout.C:
				pushKnowledgeImportNotificationV2(userID, knowledgeBaseName, model.KnowledgeDocumentInfo{
					DocumentID:      documentID,
					KnowledgeBaseID: knowledgeBaseID,
					Title:           documentTitle,
					Status:          "FAILED",
					ErrorMessage:    "processing timeout (>5m), circuit-break degraded",
				}, startedAt)
				return
			case <-ticker.C:
				client, err := rpc.RAGClient()
				if err != nil {
					continue
				}
				ctx, cancel := context.WithTimeout(context.Background(), knowledgeImportPollRPCDeadline)
				doc, ok := findKnowledgeDocument(ctx, client, userID, knowledgeBaseID, documentID)
				cancel()
				if !ok {
					continue
				}
				switch strings.ToUpper(strings.TrimSpace(doc.Status)) {
				case "READY", "FAILED":
					pushKnowledgeImportNotificationV2(userID, knowledgeBaseName, doc, startedAt)
					return
				}
			}
		}
	}()
}

func findKnowledgeDocument(ctx context.Context, client ragservice.Client, userID, knowledgeBaseID int64, documentID int64) (model.KnowledgeDocumentInfo, bool) {
	resp, err := client.ListKnowledgeDocuments(ctx, &ragpb.ListKnowledgeDocumentsRequest{
		OperatorId:      userID,
		KnowledgeBaseId: knowledgeBaseID,
	})
	if err != nil || resp == nil {
		return model.KnowledgeDocumentInfo{}, false
	}
	for _, item := range resp.Documents {
		if item == nil || item.DocumentId != documentID {
			continue
		}
		return model.KnowledgeDocumentInfo{
			DocumentID:      item.DocumentId,
			KnowledgeBaseID: item.KnowledgeBaseId,
			Title:           item.Title,
			SourceType:      item.SourceType,
			Status:          item.Status,
			ErrorMessage:    item.ErrorMessage,
			CreatedAt:       item.CreatedAt,
		}, true
	}
	return model.KnowledgeDocumentInfo{}, false
}

func pushKnowledgeImportNotification(userID int64, knowledgeBaseName string, doc model.KnowledgeDocumentInfo, startedAt time.Time) {
	status := strings.ToUpper(strings.TrimSpace(doc.Status))
	title := "知识库文件导入完成"
	if status == "FAILED" {
		title = "知识库文件导入失败"
	}

	docTitle := strings.TrimSpace(doc.Title)
	if docTitle == "" {
		docTitle = fmt.Sprintf("文档 %d", doc.DocumentID)
	}
	kbName := strings.TrimSpace(knowledgeBaseName)
	if kbName == "" {
		kbName = fmt.Sprintf("知识库 %d", doc.KnowledgeBaseID)
	}
	elapsed := formatKnowledgeImportDuration(time.Since(startedAt))

	content := fmt.Sprintf("知识库「%s」的文件「%s」导入成功，用时 %s。", kbName, docTitle, elapsed)
	if status == "FAILED" {
		reason := strings.TrimSpace(doc.ErrorMessage)
		if reason == "" {
			reason = "未返回具体错误"
		}
		content = fmt.Sprintf("知识库「%s」的文件「%s」导入失败，用时 %s。原因：%s", kbName, docTitle, elapsed, reason)
	}

	chatClient, err := rpc.ChatClient()
	if err == nil && chatClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), knowledgeImportNotifyRPCDeadline)
		defer cancel()
		resp, createErr := chatClient.CreateNotification(ctx, &chatpb.CreateNotificationRequest{
			OperatorId: userID,
			UserId:     userID,
			Type:       "KNOWLEDGE_IMPORT",
			Title:      title,
			Content:    content,
		})
		if createErr == nil && resp != nil && resp.Notification != nil {
			chatHub.SendToUsers([]int64{userID}, gatewayws.OutgoingEvent{
				Type: gatewayws.EventNotificationCreated,
				Data: gatewayws.NotificationCreatedData{
					Notification: gatewayws.ToNotificationInfo(resp.Notification),
					UnreadCount:  &resp.UnreadCount,
				},
			})
			return
		}
	}

	category, summary, detail := notificationx.Normalize("KNOWLEDGE_IMPORT", title, content)
	chatHub.SendToUsers([]int64{userID}, gatewayws.OutgoingEvent{
		Type: gatewayws.EventNotificationCreated,
		Data: gatewayws.NotificationCreatedData{
			Notification: gatewayws.NotificationInfo{
				ID:         time.Now().UnixMilli()*1000 + doc.DocumentID%1000,
				Type:       "KNOWLEDGE_IMPORT",
				Category:   category,
				Title:      title,
				Summary:    summary,
				Content:    content,
				Detail:     detail,
				IsRead:     false,
				CreatedAt:  time.Now().Unix(),
				Persistent: false,
			},
		},
	})
}

func formatKnowledgeImportDuration(duration time.Duration) string {
	if duration < time.Second {
		return fmt.Sprintf("%dms", duration.Milliseconds())
	}
	totalSeconds := int64(duration.Round(time.Second).Seconds())
	if totalSeconds < 60 {
		return fmt.Sprintf("%d 秒", totalSeconds)
	}
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60
	if seconds == 0 {
		return fmt.Sprintf("%d 分钟", minutes)
	}
	return fmt.Sprintf("%d 分 %d 秒", minutes, seconds)
}

func knowledgeBaseDisplayNameV2(ctx context.Context, client ragservice.Client, userID, knowledgeBaseID int64) string {
	fallback := fmt.Sprintf("知识库 %d", knowledgeBaseID)
	if client == nil {
		return fallback
	}
	resp, err := client.ListKnowledgeBases(ctx, &ragpb.ListKnowledgeBasesRequest{
		OperatorId: userID,
	})
	if err != nil || resp == nil {
		return fallback
	}
	for _, item := range resp.KnowledgeBases {
		if item == nil || item.KnowledgeBaseId != knowledgeBaseID {
			continue
		}
		if name := strings.TrimSpace(item.Name); name != "" {
			return name
		}
		return fallback
	}
	return fallback
}

func knowledgeFileTitle(filename string) string {
	title := strings.TrimSpace(strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename)))
	if title == "" {
		return "导入文件"
	}
	return title
}

func pushKnowledgeImportFailureNotification(task knowledgeDocumentFileImportTask, reason string) {
	docTitle := strings.TrimSpace(task.Title)
	if docTitle == "" {
		docTitle = knowledgeFileTitle(task.Filename)
	}
	pushKnowledgeImportNotificationV2(task.UserID, task.KnowledgeBaseName, model.KnowledgeDocumentInfo{
		KnowledgeBaseID: task.KnowledgeBaseID,
		Title:           docTitle,
		Status:          "FAILED",
		ErrorMessage:    strings.TrimSpace(reason),
	}, task.StartedAt)
}

func pushKnowledgeImportNotificationV2(userID int64, knowledgeBaseName string, doc model.KnowledgeDocumentInfo, startedAt time.Time) {
	status := strings.ToUpper(strings.TrimSpace(doc.Status))
	title := "知识库文件导入完成"
	if status == "FAILED" {
		title = "知识库文件导入失败"
	}

	docTitle := strings.TrimSpace(doc.Title)
	if docTitle == "" {
		docTitle = fmt.Sprintf("文档 %d", doc.DocumentID)
	}
	kbName := strings.TrimSpace(knowledgeBaseName)
	if kbName == "" {
		kbName = fmt.Sprintf("知识库 %d", doc.KnowledgeBaseID)
	}
	elapsed := formatKnowledgeImportDurationV2(time.Since(startedAt))

	content := fmt.Sprintf("知识库「%s」的文件「%s」导入成功，用时 %s。", kbName, docTitle, elapsed)
	if status == "FAILED" {
		reason := strings.TrimSpace(doc.ErrorMessage)
		if reason == "" {
			reason = "未返回具体错误"
		}
		content = fmt.Sprintf("知识库「%s」的文件「%s」导入失败，用时 %s。原因：%s", kbName, docTitle, elapsed, reason)
	}
	pushKnowledgeImportStatusNotification(userID, title, content)
}

func pushKnowledgeImportStatusNotification(userID int64, title string, content string) {
	chatClient, err := rpc.ChatClient()
	if err == nil && chatClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), knowledgeImportNotifyRPCDeadline)
		defer cancel()
		resp, createErr := chatClient.CreateNotification(ctx, &chatpb.CreateNotificationRequest{
			OperatorId: userID,
			UserId:     userID,
			Type:       "KNOWLEDGE_IMPORT",
			Title:      title,
			Content:    content,
		})
		if createErr == nil && resp != nil && resp.Notification != nil {
			chatHub.SendToUsers([]int64{userID}, gatewayws.OutgoingEvent{
				Type: gatewayws.EventNotificationCreated,
				Data: gatewayws.NotificationCreatedData{
					Notification: gatewayws.ToNotificationInfo(resp.Notification),
					UnreadCount:  &resp.UnreadCount,
				},
			})
			return
		}
	}

	category, summary, detail := notificationx.Normalize("KNOWLEDGE_IMPORT", title, content)
	chatHub.SendToUsers([]int64{userID}, gatewayws.OutgoingEvent{
		Type: gatewayws.EventNotificationCreated,
		Data: gatewayws.NotificationCreatedData{
			Notification: gatewayws.NotificationInfo{
				ID:         time.Now().UnixMilli(),
				Type:       "KNOWLEDGE_IMPORT",
				Category:   category,
				Title:      title,
				Summary:    summary,
				Content:    content,
				Detail:     detail,
				IsRead:     false,
				CreatedAt:  time.Now().Unix(),
				Persistent: false,
			},
		},
	})
}

func formatKnowledgeImportDurationV2(duration time.Duration) string {
	if duration < time.Second {
		return fmt.Sprintf("%dms", duration.Milliseconds())
	}
	totalSeconds := int64(duration.Round(time.Second).Seconds())
	if totalSeconds < 60 {
		return fmt.Sprintf("%d 秒", totalSeconds)
	}
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60
	if seconds == 0 {
		return fmt.Sprintf("%d 分钟", minutes)
	}
	return fmt.Sprintf("%d 分 %d 秒", minutes, seconds)
}

func SearchKnowledgeBase(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	kbIDValue := strings.TrimSpace(ctx.Param("knowledgeBaseId"))
	kbID, err := strconv.ParseInt(kbIDValue, 10, 64)
	if err != nil || kbID <= 0 {
		writeError(ctx, 400, "invalid knowledgeBaseId")
		return
	}

	var req model.SearchKnowledgeBaseRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Query) == "" {
		writeError(ctx, 400, "query is required")
		return
	}

	client, err := rpc.RAGClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}
	resp, err := client.SearchKnowledgeBase(ctx.Request.Context(), &ragpb.SearchKnowledgeBaseRequest{
		OperatorId:      authCtx.UserID,
		KnowledgeBaseId: kbID,
		Query:           req.Query,
		TopK:            req.TopK,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	chunks := make([]model.KnowledgeSearchChunkInfo, 0, len(resp.Chunks))
	for _, item := range resp.Chunks {
		chunks = append(chunks, model.KnowledgeSearchChunkInfo{
			ChunkID:    item.ChunkId,
			DocumentID: item.DocumentId,
			Score:      item.Score,
			Content:    item.Content,
		})
	}
	writeJSON(ctx, 200, model.APIResponse{
		Code:    0,
		Message: "success",
		Data:    chunks,
	})
}

func BindConversationKnowledgeBase(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}
	conversationID, ok := conversationIDParam(ctx)
	if !ok {
		return
	}
	var req model.BindConversationKnowledgeBaseRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}
	if req.KnowledgeBaseID <= 0 {
		writeError(ctx, 400, "knowledgeBaseId is required")
		return
	}

	client, err := rpc.RAGClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}
	resp, err := client.BindConversationKnowledgeBase(ctx.Request.Context(), &ragpb.BindConversationKnowledgeBaseRequest{
		OperatorId:      authCtx.UserID,
		ConversationId:  conversationID,
		KnowledgeBaseId: req.KnowledgeBaseID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if !resp.Success {
		writeError(ctx, statusFromMessage(resp.Message), presentableMessage(resp.Message))
		return
	}
	writeJSON(ctx, 200, model.APIResponse{Code: 0, Message: "success"})
}

func ListConversationKnowledgeBases(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}
	conversationID, ok := conversationIDParam(ctx)
	if !ok {
		return
	}

	client, err := rpc.RAGClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}
	resp, err := client.ListConversationKnowledgeBases(ctx.Request.Context(), &ragpb.ListConversationKnowledgeBasesRequest{
		OperatorId:     authCtx.UserID,
		ConversationId: conversationID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	items := make([]model.ConversationKnowledgeBaseInfo, 0, len(resp.KnowledgeBases))
	for _, item := range resp.KnowledgeBases {
		items = append(items, model.ConversationKnowledgeBaseInfo{
			ID:              item.Id,
			ConversationID:  item.ConversationId,
			KnowledgeBaseID: item.KnowledgeBaseId,
			Name:            item.Name,
			Description:     item.Description,
			Status:          item.Status,
			Enabled:         item.Enabled,
		})
	}
	writeJSON(ctx, 200, model.APIResponse{Code: 0, Message: "success", Data: items})
}

func UnbindConversationKnowledgeBase(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}
	conversationID, ok := conversationIDParam(ctx)
	if !ok {
		return
	}
	kbIDValue := strings.TrimSpace(ctx.Param("knowledgeBaseId"))
	kbID, err := strconv.ParseInt(kbIDValue, 10, 64)
	if err != nil || kbID <= 0 {
		writeError(ctx, 400, "invalid knowledgeBaseId")
		return
	}

	client, err := rpc.RAGClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}
	resp, err := client.UnbindConversationKnowledgeBase(ctx.Request.Context(), &ragpb.UnbindConversationKnowledgeBaseRequest{
		OperatorId:      authCtx.UserID,
		ConversationId:  conversationID,
		KnowledgeBaseId: kbID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if !resp.Success {
		writeError(ctx, statusFromMessage(resp.Message), presentableMessage(resp.Message))
		return
	}
	writeJSON(ctx, 200, model.APIResponse{Code: 0, Message: "success"})
}

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

func toGroupModel(group *chatpb.GroupInfo) model.GroupInfo {
	if group == nil {
		return model.GroupInfo{}
	}
	info := model.GroupInfo{
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

func toConversationModel(conversation *chatpb.ConversationInfo) model.ConversationInfo {
	if conversation == nil {
		return model.ConversationInfo{}
	}
	return model.ConversationInfo{
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

func toNotificationModel(item *chatpb.NotificationInfo) model.NotificationInfo {
	if item == nil {
		return model.NotificationInfo{}
	}
	category, summary, detail := notificationx.Normalize(item.Type, item.Title, item.Content)
	return model.NotificationInfo{
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

func toMessageModel(message *chatpb.MessageInfo) model.MessageInfo {
	if message == nil {
		return model.MessageInfo{}
	}
	var replyTo *model.ReplyPreviewInfo
	if message.ReplyTo != nil {
		replyTo = &model.ReplyPreviewInfo{
			MessageID:      message.ReplyTo.MessageId,
			SenderID:       message.ReplyTo.SenderId,
			SenderType:     message.ReplyTo.SenderType,
			MessageType:    message.ReplyTo.MessageType,
			ContentPreview: message.ReplyTo.ContentPreview,
		}
	}
	return model.MessageInfo{
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

func toBotModel(item *chatpb.BotInfo) model.BotInfo {
	if item == nil {
		return model.BotInfo{}
	}
	return model.BotInfo{
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

func toAICallLogModel(item *chatpb.AICallLogInfo) model.AICallLogInfo {
	if item == nil {
		return model.AICallLogInfo{}
	}
	return model.AICallLogInfo{
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

func broadcastConversationEvent(ctx context.Context, client chatservice.Client, resp *chatpb.ConversationEventResponse) {
	if resp == nil || resp.EventMessage == nil || len(resp.RecipientUserIds) == 0 {
		return
	}
	chatHub.SendToUsers(resp.RecipientUserIds, gatewayws.OutgoingEvent{
		Type: gatewayws.EventNewMessage,
		Data: gatewayws.ToMessageInfo(resp.EventMessage),
	})
	broadcastEventNotifications(ctx, client, resp)
}

func broadcastEventNotifications(ctx context.Context, client chatservice.Client, resp *chatpb.ConversationEventResponse) {
	if client == nil || resp == nil || resp.EventMessage == nil || len(resp.RecipientUserIds) == 0 {
		return
	}

	limit := int32(5)
	unreadOnly := true
	relatedMessageID := resp.EventMessage.Id
	for _, userID := range resp.RecipientUserIds {
		notificationResp, err := client.ListNotifications(ctx, &chatpb.ListNotificationsRequest{
			OperatorId: userID,
			UnreadOnly: &unreadOnly,
			Limit:      &limit,
		})
		if err != nil || notificationResp == nil {
			continue
		}
		for _, item := range notificationResp.Notifications {
			if item == nil || item.RelatedMessageId == nil || *item.RelatedMessageId != relatedMessageID {
				continue
			}
			chatHub.SendToUsers([]int64{userID}, gatewayws.OutgoingEvent{
				Type: gatewayws.EventNotificationCreated,
				Data: gatewayws.NotificationCreatedData{
					Notification: gatewayws.ToNotificationInfo(item),
					UnreadCount:  &notificationResp.UnreadCount,
				},
			})
			break
		}
	}
}

func broadcastMessageRecalledEvent(resp *chatpb.MessageRecalledEventResponse) {
	if resp == nil || resp.Event == nil || len(resp.RecipientUserIds) == 0 {
		return
	}
	chatHub.SendToUsers(resp.RecipientUserIds, gatewayws.OutgoingEvent{
		Type: gatewayws.EventMessageRecalled,
		Data: gatewayws.MessageRecalledInfo{
			MessageID:      resp.Event.MessageId,
			ConversationID: resp.Event.ConversationId,
		},
	})
}

func setGroupMuteAll(ctx *gin.Context, muteAll bool) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	conversationID, ok := conversationIDParam(ctx)
	if !ok {
		return
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.SetGroupMuteAll(ctx.Request.Context(), &chatpb.SetGroupMuteAllRequest{
		OperatorId:     authCtx.UserID,
		ConversationId: conversationID,
		MuteAll:        muteAll,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if !resp.Success {
		writeError(ctx, statusFromMessage(resp.Message), presentableMessage(resp.Message))
		return
	}
	broadcastConversationEvent(ctx.Request.Context(), client, resp)

	writeJSON(ctx, 200, model.APIResponse{Code: 0, Message: "success"})
}
