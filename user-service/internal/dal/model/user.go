package model

import (
	"time"

	"gorm.io/gorm"
)

const (
	UserStatusNormal  = "NORMAL"
	UserStatusBanned  = "BANNED"
	UserStatusDeleted = "DELETED"

	UserRoleUser  = "USER"
	UserRoleAdmin = "ADMIN"
)

type User struct {
	ID             uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	AimID          string         `gorm:"column:aim_id;size:64;uniqueIndex;not null" json:"aim_id"`
	Nickname       string         `gorm:"size:64;not null" json:"nickname"`
	Avatar         string         `gorm:"size:512" json:"avatar"`
	Email          string         `gorm:"size:191;uniqueIndex;not null" json:"email"`
	PasswordHash   string         `gorm:"column:password_hash;size:255;not null" json:"-"`
	Status         string         `gorm:"size:32;index;not null;default:NORMAL" json:"status"`
	Role           string         `gorm:"size:32;index;not null;default:USER" json:"role"`
	TokenVersion   uint64         `gorm:"column:token_version;not null;default:1" json:"token_version"`
	LastLoginAt    *time.Time     `json:"last_login_at,omitempty"`
	LastLoginIP    string         `gorm:"column:last_login_ip;size:64" json:"last_login_ip"`
	LoginFailCount uint32         `gorm:"column:login_fail_count;not null;default:0" json:"login_fail_count"`
	LockedUntil    *time.Time     `json:"locked_until,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

func (User) TableName() string {
	return "users"
}
