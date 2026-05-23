package handler

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"example.com/aim/gateway/internal/middleware"
	"example.com/aim/gateway/internal/model/dto"
	"example.com/aim/gateway/internal/rpc"
	chatpb "example.com/aim/gateway/kitex_gen/chat"
	"example.com/aim/gateway/kitex_gen/chat/chatservice"
	"github.com/gin-gonic/gin"
)

const (
	conversationSummaryDefaultMessageCount = 100
	conversationSummaryMinMessageCount     = 20
	conversationSummaryMaxMessageCount     = 500
	conversationSummaryBatchLimit          = int32(100)
	conversationSummarySystemPrompt        = "你是群聊总结助手。请根据提供的群聊消息，输出简洁、结构化中文总结，包含：1) 主题概览 2) 关键结论 3) 待办事项（若无写“无”）4) 风险或分歧点（若无写“无”）。不要编造。"
)

var (
	conversationSummaryModel = getenvFirstNonEmpty(
		"SUMMARY_LLM_MODEL",
		"LLM_MODEL",
		"CHUNKER_MODEL",
		"deepseek-v4-flash",
	)
	conversationSummaryBaseURL = strings.TrimRight(
		getenvFirstNonEmpty(
			"SUMMARY_LLM_BASE_URL",
			"LLM_BASE_URL",
			"CHUNKER_BASE_URL",
			"https://api.deepseek.com",
		),
		"/",
	)
	conversationSummaryAPIKey = getenvFirstNonEmpty(
		"SUMMARY_LLM_API_KEY",
		"LLM_API_KEY",
		"CHUNKER_API_KEY",
		"DEEPSEEK_API_KEY",
	)
	conversationSummaryTimeout        = getenvDuration("SUMMARY_LLM_TIMEOUT", 35*time.Second)
	conversationSummaryInsecureVerify = getenvBool("SUMMARY_LLM_INSECURE_SKIP_VERIFY", getenvBool("LLM_INSECURE_SKIP_VERIFY", false))
)

type summarizeConversationRequest struct {
	MessageCount int `json:"messageCount"`
}

type summarizeConversationData struct {
	Summary          string `json:"summary"`
	MessageCountUsed int    `json:"messageCountUsed"`
	UsedCount        int    `json:"usedCount"`
	RemainingCount   int    `json:"remainingCount"`
	Model            string `json:"model"`
}

type summaryChatCompletionRequest struct {
	Model    string                         `json:"model"`
	Messages []summaryChatCompletionMessage `json:"messages"`
	Stream   bool                           `json:"stream"`
}

type summaryChatCompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type summaryChatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func SummarizeConversation(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}
	if strings.TrimSpace(conversationSummaryAPIKey) == "" {
		writeError(ctx, 500, "summary llm api key is not configured")
		return
	}

	conversationID, ok := conversationIDParam(ctx)
	if !ok {
		return
	}

	request := summarizeConversationRequest{
		MessageCount: conversationSummaryDefaultMessageCount,
	}
	if err := ctx.ShouldBindJSON(&request); err != nil && err != io.EOF {
		writeError(ctx, 400, "invalid request body")
		return
	}
	request.MessageCount = clampSummaryMessageCount(request.MessageCount)

	chatClient, err := rpc.ChatClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	conversation, err := findAccessibleConversation(ctx.Request.Context(), chatClient, authCtx.UserID, conversationID)
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if conversation == nil {
		writeError(ctx, 404, "conversation not found")
		return
	}
	if strings.ToUpper(strings.TrimSpace(conversation.Type)) != "GROUP" {
		writeError(ctx, 400, "仅支持群聊总结")
		return
	}

	allowed, remainingAfterUse, limitErr := defaultConversationSummaryLimiter().Allow(
		ctx.Request.Context(),
		authCtx.UserID,
		parseConversationSummaryDailyLimit(),
	)
	if limitErr != nil {
		writeError(ctx, 500, "summary limiter unavailable")
		return
	}
	if !allowed {
		writeError(ctx, 429, "今日总结次数已用尽（最多 2 次）")
		return
	}

	messages, err := collectLatestConversationMessages(ctx.Request.Context(), chatClient, authCtx.UserID, conversationID, request.MessageCount)
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if len(messages) == 0 {
		writeError(ctx, 400, "当前会话暂无可总结消息")
		return
	}

	userPrompt := buildConversationSummaryPrompt(conversation, messages)
	summary, err := callSummaryLLM(ctx.Request.Context(), conversationSummarySystemPrompt, userPrompt)
	if err != nil {
		writeError(ctx, 500, presentableMessage(err.Error()))
		return
	}

	dailyLimit := parseConversationSummaryDailyLimit()
	usedCount := dailyLimit - remainingAfterUse
	if usedCount < 0 {
		usedCount = 0
	}

	writeJSON(ctx, 200, dto.APIResponse{
		Code:    0,
		Message: "success",
		Data: summarizeConversationData{
			Summary:          summary,
			MessageCountUsed: len(messages),
			UsedCount:        usedCount,
			RemainingCount:   remainingAfterUse,
			Model:            conversationSummaryModel,
		},
	})
}

func clampSummaryMessageCount(value int) int {
	if value <= 0 {
		return conversationSummaryDefaultMessageCount
	}
	if value < conversationSummaryMinMessageCount {
		return conversationSummaryMinMessageCount
	}
	if value > conversationSummaryMaxMessageCount {
		return conversationSummaryMaxMessageCount
	}
	return value
}

func findAccessibleConversation(ctx context.Context, client chatservice.Client, operatorID int64, conversationID string) (*chatpb.ConversationInfo, error) {
	resp, err := client.ListConversations(ctx, &chatpb.ListConversationsRequest{
		UserId: operatorID,
	})
	if err != nil {
		return nil, err
	}
	for _, item := range resp.Conversations {
		if item.ConversationId == conversationID {
			return item, nil
		}
	}
	return nil, nil
}

func collectLatestConversationMessages(ctx context.Context, client chatservice.Client, operatorID int64, conversationID string, limit int) ([]*chatpb.MessageInfo, error) {
	if limit <= 0 {
		return nil, nil
	}
	beforeID := (*int64)(nil)
	all := make([]*chatpb.MessageInfo, 0, limit)
	for len(all) < limit {
		batchSize := conversationSummaryBatchLimit
		left := limit - len(all)
		if left < int(batchSize) {
			batchSize = int32(left)
		}
		resp, err := client.ListMessages(ctx, &chatpb.ListMessagesRequest{
			OperatorId:     operatorID,
			ConversationId: conversationID,
			BeforeId:       beforeID,
			Limit:          batchSize,
		})
		if err != nil {
			return nil, err
		}
		if len(resp.Messages) == 0 {
			break
		}
		all = append(all, resp.Messages...)
		last := resp.Messages[len(resp.Messages)-1]
		beforeID = &last.Id
		if len(resp.Messages) < int(batchSize) {
			break
		}
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].CreatedAt == all[j].CreatedAt {
			return all[i].Id < all[j].Id
		}
		return all[i].CreatedAt < all[j].CreatedAt
	})
	return all, nil
}

func buildConversationSummaryPrompt(conversation *chatpb.ConversationInfo, messages []*chatpb.MessageInfo) string {
	lines := make([]string, 0, len(messages)+6)
	title := strings.TrimSpace(conversation.Title)
	if title == "" {
		title = conversation.ConversationId
	}
	lines = append(lines, "请总结以下群聊消息。")
	lines = append(lines, "群聊："+title)
	lines = append(lines, "消息按时间升序：")
	for _, message := range messages {
		if message == nil {
			continue
		}
		content := strings.TrimSpace(extractSearchableMessageText(message))
		if content == "" {
			continue
		}
		sender := fmt.Sprintf("%d", message.SenderId)
		if strings.ToUpper(strings.TrimSpace(message.SenderType)) == "BOT" {
			sender = "BOT-" + sender
		}
		lines = append(lines, fmt.Sprintf("[%s] %s", sender, content))
	}
	return strings.Join(lines, "\n")
}

func callSummaryLLM(ctx context.Context, systemPrompt string, userPrompt string) (string, error) {
	payload := summaryChatCompletionRequest{
		Model: conversationSummaryModel,
		Messages: []summaryChatCompletionMessage{
			{Role: "system", Content: strings.TrimSpace(systemPrompt)},
			{Role: "user", Content: strings.TrimSpace(userPrompt)},
		},
		Stream: false,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	timeout := conversationSummaryTimeout
	if timeout <= 0 {
		timeout = 35 * time.Second
	}
	httpClient := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: conversationSummaryInsecureVerify}, //nolint:gosec
		},
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		conversationSummaryBaseURL+"/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+conversationSummaryAPIKey)

	response, err := httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("summary llm request failed: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 128*1024))
	if err != nil {
		return "", fmt.Errorf("summary llm response read failed: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("summary llm request failed: status=%d %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var parsed summaryChatCompletionResponse
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return "", fmt.Errorf("summary llm response parse failed: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("summary llm response empty")
	}
	result := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if result == "" {
		return "", fmt.Errorf("summary llm response empty")
	}
	return result, nil
}

func getenvFirstNonEmpty(keys ...string) string {
	for _, key := range keys {
		value := strings.TrimSpace(os.Getenv(key))
		if value != "" {
			return value
		}
	}
	return ""
}

func getenvBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return parsed
}
