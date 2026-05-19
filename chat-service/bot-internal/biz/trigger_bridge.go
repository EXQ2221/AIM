package bot

import (
	bothandler "example.com/aim/chat-service/bot-internal/handler"
	"example.com/aim/chat-service/internal/dal/model"
)

func ShouldTriggerBot(msg model.Message) bool {
	return bothandler.ShouldTriggerBot(msg)
}

func ExtractQuestion(content string) string {
	return bothandler.ExtractQuestion(content)
}
