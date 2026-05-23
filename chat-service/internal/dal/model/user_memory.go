package model

import "time"

type UserMemory struct {
	ID                   uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID               uint64    `gorm:"not null;index:idx_user_memory_hash,unique;index:idx_user_memory_last_used" json:"userId"`
	MemoryHash           string    `gorm:"type:char(40);not null;index:idx_user_memory_hash,unique" json:"memoryHash"`
	Content              string    `gorm:"type:text;not null" json:"content"`
	SourceConversationID uint64    `gorm:"not null;default:0;index" json:"sourceConversationId"`
	SourceMessageID      *uint64   `gorm:"index" json:"sourceMessageId"`
	LastUsedAt           time.Time `gorm:"not null;index:idx_user_memory_last_used" json:"lastUsedAt"`
	CreatedAt            time.Time `json:"createdAt"`
	UpdatedAt            time.Time `json:"updatedAt"`
}

func (UserMemory) TableName() string {
	return "user_memories"
}
