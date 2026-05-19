package bot

import (
	"encoding/json"
	"strings"
	"unicode"

	"example.com/aim/chat-service/internal/dal/model"
)

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

func parseAliasesJSON(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	var aliases []string
	if err := json.Unmarshal([]byte(raw), &aliases); err != nil {
		return nil, err
	}

	return normalizeLowerStringList(aliases), nil
}

func ParseSupportedModels(raw string, fallback string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if fallback == "" {
			return nil, nil
		}
		return []string{fallback}, nil
	}

	var models []string
	if err := json.Unmarshal([]byte(raw), &models); err != nil {
		return nil, err
	}
	result := normalizeStringList(models)
	if len(result) == 0 && fallback != "" {
		return []string{fallback}, nil
	}
	return result, nil
}

func EffectiveModelName(bot model.Bot, conversationBot model.ConversationBot, fallback string) string {
	if value := strings.TrimSpace(conversationBot.ModelNameOverride); value != "" {
		return value
	}
	if value := strings.TrimSpace(bot.ModelName); value != "" {
		return value
	}
	return strings.TrimSpace(fallback)
}

func effectiveMentionName(bot model.Bot, conversationBot model.ConversationBot) string {
	if value := strings.TrimSpace(conversationBot.MentionNameOverride); value != "" {
		return strings.ToLower(value)
	}
	return strings.ToLower(strings.TrimSpace(bot.MentionName))
}

func effectiveAliases(bot model.Bot, conversationBot model.ConversationBot) ([]string, error) {
	if strings.TrimSpace(conversationBot.AliasesOverride) != "" {
		return parseAliasesJSON(conversationBot.AliasesOverride)
	}
	return parseAliasesJSON(bot.Aliases)
}

func normalizeStringList(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		key := strings.ToLower(normalized)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func normalizeLowerStringList(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}
