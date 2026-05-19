package llmconf

import "testing"

func TestLoadConfigFromEnvMissingRequired(t *testing.T) {
	t.Setenv("LLM_BASE_URL", "")
	t.Setenv("LLM_API_KEY", "")
	t.Setenv("LLM_MODEL", "")
	t.Setenv("LLM_TIMEOUT_SECONDS", "")

	if _, err := LoadConfigFromEnv(); err == nil {
		t.Fatal("expected missing env error")
	}
}
