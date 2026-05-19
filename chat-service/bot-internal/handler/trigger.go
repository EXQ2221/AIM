package bothandler

import (
	"encoding/json"
	"strings"
	"unicode"

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

func leadingMentionToken(content string) (string, bool) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" || !strings.HasPrefix(trimmed, "@") {
		return "", false
	}

	var builder strings.Builder
	for _, r := range trimmed[1:] {
		if unicode.IsSpace(r) || isMentionSeparator(r) {
			break
		}
		builder.WriteRune(unicode.ToLower(r))
	}
	token := strings.TrimSpace(builder.String())
	if token == "" {
		return "", false
	}
	return token, true
}

func trimLeadingMention(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" || !strings.HasPrefix(trimmed, "@") {
		return strings.TrimSpace(trimmed)
	}

	index := 1
	for _, r := range trimmed[1:] {
		if unicode.IsSpace(r) || isMentionSeparator(r) {
			break
		}
		index += len(string(r))
	}
	trimmed = strings.TrimSpace(trimmed[index:])
	trimmed = strings.TrimLeft(trimmed, ":：,，;；")
	return strings.TrimSpace(trimmed)
}

func isMentionSeparator(r rune) bool {
	switch r {
	case ':', '：', ',', '，', ';', '；':
		return true
	default:
		return false
	}
}
