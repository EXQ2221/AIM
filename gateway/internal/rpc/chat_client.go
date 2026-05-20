package rpc

import (
	"example.com/aim/gateway/kitex_gen/chat/chatservice"
	"example.com/aim/shared/errno"
	"github.com/cloudwego/kitex/client"
)

var chatRPCClient chatservice.Client

func InitChatClient(endpoint string) error {
	c, err := chatservice.NewClient("chat-service", client.WithHostPorts(endpoint))
	if err != nil {
		return err
	}
	chatRPCClient = c
	return nil
}

func ChatClient() (chatservice.Client, error) {
	if chatRPCClient == nil {
		return nil, errno.Internal("chat rpc client not initialized")
	}
	return chatRPCClient, nil
}
