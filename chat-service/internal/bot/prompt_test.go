package bot

import (
	"fmt"
	"strings"
	"testing"

	"example.com/aim/chat-service/internal/dal/model"
)

func TestBuildPromptWithEmptyMessages(t *testing.T) {
	prompt := BuildPrompt(nil, "@AIM 你好", 20, nil, 10001)
	if !strings.Contains(prompt, "暂无最近消息") {
		t.Fatalf("expected empty context text, got %q", prompt)
	}
	if !strings.Contains(prompt, "当前提问用户：用户10001") {
		t.Fatalf("expected current user fallback label, got %q", prompt)
	}
	if !strings.Contains(prompt, "当前用户问题：你好") {
		t.Fatalf("expected extracted question, got %q", prompt)
	}
}

func TestBuildPromptLimitsRecentMessages(t *testing.T) {
	messages := make([]model.Message, 0, 25)
	for i := 1; i <= 25; i++ {
		messages = append(messages, model.Message{
			SenderID:    uint64(i),
			SenderType:  model.SenderTypeUser,
			MessageType: model.MessageTypeText,
			Content:     fmt.Sprintf("message-%d", i),
		})
	}

	prompt := BuildPrompt(messages, "@AIM summarize", 20, nil, 25)
	if strings.Contains(prompt, "[用户1]: message-1") || strings.Contains(prompt, "[用户5]: message-5") {
		t.Fatalf("prompt kept messages outside the limit: %q", prompt)
	}
	if !strings.Contains(prompt, "message-6") || !strings.Contains(prompt, "message-25") {
		t.Fatalf("prompt did not keep the newest limited messages: %q", prompt)
	}
}

func TestBuildPromptFormatsSenders(t *testing.T) {
	prompt := BuildPrompt([]model.Message{
		{SenderID: 10001, SenderType: model.SenderTypeUser, MessageType: model.MessageTypeText, Content: "user text"},
		{SenderID: 1, SenderType: model.SenderTypeBot, MessageType: model.MessageTypeBotReply, Content: "bot text"},
	}, "@AIM next", 20, map[uint64]string{10001: "Alice"}, 10001)

	if !strings.Contains(prompt, "[Alice]: user text") {
		t.Fatalf("missing user line: %q", prompt)
	}
	if !strings.Contains(prompt, "[AIM]: bot text") {
		t.Fatalf("missing bot line: %q", prompt)
	}
	if !strings.Contains(prompt, "当前提问用户：Alice") {
		t.Fatalf("missing current user display name: %q", prompt)
	}
}
