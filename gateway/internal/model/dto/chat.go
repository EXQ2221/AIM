package dto

type CreateGroupRequest struct {
	Name         string `json:"name"`
	Avatar       string `json:"avatar"`
	Announcement string `json:"announcement"`
	JoinPolicy   string `json:"joinPolicy"`
}

type GroupInfo struct {
	ConversationID        string `json:"conversationId"`
	Type                  string `json:"type"`
	Name                  string `json:"name"`
	Avatar                string `json:"avatar"`
	Announcement          string `json:"announcement"`
	AnnouncementUpdatedBy *int64 `json:"announcementUpdatedBy,omitempty"`
	AnnouncementUpdatedAt *int64 `json:"announcementUpdatedAt,omitempty"`
	OwnerID               int64  `json:"ownerId"`
	JoinPolicy            string `json:"joinPolicy"`
	CreatedAt             int64  `json:"createdAt"`
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
	MuteAll               *bool  `json:"muteAll,omitempty"`
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
	MuteUntil       *int64   `json:"muteUntil,omitempty"`
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

type MarkConversationReadRequest struct {
	LastReadMessageID int64 `json:"lastReadMessageId"`
}

type MessageRecalledEventInfo struct {
	MessageID      int64  `json:"messageId"`
	ConversationID string `json:"conversationId"`
}

type AddConversationBotRequest struct {
	BotID               int64    `json:"botId"`
	DisplayNameOverride string   `json:"displayNameOverride"`
	MentionNameOverride string   `json:"mentionNameOverride"`
	AliasesOverride     []string `json:"aliasesOverride"`
	PermissionScope     string   `json:"permissionScope"`
	ModelNameOverride   string   `json:"modelNameOverride"`
}

type CreateCustomBotRequest struct {
	Name            string   `json:"name"`
	MentionName     string   `json:"mentionName"`
	Aliases         []string `json:"aliases"`
	Description     string   `json:"description"`
	APIBaseURL      string   `json:"apiBaseUrl"`
	APIKey          string   `json:"apiKey"`
	ModelName       string   `json:"modelName"`
	SupportedModels []string `json:"supportedModels"`
	SystemPrompt    string   `json:"systemPrompt"`
}

type InviteMemberRequest struct {
	TargetUserID int64 `json:"targetUserId"`
}

type TransferOwnerRequest struct {
	TargetUserID int64 `json:"targetUserId"`
}

type SetAdminRequest struct {
	TargetUserID int64 `json:"targetUserId"`
}

type MuteMemberRequest struct {
	MuteUntil int64 `json:"muteUntil"`
}

type UpdateGroupAnnouncementRequest struct {
	Announcement string `json:"announcement"`
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

type CreateKnowledgeBaseRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type KnowledgeBaseInfo struct {
	KnowledgeBaseID int64  `json:"knowledgeBaseId"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	Status          string `json:"status"`
}

type ListKnowledgeBasesResponse struct {
	KnowledgeBases []KnowledgeBaseInfo `json:"knowledgeBases"`
}

type AddKnowledgeDocumentTextRequest struct {
	Title      string `json:"title"`
	SourceType string `json:"sourceType"`
	Content    string `json:"content"`
}

type KnowledgeDocumentInfo struct {
	DocumentID      int64  `json:"documentId"`
	KnowledgeBaseID int64  `json:"knowledgeBaseId"`
	Title           string `json:"title"`
	SourceType      string `json:"sourceType"`
	Status          string `json:"status"`
	ErrorMessage    string `json:"errorMessage"`
	CreatedAt       int64  `json:"createdAt"`
}

type SearchKnowledgeBaseRequest struct {
	Query string `json:"query"`
	TopK  *int32 `json:"topK,omitempty"`
}

type KnowledgeSearchChunkInfo struct {
	ChunkID    int64   `json:"chunkId"`
	DocumentID int64   `json:"documentId"`
	Score      float64 `json:"score"`
	Content    string  `json:"content"`
}

type BindConversationKnowledgeBaseRequest struct {
	KnowledgeBaseID int64 `json:"knowledgeBaseId"`
}

type ConversationKnowledgeBaseInfo struct {
	ID              int64  `json:"id"`
	ConversationID  string `json:"conversationId"`
	KnowledgeBaseID int64  `json:"knowledgeBaseId"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	Status          string `json:"status"`
	Enabled         bool   `json:"enabled"`
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
}

type NotificationListResponse struct {
	Notifications []NotificationInfo `json:"notifications"`
	UnreadCount   int64              `json:"unreadCount"`
}
