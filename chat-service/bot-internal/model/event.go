package botmodel

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

type BotReplyStreamInfo struct {
	ConversationID string `json:"conversationId"`
	SenderID       int64  `json:"senderId"`
	SenderType     string `json:"senderType"`
	MessageType    string `json:"messageType"`
	Content        string `json:"content"`
	Done           bool   `json:"done"`
}

type BotReplyStreamEvent struct {
	Stream           BotReplyStreamInfo `json:"stream"`
	RecipientUserIDs []int64            `json:"recipientUserIds"`
}
