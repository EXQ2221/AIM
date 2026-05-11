package bot

import (
	"encoding/json"
	"strings"

	"example.com/aim/chat-service/internal/dal/model"
)

const fallbackQuestion = "璇烽棶浣犻渶瑕佹垜甯綘鍋氫粈涔堬紵"

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

