package handler

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"example.com/aim/gateway/internal/middleware"
	"example.com/aim/gateway/internal/model/dto"
	"example.com/aim/gateway/internal/rpc"
	chatpb "example.com/aim/gateway/kitex_gen/chat"
	"example.com/aim/gateway/kitex_gen/chat/chatservice"
	"github.com/gin-gonic/gin"
)

const (
	historySearchBatchLimit = int32(100)
	historySearchMaxFetch   = 1500
	historySearchTypeAll    = "ALL"
)

func SearchHistoryMessages(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	startAt, endAt, ok := parseHistoryTimeRange(ctx)
	if !ok {
		return
	}
	keyword := strings.TrimSpace(ctx.Query("keyword"))
	conversationType, ok := normalizeHistorySearchConversationType(ctx.Query("conversationType"))
	if !ok {
		writeError(ctx, 400, "invalid conversationType")
		return
	}

	conversationID := strings.TrimSpace(ctx.Query("conversationId"))
	client, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	conversationsResp, err := client.ListConversations(ctx.Request.Context(), &chatpb.ListConversationsRequest{
		UserId: authCtx.UserID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	conversationMap := make(map[string]*chatpb.ConversationInfo, len(conversationsResp.Conversations))
	targetConversationIDs := make([]string, 0, len(conversationsResp.Conversations))
	for _, item := range conversationsResp.Conversations {
		conversationMap[item.ConversationId] = item
	}

	if conversationID != "" {
		_, exists := conversationMap[conversationID]
		if !exists {
			writeError(ctx, 404, "conversation not found")
			return
		}
		targetConversationIDs = append(targetConversationIDs, conversationID)
	} else {
		for _, item := range conversationsResp.Conversations {
			if !isHistorySearchConversationTypeSupported(item.Type) {
				continue
			}
			if conversationType == historySearchTypeAll || item.Type == conversationType {
				targetConversationIDs = append(targetConversationIDs, item.ConversationId)
			}
		}
	}

	results := make([]dto.HistorySearchMessageItem, 0, 256)
	for _, cid := range targetConversationIDs {
		conversation := conversationMap[cid]
		items, err := collectConversationHistoryInRange(ctx, client, authCtx.UserID, conversation, startAt, endAt, keyword)
		if err != nil {
			writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
			return
		}
		results = append(results, items...)
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Message.CreatedAt == results[j].Message.CreatedAt {
			return results[i].Message.ID > results[j].Message.ID
		}
		return results[i].Message.CreatedAt > results[j].Message.CreatedAt
	})

	writeJSON(ctx, 200, dto.APIResponse{
		Code:    0,
		Message: "success",
		Data:    results,
	})
}

func normalizeHistorySearchConversationType(raw string) (string, bool) {
	value := strings.ToUpper(strings.TrimSpace(raw))
	switch value {
	case "", historySearchTypeAll:
		return historySearchTypeAll, true
	case "GROUP", "SINGLE":
		return value, true
	default:
		return "", false
	}
}

func isHistorySearchConversationTypeSupported(conversationType string) bool {
	switch strings.ToUpper(strings.TrimSpace(conversationType)) {
	case "GROUP", "SINGLE":
		return true
	default:
		return false
	}
}

func parseHistoryTimeRange(ctx *gin.Context) (int64, int64, bool) {
	startAtRaw := strings.TrimSpace(ctx.Query("startAt"))
	endAtRaw := strings.TrimSpace(ctx.Query("endAt"))
	if startAtRaw == "" || endAtRaw == "" {
		writeError(ctx, 400, "startAt and endAt are required")
		return 0, 0, false
	}

	startAt, err := strconv.ParseInt(startAtRaw, 10, 64)
	if err != nil || startAt <= 0 {
		writeError(ctx, 400, "invalid startAt")
		return 0, 0, false
	}
	endAt, err := strconv.ParseInt(endAtRaw, 10, 64)
	if err != nil || endAt <= 0 {
		writeError(ctx, 400, "invalid endAt")
		return 0, 0, false
	}
	if endAt < startAt {
		writeError(ctx, 400, "endAt must be greater than or equal to startAt")
		return 0, 0, false
	}
	return startAt, endAt, true
}

func collectConversationHistoryInRange(
	ctx *gin.Context,
	client chatservice.Client,
	operatorID int64,
	conversation *chatpb.ConversationInfo,
	startAt int64,
	endAt int64,
	keyword string,
) ([]dto.HistorySearchMessageItem, error) {
	beforeID := (*int64)(nil)
	visited := 0
	results := make([]dto.HistorySearchMessageItem, 0, 64)

	for visited < historySearchMaxFetch {
		req := &chatpb.ListMessagesRequest{
			OperatorId:     operatorID,
			ConversationId: conversation.ConversationId,
			BeforeId:       beforeID,
			Limit:          historySearchBatchLimit,
		}
		resp, err := client.ListMessages(ctx.Request.Context(), req)
		if err != nil {
			return nil, err
		}
		if len(resp.Messages) == 0 {
			break
		}

		olderThanStart := false
		for _, item := range resp.Messages {
			visited++
			if item.CreatedAt < startAt {
				olderThanStart = true
				continue
			}
			if item.CreatedAt > endAt {
				continue
			}
			if keyword != "" && !messageMatchesKeyword(item, keyword) {
				continue
			}
			results = append(results, dto.HistorySearchMessageItem{
				ConversationID:    conversation.ConversationId,
				ConversationType:  conversation.Type,
				ConversationTitle: conversation.Title,
				Message:           toMessageModel(item),
			})
		}

		last := resp.Messages[len(resp.Messages)-1]
		beforeID = &last.Id
		if olderThanStart {
			break
		}
	}

	return results, nil
}

func messageMatchesKeyword(message *chatpb.MessageInfo, keyword string) bool {
	if message == nil {
		return false
	}
	normalizedKeyword := strings.ToLower(strings.TrimSpace(keyword))
	if normalizedKeyword == "" {
		return true
	}
	text := strings.ToLower(strings.TrimSpace(extractSearchableMessageText(message)))
	if text == "" {
		return false
	}
	return strings.Contains(text, normalizedKeyword)
}

func extractSearchableMessageText(message *chatpb.MessageInfo) string {
	switch strings.ToUpper(strings.TrimSpace(message.MessageType)) {
	case "TEXT", "BOT_REPLY":
		return readJSONTextField(message.Content, "text")
	case "IMAGE":
		text := readJSONTextField(message.Content, "text")
		if strings.TrimSpace(text) != "" {
			return text
		}
		return readJSONTextField(message.Content, "name")
	case "FILE":
		return readJSONTextField(message.Content, "name")
	case "VOICE":
		return readJSONTextField(message.Content, "name")
	case "SYSTEM":
		return readJSONTextField(message.Content, "text")
	default:
		text := readJSONTextField(message.Content, "text")
		if strings.TrimSpace(text) != "" {
			return text
		}
		return message.Content
	}
}

func readJSONTextField(content string, field string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return trimmed
	}
	value, ok := payload[field]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}
