package queryrouter

import (
	"strings"
	"testing"
)

func TestBuildMessagesUsesChineseSystemPromptAndJSONInput(t *testing.T) {
	messages, err := BuildMessages(PlanningInput{
		UserQuery: "把这两份报告做成时间线",
		SelectedTargets: []Target{
			{ID: "doc_a", Type: "document", Title: "报告A"},
			{ID: "doc_b", Type: "document", Title: "报告B"},
		},
	})
	if err != nil {
		t.Fatalf("BuildMessages returned error: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if !strings.Contains(messages[0].Content, "你是 AIM 的查询路由规划器") {
		t.Fatalf("missing chinese system prompt: %q", messages[0].Content)
	}
	if !strings.Contains(messages[1].Content, `"user_query": "把这两份报告做成时间线"`) {
		t.Fatalf("missing json user query: %q", messages[1].Content)
	}
	if !strings.Contains(messages[1].Content, "示例1") {
		t.Fatalf("missing few-shot examples: %q", messages[1].Content)
	}
}

func TestExtractJSONObjectStripsCodeFence(t *testing.T) {
	value, err := extractJSONObject("```json\n{\"family\":\"READ\"}\n```")
	if err != nil {
		t.Fatalf("extractJSONObject returned error: %v", err)
	}
	if value != "{\"family\":\"READ\"}" {
		t.Fatalf("unexpected json object: %q", value)
	}
}
