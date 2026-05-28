package handler

import (
	"strconv"
	"strings"

	"example.com/aim/gateway/internal/middleware"
	"example.com/aim/gateway/internal/model/dto"
	"example.com/aim/gateway/internal/rpc"
	chatpb "example.com/aim/gateway/kitex_gen/chat"
	"github.com/gin-gonic/gin"
)

func CreateGroup(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	var req dto.CreateGroupRequest
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

	writeJSON(ctx, 200, dto.APIResponse{
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

	writeJSON(ctx, 200, dto.APIResponse{
		Code:    0,
		Message: "success",
		Data:    toGroupModel(resp.Group),
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

	writeJSON(ctx, 200, dto.APIResponse{
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
	message := strings.TrimSpace(resp.Message)
	if message == "" {
		message = "ok"
	}
	writeJSON(ctx, 200, dto.APIResponse{
		Code:    0,
		Message: "success",
		Data: dto.JoinGroupResponse{
			Message: message,
			Pending: strings.Contains(message, "等待管理员审核") || strings.Contains(message, "等待审核"),
		},
	})
}

func ListGroupJoinRequests(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	conversationID, ok := conversationIDParam(ctx)
	if !ok {
		return
	}

	limit := int32(50)
	if raw := strings.TrimSpace(ctx.Query("limit")); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 32)
		if err != nil || value <= 0 {
			writeError(ctx, 400, "invalid limit")
			return
		}
		limit = int32(value)
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.ListGroupJoinRequests(ctx.Request.Context(), &chatpb.ListGroupJoinRequestsRequest{
		OperatorId:     authCtx.UserID,
		ConversationId: conversationID,
		Limit:          &limit,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	items := make([]dto.GroupJoinRequestInfo, 0, len(resp.Requests))
	for _, item := range resp.Requests {
		items = append(items, toGroupJoinRequestModel(item))
	}
	writeJSON(ctx, 200, dto.APIResponse{Code: 0, Message: "success", Data: items})
}

func ReviewGroupJoinRequest(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	conversationID, ok := conversationIDParam(ctx)
	if !ok {
		return
	}

	requestIDRaw := strings.TrimSpace(ctx.Param("requestId"))
	requestID, err := strconv.ParseInt(requestIDRaw, 10, 64)
	if err != nil || requestID <= 0 {
		writeError(ctx, 400, "invalid requestId")
		return
	}

	var req dto.ReviewGroupJoinRequestRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}

	action := strings.ToUpper(strings.TrimSpace(req.Action))
	if action != "APPROVE" && action != "REJECT" {
		writeError(ctx, 400, "action must be APPROVE or REJECT")
		return
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.ReviewGroupJoinRequest(ctx.Request.Context(), &chatpb.ReviewGroupJoinRequestRequest{
		OperatorId:     authCtx.UserID,
		ConversationId: conversationID,
		RequestId:      requestID,
		Action:         action,
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

	message := strings.TrimSpace(resp.Message)
	if message == "" {
		message = "ok"
	}
	writeJSON(ctx, 200, dto.APIResponse{Code: 0, Message: "success", Data: map[string]any{"message": message}})
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

	var req dto.InviteMemberRequest
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

	writeJSON(ctx, 200, dto.APIResponse{Code: 0, Message: "success"})
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

	writeJSON(ctx, 200, dto.APIResponse{Code: 0, Message: "success"})
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

	var req dto.TransferOwnerRequest
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

	writeJSON(ctx, 200, dto.APIResponse{Code: 0, Message: "success"})
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

	var req dto.SetAdminRequest
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

	writeJSON(ctx, 200, dto.APIResponse{Code: 0, Message: "success"})
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

	writeJSON(ctx, 200, dto.APIResponse{Code: 0, Message: "success"})
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

	var req dto.MuteMemberRequest
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

	writeJSON(ctx, 200, dto.APIResponse{Code: 0, Message: "success"})
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

	writeJSON(ctx, 200, dto.APIResponse{Code: 0, Message: "success"})
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

	writeJSON(ctx, 200, dto.APIResponse{Code: 0, Message: "success"})
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

	var req dto.UpdateGroupAnnouncementRequest
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

	writeJSON(ctx, 200, dto.APIResponse{Code: 0, Message: "success"})
}

func UpdateGroupAvatar(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	conversationID, ok := conversationIDParam(ctx)
	if !ok {
		return
	}

	var req dto.UpdateGroupAvatarRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.UpdateGroupAvatar(ctx.Request.Context(), &chatpb.UpdateGroupAvatarRequest{
		OperatorId:     authCtx.UserID,
		ConversationId: conversationID,
		Avatar:         req.Avatar,
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

	writeJSON(ctx, 200, dto.APIResponse{Code: 0, Message: "success"})
}

func DisbandGroup(ctx *gin.Context) {
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

	resp, err := client.DisbandGroup(ctx.Request.Context(), &chatpb.DisbandGroupRequest{
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

	writeJSON(ctx, 200, dto.APIResponse{Code: 0, Message: "success"})
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

	members := make([]dto.MemberInfo, 0, len(resp.Members))
	for _, member := range resp.Members {
		info := dto.MemberInfo{
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

	writeJSON(ctx, 200, dto.APIResponse{
		Code:    0,
		Message: "success",
		Data:    members,
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

	writeJSON(ctx, 200, dto.APIResponse{Code: 0, Message: "success"})
}
