package rpc

import (
	"example.com/aim/gateway/kitex_gen/user/userservice"
	"example.com/aim/shared/errno"
	"github.com/cloudwego/kitex/client"
)

var userRPCClient userservice.Client

func InitUserClient(endpoint string) error {
	c, err := userservice.NewClient("user-service", client.WithHostPorts(endpoint))
	if err != nil {
		return err
	}
	userRPCClient = c
	return nil
}

func UserClient() (userservice.Client, error) {
	if userRPCClient == nil {
		return nil, errno.Internal("user rpc client not initialized")
	}
	return userRPCClient, nil
}
