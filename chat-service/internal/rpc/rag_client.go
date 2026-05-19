package rpc

import (
	"example.com/aim/chat-service/kitex_gen/rag/ragservice"
	"github.com/cloudwego/kitex/client"
)

func NewRAGClient(addr string) (ragservice.Client, error) {
	return ragservice.NewClient("rag-service", client.WithHostPorts(addr))
}
