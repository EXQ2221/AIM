AIM 基础聊天系统的 GORM 模型，先不包含 RAG，只覆盖：


用户

好友分组

好友关系

会话

会话成员

群聊信息

消息

文件资源

Bot

会话 Bot 绑定

AI 调用日志



可以先按这个作为第一版数据库模型，后面再加 RAG 表。

模型演进记录：

2026-05-07 P3 AI Bot 成员化调整

- 当前仍处于开发阶段，数据库没有重要历史数据，允许清库重建。
- conversation_members 不再保留旧 user_id。
- conversation_members 统一使用 member_type + member_id 表达 USER/BOT 成员。
- conversation_members 唯一索引改为 conversation_id + member_type + member_id。
- bots 增加 mention_name / aliases，其中 aliases 使用 JSON 文本，API/DTO 使用 []string。
- conversation_bots 增加 display_name_override / mention_name_override / aliases_override，其中 aliases_override 使用 JSON 文本。
- 本文档作为模型演进记录和参考基线，具体实现以当前 task spec 和代码为准。

2026-05-07 P3 Task 01 落地说明

- chat-service GORM 模型已切换到 conversation_members.member_type + member_id。
- chat-service GORM 模型已移除 conversation_members.user_id。
- chat-service MySQL 初始化在检测到旧 conversation_members.user_id 列时，会删除旧 conversation_members 表并按新模型重建。
- 为了让新 schema 下的 chat-service 可以编译并通过测试，repository 与 USER 成员逻辑已同步切换到 member_type=USER + member_id 语义。

2026-05-07 P3 全部完成（Task 12 文档对齐）

- P3 所有 Task（00~12）已完成落地，当前模型与代码一致。
- `bots`、`conversation_members`（member_type + member_id）、`conversation_bots`、`ai_call_logs` 四张表已全部接入 AutoMigrate。
- `conversation_members` 不保留旧 `user_id`，唯一索引为 `conversation_id + member_type + member_id`。
- 开发阶段允许清库重建，不做旧数据兼容迁移。
- 本文档作为模型演进记录和参考基线，具体实现以 `docs/specs/tasks/*.md` 当前 task spec 和代码为准。

package model

import (
	"time"

	"gorm.io/gorm"
)

type UserStatus string

const (
	UserStatusNormal  UserStatus = "NORMAL"
	UserStatusBanned  UserStatus = "BANNED"
	UserStatusDeleted UserStatus = "DELETED"
)

type User struct {
	ID           uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	Username     string         `gorm:"type:varchar(64);not null;uniqueIndex" json:"username"`
	PasswordHash string         `gorm:"type:varchar(255);not null" json:"-"`
	Nickname     string         `gorm:"type:varchar(64);not null" json:"nickname"`
	Avatar       string         `gorm:"type:varchar(512)" json:"avatar"`
	Email        string         `gorm:"type:varchar(128);index" json:"email"`
	Phone        string         `gorm:"type:varchar(32);index" json:"phone"`
	Status       UserStatus     `gorm:"type:varchar(32);not null;default:'NORMAL'" json:"status"`
	LastLoginAt  *time.Time     `json:"lastLoginAt"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}


好友相关

type FriendRelationStatus string

const (
	FriendStatusPending  FriendRelationStatus = "PENDING"
	FriendStatusAccepted FriendRelationStatus = "ACCEPTED"
	FriendStatusBlocked  FriendRelationStatus = "BLOCKED"
	FriendStatusDeleted  FriendRelationStatus = "DELETED"
)

type FriendGroup struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint64    `gorm:"not null;index:idx_user_group_name,unique" json:"userId"`
	Name      string    `gorm:"type:varchar(64);not null;index:idx_user_group_name,unique" json:"name"`
	SortOrder int       `gorm:"not null;default:0" json:"sortOrder"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type FriendRelation struct {
	ID        uint64               `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint64               `gorm:"not null;index:idx_user_friend,unique" json:"userId"`
	FriendID  uint64               `gorm:"not null;index:idx_user_friend,unique" json:"friendId"`
	GroupID   *uint64              `gorm:"index" json:"groupId"`
	Remark    string               `gorm:"type:varchar(64)" json:"remark"`
	Status    FriendRelationStatus `gorm:"type:varchar(32);not null;default:'PENDING'" json:"status"`
	CreatedAt time.Time            `json:"createdAt"`
	UpdatedAt time.Time            `json:"updatedAt"`

	User   User         `gorm:"foreignKey:UserID" json:"-"`
	Friend User         `gorm:"foreignKey:FriendID" json:"-"`
	Group  *FriendGroup `gorm:"foreignKey:GroupID" json:"-"`
}


这里 FriendRelation 是单向关系。
比如 A 和 B 成为好友，可以插入两条：

A -> B
B -> A


这样每个人都可以有自己的备注和分组。

会话相关

type ConversationType string

const (
	ConversationTypeSingle ConversationType = "SINGLE"
	ConversationTypeGroup  ConversationType = "GROUP"
	ConversationTypeBot    ConversationType = "BOT"
	ConversationTypeSystem ConversationType = "SYSTEM"
)

type Conversation struct {
	ID            uint64           `gorm:"primaryKey;autoIncrement" json:"id"`
	Type          ConversationType `gorm:"type:varchar(32);not null;index" json:"type"`
	Title         string           `gorm:"type:varchar(128)" json:"title"`
	Avatar        string           `gorm:"type:varchar(512)" json:"avatar"`
	CreatedBy     uint64           `gorm:"not null;index" json:"createdBy"`
	LastMessageID *uint64          `gorm:"index" json:"lastMessageId"`
	LastMessageAt *time.Time       `gorm:"index" json:"lastMessageAt"`
	CreatedAt     time.Time        `json:"createdAt"`
	UpdatedAt     time.Time        `json:"updatedAt"`
	DeletedAt     gorm.DeletedAt   `gorm:"index" json:"-"`

	Creator User `gorm:"foreignKey:CreatedBy" json:"-"`
}


conversation 是聊天窗口。单聊、群聊、Bot 私聊都可以放这里。

会话成员

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

	Conversation Conversation `gorm:"foreignKey:ConversationID" json:"-"`
}


这个表很重要，建议保留。
它可以支持：


群成员

单聊成员

Bot 成员

已读位置

免打扰

置顶

群昵称

禁言状态



群聊信息

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

	Conversation Conversation `gorm:"foreignKey:ConversationID" json:"-"`
	Owner        User         `gorm:"foreignKey:OwnerID" json:"-"`
}


GroupInfo 只在 conversation.type = GROUP 时存在。

消息表

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
	MessageStatusNormal  MessageStatus = "NORMAL"
	MessageStatusRecalled MessageStatus = "RECALLED"
	MessageStatusDeleted MessageStatus = "DELETED"
	MessageStatusFailed  MessageStatus = "FAILED"
)

type Message struct {
	ID             uint64        `gorm:"primaryKey;autoIncrement" json:"id"`
	ConversationID uint64        `gorm:"not null;index:idx_conversation_created" json:"conversationId"`
	SenderID       uint64        `gorm:"not null;index" json:"senderId"`
	SenderType     SenderType    `gorm:"type:varchar(32);not null" json:"senderType"`
	MessageType    MessageType   `gorm:"type:varchar(32);not null;default:'TEXT'" json:"messageType"`
	Content        string        `gorm:"type:text" json:"content"`
	ReplyToID      *uint64       `gorm:"index" json:"replyToId"`
	Status         MessageStatus `gorm:"type:varchar(32);not null;default:'NORMAL'" json:"status"`
	CreatedAt      time.Time     `gorm:"index:idx_conversation_created" json:"createdAt"`
	UpdatedAt      time.Time     `json:"updatedAt"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`

	Conversation Conversation `gorm:"foreignKey:ConversationID" json:"-"`
	ReplyTo      *Message     `gorm:"foreignKey:ReplyToID" json:"-"`
}


注意这里没有直接关联 Sender User，因为 sender_id 可能是：


用户 ID

Bot ID

系统 ID



所以要结合 sender_type 判断。

如果你想简单一点，也可以规定 Bot 在 user 表里也有一个账号，这样 sender_id 永远指向 user.id。但长期来看，sender_type + sender_id 更灵活。

文件资源表

type FileStorageType string

const (
	FileStorageLocal FileStorageType = "LOCAL"
	FileStorageOSS   FileStorageType = "OSS"
	FileStorageS3    FileStorageType = "S3"
)

type FileResource struct {
	ID          uint64          `gorm:"primaryKey;autoIncrement" json:"id"`
	UploaderID  uint64          `gorm:"not null;index" json:"uploaderId"`
	FileName    string          `gorm:"type:varchar(255);not null" json:"fileName"`
	FileURL     string          `gorm:"type:varchar(1024);not null" json:"fileUrl"`
	FileType    string          `gorm:"type:varchar(64);index" json:"fileType"`
	MimeType    string          `gorm:"type:varchar(128)" json:"mimeType"`
	FileSize    int64           `gorm:"not null;default:0" json:"fileSize"`
	StorageType FileStorageType `gorm:"type:varchar(32);not null;default:'LOCAL'" json:"storageType"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`

	Uploader User `gorm:"foreignKey:UploaderID" json:"-"`
}


如果你要让消息关联文件，可以加一个中间表。

type MessageAttachment struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	MessageID uint64    `gorm:"not null;index" json:"messageId"`
	FileID    uint64    `gorm:"not null;index" json:"fileId"`
	CreatedAt time.Time `json:"createdAt"`

	Message Message      `gorm:"foreignKey:MessageID" json:"-"`
	File    FileResource `gorm:"foreignKey:FileID" json:"-"`
}


这样一条消息可以挂多个文件。

Bot 相关

type BotStatus string

const (
	BotStatusEnabled  BotStatus = "ENABLED"
	BotStatusDisabled BotStatus = "DISABLED"
)

type Bot struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Name         string    `gorm:"type:varchar(64);not null" json:"name"`
	MentionName  string    `gorm:"type:varchar(64);not null;uniqueIndex" json:"mentionName"`
	Aliases      string    `gorm:"type:text" json:"aliases"` // JSON array text
	Avatar       string    `gorm:"type:varchar(512)" json:"avatar"`
	Description  string    `gorm:"type:varchar(512)" json:"description"`
	ModelName    string    `gorm:"type:varchar(128);not null" json:"modelName"`
	SystemPrompt string    `gorm:"type:text" json:"systemPrompt"`
	CreatedBy    uint64    `gorm:"not null;index" json:"createdBy"`
	Status       BotStatus `gorm:"type:varchar(32);not null;default:'ENABLED'" json:"status"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`

	Creator User `gorm:"foreignKey:CreatedBy" json:"-"`
}


会话 Bot 绑定

type BotPermissionScope string

const (
	BotScopeConversationOnly       BotPermissionScope = "CONVERSATION_ONLY"
	BotScopeKnowledgeBaseOnly      BotPermissionScope = "KNOWLEDGE_BASE_ONLY"
	BotScopeConversationAndKB      BotPermissionScope = "CONVERSATION_AND_KB"
)

type ConversationBot struct {
	ID                  uint64             `gorm:"primaryKey;autoIncrement" json:"id"`
	ConversationID      uint64             `gorm:"not null;index:idx_conversation_bot,unique" json:"conversationId"`
	BotID               uint64             `gorm:"not null;index:idx_conversation_bot,unique" json:"botId"`
	Enabled             bool               `gorm:"not null;default:true" json:"enabled"`
	PermissionScope     BotPermissionScope `gorm:"type:varchar(64);not null;default:'CONVERSATION_ONLY'" json:"permissionScope"`
	DisplayNameOverride string             `gorm:"type:varchar(64)" json:"displayNameOverride"`
	MentionNameOverride string             `gorm:"type:varchar(64)" json:"mentionNameOverride"`
	AliasesOverride     string             `gorm:"type:text" json:"aliasesOverride"` // JSON array text
	CreatedAt           time.Time          `json:"createdAt"`
	UpdatedAt           time.Time          `json:"updatedAt"`

	Conversation Conversation `gorm:"foreignKey:ConversationID" json:"-"`
	Bot          Bot          `gorm:"foreignKey:BotID" json:"-"`
}


AI 调用日志

type AICallStatus string

const (
	AICallStatusSuccess AICallStatus = "SUCCESS"
	AICallStatusFailed  AICallStatus = "FAILED"
)

type AICallLog struct {
	ID               uint64       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID           uint64       `gorm:"not null;index" json:"userId"`
	BotID            uint64       `gorm:"not null;index" json:"botId"`
	ConversationID   uint64       `gorm:"not null;index" json:"conversationId"`
	RequestMessageID  *uint64      `gorm:"index" json:"requestMessageId"`
	ResponseMessageID *uint64      `gorm:"index" json:"responseMessageId"`
	ModelName         string       `gorm:"type:varchar(128);not null" json:"modelName"`
	PromptTokens      int          `gorm:"not null;default:0" json:"promptTokens"`
	CompletionTokens  int          `gorm:"not null;default:0" json:"completionTokens"`
	TotalTokens       int          `gorm:"not null;default:0" json:"totalTokens"`
	LatencyMS         int64        `gorm:"not null;default:0" json:"latencyMs"`
	Status            AICallStatus `gorm:"type:varchar(32);not null" json:"status"`
	ErrorMessage      string       `gorm:"type:text" json:"errorMessage"`
	CreatedAt         time.Time    `json:"createdAt"`

	User         User         `gorm:"foreignKey:UserID" json:"-"`
	Bot          Bot          `gorm:"foreignKey:BotID" json:"-"`
	Conversation Conversation `gorm:"foreignKey:ConversationID" json:"-"`
}


还可以补一个通知表

这个不是聊天消息，是用户级提醒。

type NotificationType string

const (
	NotificationTypeFriendRequest NotificationType = "FRIEND_REQUEST"
	NotificationTypeGroupInvite   NotificationType = "GROUP_INVITE"
	NotificationTypeSystem        NotificationType = "SYSTEM"
)

type Notification struct {
	ID        uint64           `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint64           `gorm:"not null;index" json:"userId"`
	Type      NotificationType `gorm:"type:varchar(64);not null;index" json:"type"`
	Title     string           `gorm:"type:varchar(128);not null" json:"title"`
	Content   string           `gorm:"type:text" json:"content"`
	RelatedID *uint64          `gorm:"index" json:"relatedId"`
	IsRead    bool             `gorm:"not null;default:false" json:"isRead"`
	CreatedAt time.Time        `json:"createdAt"`
	UpdatedAt time.Time        `json:"updatedAt"`

	User User `gorm:"foreignKey:UserID" json:"-"`
}



文件和通知可以稍后加：

FileResource
MessageAttachment
Notification


RAG 之后再加：

KnowledgeBase
KnowledgeDocument
DocumentChunk
ConversationKnowledgeBase



