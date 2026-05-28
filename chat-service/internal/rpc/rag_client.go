package rpc

import (
	"fmt"

	"example.com/aim/chat-service/kitex_gen/rag/ragservice"
	"github.com/cloudwego/kitex/client"
)

func NewRAGClient(addr string) (ragservice.Client, error) {
	endpoint, err := NormalizeHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("normalize rag endpoint failed: %w", err)
	}
	return ragservice.NewClient("rag-service", client.WithHostPorts(endpoint))
}
