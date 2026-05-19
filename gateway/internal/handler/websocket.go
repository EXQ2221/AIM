package handler

import (
	"context"

	gatewayws "example.com/aim/gateway/internal/websocket"
	"github.com/gin-gonic/gin"
	redisv9 "github.com/redis/go-redis/v9"
)

var chatHub = gatewayws.NewHub()

func StartBotReplySubscriber(ctx context.Context, client *redisv9.Client) {
	gatewayws.StartBotReplySubscriber(ctx, client, chatHub)
}

func StartBotReplyStreamSubscriber(ctx context.Context, client *redisv9.Client) {
	gatewayws.StartBotReplyStreamSubscriber(ctx, client, chatHub)
}

func StartFriendSyncSubscriber(ctx context.Context, client *redisv9.Client) {
	gatewayws.StartFriendSyncSubscriber(ctx, client, chatHub)
}

func ChatWebSocket(ctx *gin.Context) {
	gatewayws.HandleChat(ctx, chatHub)
}
