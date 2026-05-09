package model

import (
	"time"

	"gorm.io/gorm"
)

type ConversationType string

const (
	ConversationTypeSingle ConversationType = "SINGLE"
	ConversationTypeGroup  ConversationType = "GROUP"
	ConversationTypeBot    ConversationType = "BOT"
	ConversationTypeSystem ConversationType = "SYSTEM"
)

type Conversation struct {
	ID             uint64           `gorm:"primaryKey;autoIncrement" json:"id"`
	ConversationID string           `gorm:"column:conversation_id;type:varchar(32);uniqueIndex" json:"conversationId"`
	Type           ConversationType `gorm:"type:varchar(32);not null;index" json:"type"`
	Title          string           `gorm:"type:varchar(128)" json:"title"`
	Avatar         string           `gorm:"type:varchar(512)" json:"avatar"`
	CreatedBy      uint64           `gorm:"not null;index" json:"createdBy"`
	LastMessageID  *uint64          `gorm:"index" json:"lastMessageId"`
	LastMessageAt  *time.Time       `gorm:"index" json:"lastMessageAt"`
	CreatedAt      time.Time        `json:"createdAt"`
	UpdatedAt      time.Time        `json:"updatedAt"`
	DeletedAt      gorm.DeletedAt   `gorm:"index" json:"-"`
}

func (Conversation) TableName() string {
	return "conversations"
}

type GroupJoinPolicy string

const (
	GroupJoinFree       GroupJoinPolicy = "FREE"
	GroupJoinApproval   GroupJoinPolicy = "APPROVAL"
	GroupJoinInviteOnly GroupJoinPolicy = "INVITE_ONLY"
)

type GroupInfo struct {
	ID             uint64          `gorm:"primaryKey;autoIncrement" json:"id"`
	ConversationID uint64          `gorm:"not null;uniqueIndex" json:"conversationId"`
	Name           string          `gorm:"type:varchar(128);not null" json:"name"`
	Avatar         string          `gorm:"type:varchar(512)" json:"avatar"`
	Announcement   string          `gorm:"type:text" json:"announcement"`
	OwnerID        uint64          `gorm:"not null;index" json:"ownerId"`
	JoinPolicy     GroupJoinPolicy `gorm:"type:varchar(32);not null;default:'INVITE_ONLY'" json:"joinPolicy"`
	MuteAll        bool            `gorm:"not null;default:false" json:"muteAll"`
	MaxMembers     int             `gorm:"not null;default:500" json:"maxMembers"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

func (GroupInfo) TableName() string {
	return "group_infos"
}

type MemberType string

const (
	MemberTypeUser MemberType = "USER"
	MemberTypeBot  MemberType = "BOT"
)

type ConversationMemberRole string

const (
	MemberRoleOwner  ConversationMemberRole = "OWNER"
	MemberRoleAdmin  ConversationMemberRole = "ADMIN"
	MemberRoleMember ConversationMemberRole = "MEMBER"
	MemberRoleBot    ConversationMemberRole = "BOT"
)

type ConversationMemberStatus string

const (
	MemberStatusNormal  ConversationMemberStatus = "NORMAL"
	MemberStatusMuted   ConversationMemberStatus = "MUTED"
	MemberStatusRemoved ConversationMemberStatus = "REMOVED"
)

type ConversationMember struct {
	ID                uint64                   `gorm:"primaryKey;autoIncrement" json:"id"`
	ConversationID    uint64                   `gorm:"not null;index:idx_conversation_member_identity,unique" json:"conversationId"`
	MemberType        MemberType               `gorm:"type:varchar(32);not null;default:'USER';index:idx_conversation_member_identity,unique" json:"memberType"`
	MemberID          uint64                   `gorm:"not null;index:idx_conversation_member_identity,unique" json:"memberId"`
	Role              ConversationMemberRole   `gorm:"type:varchar(32);not null;default:'MEMBER'" json:"role"`
	Status            ConversationMemberStatus `gorm:"type:varchar(32);not null;default:'NORMAL'" json:"status"`
	NicknameInGroup   string                   `gorm:"type:varchar(64)" json:"nicknameInGroup"`
	MuteUntil         *time.Time               `json:"muteUntil"`
	IsPinned          bool                     `gorm:"not null;default:false" json:"isPinned"`
	IsMuted           bool                     `gorm:"not null;default:false" json:"isMuted"`
	LastReadMessageID *uint64                  `gorm:"index" json:"lastReadMessageId"`
	JoinedAt          time.Time                `gorm:"not null" json:"joinedAt"`
	CreatedAt         time.Time                `json:"createdAt"`
	UpdatedAt         time.Time                `json:"updatedAt"`
}

func (ConversationMember) TableName() string {
	return "conversation_members"
}

type SenderType string

const (
	SenderTypeUser   SenderType = "USER"
	SenderTypeBot    SenderType = "BOT"
	SenderTypeSystem SenderType = "SYSTEM"
)

type MessageType string

const (
	MessageTypeText     MessageType = "TEXT"
	MessageTypeImage    MessageType = "IMAGE"
	MessageTypeFile     MessageType = "FILE"
	MessageTypeVoice    MessageType = "VOICE"
	MessageTypeBotReply MessageType = "BOT_REPLY"
	MessageTypeSystem   MessageType = "SYSTEM"
)

type MessageStatus string

const (
	MessageStatusNormal   MessageStatus = "NORMAL"
	MessageStatusRecalled MessageStatus = "RECALLED"
	MessageStatusDeleted  MessageStatus = "DELETED"
	MessageStatusFailed   MessageStatus = "FAILED"
)

type Message struct {
	ID             uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	ConversationID uint64         `gorm:"not null;index:idx_conversation_created" json:"conversationId"`
	SenderID       uint64         `gorm:"not null;index" json:"senderId"`
	SenderType     SenderType     `gorm:"type:varchar(32);not null" json:"senderType"`
	MessageType    MessageType    `gorm:"type:varchar(32);not null;default:'TEXT'" json:"messageType"`
	Content        string         `gorm:"type:text" json:"content"`
	ReplyToID      *uint64        `gorm:"index" json:"replyToId"`
	Status         MessageStatus  `gorm:"type:varchar(32);not null;default:'NORMAL'" json:"status"`
	CreatedAt      time.Time      `gorm:"index:idx_conversation_created" json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`

	ReplyTo *Message `gorm:"foreignKey:ReplyToID" json:"-"`
}

func (Message) TableName() string {
	return "messages"
}
