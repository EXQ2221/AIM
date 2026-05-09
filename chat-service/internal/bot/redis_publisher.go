package bot

import (
	"context"
	"encoding/json"
	"errors"

	redisv9 "github.com/redis/go-redis/v9"
)

type RedisReplyPublisher struct {
	client  *redisv9.Client
	channel string
}

func NewRedisReplyPublisher(client *redisv9.Client) *RedisReplyPublisher {
	return &RedisReplyPublisher{
		client:  client,
		channel: BotReplyCreatedChannel,
	}
}

func (p *RedisReplyPublisher) PublishBotReplyCreated(ctx context.Context, event BotReplyCreatedEvent) error {
	if p == nil || p.client == nil {
		return errors.New("redis reply publisher is nil")
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return p.client.Publish(ctx, p.channel, payload).Err()
}

var _ ReplyPublisher = (*RedisReplyPublisher)(nil)
