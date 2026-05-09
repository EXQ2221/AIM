package websocket

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	chatpb "example.com/aim/gateway/kitex_gen/chat"
	"example.com/aim/gateway/kitex_gen/chat/chatservice"
	gwebsocket "github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 45 * time.Second
	sendBufferSize = 32
)

type Client struct {
	UserID int64

	conn       *gwebsocket.Conn
	hub        *Hub
	chatClient chatservice.Client
	send       chan OutgoingEvent
	done       chan struct{}
	closeOnce  sync.Once
}

func NewClient(userID int64, conn *gwebsocket.Conn, hub *Hub, chatClient chatservice.Client) *Client {
	return &Client{
		UserID:     userID,
		conn:       conn,
		hub:        hub,
		chatClient: chatClient,
		send:       make(chan OutgoingEvent, sendBufferSize),
		done:       make(chan struct{}),
	}
}

func (c *Client) Run(ctx context.Context) {
	c.hub.Add(c)
	defer c.hub.Remove(c)

	go c.writeLoop()
	c.Send(OutgoingEvent{
		Type: EventConnected,
		Data: ConnectedData{UserID: c.UserID},
	})
	c.readLoop(ctx)
}

func (c *Client) Send(event OutgoingEvent) bool {
	select {
	case <-c.done:
		return false
	case c.send <- event:
		return true
	default:
		c.Close()
		return false
	}
}

func (c *Client) Close() {
	c.closeOnce.Do(func() {
		close(c.done)
		_ = c.conn.Close()
	})
}

func (c *Client) readLoop(ctx context.Context) {
	c.conn.SetReadLimit(64 * 1024)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		var event IncomingEvent
		if err := c.conn.ReadJSON(&event); err != nil {
			return
		}

		switch strings.ToUpper(strings.TrimSpace(event.Type)) {
		case EventSendMessage:
			c.handleSendMessage(ctx, event)
		default:
			c.sendFailedAck(event.ClientMsgID, "BAD_REQUEST", "unsupported event type")
		}
	}
}

func (c *Client) writeLoop() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case event := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteJSON(event); err != nil {
				c.Close()
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(gwebsocket.PingMessage, nil); err != nil {
				c.Close()
				return
			}
		case <-c.done:
			return
		}
	}
}

func (c *Client) handleSendMessage(ctx context.Context, event IncomingEvent) {
	var payload SendMessageData
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		c.sendFailedAck(event.ClientMsgID, "BAD_REQUEST", "invalid message data")
		return
	}
	payload.ConversationID = strings.TrimSpace(payload.ConversationID)
	if payload.ConversationID == "" {
		c.sendFailedAck(event.ClientMsgID, "BAD_REQUEST", "conversationId is required")
		return
	}

	resp, err := c.chatClient.CreateMessage(ctx, &chatpb.CreateMessageRequest{
		OperatorId:     c.UserID,
		ConversationId: payload.ConversationID,
		Content:        payload.Content,
		ReplyToId:      payload.ReplyToID,
	})
	if err != nil {
		c.sendFailedAck(event.ClientMsgID, errorCode(err.Error()), publicErrorMessage(err.Error()))
		return
	}
	if resp == nil || resp.Message == nil {
		c.sendFailedAck(event.ClientMsgID, "INTERNAL_ERROR", "message response is empty")
		return
	}

	c.Send(OutgoingEvent{
		Type:        EventMessageAck,
		ClientMsgID: event.ClientMsgID,
		Data: MessageAckData{
			MessageID: resp.Message.Id,
			Status:    "SUCCESS",
		},
	})
	c.broadcastNewMessage(ctx, resp.Message)
}

func (c *Client) broadcastNewMessage(ctx context.Context, message *chatpb.MessageInfo) {
	members, err := c.chatClient.ListMembers(ctx, &chatpb.ListMembersRequest{
		OperatorId:     c.UserID,
		ConversationId: message.ConversationId,
	})
	if err != nil || members == nil {
		return
	}

	userIDs := make([]int64, 0, len(members.Members))
	for _, member := range members.Members {
		if member != nil && member.UserId != c.UserID && strings.EqualFold(member.MemberType, "USER") {
			userIDs = append(userIDs, member.UserId)
		}
	}

	c.hub.SendToUsers(userIDs, OutgoingEvent{
		Type: EventNewMessage,
		Data: toMessageInfo(message),
	})
}

func (c *Client) sendFailedAck(clientMsgID, code, message string) {
	c.Send(OutgoingEvent{
		Type:        EventMessageAck,
		ClientMsgID: clientMsgID,
		Data: MessageAckData{
			Status:       "FAILED",
			ErrorCode:    code,
			ErrorMessage: message,
		},
	})
}
