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

	messages := make([]dto.MessageInfo, 0, len(resp.Messages))
	for _, message := range resp.Messages {
		messages = append(messages, toMessageModel(message))
	}

	writeJSON(ctx, 200, dto.APIResponse{
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

	var req dto.MarkConversationReadRequest
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

	writeJSON(ctx, 200, dto.APIResponse{Code: 0, Message: "success"})
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
	writeJSON(ctx, 200, dto.APIResponse{Code: 0, Message: "success"})
}
