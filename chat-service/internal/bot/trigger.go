package bot

import "example.com/aim/chat-service/internal/dal/model"

const fallbackQuestion = "请问你需要我帮你做什么？"

func ShouldTriggerBot(msg model.Message) bool {
	if msg.SenderType != model.SenderTypeUser {
		return false
	}
	if msg.MessageType != model.MessageTypeText {
		return false
	}
	_, ok := leadingMentionToken(model.ExtractTextMessageContent(msg.Content))
	return ok
}

func ExtractQuestion(content string) string {
	trimmed := trimLeadingMention(content)
	if trimmed == "" {
		return fallbackQuestion
	}
	return trimmed
}
