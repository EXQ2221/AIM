package repository

import (
	"context"
	"strconv"
	"strings"

	"example.com/aim/rag-service/internal/dal/model"
	"gorm.io/gorm"
)

type ConversationRepository interface {
	WithTx(tx *gorm.DB) ConversationRepository
	GetByConversationID(ctx context.Context, conversationID string) (*model.Conversation, error)
}

type MemberRepository interface {
	WithTx(tx *gorm.DB) MemberRepository
	GetUserMember(ctx context.Context, conversationID, userID uint64) (*model.ConversationMember, error)
}

type NotificationRepository interface {
	WithTx(tx *gorm.DB) NotificationRepository
	Create(ctx context.Context, notification *model.Notification) error
}

type GormConversationRepository struct {
	db *gorm.DB
}

func NewConversationRepository(db *gorm.DB) *GormConversationRepository {
	return &GormConversationRepository{db: db}
}

func (r *GormConversationRepository) WithTx(tx *gorm.DB) ConversationRepository {
	return &GormConversationRepository{db: tx}
}

func (r *GormConversationRepository) GetByConversationID(ctx context.Context, conversationID string) (*model.Conversation, error) {
	var conversation model.Conversation
	value := strings.TrimSpace(conversationID)
	query := r.db.WithContext(ctx)
	if id, err := strconv.ParseUint(value, 10, 64); err == nil && id > 0 {
		query = query.Where("conversation_id = ? OR id = ?", value, id)
	} else {
		query = query.Where("conversation_id = ?", value)
	}
	if err := query.First(&conversation).Error; err != nil {
		return nil, err
	}
	return &conversation, nil
}

type GormMemberRepository struct {
	db *gorm.DB
}

func NewMemberRepository(db *gorm.DB) *GormMemberRepository {
	return &GormMemberRepository{db: db}
}

func (r *GormMemberRepository) WithTx(tx *gorm.DB) MemberRepository {
	return &GormMemberRepository{db: tx}
}

func (r *GormMemberRepository) GetUserMember(ctx context.Context, conversationID, userID uint64) (*model.ConversationMember, error) {
	var member model.ConversationMember
	err := r.db.WithContext(ctx).
		Where("conversation_id = ? AND member_type = ? AND member_id = ?", conversationID, model.MemberTypeUser, userID).
		First(&member).Error
	if err != nil {
		return nil, err
	}
	return &member, nil
}

type GormNotificationRepository struct {
	db *gorm.DB
}

func NewNotificationRepository(db *gorm.DB) *GormNotificationRepository {
	return &GormNotificationRepository{db: db}
}

func (r *GormNotificationRepository) WithTx(tx *gorm.DB) NotificationRepository {
	return &GormNotificationRepository{db: tx}
}

func (r *GormNotificationRepository) Create(ctx context.Context, notification *model.Notification) error {
	return r.db.WithContext(ctx).Create(notification).Error
}
