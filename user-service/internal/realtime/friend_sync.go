package realtime

import "context"

const FriendSyncChannel = "aim:friend_sync"

const (
	FriendSyncReasonRequestCreated   = "REQUEST_CREATED"
	FriendSyncReasonRequestResponded = "REQUEST_RESPONDED"
	FriendSyncReasonGroupCreated     = "GROUP_CREATED"
	FriendSyncReasonFriendUpdated    = "FRIEND_UPDATED"
	FriendSyncReasonFriendDeleted    = "FRIEND_DELETED"
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

type FriendSyncPublisher interface {
	PublishFriendSync(ctx context.Context, event FriendSyncEvent) error
}
