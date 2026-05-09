package model

type CreateGroupRequest struct {
	Name         string `json:"name"`
	Avatar       string `json:"avatar"`
	Announcement string `json:"announcement"`
	JoinPolicy   string `json:"joinPolicy"`
}

type GroupInfo struct {
	ConversationID string `json:"conversationId"`
	Type           string `json:"type"`
	Name           string `json:"name"`
	Avatar         string `json:"avatar"`
	Announcement   string `json:"announcement"`
	OwnerID        int64  `json:"ownerId"`
	JoinPolicy     string `json:"joinPolicy"`
	CreatedAt      int64  `json:"createdAt"`
}

type ConversationInfo struct {
	ConversationID        string `json:"conversationId"`
	Type                  string `json:"type"`
	Title                 string `json:"title"`
	Avatar                string `json:"avatar"`
	LastMessageID         *int64 `json:"lastMessageId,omitempty"`
	LastMessageAt         *int64 `json:"lastMessageAt,omitempty"`
	LastMessageSenderID   *int64 `json:"lastMessageSenderId,omitempty"`
	LastMessageSenderName string `json:"lastMessageSenderName"`
	LastMessageContent    string `json:"lastMessageContent"`
	Role                  string `json:"role"`
	IsPinned              bool   `json:"isPinned"`
	IsMuted               bool   `json:"isMuted"`
	UpdatedAt             int64  `json:"updatedAt"`
}

type MemberInfo struct {
	UserID          int64    `json:"userId"`
	Nickname        string   `json:"nickname"`
	Avatar          string   `json:"avatar"`
	Role            string   `json:"role"`
	Status          string   `json:"status"`
	JoinedAt        int64    `json:"joinedAt"`
	MemberType      string   `json:"memberType"`
	BotID           *int64   `json:"botId,omitempty"`
	MentionName     *string  `json:"mentionName,omitempty"`
	Aliases         []string `json:"aliases,omitempty"`
	Enabled         *bool    `json:"enabled,omitempty"`
	PermissionScope *string  `json:"permissionScope,omitempty"`
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

type AddConversationBotRequest struct {
	BotID               int64    `json:"botId"`
	DisplayNameOverride string   `json:"displayNameOverride"`
	MentionNameOverride string   `json:"mentionNameOverride"`
	AliasesOverride     []string `json:"aliasesOverride"`
	PermissionScope     string   `json:"permissionScope"`
	ModelNameOverride   string   `json:"modelNameOverride"`
}

type InviteMemberRequest struct {
	TargetUserID int64 `json:"targetUserId"`
}

type BotInfo struct {
	BotID           int64    `json:"botId"`
	MemberType      string   `json:"memberType"`
	MemberID        int64    `json:"memberId"`
	Name            string   `json:"name"`
	DisplayName     string   `json:"displayName"`
	MentionName     string   `json:"mentionName"`
	Aliases         []string `json:"aliases"`
	Avatar          string   `json:"avatar"`
	Description     string   `json:"description"`
	Enabled         bool     `json:"enabled"`
	PermissionScope string   `json:"permissionScope"`
	MemberStatus    string   `json:"memberStatus"`
	ModelName       string   `json:"modelName"`
	SupportedModels []string `json:"supportedModels"`
}

type AICallLogInfo struct {
	ID                int64  `json:"id"`
	ConversationID    string `json:"conversationId"`
	UserID            int64  `json:"userId"`
	BotID             int64  `json:"botId"`
	BotName           string `json:"botName"`
	RequestMessageID  *int64 `json:"requestMessageId,omitempty"`
	ResponseMessageID *int64 `json:"responseMessageId,omitempty"`
	ModelName         string `json:"modelName"`
	PromptTokens      int32  `json:"promptTokens"`
	CompletionTokens  int32  `json:"completionTokens"`
	TotalTokens       int32  `json:"totalTokens"`
	LatencyMS         int64  `json:"latencyMs"`
	Status            string `json:"status"`
	ErrorMessage      string `json:"errorMessage"`
	CreatedAt         int64  `json:"createdAt"`
}

type AICallLogQuotaInfo struct {
	DailyTotalTokens int64 `json:"dailyTotalTokens"`
	DailyTokenLimit  int64 `json:"dailyTokenLimit"`
	RemainingTokens  int64 `json:"remainingTokens"`
}

type AICallLogListResponse struct {
	Logs  []AICallLogInfo    `json:"logs"`
	Quota AICallLogQuotaInfo `json:"quota"`
}
