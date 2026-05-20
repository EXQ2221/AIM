package rpc

import (
	"example.com/aim/gateway/kitex_gen/rag/ragservice"
	"example.com/aim/shared/errno"
	"github.com/cloudwego/kitex/client"
)

var ragRPCClient ragservice.Client

func InitRAGClient(endpoint string) error {
	c, err := ragservice.NewClient("rag-service", client.WithHostPorts(endpoint))
	if err != nil {
		return err
	}
	ragRPCClient = c
	return nil
}

func RAGClient() (ragservice.Client, error) {
	if ragRPCClient == nil {
		return nil, errno.Internal("rag rpc client not initialized")
	}
	return ragRPCClient, nil
}
