package handler

import (
	"strconv"
	"strings"

	"example.com/aim/gateway/internal/middleware"
	"example.com/aim/gateway/internal/model/dto"
	"example.com/aim/gateway/internal/presence"
	"example.com/aim/gateway/internal/rpc"
	userpb "example.com/aim/gateway/kitex_gen/user"
	"github.com/gin-gonic/gin"
)

func Me(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	client, err := rpc.UserClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	userResp, err := client.GetUser(ctx.Request.Context(), &userpb.GetUserRequest{
		UserId: authCtx.UserID,
	})
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	writeJSON(ctx, 200, dto.APIResponse{
		Code:    0,
		Message: "success",
		Data: dto.UserInfo{
			UserID:       userResp.User.UserId,
			AimID:        userResp.User.AimId,
			Email:        userResp.User.Email,
			Nickname:     userResp.User.Nickname,
			Avatar:       userResp.User.Avatar,
			Status:       userResp.User.Status,
			Role:         userResp.User.Role,
			TokenVersion: userResp.User.TokenVersion,
			CreatedAt:    userResp.User.CreatedAt,
			UpdatedAt:    userResp.User.UpdatedAt,
		},
	})
}

func CreateFriendGroup(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	var req dto.CreateFriendGroupRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(ctx, 400, "name is required")
		return
	}

	client, err := rpc.UserClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.CreateFriendGroup(ctx.Request.Context(), &userpb.CreateFriendGroupRequest{
		UserId: authCtx.UserID,
		Name:   req.Name,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	writeJSON(ctx, 200, dto.APIResponse{
		Code:    0,
		Message: "success",
		Data:    toFriendGroupModel(resp.Group),
	})
}

func ListFriendGroups(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	client, err := rpc.UserClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.ListFriendGroups(ctx.Request.Context(), &userpb.ListFriendGroupsRequest{
		UserId: authCtx.UserID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	groups := make([]dto.FriendGroupInfo, 0, len(resp.Groups))
	for _, group := range resp.Groups {
		groups = append(groups, toFriendGroupModel(group))
	}

	writeJSON(ctx, 200, dto.APIResponse{
		Code:    0,
		Message: "success",
		Data:    groups,
	})
}

func AddFriend(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	var req dto.AddFriendRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}
	if strings.TrimSpace(req.TargetAimID) == "" {
		writeError(ctx, 400, "target_aim_id is required")
		return
	}

	client, err := rpc.UserClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.AddFriend(ctx.Request.Context(), &userpb.AddFriendRequest{
		UserId:      authCtx.UserID,
		TargetAimId: req.TargetAimID,
		Remark:      req.Remark,
		GroupId:     req.GroupID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if resp == nil || resp.Request == nil {
		writeError(ctx, 500, "friend request response is empty")
		return
	}

	writeJSON(ctx, 200, dto.APIResponse{
		Code:    0,
		Message: "success",
		Data:    toFriendRequestModel(resp.Request),
	})
}

func ListFriends(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	client, err := rpc.UserClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.ListFriends(ctx.Request.Context(), &userpb.ListFriendsRequest{
		UserId: authCtx.UserID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	friends := make([]dto.FriendInfo, 0, len(resp.Friends))
	for _, friend := range resp.Friends {
		model := toFriendModel(friend)
		invisible, err := presence.DefaultStore().GetInvisible(ctx.Request.Context(), model.UserID)
		if err == nil && !invisible && chatHub.IsUserOnline(model.UserID) {
			model.IsOnline = true
			model.Presence = "ONLINE"
		} else {
			model.IsOnline = false
			model.Presence = "OFFLINE"
		}
		friends = append(friends, model)
	}

	writeJSON(ctx, 200, dto.APIResponse{
		Code:    0,
		Message: "success",
		Data:    friends,
	})
}

func UpdateFriend(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	friendUserID, ok := friendUserIDParam(ctx)
	if !ok {
		return
	}

	var req dto.UpdateFriendRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}

	client, err := rpc.UserClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.UpdateFriend(ctx.Request.Context(), &userpb.UpdateFriendRequest{
		UserId:       authCtx.UserID,
		FriendUserId: friendUserID,
		Remark:       req.Remark,
		GroupId:      req.GroupID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	writeJSON(ctx, 200, dto.APIResponse{
		Code:    0,
		Message: "success",
		Data:    toFriendModel(resp.Friend),
	})
}

func DeleteFriend(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	friendUserID, ok := friendUserIDParam(ctx)
	if !ok {
		return
	}

	client, err := rpc.UserClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.DeleteFriend(ctx.Request.Context(), &userpb.DeleteFriendRequest{
		UserId:       authCtx.UserID,
		FriendUserId: friendUserID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if !resp.Success {
		writeError(ctx, statusFromMessage(resp.Message), presentableMessage(resp.Message))
		return
	}

	writeJSON(ctx, 200, dto.APIResponse{Code: 0, Message: "success"})
}

func ListFriendRequests(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	client, err := rpc.UserClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.ListFriendRequests(ctx.Request.Context(), &userpb.ListFriendRequestsRequest{
		UserId: authCtx.UserID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	requests := make([]dto.FriendRequestInfo, 0, len(resp.Requests))
	for _, request := range resp.Requests {
		requests = append(requests, toFriendRequestModel(request))
	}

	writeJSON(ctx, 200, dto.APIResponse{
		Code:    0,
		Message: "success",
		Data:    requests,
	})
}

func RespondFriendRequest(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	requestID, ok := requestIDParam(ctx)
	if !ok {
		return
	}

	var req dto.RespondFriendRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}
	req.Action = strings.ToUpper(strings.TrimSpace(req.Action))
	if req.Action == "" {
		writeError(ctx, 400, "action is required")
		return
	}

	client, err := rpc.UserClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.RespondFriendRequest(ctx.Request.Context(), &userpb.RespondFriendRequestRequest{
		UserId:    authCtx.UserID,
		RequestId: requestID,
		Action:    req.Action,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if resp == nil || resp.Request == nil {
		writeError(ctx, 500, "friend request response is empty")
		return
	}

	writeJSON(ctx, 200, dto.APIResponse{
		Code:    0,
		Message: "success",
		Data: map[string]any{
			"request": toFriendRequestModel(resp.Request),
			"friend":  toFriendModel(resp.Friend),
		},
	})
}

func friendUserIDParam(ctx *gin.Context) (int64, bool) {
	value := strings.TrimSpace(ctx.Param("friendUserId"))
	if value == "" {
		writeError(ctx, 400, "invalid friendUserId")
		return 0, false
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		writeError(ctx, 400, "invalid friendUserId")
		return 0, false
	}
	return parsed, true
}

func requestIDParam(ctx *gin.Context) (int64, bool) {
	value := strings.TrimSpace(ctx.Param("requestId"))
	if value == "" {
		writeError(ctx, 400, "invalid requestId")
		return 0, false
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		writeError(ctx, 400, "invalid requestId")
		return 0, false
	}
	return parsed, true
}

func toFriendGroupModel(group *userpb.FriendGroupInfo) dto.FriendGroupInfo {
	if group == nil {
		return dto.FriendGroupInfo{}
	}
	return dto.FriendGroupInfo{
		ID:        group.Id,
		Name:      group.Name,
		SortOrder: group.SortOrder,
		CreatedAt: group.CreatedAt,
		UpdatedAt: group.UpdatedAt,
	}
}

func toFriendModel(friend *userpb.FriendInfo) dto.FriendInfo {
	if friend == nil {
		return dto.FriendInfo{}
	}
	return dto.FriendInfo{
		UserID:    friend.UserId,
		AimID:     friend.AimId,
		Nickname:  friend.Nickname,
		Avatar:    friend.Avatar,
		Remark:    friend.Remark,
		GroupID:   friend.GroupId,
		Status:    friend.Status,
		IsOnline:  false,
		Presence:  "OFFLINE",
		CreatedAt: friend.CreatedAt,
		UpdatedAt: friend.UpdatedAt,
	}
}

func toFriendRequestModel(request *userpb.FriendRequestInfo) dto.FriendRequestInfo {
	if request == nil {
		return dto.FriendRequestInfo{}
	}
	return dto.FriendRequestInfo{
		ID:        request.Id,
		UserID:    request.UserId,
		AimID:     request.AimId,
		Nickname:  request.Nickname,
		Avatar:    request.Avatar,
		Direction: request.Direction,
		Status:    request.Status,
		Remark:    request.Remark,
		GroupID:   request.GroupId,
		CreatedAt: request.CreatedAt,
		UpdatedAt: request.UpdatedAt,
	}
}
