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

	bots := make([]dto.BotInfo, 0, len(resp.Bots))
	for _, item := range resp.Bots {
		bots = append(bots, toBotModel(item))
	}
	writeJSON(ctx, 200, dto.APIResponse{Code: 0, Message: "success", Data: bots})
}

func ListCustomBots(ctx *gin.Context) {
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

	resp, err := client.ListCustomBots(ctx.Request.Context(), &chatpb.ListCustomBotsRequest{
		OperatorId: authCtx.UserID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	bots := make([]dto.BotInfo, 0, len(resp.Bots))
	for _, item := range resp.Bots {
		bots = append(bots, toBotModel(item))
	}
	writeJSON(ctx, 200, dto.APIResponse{Code: 0, Message: "success", Data: bots})
}

func CreateCustomBot(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}
	var req dto.CreateCustomBotRequest
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
	writeJSON(ctx, 200, dto.APIResponse{Code: 0, Message: "success", Data: toBotModel(resp.Bot)})
}

func UpdateCustomBot(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}
	botIDValue := strings.TrimSpace(ctx.Param("botId"))
	botID, err := strconv.ParseInt(botIDValue, 10, 64)
	if err != nil || botID <= 0 {
		writeError(ctx, 400, "invalid botId")
		return
	}
	var req dto.UpdateCustomBotRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.UpdateCustomBot(ctx.Request.Context(), &chatpb.UpdateCustomBotRequest{
		OperatorId:      authCtx.UserID,
		BotId:           botID,
		Name:            req.Name,
		MentionName:     req.MentionName,
		Aliases:         req.Aliases,
		Description:     req.Description,
		ApiBaseUrl:      req.APIBaseURL,
		ApiKey:          req.APIKey,
		ModelName:       req.ModelName,
		SupportedModels: req.SupportedModels,
		SystemPrompt:    req.SystemPrompt,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	writeJSON(ctx, 200, dto.APIResponse{Code: 0, Message: "success", Data: toBotModel(resp.Bot)})
}

func DeleteCustomBot(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
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
	resp, err := client.DeleteCustomBot(ctx.Request.Context(), &chatpb.DeleteCustomBotRequest{
		OperatorId: authCtx.UserID,
		BotId:      botID,
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

	bots := make([]dto.BotInfo, 0, len(resp.Bots))
	for _, item := range resp.Bots {
		bots = append(bots, toBotModel(item))
	}
	writeJSON(ctx, 200, dto.APIResponse{Code: 0, Message: "success", Data: bots})
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

	var req dto.AddConversationBotRequest
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
	writeJSON(ctx, 200, dto.APIResponse{Code: 0, Message: "success", Data: toBotModel(resp.Bot)})
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
	writeJSON(ctx, 200, dto.APIResponse{Code: 0, Message: "success"})
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

	logs := make([]dto.AICallLogInfo, 0, len(resp.Logs))
	for _, item := range resp.Logs {
		logs = append(logs, toAICallLogModel(item))
	}
	quota := dto.AICallLogQuotaInfo{}
	if resp.Quota != nil {
		quota = dto.AICallLogQuotaInfo{
			DailyTotalTokens: resp.Quota.DailyTotalTokens,
			DailyTokenLimit:  resp.Quota.DailyTokenLimit,
			RemainingTokens:  resp.Quota.RemainingTokens,
		}
	}
	writeJSON(ctx, 200, dto.APIResponse{
		Code:    0,
		Message: "success",
		Data: dto.AICallLogListResponse{
			Logs:  logs,
			Quota: quota,
		},
	})
}
