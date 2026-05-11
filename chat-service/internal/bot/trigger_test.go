package bot

import (
	"testing"

	"example.com/aim/chat-service/internal/dal/model"
)

func TestShouldTriggerBot(t *testing.T) {
	tests := []struct {
		name string
		msg  model.Message
		want bool
	}{
		{
			name: "普通文本不触发",
			msg:  model.Message{SenderType: model.SenderTypeUser, MessageType: model.MessageTypeText, Content: `{"text":"hello"}`},
		},
		{
			name: "文本 mention 触发",
			msg:  model.Message{SenderType: model.SenderTypeUser, MessageType: model.MessageTypeText, Content: `{"text":"@aim hello"}`},
			want: true,
		},
		{
			name: "带冒号 mention 触发",
			msg:  model.Message{SenderType: model.SenderTypeUser, MessageType: model.MessageTypeText, Content: `{"text":"@helper: hello"}`},
			want: true,
		},
		{
			name: "bot 回复不触发",
			msg:  model.Message{SenderType: model.SenderTypeBot, MessageType: model.MessageTypeBotReply, Content: "@aim hello"},
		},
		{
			name: "图片消息 text mention 触发",
			msg:  model.Message{SenderType: model.SenderTypeUser, MessageType: model.MessageTypeImage, Content: `{"url":"https://cdn.example.com/a.png","name":"a.png","mimeType":"image/png","text":"@aim 请看图"}`},
			want: true,
		},
		{
			name: "图片消息无 mention 不触发",
			msg:  model.Message{SenderType: model.SenderTypeUser, MessageType: model.MessageTypeImage, Content: `{"url":"https://cdn.example.com/a.png","name":"a.png","mimeType":"image/png","text":"这是一张图"}`},
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
		{content: "@AIM 总结一下刚才讨论了什么", want: "总结一下刚才讨论了什么"},
		{content: "@bot: hello", want: "hello"},
		{content: "@helper，继续", want: "继续"},
		{content: "@aim", want: fallbackQuestion},
	}

	for _, tt := range tests {
		if got := ExtractQuestion(tt.content); got != tt.want {
			t.Fatalf("ExtractQuestion(%q) = %q, want %q", tt.content, got, tt.want)
		}
	}
}
