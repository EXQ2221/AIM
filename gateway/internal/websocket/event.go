package websocket

import (
	"encoding/json"
	"strings"

	chatpb "example.com/aim/gateway/kitex_gen/chat"
)

const (
	EventConnected   = "CONNECTED"
	EventSendMessage = "SEND_MESSAGE"
	EventMessageAck  = "MESSAGE_ACK"
	EventNewMessage  = "NEW_MESSAGE"
)

type IncomingEvent struct {
	Type        string          `json:"type"`
	ClientMsgID string          `json:"clientMsgId,omitempty"`
	Data        json.RawMessage `json:"data"`
}

type OutgoingEvent struct {
	Type        string `json:"type"`
	ClientMsgID string `json:"clientMsgId,omitempty"`
	Data        any    `json:"data,omitempty"`
}

type ConnectedData struct {
	UserID int64 `json:"userId"`
}

type SendMessageData struct {
	ConversationID string `json:"conversationId"`
	Content        string `json:"content"`
	ReplyToID      *int64 `json:"replyToId,omitempty"`
}

type MessageAckData struct {
	MessageID    int64  `json:"messageId,omitempty"`
	Status       string `json:"status"`
	ErrorCode    string `json:"errorCode,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

type MessageInfo struct {
	ID             int64  `json:"id"`
	ConversationID string `json:"conversationId"`
	SenderID       int64  `json:"senderId"`
	SenderType     string `json:"senderType"`
	MessageType    string `json:"messageType"`
	Content        string `json:"content"`
	ReplyToID      *int64 `json:"replyToId,omitempty"`
	Status         string `json:"status"`
	CreatedAt      int64  `json:"createdAt"`
}

func toMessageInfo(message *chatpb.MessageInfo) MessageInfo {
	if message == nil {
		return MessageInfo{}
	}
	return MessageInfo{
		ID:             message.Id,
		ConversationID: message.ConversationId,
		SenderID:       message.SenderId,
		SenderType:     message.SenderType,
		MessageType:    message.MessageType,
		Content:        message.Content,
		ReplyToID:      message.ReplyToId,
		Status:         message.Status,
		CreatedAt:      message.CreatedAt,
	}
}

func errorCode(message string) string {
	switch {
	case strings.Contains(message, "bad_request:"):
		return "BAD_REQUEST"
	case strings.Contains(message, "unauthorized:"):
		return "UNAUTHORIZED"
	case strings.Contains(message, "forbidden:"):
		return "FORBIDDEN"
	case strings.Contains(message, "not_found:"):
		return "NOT_FOUND"
	default:
		return "INTERNAL_ERROR"
	}
}

func publicErrorMessage(message string) string {
	for _, prefix := range []string{"bad_request: ", "unauthorized: ", "forbidden: ", "not_found: "} {
		if idx := strings.Index(message, prefix); idx >= 0 {
			return message[idx+len(prefix):]
		}
	}
	return message
}
