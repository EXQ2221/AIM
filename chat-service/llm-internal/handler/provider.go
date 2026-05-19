package llmhandler

import "strings"

func NormalizeProviderName(value string, fallback string) string {
	name := strings.TrimSpace(value)
	if name != "" {
		return name
	}
	return strings.TrimSpace(fallback)
}
