package realtime

import (
	"context"
	"encoding/json"
	"errors"

	redisv9 "github.com/redis/go-redis/v9"
)

type RedisFriendSyncPublisher struct {
	client  *redisv9.Client
	channel string
}

func NewRedisFriendSyncPublisher(client *redisv9.Client) *RedisFriendSyncPublisher {
	return &RedisFriendSyncPublisher{
		client:  client,
		channel: FriendSyncChannel,
	}
}

func (p *RedisFriendSyncPublisher) PublishFriendSync(ctx context.Context, event FriendSyncEvent) error {
	if p == nil || p.client == nil {
		return errors.New("redis friend sync publisher is nil")
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return p.client.Publish(ctx, p.channel, payload).Err()
}

var _ FriendSyncPublisher = (*RedisFriendSyncPublisher)(nil)
