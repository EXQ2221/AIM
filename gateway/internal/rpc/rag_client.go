package rpc

import (
	"errors"

	"example.com/aim/gateway/kitex_gen/rag/ragservice"
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
		return nil, errors.New("rag rpc client not initialized")
	}
	return ragRPCClient, nil
}
