package repository

import (
	"context"

	"example.com/aim/user-service/internal/dal/model"
	"gorm.io/gorm"
)

type FriendGroupRepository interface {
	WithTx(tx *gorm.DB) FriendGroupRepository
	Create(ctx context.Context, group *model.FriendGroup) error
	ListByUserID(ctx context.Context, userID uint64) ([]model.FriendGroup, error)
	GetByIDAndUserID(ctx context.Context, id, userID uint64) (*model.FriendGroup, error)
}

type FriendRelationRepository interface {
	WithTx(tx *gorm.DB) FriendRelationRepository
	Create(ctx context.Context, relation *model.FriendRelation) error
	GetByUserPair(ctx context.Context, userID, friendUserID uint64) (*model.FriendRelation, error)
	ListByUserID(ctx context.Context, userID uint64) ([]model.FriendRelation, error)
	Update(ctx context.Context, relation *model.FriendRelation) error
	DeleteByUserPair(ctx context.Context, userID, friendUserID uint64) error
}

type FriendRequestRepository interface {
	WithTx(tx *gorm.DB) FriendRequestRepository
	Create(ctx context.Context, request *model.FriendRequest) error
	GetByID(ctx context.Context, id uint64) (*model.FriendRequest, error)
	GetByUserPair(ctx context.Context, userID, targetUserID uint64) (*model.FriendRequest, error)
	ListByUserID(ctx context.Context, userID uint64) ([]model.FriendRequest, error)
	Update(ctx context.Context, request *model.FriendRequest) error
}

type GormFriendGroupRepository struct {
	db *gorm.DB
}

func NewFriendGroupRepository(db *gorm.DB) *GormFriendGroupRepository {
	return &GormFriendGroupRepository{db: db}
}

func (r *GormFriendGroupRepository) WithTx(tx *gorm.DB) FriendGroupRepository {
	return &GormFriendGroupRepository{db: tx}
}

func (r *GormFriendGroupRepository) Create(ctx context.Context, group *model.FriendGroup) error {
	return r.db.WithContext(ctx).Create(group).Error
}

func (r *GormFriendGroupRepository) ListByUserID(ctx context.Context, userID uint64) ([]model.FriendGroup, error) {
	var groups []model.FriendGroup
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("sort_order ASC, id ASC").
		Find(&groups).Error
	return groups, err
}

func (r *GormFriendGroupRepository) GetByIDAndUserID(ctx context.Context, id, userID uint64) (*model.FriendGroup, error) {
	var group model.FriendGroup
	if err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		First(&group).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

type GormFriendRelationRepository struct {
	db *gorm.DB
}

func NewFriendRelationRepository(db *gorm.DB) *GormFriendRelationRepository {
	return &GormFriendRelationRepository{db: db}
}

func (r *GormFriendRelationRepository) WithTx(tx *gorm.DB) FriendRelationRepository {
	return &GormFriendRelationRepository{db: tx}
}

func (r *GormFriendRelationRepository) Create(ctx context.Context, relation *model.FriendRelation) error {
	return r.db.WithContext(ctx).Create(relation).Error
}

func (r *GormFriendRelationRepository) GetByUserPair(ctx context.Context, userID, friendUserID uint64) (*model.FriendRelation, error) {
	var relation model.FriendRelation
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND friend_user_id = ?", userID, friendUserID).
		First(&relation).Error; err != nil {
		return nil, err
	}
	return &relation, nil
}

func (r *GormFriendRelationRepository) ListByUserID(ctx context.Context, userID uint64) ([]model.FriendRelation, error) {
	var relations []model.FriendRelation
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("updated_at DESC, id DESC").
		Find(&relations).Error
	return relations, err
}

func (r *GormFriendRelationRepository) Update(ctx context.Context, relation *model.FriendRelation) error {
	return r.db.WithContext(ctx).Save(relation).Error
}

func (r *GormFriendRelationRepository) DeleteByUserPair(ctx context.Context, userID, friendUserID uint64) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND friend_user_id = ?", userID, friendUserID).
		Delete(&model.FriendRelation{}).Error
}

type GormFriendRequestRepository struct {
	db *gorm.DB
}

func NewFriendRequestRepository(db *gorm.DB) *GormFriendRequestRepository {
	return &GormFriendRequestRepository{db: db}
}

func (r *GormFriendRequestRepository) WithTx(tx *gorm.DB) FriendRequestRepository {
	return &GormFriendRequestRepository{db: tx}
}

func (r *GormFriendRequestRepository) Create(ctx context.Context, request *model.FriendRequest) error {
	return r.db.WithContext(ctx).Create(request).Error
}

func (r *GormFriendRequestRepository) GetByID(ctx context.Context, id uint64) (*model.FriendRequest, error) {
	var request model.FriendRequest
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&request).Error; err != nil {
		return nil, err
	}
	return &request, nil
}

func (r *GormFriendRequestRepository) GetByUserPair(ctx context.Context, userID, targetUserID uint64) (*model.FriendRequest, error) {
	var request model.FriendRequest
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND target_user_id = ?", userID, targetUserID).
		First(&request).Error; err != nil {
		return nil, err
	}
	return &request, nil
}

func (r *GormFriendRequestRepository) ListByUserID(ctx context.Context, userID uint64) ([]model.FriendRequest, error) {
	var requests []model.FriendRequest
	err := r.db.WithContext(ctx).
		Where("user_id = ? OR target_user_id = ?", userID, userID).
		Order("updated_at DESC, id DESC").
		Find(&requests).Error
	return requests, err
}

func (r *GormFriendRequestRepository) Update(ctx context.Context, request *model.FriendRequest) error {
	return r.db.WithContext(ctx).Save(request).Error
}
