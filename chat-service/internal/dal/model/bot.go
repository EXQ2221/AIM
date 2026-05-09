package model

import "time"

type BotStatus string

const (
	BotStatusEnabled  BotStatus = "ENABLED"
	BotStatusDisabled BotStatus = "DISABLED"
)

type Bot struct {
	ID              uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Name            string    `gorm:"type:varchar(64);not null" json:"name"`
	MentionName     string    `gorm:"type:varchar(64);not null;uniqueIndex" json:"mentionName"`
	Aliases         string    `gorm:"type:text" json:"aliases"`
	Avatar          string    `gorm:"type:varchar(512)" json:"avatar"`
	Description     string    `gorm:"type:varchar(512)" json:"description"`
	ModelName       string    `gorm:"type:varchar(128);not null" json:"modelName"`
	SupportedModels string    `gorm:"type:text" json:"supportedModels"`
	SystemPrompt    string    `gorm:"type:text" json:"systemPrompt"`
	CreatedBy       uint64    `gorm:"not null;index" json:"createdBy"`
	Status          BotStatus `gorm:"type:varchar(32);not null;default:'ENABLED'" json:"status"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func (Bot) TableName() string {
	return "bots"
}

type ConversationBot struct {
	ID                  uint64             `gorm:"primaryKey;autoIncrement" json:"id"`
	ConversationID      uint64             `gorm:"not null;index:idx_conversation_bot,unique" json:"conversationId"`
	BotID               uint64             `gorm:"not null;index:idx_conversation_bot,unique" json:"botId"`
	Enabled             bool               `gorm:"not null;default:true" json:"enabled"`
	PermissionScope     BotPermissionScope `gorm:"type:varchar(64);not null;default:'CONVERSATION_ONLY'" json:"permissionScope"`
	ModelNameOverride   string             `gorm:"type:varchar(128)" json:"modelNameOverride"`
	DisplayNameOverride string             `gorm:"type:varchar(64)" json:"displayNameOverride"`
	MentionNameOverride string             `gorm:"type:varchar(64)" json:"mentionNameOverride"`
	AliasesOverride     string             `gorm:"type:text" json:"aliasesOverride"`
	CreatedAt           time.Time          `json:"createdAt"`
	UpdatedAt           time.Time          `json:"updatedAt"`
}

func (ConversationBot) TableName() string {
	return "conversation_bots"
}

type BotPermissionScope string

const (
	BotScopeConversationOnly  BotPermissionScope = "CONVERSATION_ONLY"
	BotScopeKnowledgeBaseOnly BotPermissionScope = "KNOWLEDGE_BASE_ONLY"
	BotScopeConversationAndKB BotPermissionScope = "CONVERSATION_AND_KB"
)

type AICallStatus string

const (
	AICallStatusSuccess AICallStatus = "SUCCESS"
	AICallStatusFailed  AICallStatus = "FAILED"
)

type AICallLog struct {
	ID                uint64       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID            uint64       `gorm:"not null;index" json:"userId"`
	BotID             uint64       `gorm:"not null;index" json:"botId"`
	ConversationID    uint64       `gorm:"not null;index" json:"conversationId"`
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
}

func (AICallLog) TableName() string {
	return "ai_call_logs"
}
