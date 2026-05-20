package bot

import (
	"fmt"
	"strings"
	"testing"

	"example.com/aim/chat-service/internal/dal/model"
	"gorm.io/datatypes"
)

func TestBuildPromptWithEmptyMessages(t *testing.T) {
	prompt := BuildPrompt(nil, "@AIM \u4f60\u597d", 20, nil, 10001)
	if !strings.Contains(prompt, "\u6682\u65e0\u6700\u8fd1\u6d88\u606f") {
		t.Fatalf("expected empty context text, got %q", prompt)
	}
	if !strings.Contains(prompt, "\u3010\u5f53\u524d\u63d0\u95ee\u7528\u6237\u3011\u7528\u623710001") {
		t.Fatalf("expected current user fallback label, got %q", prompt)
	}
	if !strings.Contains(prompt, "\u3010\u7528\u6237\u95ee\u9898\u3011\u4f60\u597d") {
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
			Content:     datatypes.JSON(fmt.Sprintf(`{"text":"message-%d"}`, i)),
		})
	}

	prompt := BuildPrompt(messages, "@AIM summarize", 20, nil, 25)
	if strings.Contains(prompt, "[\u7528\u62371]: message-1") || strings.Contains(prompt, "[\u7528\u62375]: message-5") {
		t.Fatalf("prompt kept messages outside the limit: %q", prompt)
	}
	if !strings.Contains(prompt, "message-6") || !strings.Contains(prompt, "message-25") {
		t.Fatalf("prompt did not keep the newest limited messages: %q", prompt)
	}
}

func TestBuildPromptFormatsSenders(t *testing.T) {
	prompt := BuildPrompt([]model.Message{
		{SenderID: 10001, SenderType: model.SenderTypeUser, MessageType: model.MessageTypeText, Content: datatypes.JSON(`{"text":"user text"}`)},
		{SenderID: 10002, SenderType: model.SenderTypeUser, MessageType: model.MessageTypeImage, Content: datatypes.JSON(`{"url":"https://cdn.example.com/a.png","name":"a.png","size":123,"mimeType":"image/png","width":640,"height":480}`)},
		{SenderID: 1, SenderType: model.SenderTypeBot, MessageType: model.MessageTypeBotReply, Content: datatypes.JSON(`{"text":"bot text"}`)},
	}, "@AIM next", 20, map[uint64]string{10001: "Alice"}, 10001)

	if !strings.Contains(prompt, "[Alice]: user text") {
		t.Fatalf("missing user line: %q", prompt)
	}
	if !strings.Contains(prompt, "[\u7528\u623710002]: [\u56fe\u7247]") {
		t.Fatalf("missing image placeholder line: %q", prompt)
	}
	if !strings.Contains(prompt, "[AIM]: bot text") {
		t.Fatalf("missing bot line: %q", prompt)
	}
	if !strings.Contains(prompt, "\u3010\u5f53\u524d\u63d0\u95ee\u7528\u6237\u3011Alice") {
		t.Fatalf("missing current user display name: %q", prompt)
	}
}

func TestBuildPromptWithRAGUsesLocalKnowledgeBaseSection(t *testing.T) {
	prompt := BuildPromptWithRAG(
		nil,
		"@ai \u8bdd\u5267\u7684\u5185\u5bb9\u662f\u4ec0\u4e48",
		20,
		map[uint64]string{10001: "Alice"},
		10001,
		model.BotScopeKnowledgeBaseOnly,
		[]RAGChunk{
			{Index: 1, Content: "\u8fd9\u662f\u8bdd\u5267\u7247\u6bb51", Score: 0.9},
			{Index: 2, Content: "\u8fd9\u662f\u8bdd\u5267\u7247\u6bb52", Score: 0.8},
		},
	)

	if !strings.Contains(prompt, "\u4ee5\u4e0b\u662f\u7528\u6237\u672c\u5730\u77e5\u8bc6\u5e93\u4e2d\u7684\u76f8\u5173\u5185\u5bb9") {
		t.Fatalf("missing local knowledge source disclaimer: %q", prompt)
	}
	if !strings.Contains(prompt, "\u3010\u672c\u5730\u77e5\u8bc6\u5e93\u3011") {
		t.Fatalf("missing local knowledge section title: %q", prompt)
	}
	if !strings.Contains(prompt, "[1] \u8fd9\u662f\u8bdd\u5267\u7247\u6bb51") || !strings.Contains(prompt, "[2] \u8fd9\u662f\u8bdd\u5267\u7247\u6bb52") {
		t.Fatalf("missing rag chunk lines: %q", prompt)
	}
	if !strings.Contains(prompt, "\u6839\u636e\u60a8\u4e0a\u4f20\u7684\u6587\u6863") {
		t.Fatalf("missing citation wording requirement: %q", prompt)
	}
}
