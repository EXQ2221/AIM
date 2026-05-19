package notification

import "strings"

const (
	CategoryGroupSystem = "GROUP_SYSTEM"
	CategoryUserCenter  = "USER_CENTER"
	CategorySystem      = "SYSTEM"
)

func Normalize(notificationType string, title string, content string) (category string, summary string, detail string) {
	typeValue := strings.ToUpper(strings.TrimSpace(notificationType))
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)

	summary = title
	detail = content

	switch {
	case typeValue == "GROUP_EVENT" || strings.HasPrefix(typeValue, "GROUP_"):
		category = CategoryGroupSystem
	case typeValue == "KNOWLEDGE_IMPORT" || strings.HasPrefix(typeValue, "KNOWLEDGE_"):
		category = CategoryUserCenter
	case typeValue == "SYSTEM":
		category = CategorySystem
	default:
		category = CategoryUserCenter
	}

	if summary == "" {
		summary = firstLine(detail)
	}
	if detail == "" {
		detail = summary
	}
	return category, summary, detail
}

func firstLine(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	lines := strings.Split(value, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return value
}
