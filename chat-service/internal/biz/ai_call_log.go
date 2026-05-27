package biz

import (
	"context"
	"strings"
	"time"

	"example.com/aim/chat-service/internal/dal/model"
)

const dailyAICallTokenLimit int64 = 50_000

func (s *ChatService) ListAICallLogs(
	ctx context.Context,
	operatorID uint64,
	conversationID string,
	beforeID *uint64,
	limit int,
	botID *uint64,
	status string,
) (*AICallLogListView, error) {
	if s.AICallLogRepo == nil {
		return nil, ErrBotManagementUnavailable
	}

	conversation, err := s.requireConversation(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if _, err := s.requireMember(ctx, conversation.ID, operatorID); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}

	status = strings.ToUpper(strings.TrimSpace(status))
	if status != "" && status != string(model.AICallStatusSuccess) && status != string(model.AICallStatusFailed) {
		return nil, ErrBadRequest
	}

	quota, err := s.buildAICallLogQuota(ctx, conversation.ID)
	if err != nil {
		return nil, err
	}

	logs, err := s.AICallLogRepo.ListByConversationID(ctx, conversation.ID, beforeID, limit, botID, status)
	if err != nil {
		return nil, err
	}

	botNames := make(map[uint64]string)
	if s.BotRepo != nil {
		for _, item := range logs {
			if item.BotID == 0 {
				continue
			}
			if _, ok := botNames[item.BotID]; ok {
				continue
			}
			botModel, err := s.BotRepo.GetByID(ctx, item.BotID)
			if err != nil || botModel == nil {
				botNames[item.BotID] = ""
				continue
			}
			name := strings.TrimSpace(botModel.Name)
			// Keep logs complete but mark custom bots, so clients can exclude them from platform quota stats.
			if botModel.CreatedBy > 0 {
				name = "[自建] " + name
			}
			botNames[item.BotID] = name
		}
	}

	result := make([]AICallLogView, 0, len(logs))
	for _, item := range logs {
		result = append(result, AICallLogView{
			ID:                item.ID,
			ConversationID:    conversation.ConversationID,
			UserID:            item.UserID,
			BotID:             item.BotID,
			BotName:           botNames[item.BotID],
			RequestMessageID:  item.RequestMessageID,
			ResponseMessageID: item.ResponseMessageID,
			ModelName:         item.ModelName,
			PromptTokens:      item.PromptTokens,
			CompletionTokens:  item.CompletionTokens,
			TotalTokens:       item.TotalTokens,
			LatencyMS:         item.LatencyMS,
			Status:            string(item.Status),
			ErrorMessage:      item.ErrorMessage,
			CreatedAt:         item.CreatedAt.Unix(),
		})
	}
	return &AICallLogListView{
		Logs:  result,
		Quota: quota,
	}, nil
}

func (s *ChatService) buildAICallLogQuota(ctx context.Context, conversationID uint64) (AICallLogQuotaView, error) {
	startAt, endAt := currentDayWindow()
	total, err := s.AICallLogRepo.SumPlatformTotalTokensByConversationBetween(ctx, conversationID, startAt, endAt)
	if err != nil {
		return AICallLogQuotaView{}, err
	}
	remaining := dailyAICallTokenLimit - total
	if remaining < 0 {
		remaining = 0
	}
	return AICallLogQuotaView{
		DailyTotalTokens: total,
		DailyTokenLimit:  dailyAICallTokenLimit,
		RemainingTokens:  remaining,
	}, nil
}

func currentDayWindow() (time.Time, time.Time) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return start, start.Add(24 * time.Hour)
}
