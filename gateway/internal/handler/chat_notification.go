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

	items := make([]dto.NotificationInfo, 0, len(resp.Notifications))
	for _, item := range resp.Notifications {
		items = append(items, toNotificationModel(item))
	}
	writeJSON(ctx, 200, dto.APIResponse{
		Code:    0,
		Message: "success",
		Data: dto.NotificationListResponse{
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
	writeJSON(ctx, 200, dto.APIResponse{Code: 0, Message: "success"})
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
	writeJSON(ctx, 200, dto.APIResponse{Code: 0, Message: "success"})
}
