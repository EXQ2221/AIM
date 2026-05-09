package model

import "time"

const FriendRelationStatusActive = "ACTIVE"

const (
	FriendRequestStatusPending  = "PENDING"
	FriendRequestStatusAccepted = "ACCEPTED"
	FriendRequestStatusRejected = "REJECTED"

	FriendRequestDirectionIncoming = "INCOMING"
	FriendRequestDirectionOutgoing = "OUTGOING"
)

type FriendGroup struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint64    `gorm:"not null;index:idx_friend_group_user_name,priority:1" json:"user_id"`
	Name      string    `gorm:"size:64;not null;index:idx_friend_group_user_name,priority:2" json:"name"`
	SortOrder int32     `gorm:"not null;default:0" json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (FriendGroup) TableName() string {
	return "friend_groups"
}

type FriendRelation struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID       uint64    `gorm:"not null;index:idx_friend_user_pair,unique,priority:1;index" json:"user_id"`
	FriendUserID uint64    `gorm:"not null;index:idx_friend_user_pair,unique,priority:2;index" json:"friend_user_id"`
	Remark       string    `gorm:"size:128" json:"remark"`
	GroupID      *uint64   `gorm:"index" json:"group_id,omitempty"`
	Status       string    `gorm:"size:32;not null;default:ACTIVE" json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (FriendRelation) TableName() string {
	return "friend_relations"
}

type FriendRequest struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID       uint64    `gorm:"not null;index:idx_friend_request_user_pair,unique,priority:1;index" json:"user_id"`
	TargetUserID uint64    `gorm:"not null;index:idx_friend_request_user_pair,unique,priority:2;index" json:"target_user_id"`
	Remark       string    `gorm:"size:128" json:"remark"`
	GroupID      *uint64   `gorm:"index" json:"group_id,omitempty"`
	Status       string    `gorm:"size:32;not null;default:PENDING;index" json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (FriendRequest) TableName() string {
	return "friend_requests"
}
