package websocket

import (
	"context"
	"encoding/json"
	"log"

	redisv9 "github.com/redis/go-redis/v9"
)

const BotReplyStreamChannel = "aim:bot_reply_stream"

type BotReplyStreamEvent struct {
	Stream           BotReplyStreamData `json:"stream"`
	RecipientUserIDs []int64            `json:"recipientUserIds"`
}

func StartBotReplyStreamSubscriber(ctx context.Context, client *redisv9.Client, hub *Hub) {
	if client == nil || hub == nil {
		return
	}
	go runBotReplyStreamSubscriber(ctx, client, hub)
}

func runBotReplyStreamSubscriber(ctx context.Context, client *redisv9.Client, hub *Hub) {
	pubsub := client.Subscribe(ctx, BotReplyStreamChannel)
	defer pubsub.Close()

	if _, err := pubsub.Receive(ctx); err != nil {
		log.Printf("bot reply stream subscriber failed to subscribe: %v", err)
		return
	}
	ch := pubsub.Channel()
	log.Printf("bot reply stream subscriber listening on %s", BotReplyStreamChannel)

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var event BotReplyStreamEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				log.Printf("bot reply stream subscriber ignored invalid payload: %v", err)
				continue
			}
			if event.Stream.ConversationID == "" || len(event.RecipientUserIDs) == 0 {
				continue
			}
			hub.SendToUsers(event.RecipientUserIDs, OutgoingEvent{
				Type: EventBotReplyStream,
				Data: event.Stream,
			})
		}
	}
}
