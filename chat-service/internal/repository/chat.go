package repository

import (
	"context"
	"errors"
	"time"

	"example.com/aim/chat-service/internal/dal/model"
	"gorm.io/gorm"
)

type ConversationListRow struct {
	ConversationID        string
	Type                  string
	Title                 string
	Avatar                string
	LastMessageID         *uint64
	LastMessageAt         *time.Time
	LastMessageSenderID   *uint64
	LastMessageSenderType string
	LastMessageContent    string
	Role                  string
	IsPinned              bool
	IsMuted               bool
	UpdatedAt             time.Time
}

type ConversationRepository interface {
	WithTx(tx *gorm.DB) ConversationRepository
	Create(ctx context.Context, conversation *model.Conversation) error
	GetByID(ctx context.Context, id uint64) (*model.Conversation, error)
	GetByConversationID(ctx context.Context, conversationID string) (*model.Conversation, error)
	FindSingleByUsers(ctx context.Context, userID uint64, peerUserID uint64) (*model.Conversation, error)
	ListByUserID(ctx context.Context, userID uint64) ([]ConversationListRow, error)
	UpdateLastMessage(ctx context.Context, conversationID uint64, messageID uint64, at time.Time) error
}

type GroupRepository interface {
	WithTx(tx *gorm.DB) GroupRepository
	Create(ctx context.Context, group *model.GroupInfo) error
	GetByConversationID(ctx context.Context, conversationID uint64) (*model.GroupInfo, error)
}

type MemberRepository interface {
	WithTx(tx *gorm.DB) MemberRepository
	Create(ctx context.Context, member *model.ConversationMember) error
	Update(ctx context.Context, member *model.ConversationMember) error
	GetUserMember(ctx context.Context, conversationID, userID uint64) (*model.ConversationMember, error)
	IsUserMember(ctx context.Context, conversationID, userID uint64) (bool, error)
	ListUserMembers(ctx context.Context, conversationID uint64) ([]model.ConversationMember, error)
	ListUserMemberIDs(ctx context.Context, conversationID uint64) ([]uint64, error)
	GetBotMember(ctx context.Context, conversationID, botID uint64) (*model.ConversationMember, error)
	IsBotMember(ctx context.Context, conversationID, botID uint64) (bool, error)
	ListBotMembers(ctx context.Context, conversationID uint64) ([]model.ConversationMember, error)
	ListByConversationID(ctx context.Context, conversationID uint64) ([]model.ConversationMember, error)
	GetDB() *gorm.DB
}

type MessageRepository interface {
	WithTx(tx *gorm.DB) MessageRepository
	Create(ctx context.Context, message *model.Message) error
	ListByConversationID(ctx context.Context, conversationID uint64, beforeID *uint64, limit int) ([]model.Message, error)
}

type AICallLogRepository interface {
	WithTx(tx *gorm.DB) AICallLogRepository
	Create(ctx context.Context, callLog *model.AICallLog) error
	ListByConversationID(ctx context.Context, conversationID uint64, beforeID *uint64, limit int, botID *uint64, status string) ([]model.AICallLog, error)
	SumTotalTokensByConversationBetween(ctx context.Context, conversationID uint64, startAt time.Time, endAt time.Time) (int64, error)
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

func (r *GormConversationRepository) Create(ctx context.Context, conversation *model.Conversation) error {
	return r.db.WithContext(ctx).Create(conversation).Error
}

func (r *GormConversationRepository) GetByID(ctx context.Context, id uint64) (*model.Conversation, error) {
	var conversation model.Conversation
	if err := r.db.WithContext(ctx).First(&conversation, id).Error; err != nil {
		return nil, err
	}
	return &conversation, nil
}

func (r *GormConversationRepository) GetByConversationID(ctx context.Context, conversationID string) (*model.Conversation, error) {
	var conversation model.Conversation
	if err := r.db.WithContext(ctx).Where("conversation_id = ?", conversationID).First(&conversation).Error; err != nil {
		return nil, err
	}
	return &conversation, nil
}

func (r *GormConversationRepository) FindSingleByUsers(ctx context.Context, userID uint64, peerUserID uint64) (*model.Conversation, error) {
	var conversation model.Conversation
	err := r.db.WithContext(ctx).
		Table("conversations AS c").
		Select("c.*").
		Joins("JOIN conversation_members AS cm_self ON cm_self.conversation_id = c.id").
		Joins("JOIN conversation_members AS cm_peer ON cm_peer.conversation_id = c.id").
		Where("c.type = ? AND c.deleted_at IS NULL", model.ConversationTypeSingle).
		Where("cm_self.member_type = ? AND cm_self.member_id = ? AND cm_self.status <> ?", model.MemberTypeUser, userID, model.MemberStatusRemoved).
		Where("cm_peer.member_type = ? AND cm_peer.member_id = ? AND cm_peer.status <> ?", model.MemberTypeUser, peerUserID, model.MemberStatusRemoved).
		Order("c.id ASC").
		Limit(1).
		Scan(&conversation).Error
	if err != nil {
		return nil, err
	}
	if conversation.ID == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &conversation, nil
}

func (r *GormConversationRepository) ListByUserID(ctx context.Context, userID uint64) ([]ConversationListRow, error) {
	var rows []ConversationListRow
	err := r.db.WithContext(ctx).
		Table("conversation_members AS cm").
		Select(`c.conversation_id, c.type, c.title, c.avatar, c.last_message_id, c.last_message_at, m.sender_id AS last_message_sender_id, m.sender_type AS last_message_sender_type, m.content AS last_message_content, cm.role, cm.is_pinned, cm.is_muted, c.updated_at`).
		Joins("JOIN conversations AS c ON c.id = cm.conversation_id AND c.deleted_at IS NULL").
		Joins("LEFT JOIN messages AS m ON m.id = c.last_message_id AND m.deleted_at IS NULL").
		Where("cm.member_type = ? AND cm.member_id = ? AND cm.status <> ?", model.MemberTypeUser, userID, model.MemberStatusRemoved).
		Order("c.last_message_at DESC, c.updated_at DESC").
		Scan(&rows).Error
	return rows, err
}

func (r *GormConversationRepository) UpdateLastMessage(ctx context.Context, conversationID uint64, messageID uint64, at time.Time) error {
	return r.db.WithContext(ctx).
		Model(&model.Conversation{}).
		Where("id = ?", conversationID).
		Updates(map[string]any{
			"last_message_id": messageID,
			"last_message_at": at,
		}).Error
}

type GormGroupRepository struct {
	db *gorm.DB
}

func NewGroupRepository(db *gorm.DB) *GormGroupRepository {
	return &GormGroupRepository{db: db}
}

func (r *GormGroupRepository) WithTx(tx *gorm.DB) GroupRepository {
	return &GormGroupRepository{db: tx}
}

func (r *GormGroupRepository) Create(ctx context.Context, group *model.GroupInfo) error {
	return r.db.WithContext(ctx).Create(group).Error
}

func (r *GormGroupRepository) GetByConversationID(ctx context.Context, conversationID uint64) (*model.GroupInfo, error) {
	var group model.GroupInfo
	if err := r.db.WithContext(ctx).Where("conversation_id = ?", conversationID).First(&group).Error; err != nil {
		return nil, err
	}
	return &group, nil
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

func (r *GormMemberRepository) Create(ctx context.Context, member *model.ConversationMember) error {
	return r.db.WithContext(ctx).Create(member).Error
}

func (r *GormMemberRepository) Update(ctx context.Context, member *model.ConversationMember) error {
	return r.db.WithContext(ctx).Save(member).Error
}

func (r *GormMemberRepository) GetDB() *gorm.DB {
	return r.db
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

func (r *GormMemberRepository) IsUserMember(ctx context.Context, conversationID, userID uint64) (bool, error) {
	_, err := r.GetUserMember(ctx, conversationID, userID)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return false, err
}

func (r *GormMemberRepository) ListUserMembers(ctx context.Context, conversationID uint64) ([]model.ConversationMember, error) {
	var members []model.ConversationMember
	err := r.db.WithContext(ctx).
		Where("conversation_id = ? AND member_type = ? AND status = ?", conversationID, model.MemberTypeUser, model.MemberStatusNormal).
		Order("role ASC, joined_at ASC").
		Find(&members).Error
	return members, err
}

func (r *GormMemberRepository) ListUserMemberIDs(ctx context.Context, conversationID uint64) ([]uint64, error) {
	var memberIDs []uint64
	err := r.db.WithContext(ctx).
		Model(&model.ConversationMember{}).
		Where("conversation_id = ? AND member_type = ? AND status = ?", conversationID, model.MemberTypeUser, model.MemberStatusNormal).
		Order("joined_at ASC").
		Pluck("member_id", &memberIDs).Error
	return memberIDs, err
}

func (r *GormMemberRepository) GetBotMember(ctx context.Context, conversationID, botID uint64) (*model.ConversationMember, error) {
	var member model.ConversationMember
	err := r.db.WithContext(ctx).
		Where("conversation_id = ? AND member_type = ? AND member_id = ?", conversationID, model.MemberTypeBot, botID).
		First(&member).Error
	if err != nil {
		return nil, err
	}
	return &member, nil
}

func (r *GormMemberRepository) IsBotMember(ctx context.Context, conversationID, botID uint64) (bool, error) {
	_, err := r.GetBotMember(ctx, conversationID, botID)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return false, err
}

func (r *GormMemberRepository) ListBotMembers(ctx context.Context, conversationID uint64) ([]model.ConversationMember, error) {
	var members []model.ConversationMember
	err := r.db.WithContext(ctx).
		Where("conversation_id = ? AND member_type = ? AND status = ?", conversationID, model.MemberTypeBot, model.MemberStatusNormal).
		Order("joined_at ASC").
		Find(&members).Error
	return members, err
}

func (r *GormMemberRepository) ListByConversationID(ctx context.Context, conversationID uint64) ([]model.ConversationMember, error) {
	var members []model.ConversationMember
	err := r.db.WithContext(ctx).
		Where("conversation_id = ? AND status <> ?", conversationID, model.MemberStatusRemoved).
		Order("role ASC, joined_at ASC").
		Find(&members).Error
	return members, err
}

type GormMessageRepository struct {
	db *gorm.DB
}

func NewMessageRepository(db *gorm.DB) *GormMessageRepository {
	return &GormMessageRepository{db: db}
}

func (r *GormMessageRepository) WithTx(tx *gorm.DB) MessageRepository {
	return &GormMessageRepository{db: tx}
}

func (r *GormMessageRepository) Create(ctx context.Context, message *model.Message) error {
	return r.db.WithContext(ctx).Create(message).Error
}

func (r *GormMessageRepository) ListByConversationID(ctx context.Context, conversationID uint64, beforeID *uint64, limit int) ([]model.Message, error) {
	db := r.db.WithContext(ctx).
		Where("conversation_id = ? AND status <> ?", conversationID, model.MessageStatusDeleted)
	if beforeID != nil && *beforeID > 0 {
		db = db.Where("id < ?", *beforeID)
	}

	var messages []model.Message
	err := db.Order("id DESC").Limit(limit).Find(&messages).Error
	return messages, err
}

type GormAICallLogRepository struct {
	db *gorm.DB
}

func NewAICallLogRepository(db *gorm.DB) *GormAICallLogRepository {
	return &GormAICallLogRepository{db: db}
}

func (r *GormAICallLogRepository) WithTx(tx *gorm.DB) AICallLogRepository {
	return &GormAICallLogRepository{db: tx}
}

func (r *GormAICallLogRepository) Create(ctx context.Context, callLog *model.AICallLog) error {
	return r.db.WithContext(ctx).Create(callLog).Error
}

func (r *GormAICallLogRepository) ListByConversationID(ctx context.Context, conversationID uint64, beforeID *uint64, limit int, botID *uint64, status string) ([]model.AICallLog, error) {
	db := r.db.WithContext(ctx).Where("conversation_id = ?", conversationID)
	if beforeID != nil && *beforeID > 0 {
		db = db.Where("id < ?", *beforeID)
	}
	if botID != nil && *botID > 0 {
		db = db.Where("bot_id = ?", *botID)
	}
	if status != "" {
		db = db.Where("status = ?", status)
	}

	var logs []model.AICallLog
	err := db.Order("id DESC").Limit(limit).Find(&logs).Error
	return logs, err
}

func (r *GormAICallLogRepository) SumTotalTokensByConversationBetween(ctx context.Context, conversationID uint64, startAt time.Time, endAt time.Time) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).
		Model(&model.AICallLog{}).
		Where("conversation_id = ? AND created_at >= ? AND created_at < ?", conversationID, startAt, endAt).
		Select("COALESCE(SUM(total_tokens), 0)").
		Scan(&total).Error
	return total, err
}
