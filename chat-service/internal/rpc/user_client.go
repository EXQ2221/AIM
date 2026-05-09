package rpc

import (
	"context"

	userpb "example.com/aim/chat-service/kitex_gen/user"
	"example.com/aim/chat-service/kitex_gen/user/userservice"
	"github.com/cloudwego/kitex/client"
)

type UserInfo struct {
	UserID   uint64
	Nickname string
	Avatar   string
	Status   string
}

type UserClient interface {
	GetUser(ctx context.Context, userID uint64) (*UserInfo, error)
	CheckFriendRelation(ctx context.Context, userID uint64, friendUserID uint64) (bool, error)
}

type KitexUserClient struct {
	client userservice.Client
}

func NewUserClient(addr string) (*KitexUserClient, error) {
	c, err := userservice.NewClient("user-service", client.WithHostPorts(addr))
	if err != nil {
		return nil, err
	}
	return &KitexUserClient{client: c}, nil
}

func (c *KitexUserClient) GetUser(ctx context.Context, userID uint64) (*UserInfo, error) {
	resp, err := c.client.GetUser(ctx, &userpb.GetUserRequest{UserId: int64(userID)})
	if err != nil {
		return nil, err
	}
	if resp.User == nil {
		return &UserInfo{UserID: userID}, nil
	}
	return &UserInfo{
		UserID:   uint64(resp.User.UserId),
		Nickname: resp.User.Nickname,
		Avatar:   resp.User.Avatar,
		Status:   resp.User.Status,
	}, nil
}

func (c *KitexUserClient) CheckFriendRelation(ctx context.Context, userID uint64, friendUserID uint64) (bool, error) {
	resp, err := c.client.CheckFriendRelation(ctx, &userpb.CheckFriendRelationRequest{
		UserId:       int64(userID),
		FriendUserId: int64(friendUserID),
	})
	if err != nil {
		return false, err
	}
	return resp.IsFriend, nil
}
