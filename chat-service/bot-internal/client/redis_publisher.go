package botclient

import (
	"context"
	"encoding/json"
	"errors"

	botmodel "example.com/aim/chat-service/bot-internal/model"
	redisv9 "github.com/redis/go-redis/v9"
)

const BotReplyCreatedChannel = "aim:bot_reply_created"
const BotReplyStreamChannel = "aim:bot_reply_stream"

type ReplyPublisher interface {
	PublishBotReplyCreated(ctx context.Context, event botmodel.BotReplyCreatedEvent) error
	PublishBotReplyStream(ctx context.Context, event botmodel.BotReplyStreamEvent) error
}

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

func (p *RedisReplyPublisher) PublishBotReplyCreated(ctx context.Context, event botmodel.BotReplyCreatedEvent) error {
	if p == nil || p.client == nil {
		return errors.New("redis reply publisher is nil")
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return p.client.Publish(ctx, p.channel, payload).Err()
}

func (p *RedisReplyPublisher) PublishBotReplyStream(ctx context.Context, event botmodel.BotReplyStreamEvent) error {
	if p == nil || p.client == nil {
		return errors.New("redis reply publisher is nil")
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return p.client.Publish(ctx, BotReplyStreamChannel, payload).Err()
}

var _ ReplyPublisher = (*RedisReplyPublisher)(nil)
