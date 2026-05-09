package biz

type CreateGroupInput struct {
	OperatorID   uint64
	Name         string
	Avatar       string
	Announcement string
	JoinPolicy   string
}

type CreateSingleConversationInput struct {
	OperatorID uint64
	TargetID   uint64
}

type InviteMemberInput struct {
	OperatorID  uint64
	TargetUserID uint64
}

type GroupView struct {
	ConversationID string
	Type           string
	Name           string
	Avatar         string
	Announcement   string
	OwnerID        uint64
	JoinPolicy     string
	CreatedAt      int64
}

type ConversationView struct {
	ConversationID        string
	Type                  string
	Title                 string
	Avatar                string
	LastMessageID         *uint64
	LastMessageAt         *int64
	LastMessageSenderID   *uint64
	LastMessageSenderType string
	LastMessageSenderName string
	LastMessageContent    string
	Role                  string
	IsPinned              bool
	IsMuted               bool
	UpdatedAt             int64
}

type MemberView struct {
	UserID   uint64
	Nickname string
	Avatar   string
	Role     string
	Status   string
	JoinedAt int64
}

type MemberListView struct {
	UserID          uint64
	Nickname        string
	Avatar          string
	Role            string
	Status          string
	JoinedAt        int64
	MemberType      string
	BotID           uint64
	MentionName     string
	Aliases         []string
	Enabled         *bool
	PermissionScope string
}

type MessageView struct {
	ID             uint64
	ConversationID string
	SenderID       uint64
	SenderType     string
	MessageType    string
	Content        string
	ReplyToID      *uint64
	Status         string
	CreatedAt      int64
}

type AddConversationBotInput struct {
	OperatorID          uint64
	ConversationID      string
	BotID               uint64
	DisplayNameOverride string
	MentionNameOverride string
	AliasesOverride     []string
	PermissionScope     string
	ModelNameOverride   string
}

type BotView struct {
	BotID           uint64
	MemberType      string
	MemberID        uint64
	Name            string
	DisplayName     string
	MentionName     string
	Aliases         []string
	Avatar          string
	Description     string
	Enabled         bool
	PermissionScope string
	MemberStatus    string
	ModelName       string
	SupportedModels []string
}

type AICallLogView struct {
	ID                uint64
	ConversationID    string
	UserID            uint64
	BotID             uint64
	BotName           string
	RequestMessageID  *uint64
	ResponseMessageID *uint64
	ModelName         string
	PromptTokens      int
	CompletionTokens  int
	TotalTokens       int
	LatencyMS         int64
	Status            string
	ErrorMessage      string
	CreatedAt         int64
}

type AICallLogQuotaView struct {
	DailyTotalTokens int64
	DailyTokenLimit  int64
	RemainingTokens  int64
}

type AICallLogListView struct {
	Logs  []AICallLogView
	Quota AICallLogQuotaView
}
