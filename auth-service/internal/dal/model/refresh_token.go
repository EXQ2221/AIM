package model

import "time"

type RefreshToken struct {
	ID                uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	SessionID         string     `gorm:"size:64;index;not null" json:"session_id"`
	UserID            uint64     `gorm:"index;not null" json:"user_id"`
	TokenHash         string     `gorm:"size:64;uniqueIndex;not null" json:"token_hash"`
	Status            string     `gorm:"size:32;index;not null" json:"status"`
	ExpiresAt         time.Time  `json:"expires_at"`
	UsedAt            *time.Time `json:"used_at,omitempty"`
	RevokedAt         *time.Time `json:"revoked_at,omitempty"`
	RevokeReason      string     `gorm:"size:128" json:"revoke_reason"`
	RotatedTo         string     `gorm:"size:64" json:"rotated_to"`
	LastUsedIP        string     `gorm:"size:64" json:"last_used_ip"`
	LastUsedUserAgent string     `gorm:"size:512" json:"last_used_user_agent"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

func (RefreshToken) TableName() string {
	return "refresh_tokens"
}
