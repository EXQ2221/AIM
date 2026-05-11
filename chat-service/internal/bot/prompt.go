package bot

import (
	"fmt"
	"strings"

	"example.com/aim/chat-service/internal/dal/model"
)

const DefaultSystemPrompt = "你是 AIM 群聊中的 AI 助手。请基于群聊上下文回答用户问题。要求：1. 回答简洁、准确。2. 如果上下文不足，请直接说明不确定。3. 不要编造群聊中没有的信息。"

func BuildPrompt(recentMessages []model.Message, currentContent string, limit int, userDisplayNames map[uint64]string, currentUserID uint64) string {
	if limit <= 0 {
		limit = 20
	}
	if len(recentMessages) > limit {
		recentMessages = recentMessages[len(recentMessages)-limit:]
	}

	var builder strings.Builder
	builder.WriteString("以下是最近的群聊消息：\n")
	if len(recentMessages) == 0 {
		builder.WriteString("（暂无最近消息）\n")
	} else {
		for _, msg := range recentMessages {
			builder.WriteString(formatMessageLine(msg, userDisplayNames))
			builder.WriteByte('\n')
		}
	}
	builder.WriteString("\n当前提问用户：")
	builder.WriteString(userDisplayName(currentUserID, userDisplayNames))
	builder.WriteString("\n当前用户问题：")
	builder.WriteString(ExtractQuestion(currentContent))
	return builder.String()
}

func formatMessageLine(msg model.Message, userDisplayNames map[uint64]string) string {
	content := strings.TrimSpace(model.MessagePreview(msg.MessageType, msg.Content))
	switch msg.SenderType {
	case model.SenderTypeBot:
		return fmt.Sprintf("[AIM]: %s", content)
	case model.SenderTypeSystem:
		return fmt.Sprintf("[SYSTEM]: %s", content)
	default:
		return fmt.Sprintf("[%s]: %s", userDisplayName(msg.SenderID, userDisplayNames), content)
	}
}

func userDisplayName(userID uint64, userDisplayNames map[uint64]string) string {
	if userDisplayNames != nil {
		if name := strings.TrimSpace(userDisplayNames[userID]); name != "" {
			return name
		}
	}
	return fmt.Sprintf("用户%d", userID)
}

