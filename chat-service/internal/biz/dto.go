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

type UpdateGroupAnnouncementInput struct {
	OperatorID     uint64
	ConversationID string
	Announcement   string
}

type InviteMemberInput struct {
	OperatorID   uint64
	TargetUserID uint64
}

type TransferOwnerInput struct {
	OperatorID     uint64
	ConversationID string
	TargetUserID   uint64
}

type SetAdminInput struct {
	OperatorID     uint64
	ConversationID string
	TargetUserID   uint64
}

type RemoveAdminInput struct {
	OperatorID     uint64
	ConversationID string
	TargetUserID   uint64
}

type MuteMemberInput struct {
	OperatorID     uint64
	ConversationID string
	TargetUserID   uint64
	MuteUntil      int64
}

type UnmuteMemberInput struct {
	OperatorID     uint64
	ConversationID string
	TargetUserID   uint64
}

type RemoveMemberInput struct {
	OperatorID     uint64
	ConversationID string
	TargetUserID   uint64
}

type SetGroupMuteAllInput struct {
	OperatorID     uint64
	ConversationID string
	MuteAll        bool
}

type MarkConversationReadInput struct {
	OperatorID        uint64
	ConversationID    string
	LastReadMessageID uint64
}

type RecallMessageInput struct {
	OperatorID     uint64
	ConversationID string
	MessageID      uint64
}

type GroupView struct {
	ConversationID        string
	Type                  string
	Name                  string
	Avatar                string
	Announcement          string
	AnnouncementUpdatedBy *uint64
	AnnouncementUpdatedAt *int64
	OwnerID               uint64
	JoinPolicy            string
	CreatedAt             int64
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
	MuteAll               *bool
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
	MuteUntil       *int64
}

type MessageView struct {
	ID             uint64
	ConversationID string
	SenderID       uint64
	SenderType     string
	MessageType    string
	Content        string
	ReplyToID      *uint64
	ReplyTo        *ReplyPreviewView
	Status         string
	CreatedAt      int64
	ReadByPeer     *bool
	ReadCount      *int32
}

type ReplyPreviewView struct {
	MessageID      uint64
	SenderID       uint64
	SenderType     string
	MessageType    string
	ContentPreview string
}

type ConversationEventView struct {
	Message          *MessageView
	RecipientUserIDs []uint64
}

type MessageRecalledEventView struct {
	MessageID        uint64
	ConversationID   string
	RecipientUserIDs []uint64
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

type CreateCustomBotInput struct {
	OperatorID      uint64
	Name            string
	MentionName     string
	Aliases         []string
	Description     string
	APIBaseURL      string
	APIKey          string
	ModelName       string
	SupportedModels []string
	SystemPrompt    string
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

type NotificationView struct {
	ID               uint64
	Type             string
	Title            string
	Content          string
	ConversationID   string
	RelatedMessageID *uint64
	IsRead           bool
	CreatedAt        int64
}

type NotificationListView struct {
	Notifications []NotificationView
	UnreadCount   int64
}

type CreateNotificationInput struct {
	OperatorID       uint64
	UserID           uint64
	Type             string
	Title            string
	Content          string
	ConversationID   string
	RelatedMessageID *uint64
}
