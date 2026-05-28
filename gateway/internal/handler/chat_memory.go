package handler

import (
	"sort"
	"strconv"
	"strings"

	"example.com/aim/gateway/internal/middleware"
	"example.com/aim/gateway/internal/model/dto"
	"example.com/aim/gateway/internal/rpc"
	chatpb "example.com/aim/gateway/kitex_gen/chat"
	"github.com/gin-gonic/gin"
)

func WriteUserMemory(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	var req dto.WriteUserMemoryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.WriteUserMemory(ctx.Request.Context(), &chatpb.WriteUserMemoryRequest{
		OperatorId: authCtx.UserID,
		Content:    req.Content,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	writeJSON(ctx, 200, dto.APIResponse{
		Code:    0,
		Message: "success",
		Data:    toUserMemoryModel(resp.Memory),
	})
}

func ListUserMemories(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	var limit *int32
	if raw := strings.TrimSpace(ctx.Query("limit")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 32)
		if err != nil || parsed <= 0 {
			writeError(ctx, 400, "invalid limit")
			return
		}
		value := int32(parsed)
		limit = &value
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.ListUserMemories(ctx.Request.Context(), &chatpb.ListUserMemoriesRequest{
		OperatorId: authCtx.UserID,
		Limit:      limit,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	items := make([]dto.UserMemoryInfo, 0, len(resp.Memories))
	for _, item := range resp.Memories {
		items = append(items, toUserMemoryModel(item))
	}
	writeJSON(ctx, 200, dto.APIResponse{
		Code:    0,
		Message: "success",
		Data:    items,
	})
}

func UpdateUserMemory(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}
	memoryIDValue := strings.TrimSpace(ctx.Param("memoryId"))
	memoryID, err := strconv.ParseInt(memoryIDValue, 10, 64)
	if err != nil || memoryID <= 0 {
		writeError(ctx, 400, "invalid memoryId")
		return
	}

	var req dto.UpdateUserMemoryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.UpdateUserMemory(ctx.Request.Context(), &chatpb.UpdateUserMemoryRequest{
		OperatorId: authCtx.UserID,
		MemoryId:   memoryID,
		Content:    req.Content,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	writeJSON(ctx, 200, dto.APIResponse{
		Code:    0,
		Message: "success",
		Data:    toUserMemoryModel(resp.Memory),
	})
}

func GetUserMemorySetting(ctx *gin.Context) {
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

	resp, err := client.GetUserMemorySetting(ctx.Request.Context(), &chatpb.GetUserMemorySettingRequest{
		OperatorId: authCtx.UserID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	writeJSON(ctx, 200, dto.APIResponse{
		Code:    0,
		Message: "success",
		Data:    toUserMemorySettingModel(resp.Setting),
	})
}

func UpdateUserMemorySetting(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	var req dto.UpdateUserMemorySettingRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}
	if req.Scope != nil {
		scope := strings.ToUpper(strings.TrimSpace(*req.Scope))
		switch scope {
		case "", "ALL_GROUPS", "SELECTED_GROUPS":
			if scope == "" {
				scope = "ALL_GROUPS"
			}
			req.Scope = &scope
		default:
			writeError(ctx, 400, "invalid scope")
			return
		}
	}
	if req.ConversationIDs != nil {
		seen := make(map[string]struct{}, len(req.ConversationIDs))
		normalized := make([]string, 0, len(req.ConversationIDs))
		for _, id := range req.ConversationIDs {
			value := strings.TrimSpace(id)
			if value == "" || !strings.HasPrefix(value, "c_") {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			normalized = append(normalized, value)
		}
		sort.Strings(normalized)
		req.ConversationIDs = normalized
	}

	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.UpdateUserMemorySetting(ctx.Request.Context(), &chatpb.UpdateUserMemorySettingRequest{
		OperatorId:      authCtx.UserID,
		Enabled:         req.Enabled,
		Scope:           req.Scope,
		ConversationIds: req.ConversationIDs,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	writeJSON(ctx, 200, dto.APIResponse{
		Code:    0,
		Message: "success",
		Data:    toUserMemorySettingModel(resp.Setting),
	})
}
