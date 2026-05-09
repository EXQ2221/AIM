package bot

import "context"

const BotReplyCreatedChannel = "aim:bot_reply_created"

type BotReplyMessageInfo struct {
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

type BotReplyCreatedEvent struct {
	Message          BotReplyMessageInfo `json:"message"`
	RecipientUserIDs []int64             `json:"recipientUserIds"`
}

type ReplyPublisher interface {
	PublishBotReplyCreated(ctx context.Context, event BotReplyCreatedEvent) error
}
