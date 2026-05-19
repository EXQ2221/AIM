package ragconf

import "testing"

func TestLoadConfigFromEnvMissingRequired(t *testing.T) {
	t.Setenv("EMBEDDING_BASE_URL", "")
	t.Setenv("EMBEDDING_API_KEY", "")
	t.Setenv("EMBEDDING_MODEL", "")
	if _, err := LoadConfigFromEnv(); err == nil {
		t.Fatal("expected missing env error")
	}
}

func TestLoadConfigFromEnvRetryOptions(t *testing.T) {
	t.Setenv("EMBEDDING_BASE_URL", "https://example.com/v1")
	t.Setenv("EMBEDDING_API_KEY", "k")
	t.Setenv("EMBEDDING_MODEL", "text-embedding-3-small")
	t.Setenv("EMBEDDING_MAX_RETRIES", "4")
	t.Setenv("EMBEDDING_RETRY_BACKOFF_MS", "2500")

	cfg, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxRetries != 4 {
		t.Fatalf("expected MaxRetries=4, got %d", cfg.MaxRetries)
	}
	if cfg.RetryBackoff.Milliseconds() != 2500 {
		t.Fatalf("expected RetryBackoff=2500ms, got %dms", cfg.RetryBackoff.Milliseconds())
	}
}

func TestLoadConfigFromEnvRetryOptionsInvalid(t *testing.T) {
	t.Setenv("EMBEDDING_BASE_URL", "https://example.com/v1")
	t.Setenv("EMBEDDING_API_KEY", "k")
	t.Setenv("EMBEDDING_MODEL", "text-embedding-3-small")
	t.Setenv("EMBEDDING_MAX_RETRIES", "-1")
	if _, err := LoadConfigFromEnv(); err == nil {
		t.Fatal("expected EMBEDDING_MAX_RETRIES validation error")
	}
}
