package model

import (
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/datatypes"
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
	ID                    uint64          `gorm:"primaryKey;autoIncrement" json:"id"`
	ConversationID        uint64          `gorm:"not null;uniqueIndex" json:"conversationId"`
	Name                  string          `gorm:"type:varchar(128);not null" json:"name"`
	Avatar                string          `gorm:"type:varchar(512)" json:"avatar"`
	Announcement          string          `gorm:"type:text" json:"announcement"`
	AnnouncementUpdatedBy *uint64         `gorm:"index" json:"announcementUpdatedBy"`
	AnnouncementUpdatedAt *time.Time      `json:"announcementUpdatedAt"`
	OwnerID               uint64          `gorm:"not null;index" json:"ownerId"`
	JoinPolicy            GroupJoinPolicy `gorm:"type:varchar(32);not null;default:'INVITE_ONLY'" json:"joinPolicy"`
	MuteAll               bool            `gorm:"not null;default:false" json:"muteAll"`
	MuteAllUpdatedBy      *uint64         `gorm:"index" json:"muteAllUpdatedBy"`
	MuteAllUpdatedAt      *time.Time      `json:"muteAllUpdatedAt"`
	MaxMembers            int             `gorm:"not null;default:500" json:"maxMembers"`
	CreatedAt             time.Time       `json:"createdAt"`
	UpdatedAt             time.Time       `json:"updatedAt"`
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

const RecalledMessagePlaceholder = "消息已撤回"

type Message struct {
	ID             uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	ConversationID uint64         `gorm:"not null;index:idx_conversation_created" json:"conversationId"`
	SenderID       uint64         `gorm:"not null;index" json:"senderId"`
	SenderType     SenderType     `gorm:"type:varchar(32);not null" json:"senderType"`
	MessageType    MessageType    `gorm:"type:varchar(32);not null;default:'TEXT'" json:"messageType"`
	Content        datatypes.JSON `gorm:"type:jsonb;not null" json:"content"`
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

type TextMessageContent struct {
	Text string `json:"text"`
}

type ImageMessageContent struct {
	URL      string `json:"url"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	MimeType string `json:"mimeType"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Text     string `json:"text,omitempty"`
}

type FileMessageContent struct {
	URL      string `json:"url"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	MimeType string `json:"mimeType"`
}

type VoiceMessageContent struct {
	URL        string `json:"url"`
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	MimeType   string `json:"mimeType"`
	DurationMS int64  `json:"durationMs"`
}

type SystemMessageContent struct {
	EventType     string   `json:"eventType"`
	ActorUserID   uint64   `json:"actorUserId"`
	TargetUserIDs []uint64 `json:"targetUserIds"`
	Text          string   `json:"text"`
}

const (
	SystemEventMemberJoined        = "MEMBER_JOINED"
	SystemEventMemberLeft          = "MEMBER_LEFT"
	SystemEventMemberInvited       = "MEMBER_INVITED"
	SystemEventMemberRemoved       = "MEMBER_REMOVED"
	SystemEventMemberMuted         = "MEMBER_MUTED"
	SystemEventMemberUnmuted       = "MEMBER_UNMUTED"
	SystemEventGroupMuted          = "GROUP_MUTED"
	SystemEventGroupUnmuted        = "GROUP_UNMUTED"
	SystemEventGroupAvatarUpdated  = "GROUP_AVATAR_UPDATED"
	SystemEventGroupDisbanded      = "GROUP_DISBANDED"
	SystemEventAdminAdded          = "ADMIN_ADDED"
	SystemEventAdminRemoved        = "ADMIN_REMOVED"
	SystemEventOwnerTransferred    = "OWNER_TRANSFERRED"
	SystemEventAnnouncementUpdated = "ANNOUNCEMENT_UPDATED"
)

func NormalizeTextMessageContent(content string) (datatypes.JSON, error) {
	payload := TextMessageContent{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &payload); err == nil {
		payload.Text = strings.TrimSpace(payload.Text)
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		return datatypes.JSON(encoded), nil
	}

	payload.Text = strings.TrimSpace(content)
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return datatypes.JSON(encoded), nil
}

func ExtractTextMessageContent(content datatypes.JSON) string {
	trimmed := strings.TrimSpace(string(content))
	if trimmed == "" {
		return ""
	}

	var payload TextMessageContent
	if err := json.Unmarshal([]byte(trimmed), &payload); err == nil {
		return strings.TrimSpace(payload.Text)
	}
	return trimmed
}

func MessagePreview(messageType MessageType, content datatypes.JSON) string {
	switch messageType {
	case MessageTypeImage:
		var payload ImageMessageContent
		if err := json.Unmarshal([]byte(strings.TrimSpace(string(content))), &payload); err == nil {
			if text := strings.TrimSpace(payload.Text); text != "" {
				return "[图片] " + text
			}
		}
		return "[图片]"
	case MessageTypeFile:
		var payload FileMessageContent
		if err := json.Unmarshal([]byte(strings.TrimSpace(string(content))), &payload); err == nil {
			if name := strings.TrimSpace(payload.Name); name != "" {
				return name
			}
		}
		return "[文件]"
	case MessageTypeVoice:
		return "[语音]"
	case MessageTypeSystem:
		var payload SystemMessageContent
		if err := json.Unmarshal([]byte(strings.TrimSpace(string(content))), &payload); err == nil {
			return strings.TrimSpace(payload.Text)
		}
		return strings.TrimSpace(string(content))
	case MessageTypeBotReply:
		text := ExtractTextMessageContent(content)
		if text != "" {
			return text
		}
		return strings.TrimSpace(string(content))
	case MessageTypeText:
		fallthrough
	default:
		return ExtractTextMessageContent(content)
	}
}

func MessagePreviewWithStatus(status MessageStatus, messageType MessageType, content datatypes.JSON) string {
	if status == MessageStatusRecalled {
		return RecalledMessagePlaceholder
	}
	return MessagePreview(messageType, content)
}

func ReplyContentPreview(messageType MessageType, content datatypes.JSON, limit int) string {
	return TruncatePreview(MessagePreview(messageType, content), limit)
}

func ReplyContentPreviewWithStatus(status MessageStatus, messageType MessageType, content datatypes.JSON, limit int) string {
	return TruncatePreview(MessagePreviewWithStatus(status, messageType, content), limit)
}

func TruncatePreview(content string, limit int) string {
	trimmed := strings.TrimSpace(content)
	if limit <= 0 || utf8.RuneCountInString(trimmed) <= limit {
		return trimmed
	}
	runes := []rune(trimmed)
	return string(runes[:limit]) + "..."
}
