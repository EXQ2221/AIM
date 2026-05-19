package raghandler

import (
	"strings"

	ragmodel "example.com/aim/rag-service/rag-internal/model"
)

func NormalizeProvider(value ragmodel.Provider) ragmodel.Provider {
	normalized := ragmodel.Provider(strings.ToLower(strings.TrimSpace(string(value))))
	return normalized
}
