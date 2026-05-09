package websocket

import (
	"context"
	"encoding/json"
	"log"

	redisv9 "github.com/redis/go-redis/v9"
)

const BotReplyCreatedChannel = "aim:bot_reply_created"

type BotReplyCreatedEvent struct {
	Message          MessageInfo `json:"message"`
	RecipientUserIDs []int64     `json:"recipientUserIds"`
}

func StartBotReplySubscriber(ctx context.Context, client *redisv9.Client, hub *Hub) {
	if client == nil || hub == nil {
		return
	}
	go runBotReplySubscriber(ctx, client, hub)
}

func runBotReplySubscriber(ctx context.Context, client *redisv9.Client, hub *Hub) {
	pubsub := client.Subscribe(ctx, BotReplyCreatedChannel)
	defer pubsub.Close()

	if _, err := pubsub.Receive(ctx); err != nil {
		log.Printf("bot reply subscriber failed to subscribe: %v", err)
		return
	}
	ch := pubsub.Channel()
	log.Printf("bot reply subscriber listening on %s", BotReplyCreatedChannel)

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var event BotReplyCreatedEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				log.Printf("bot reply subscriber ignored invalid payload: %v", err)
				continue
			}
			if event.Message.ID == 0 || event.Message.ConversationID == "" || len(event.RecipientUserIDs) == 0 {
				continue
			}
			hub.SendToUsers(event.RecipientUserIDs, OutgoingEvent{
				Type: EventNewMessage,
				Data: event.Message,
			})
		}
	}
}
