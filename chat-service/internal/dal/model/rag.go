package model

import "time"

type KnowledgeBaseScope string

const (
	KnowledgeBaseScopeConversation KnowledgeBaseScope = "CONVERSATION"
)

type KnowledgeBaseStatus string

const (
	KnowledgeBaseStatusActive   KnowledgeBaseStatus = "ACTIVE"
	KnowledgeBaseStatusDisabled KnowledgeBaseStatus = "DISABLED"
)

type KnowledgeBase struct {
	ID          uint64              `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string              `gorm:"type:varchar(128);not null" json:"name"`
	Description string              `gorm:"type:text" json:"description"`
	OwnerID     uint64              `gorm:"not null;index" json:"ownerId"`
	Scope       KnowledgeBaseScope  `gorm:"type:varchar(32);not null;default:'CONVERSATION'" json:"scope"`
	Status      KnowledgeBaseStatus `gorm:"type:varchar(32);not null;default:'ACTIVE'" json:"status"`
	CreatedAt   time.Time           `json:"createdAt"`
	UpdatedAt   time.Time           `json:"updatedAt"`
}

func (KnowledgeBase) TableName() string {
	return "knowledge_bases"
}

type KnowledgeDocumentSourceType string

const (
	KnowledgeDocumentSourceText     KnowledgeDocumentSourceType = "TEXT"
	KnowledgeDocumentSourceMarkdown KnowledgeDocumentSourceType = "MARKDOWN"
)

type KnowledgeDocumentStatus string

const (
	KnowledgeDocumentStatusPending    KnowledgeDocumentStatus = "PENDING"
	KnowledgeDocumentStatusProcessing KnowledgeDocumentStatus = "PROCESSING"
	KnowledgeDocumentStatusReady      KnowledgeDocumentStatus = "READY"
	KnowledgeDocumentStatusFailed     KnowledgeDocumentStatus = "FAILED"
)

type KnowledgeDocument struct {
	ID              uint64                      `gorm:"primaryKey;autoIncrement" json:"id"`
	KnowledgeBaseID uint64                      `gorm:"not null;index" json:"knowledgeBaseId"`
	Title           string                      `gorm:"type:varchar(255);not null" json:"title"`
	SourceType      KnowledgeDocumentSourceType `gorm:"type:varchar(32);not null" json:"sourceType"`
	SourceURL       string                      `gorm:"type:text" json:"sourceUrl"`
	Status          KnowledgeDocumentStatus     `gorm:"type:varchar(32);not null;default:'PENDING'" json:"status"`
	ErrorMessage    string                      `gorm:"type:text" json:"errorMessage"`
	CreatedBy       uint64                      `gorm:"not null;index" json:"createdBy"`
	CreatedAt       time.Time                   `json:"createdAt"`
	UpdatedAt       time.Time                   `json:"updatedAt"`
}

func (KnowledgeDocument) TableName() string {
	return "knowledge_documents"
}

type ConversationKnowledgeBase struct {
	ID              uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	ConversationID  uint64    `gorm:"not null;index:idx_conversation_kb,unique" json:"conversationId"`
	KnowledgeBaseID uint64    `gorm:"not null;index:idx_conversation_kb,unique" json:"knowledgeBaseId"`
	Enabled         bool      `gorm:"not null;default:true" json:"enabled"`
	CreatedBy       uint64    `gorm:"not null;index" json:"createdBy"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func (ConversationKnowledgeBase) TableName() string {
	return "conversation_knowledge_bases"
}
