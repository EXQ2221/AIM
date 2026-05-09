package rpc

import (
	"errors"

	"example.com/aim/gateway/kitex_gen/auth/authservice"
	"github.com/cloudwego/kitex/client"
)

var authRPCClient authservice.Client

func InitAuthClient(endpoint string) error {
	c, err := authservice.NewClient("auth-service", client.WithHostPorts(endpoint))
	if err != nil {
		return err
	}
	authRPCClient = c
	return nil
}

func AuthClient() (authservice.Client, error) {
	if authRPCClient == nil {
		return nil, errors.New("auth rpc client not initialized")
	}
	return authRPCClient, nil
}
