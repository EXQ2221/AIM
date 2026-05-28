package model

import "time"

type GroupJoinRequestStatus string

const (
	GroupJoinRequestPending  GroupJoinRequestStatus = "PENDING"
	GroupJoinRequestApproved GroupJoinRequestStatus = "APPROVED"
	GroupJoinRequestRejected GroupJoinRequestStatus = "REJECTED"
	GroupJoinRequestCanceled GroupJoinRequestStatus = "CANCELED"
)

type GroupJoinRequest struct {
	ID              uint64                 `gorm:"primaryKey;autoIncrement" json:"id"`
	ConversationID  uint64                 `gorm:"not null;index:idx_group_join_conversation_applicant_status,priority:1" json:"conversationId"`
	ApplicantUserID uint64                 `gorm:"not null;index:idx_group_join_conversation_applicant_status,priority:2" json:"applicantUserId"`
	Reason          string                 `gorm:"type:varchar(512)" json:"reason"`
	Status          GroupJoinRequestStatus `gorm:"type:varchar(32);not null;default:'PENDING';index:idx_group_join_conversation_applicant_status,priority:3" json:"status"`
	ReviewedBy      *uint64                `gorm:"index" json:"reviewedBy"`
	ReviewedAt      *time.Time             `json:"reviewedAt"`
	CreatedAt       time.Time              `json:"createdAt"`
	UpdatedAt       time.Time              `json:"updatedAt"`
}

func (GroupJoinRequest) TableName() string {
	return "group_join_requests"
}
