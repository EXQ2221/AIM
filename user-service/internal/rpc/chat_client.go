package rpc

import (
	"context"

	chatpb "example.com/aim/chat-service/kitex_gen/chat"
	"example.com/aim/chat-service/kitex_gen/chat/chatservice"
	"github.com/cloudwego/kitex/client"
)

type ChatClient interface {
	CreateSingleConversation(ctx context.Context, operatorID, targetUserID uint64) (*chatpb.ConversationInfo, error)
}

type KitexChatClient struct {
	client chatservice.Client
}

func NewChatClient(addr string) (*KitexChatClient, error) {
	c, err := chatservice.NewClient("chat-service", client.WithHostPorts(addr))
	if err != nil {
		return nil, err
	}
	return &KitexChatClient{client: c}, nil
}

func (c *KitexChatClient) CreateSingleConversation(ctx context.Context, operatorID, targetUserID uint64) (*chatpb.ConversationInfo, error) {
	resp, err := c.client.CreateSingleConversation(ctx, &chatpb.CreateSingleConversationRequest{
		OperatorId:   int64(operatorID),
		TargetUserId: int64(targetUserID),
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}
	return resp.Conversation, nil
}
