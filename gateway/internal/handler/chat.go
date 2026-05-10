package handler

import (
	"strconv"
	"strings"

	"example.com/aim/gateway/internal/middleware"
	"example.com/aim/gateway/internal/model"
	"example.com/aim/gateway/internal/rpc"
	gatewayws "example.com/aim/gateway/internal/websocket"
	chatpb "example.com/aim/gateway/kitex_gen/chat"
	"github.com/gin-gonic/gin"
)

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
	broadcastConversationEvent(resp)

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
	broadcastConversationEvent(resp)

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
	broadcastConversationEvent(resp)

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
	broadcastConversationEvent(resp)

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
	broadcastConversationEvent(resp)

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
	broadcastConversationEvent(resp)

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
	broadcastConversationEvent(resp)

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
	broadcastConversationEvent(resp)

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
	broadcastConversationEvent(resp)

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
	broadcastConversationEvent(resp)

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

func broadcastConversationEvent(resp *chatpb.ConversationEventResponse) {
	if resp == nil || resp.EventMessage == nil || len(resp.RecipientUserIds) == 0 {
		return
	}
	chatHub.SendToUsers(resp.RecipientUserIds, gatewayws.OutgoingEvent{
		Type: gatewayws.EventNewMessage,
		Data: gatewayws.ToMessageInfo(resp.EventMessage),
	})
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
	broadcastConversationEvent(resp)

	writeJSON(ctx, 200, model.APIResponse{Code: 0, Message: "success"})
}
