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

func (h *Hub) Add(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.userConns[client.UserID] == nil {
		h.userConns[client.UserID] = make(map[*Client]struct{})
	}
	h.userConns[client.UserID][client] = struct{}{}
}

func (h *Hub) Remove(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	conns := h.userConns[client.UserID]
	if len(conns) == 0 {
		client.Close()
		return
	}
	delete(conns, client)
	if len(conns) == 0 {
		delete(h.userConns, client.UserID)
	}
	client.Close()
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
