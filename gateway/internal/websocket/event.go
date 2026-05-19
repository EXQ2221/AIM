package websocket

import (
	"encoding/json"
	"strings"

	notificationx "example.com/aim/gateway/internal/notification"
	chatpb "example.com/aim/gateway/kitex_gen/chat"
)

const (
	EventConnected           = "CONNECTED"
	EventSendMessage         = "SEND_MESSAGE"
	EventTyping              = "TYPING"
	EventMessageAck          = "MESSAGE_ACK"
	EventNewMessage          = "NEW_MESSAGE"
	EventBotReplyStream      = "BOT_REPLY_STREAM"
	EventMessageRecalled     = "MESSAGE_RECALLED"
	EventNotificationCreated = "NOTIFICATION_CREATED"
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
	ConversationID string          `json:"conversationId"`
	MessageType    string          `json:"messageType,omitempty"`
	Content        string          `json:"content,omitempty"`
	ContentPayload json.RawMessage `json:"contentPayload,omitempty"`
	ReplyToID      *int64          `json:"replyToId,omitempty"`
}

type TypingData struct {
	ConversationID string `json:"conversationId"`
	IsTyping       bool   `json:"isTyping"`
	UserID         int64  `json:"userId,omitempty"`
	At             int64  `json:"at,omitempty"`
}

type MessageAckData struct {
	MessageID    int64  `json:"messageId,omitempty"`
	Status       string `json:"status"`
	ErrorCode    string `json:"errorCode,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

type MessageInfo struct {
	ID             int64             `json:"id"`
	ConversationID string            `json:"conversationId"`
	SenderID       int64             `json:"senderId"`
	SenderType     string            `json:"senderType"`
	MessageType    string            `json:"messageType"`
	Content        string            `json:"content"`
	ReplyToID      *int64            `json:"replyToId,omitempty"`
	ReplyTo        *ReplyPreviewInfo `json:"replyTo,omitempty"`
	Status         string            `json:"status"`
	CreatedAt      int64             `json:"createdAt"`
	ReadByPeer     *bool             `json:"readByPeer,omitempty"`
	ReadCount      *int32            `json:"readCount,omitempty"`
}

type ReplyPreviewInfo struct {
	MessageID      int64  `json:"messageId"`
	SenderID       int64  `json:"senderId"`
	SenderType     string `json:"senderType"`
	MessageType    string `json:"messageType"`
	ContentPreview string `json:"contentPreview"`
}

type MessageRecalledInfo struct {
	MessageID      int64  `json:"messageId"`
	ConversationID string `json:"conversationId"`
}

type BotReplyStreamData struct {
	ConversationID string `json:"conversationId"`
	SenderID       int64  `json:"senderId"`
	SenderType     string `json:"senderType"`
	MessageType    string `json:"messageType"`
	Content        string `json:"content"`
	Done           bool   `json:"done"`
}

type NotificationInfo struct {
	ID               int64  `json:"id"`
	Type             string `json:"type"`
	Category         string `json:"category"`
	Title            string `json:"title"`
	Summary          string `json:"summary"`
	Content          string `json:"content"`
	Detail           string `json:"detail"`
	ConversationID   string `json:"conversationId"`
	RelatedMessageID *int64 `json:"relatedMessageId,omitempty"`
	IsRead           bool   `json:"isRead"`
	CreatedAt        int64  `json:"createdAt"`
	Persistent       bool   `json:"persistent"`
}

type NotificationCreatedData struct {
	Notification NotificationInfo `json:"notification"`
	UnreadCount  *int64           `json:"unreadCount,omitempty"`
}

func ToMessageInfo(message *chatpb.MessageInfo) MessageInfo {
	if message == nil {
		return MessageInfo{}
	}
	var replyTo *ReplyPreviewInfo
	if message.ReplyTo != nil {
		replyTo = &ReplyPreviewInfo{
			MessageID:      message.ReplyTo.MessageId,
			SenderID:       message.ReplyTo.SenderId,
			SenderType:     message.ReplyTo.SenderType,
			MessageType:    message.ReplyTo.MessageType,
			ContentPreview: message.ReplyTo.ContentPreview,
		}
	}
	return MessageInfo{
		ID:             message.Id,
		ConversationID: message.ConversationId,
		SenderID:       message.SenderId,
		SenderType:     message.SenderType,
		MessageType:    message.MessageType,
		Content:        message.Content,
		ReplyToID:      message.ReplyToId,
		ReplyTo:        replyTo,
		Status:         message.Status,
		CreatedAt:      message.CreatedAt,
		ReadByPeer:     message.ReadByPeer,
		ReadCount:      message.ReadCount,
	}
}

func ToNotificationInfo(item *chatpb.NotificationInfo) NotificationInfo {
	if item == nil {
		return NotificationInfo{}
	}
	category, summary, detail := notificationx.Normalize(item.Type, item.Title, item.Content)
	return NotificationInfo{
		ID:               item.Id,
		Type:             item.Type,
		Category:         category,
		Title:            item.Title,
		Summary:          summary,
		Content:          item.Content,
		Detail:           detail,
		ConversationID:   item.ConversationId,
		RelatedMessageID: item.RelatedMessageId,
		IsRead:           item.IsRead,
		CreatedAt:        item.CreatedAt,
		Persistent:       true,
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
