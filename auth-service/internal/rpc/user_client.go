package rpc

import (
	"context"
	"errors"

	userpb "example.com/aim/auth-service/kitex_gen/user"
	"example.com/aim/auth-service/kitex_gen/user/userservice"
	"example.com/aim/shared/errno"
	"github.com/cloudwego/kitex/client"
)

type UserInfo struct {
	UserID       uint64
	AimID        string
	Email        string
	Nickname     string
	Status       string
	Role         string
	TokenVersion uint64
}

type UserClient interface {
	CreateUser(ctx context.Context, aimID, email, nickname, password string) (*UserInfo, error)
	GetUser(ctx context.Context, userID uint64) (*UserInfo, error)
	VerifyCredential(ctx context.Context, email, password string) (*UserInfo, bool, error)
	CheckPassword(ctx context.Context, userID uint64, password string) (bool, error)
	UpdateLoginState(ctx context.Context, userID uint64, ip string) error
	BumpTokenVersion(ctx context.Context, userID uint64) error
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

func (c *KitexUserClient) CreateUser(ctx context.Context, aimID, email, nickname, password string) (*UserInfo, error) {
	resp, err := c.client.CreateUser(ctx, &userpb.CreateUserRequest{
		AimId:    aimID,
		Email:    email,
		Nickname: nickname,
		Password: password,
	})
	if err != nil {
		return nil, err
	}
	return fromUserPB(resp.User), nil
}

func (c *KitexUserClient) GetUser(ctx context.Context, userID uint64) (*UserInfo, error) {
	resp, err := c.client.GetUser(ctx, &userpb.GetUserRequest{UserId: int64(userID)})
	if err != nil {
		return nil, err
	}
	return fromUserPB(resp.User), nil
}

func (c *KitexUserClient) VerifyCredential(ctx context.Context, email, password string) (*UserInfo, bool, error) {
	resp, err := c.client.VerifyCredential(ctx, &userpb.VerifyCredentialRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		return nil, false, err
	}
	if resp.Ok && resp.User != nil {
		return fromUserPB(resp.User), true, nil
	}

	user := fromUserPB(resp.User)
	switch resp.Reason {
	case "user not found":
		return user, false, errno.New(errno.ErrUserNotFound, resp.Reason)
	case "password incorrect":
		if user != nil {
			return user, false, errno.NewWithUser(errno.ErrPasswordWrong, resp.Reason, user.UserID)
		}
		return user, false, errno.New(errno.ErrPasswordWrong, resp.Reason)
	case "user is not available":
		if user != nil {
			return user, false, errno.NewWithUser(errno.ErrUserNotAvailable, resp.Reason, user.UserID)
		}
		return user, false, errno.New(errno.ErrUserNotAvailable, resp.Reason)
	default:
		if resp.Reason == "" {
			return user, false, errno.New(errno.ErrUnauthorized, "invalid credentials")
		}
		return user, false, errors.New(resp.Reason)
	}
}

func (c *KitexUserClient) CheckPassword(ctx context.Context, userID uint64, password string) (bool, error) {
	resp, err := c.client.CheckPassword(ctx, &userpb.CheckPasswordRequest{
		UserId:   int64(userID),
		Password: password,
	})
	if err != nil {
		return false, err
	}
	return resp.Ok, nil
}

func (c *KitexUserClient) UpdateLoginState(ctx context.Context, userID uint64, ip string) error {
	resp, err := c.client.UpdateLoginState(ctx, &userpb.UpdateLoginStateRequest{
		UserId:      int64(userID),
		LastLoginIp: ip,
	})
	if err != nil {
		return err
	}
	if !resp.Success {
		return errors.New(resp.Message)
	}
	return nil
}

func (c *KitexUserClient) BumpTokenVersion(ctx context.Context, userID uint64) error {
	resp, err := c.client.BumpTokenVersion(ctx, &userpb.BumpTokenVersionRequest{UserId: int64(userID)})
	if err != nil {
		return err
	}
	if !resp.Success {
		return errors.New(resp.Message)
	}
	return nil
}

func fromUserPB(user *userpb.UserInfo) *UserInfo {
	if user == nil {
		return nil
	}
	return &UserInfo{
		UserID:       uint64(user.UserId),
		AimID:        user.AimId,
		Email:        user.Email,
		Nickname:     user.Nickname,
		Status:       user.Status,
		Role:         user.Role,
		TokenVersion: uint64(user.TokenVersion),
	}
}
