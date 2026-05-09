package redis

import (
	"context"

	redisv9 "github.com/redis/go-redis/v9"
)

func Init(addr string) (*redisv9.Client, error) {
	client := redisv9.NewClient(&redisv9.Options{Addr: addr})
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}
	return client, nil
}
