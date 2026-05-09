package upload

import (
	"os"
	"strconv"
	"strings"
)

const (
	defaultDir          = "uploads"
	defaultPublicPrefix = "/uploads"
	defaultMaxBytes     = 5 << 20
)

func Dir() string {
	if value := strings.TrimSpace(os.Getenv("UPLOAD_DIR")); value != "" {
		return value
	}
	return defaultDir
}

func PublicPrefix() string {
	value := strings.TrimSpace(os.Getenv("UPLOAD_PUBLIC_PREFIX"))
	if value == "" {
		return defaultPublicPrefix
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	value = strings.TrimRight(value, "/")
	if value == "" {
		return defaultPublicPrefix
	}
	return value
}

func MaxBytes() int64 {
	value := strings.TrimSpace(os.Getenv("UPLOAD_MAX_BYTES"))
	if value == "" {
		return defaultMaxBytes
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return defaultMaxBytes
	}
	return parsed
}
