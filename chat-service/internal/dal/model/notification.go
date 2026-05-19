package model

import "time"

type NotificationType string

const (
	NotificationTypeGroupEvent  NotificationType = "GROUP_EVENT"
	NotificationTypeAdminNotice NotificationType = "ADMIN_NOTICE"
	NotificationTypeSystem      NotificationType = "SYSTEM"
)

type Notification struct {
	ID               uint64           `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID           uint64           `gorm:"not null;index" json:"userId"`
	Type             NotificationType `gorm:"type:varchar(64);not null;index" json:"type"`
	Title            string           `gorm:"type:varchar(128);not null" json:"title"`
	Content          string           `gorm:"type:text" json:"content"`
	ConversationRef  string           `gorm:"type:varchar(32);index" json:"conversationRef"`
	RelatedMessageID *uint64          `gorm:"index" json:"relatedMessageId"`
	IsRead           bool             `gorm:"not null;default:false;index" json:"isRead"`
	CreatedAt        time.Time        `json:"createdAt"`
	UpdatedAt        time.Time        `json:"updatedAt"`
}

func (Notification) TableName() string {
	return "notifications"
}
