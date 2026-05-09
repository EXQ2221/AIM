package model

import "time"

type SecurityEvent struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint64    `gorm:"index;not null" json:"user_id"`
	SessionID string    `gorm:"size:64;index" json:"session_id"`
	EventType string    `gorm:"size:64;index;not null" json:"event_type"`
	IP        string    `gorm:"size:64" json:"ip"`
	DeviceID  string    `gorm:"size:128" json:"device_id"`
	UserAgent string    `gorm:"size:512" json:"user_agent"`
	Detail    string    `gorm:"type:text" json:"detail"`
	CreatedAt time.Time `json:"created_at"`
}

func (SecurityEvent) TableName() string {
	return "security_events"
}
