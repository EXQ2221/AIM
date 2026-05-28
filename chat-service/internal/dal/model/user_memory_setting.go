package model

import "time"

const (
	MemoryScopeAllGroups      = "ALL_GROUPS"
	MemoryScopeSelectedGroups = "SELECTED_GROUPS"
)

type UserMemorySetting struct {
	ID              uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID          uint64    `gorm:"not null;uniqueIndex" json:"userId"`
	Enabled         bool      `gorm:"not null;default:true" json:"enabled"`
	Scope           string    `gorm:"type:varchar(32);not null;default:'ALL_GROUPS'" json:"scope"`
	ConversationIDs string    `gorm:"type:text;not null;default:'[]'" json:"conversationIds"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func (UserMemorySetting) TableName() string {
	return "user_memory_settings"
}
