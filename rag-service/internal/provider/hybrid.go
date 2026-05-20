package embedding

import (
	"context"
	"errors"

	ragmodel "example.com/aim/rag-service/internal/dal/model"
)

type hybridClient struct {
	primary  Client
	fallback Client
}

func (c *hybridClient) Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error) {
	if c == nil || c.primary == nil {
		return nil, errors.New("embedding client is nil")
	}
	if hasMultimodalInput(req) && c.fallback != nil {
		return c.fallback.Embed(ctx, req)
	}
	return c.primary.Embed(ctx, req)
}

func hasMultimodalInput(req EmbedRequest) bool {
	for _, part := range req.Input {
		if part.Type == ragmodel.InputPartImage || part.Type == ragmodel.InputPartVideo {
			return true
		}
	}
	return false
}

var _ Client = (*hybridClient)(nil)
