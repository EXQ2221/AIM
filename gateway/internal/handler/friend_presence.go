package handler

import (
	"example.com/aim/gateway/internal/middleware"
	"example.com/aim/gateway/internal/model/dto"
	"example.com/aim/gateway/internal/presence"
	"github.com/gin-gonic/gin"
)

type PresenceSettingsResponse struct {
	Invisible bool `json:"invisible"`
}

type UpdatePresenceSettingsRequest struct {
	Invisible bool `json:"invisible"`
}

func GetPresenceSettings(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	invisible, err := presence.DefaultStore().GetInvisible(ctx.Request.Context(), authCtx.UserID)
	if err != nil {
		writeError(ctx, 500, "failed to load presence settings")
		return
	}

	writeJSON(ctx, 200, dto.APIResponse{
		Code:    0,
		Message: "success",
		Data: PresenceSettingsResponse{
			Invisible: invisible,
		},
	})
}

func UpdatePresenceSettings(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	var req UpdatePresenceSettingsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}

	if err := presence.DefaultStore().SetInvisible(ctx.Request.Context(), authCtx.UserID, req.Invisible); err != nil {
		writeError(ctx, 500, "failed to save presence settings")
		return
	}

	if !req.Invisible && chatHub.IsUserOnline(authCtx.UserID) {
		chatHub.BroadcastFriendPresence(authCtx.UserID, "ONLINE")
	} else {
		chatHub.BroadcastFriendPresence(authCtx.UserID, "OFFLINE")
	}

	writeJSON(ctx, 200, dto.APIResponse{
		Code:    0,
		Message: "success",
		Data: PresenceSettingsResponse{
			Invisible: req.Invisible,
		},
	})
}
