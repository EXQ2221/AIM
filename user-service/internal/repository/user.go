package repository

import (
	"context"
	"time"

	"example.com/aim/user-service/internal/dal/model"
	"gorm.io/gorm"
)

type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	GetByID(ctx context.Context, id uint64) (*model.User, error)
	ListByIDs(ctx context.Context, ids []uint64) ([]model.User, error)
	GetByAimID(ctx context.Context, aimID string) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	UpdateLoginState(ctx context.Context, userID uint64, ip string, loginAt time.Time) error
	BumpTokenVersion(ctx context.Context, userID uint64) error
	UpdateAvatar(ctx context.Context, userID uint64, avatar string) error
}

type GormUserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *GormUserRepository {
	return &GormUserRepository{db: db}
}

func (r *GormUserRepository) Create(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *GormUserRepository) GetByID(ctx context.Context, id uint64) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *GormUserRepository) ListByIDs(ctx context.Context, ids []uint64) ([]model.User, error) {
	if len(ids) == 0 {
		return []model.User{}, nil
	}

	var users []model.User
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (r *GormUserRepository) GetByAimID(ctx context.Context, aimID string) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).Where("aim_id = ?", aimID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *GormUserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *GormUserRepository) UpdateLoginState(ctx context.Context, userID uint64, ip string, loginAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", userID).
		Updates(map[string]any{
			"last_login_at":    loginAt,
			"last_login_ip":    ip,
			"login_fail_count": 0,
		}).Error
}

func (r *GormUserRepository) BumpTokenVersion(ctx context.Context, userID uint64) error {
	return r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", userID).
		UpdateColumn("token_version", gorm.Expr("token_version + ?", 1)).Error
}

func (r *GormUserRepository) UpdateAvatar(ctx context.Context, userID uint64, avatar string) error {
	return r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", userID).
		Update("avatar", avatar).Error
}
