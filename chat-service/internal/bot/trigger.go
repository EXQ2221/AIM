package bot

import (
	"encoding/json"
	"strings"

	"example.com/aim/chat-service/internal/dal/model"
)

const fallbackQuestion = "\u8bf7\u8bf4\u660e\u4f60\u5e0c\u671b AI \u5e2e\u4f60\u5b8c\u6210\u4ec0\u4e48\u3002"

func ShouldTriggerBot(msg model.Message) bool {
	if msg.SenderType != model.SenderTypeUser {
		return false
	}
	if msg.MessageType != model.MessageTypeText && msg.MessageType != model.MessageTypeImage {
		return false
	}
	content := model.ExtractTextMessageContent(msg.Content)
	if msg.MessageType == model.MessageTypeImage {
		var image model.ImageMessageContent
		if err := json.Unmarshal([]byte(strings.TrimSpace(string(msg.Content))), &image); err == nil {
			content = strings.TrimSpace(image.Text)
		}
	}
	_, ok := leadingMentionToken(content)
	return ok
}

func ExtractQuestion(content string) string {
	trimmed := trimLeadingMention(content)
	if trimmed == "" {
		return fallbackQuestion
	}
	return trimmed
}
