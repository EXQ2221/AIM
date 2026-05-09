package model

import "time"

type Session struct {
	ID                   uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	SessionID            string     `gorm:"size:64;uniqueIndex;not null" json:"session_id"`
	UserID               uint64     `gorm:"index;not null" json:"user_id"`
	Status               string     `gorm:"size:32;index;not null" json:"status"`
	DeviceID             string     `gorm:"size:128;index" json:"device_id"`
	DeviceName           string     `gorm:"size:128" json:"device_name"`
	UserAgent            string     `gorm:"size:512" json:"user_agent"`
	BrowserName          string     `gorm:"size:64" json:"browser_name"`
	BrowserVersion       string     `gorm:"size:64" json:"browser_version"`
	OSName               string     `gorm:"size:64" json:"os_name"`
	DeviceType           string     `gorm:"size:32" json:"device_type"`
	BrowserKey           string     `gorm:"size:191;index" json:"browser_key"`
	LoginIP              string     `gorm:"size:64" json:"login_ip"`
	LastIP               string     `gorm:"size:64" json:"last_ip"`
	LastSeenAt           time.Time  `json:"last_seen_at"`
	CurrentAccessJTI     string     `gorm:"size:64;index" json:"current_access_jti"`
	CurrentAccessExpires time.Time  `json:"current_access_expires"`
	RevokedAt            *time.Time `json:"revoked_at,omitempty"`
	RevokeReason         string     `gorm:"size:128" json:"revoke_reason"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

func (Session) TableName() string {
	return "sessions"
}
