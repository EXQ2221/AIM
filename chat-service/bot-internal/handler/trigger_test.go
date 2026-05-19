package bothandler

import (
	"testing"

	"example.com/aim/chat-service/internal/dal/model"
	"gorm.io/datatypes"
)

func TestShouldTriggerBot(t *testing.T) {
	tests := []struct {
		name string
		msg  model.Message
		want bool
	}{
		{
			name: "plain text should not trigger",
			msg:  model.Message{SenderType: model.SenderTypeUser, MessageType: model.MessageTypeText, Content: datatypes.JSON(`{"text":"hello"}`)},
		},
		{
			name: "text mention should trigger",
			msg:  model.Message{SenderType: model.SenderTypeUser, MessageType: model.MessageTypeText, Content: datatypes.JSON(`{"text":"@aim hello"}`)},
			want: true,
		},
		{
			name: "mention with colon should trigger",
			msg:  model.Message{SenderType: model.SenderTypeUser, MessageType: model.MessageTypeText, Content: datatypes.JSON(`{"text":"@helper: hello"}`)},
			want: true,
		},
		{
			name: "bot message should not trigger",
			msg:  model.Message{SenderType: model.SenderTypeBot, MessageType: model.MessageTypeBotReply, Content: datatypes.JSON(`{"text":"@aim hello"}`)},
		},
		{
			name: "image text mention should trigger",
			msg:  model.Message{SenderType: model.SenderTypeUser, MessageType: model.MessageTypeImage, Content: datatypes.JSON(`{"url":"https://cdn.example.com/a.png","name":"a.png","mimeType":"image/png","text":"@aim look"}`)},
			want: true,
		},
		{
			name: "image without mention should not trigger",
			msg:  model.Message{SenderType: model.SenderTypeUser, MessageType: model.MessageTypeImage, Content: datatypes.JSON(`{"url":"https://cdn.example.com/a.png","name":"a.png","mimeType":"image/png","text":"just image"}`)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldTriggerBot(tt.msg); got != tt.want {
				t.Fatalf("ShouldTriggerBot() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractQuestion(t *testing.T) {
	tests := []struct {
		content string
		want    string
	}{
		{content: "@AIM summarize this", want: "summarize this"},
		{content: "@bot: hello", want: "hello"},
		{content: "@helper，continue", want: "continue"},
		{content: "@aim", want: fallbackQuestion},
	}

	for _, tt := range tests {
		if got := ExtractQuestion(tt.content); got != tt.want {
			t.Fatalf("ExtractQuestion(%q) = %q, want %q", tt.content, got, tt.want)
		}
	}
}
