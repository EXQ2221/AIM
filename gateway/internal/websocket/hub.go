package websocket

import "sync"

type Hub struct {
	mu        sync.RWMutex
	userConns map[int64]map[*Client]struct{}
}

func NewHub() *Hub {
	return &Hub{
		userConns: make(map[int64]map[*Client]struct{}),
	}
}

func (h *Hub) Add(client *Client) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	wasOffline := len(h.userConns[client.UserID]) == 0
	if h.userConns[client.UserID] == nil {
		h.userConns[client.UserID] = make(map[*Client]struct{})
	}
	h.userConns[client.UserID][client] = struct{}{}
	return wasOffline
}

func (h *Hub) Remove(client *Client) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	conns := h.userConns[client.UserID]
	if len(conns) == 0 {
		client.Close()
		return false
	}
	delete(conns, client)
	becameOffline := len(conns) == 0
	if len(conns) == 0 {
		delete(h.userConns, client.UserID)
	}
	client.Close()
	return becameOffline
}

func (h *Hub) SendToUsers(userIDs []int64, event OutgoingEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, userID := range userIDs {
		for client := range h.userConns[userID] {
			client.Send(event)
		}
	}
}

func (h *Hub) IsUserOnline(userID int64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.userConns[userID]) > 0
}

func (h *Hub) BroadcastFriendPresence(friendUserID int64, presence string) {
	h.mu.RLock()
	recipients := make([]int64, 0, len(h.userConns))
	for userID := range h.userConns {
		recipients = append(recipients, userID)
	}
	h.mu.RUnlock()
	if len(recipients) == 0 {
		return
	}
	h.SendToUsers(recipients, OutgoingEvent{
		Type: EventFriendSync,
		Data: FriendSyncData{
			Reason:       "PRESENCE_CHANGED",
			FriendUserID: friendUserID,
			Status:       presence,
		},
	})
}
