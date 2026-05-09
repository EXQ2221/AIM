package websocket

import (
	"context"
	"encoding/json"
	"log"

	redisv9 "github.com/redis/go-redis/v9"
)

const (
	FriendSyncChannel = "aim:friend_sync"
	EventFriendSync   = "FRIEND_SYNC"
)

type FriendSyncEvent struct {
	Reason         string  `json:"reason"`
	RequestID      int64   `json:"requestId,omitempty"`
	Status         string  `json:"status,omitempty"`
	ActorUserID    int64   `json:"actorUserId,omitempty"`
	FriendUserID   int64   `json:"friendUserId,omitempty"`
	ConversationID string  `json:"conversationId,omitempty"`
	UserIDs        []int64 `json:"userIds"`
}

type FriendSyncData struct {
	Reason         string `json:"reason"`
	RequestID      int64  `json:"requestId,omitempty"`
	Status         string `json:"status,omitempty"`
	ActorUserID    int64  `json:"actorUserId,omitempty"`
	FriendUserID   int64  `json:"friendUserId,omitempty"`
	ConversationID string `json:"conversationId,omitempty"`
}

func StartFriendSyncSubscriber(ctx context.Context, client *redisv9.Client, hub *Hub) {
	if client == nil || hub == nil {
		return
	}
	go runFriendSyncSubscriber(ctx, client, hub)
}

func runFriendSyncSubscriber(ctx context.Context, client *redisv9.Client, hub *Hub) {
	pubsub := client.Subscribe(ctx, FriendSyncChannel)
	defer pubsub.Close()

	if _, err := pubsub.Receive(ctx); err != nil {
		log.Printf("friend sync subscriber failed to subscribe: %v", err)
		return
	}
	ch := pubsub.Channel()
	log.Printf("friend sync subscriber listening on %s", FriendSyncChannel)

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var event FriendSyncEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				log.Printf("friend sync subscriber ignored invalid payload: %v", err)
				continue
			}
			if len(event.UserIDs) == 0 || event.Reason == "" {
				continue
			}
			hub.SendToUsers(event.UserIDs, OutgoingEvent{
				Type: EventFriendSync,
				Data: FriendSyncData{
					Reason:         event.Reason,
					RequestID:      event.RequestID,
					Status:         event.Status,
					ActorUserID:    event.ActorUserID,
					FriendUserID:   event.FriendUserID,
					ConversationID: event.ConversationID,
				},
			})
		}
	}
}
