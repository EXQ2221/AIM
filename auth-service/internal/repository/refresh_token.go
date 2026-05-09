package repository

import (
	"context"
	"time"

	"example.com/aim/auth-service/internal/dal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RefreshTokenRepository interface {
	WithTx(tx *gorm.DB) RefreshTokenRepository
	Create(ctx context.Context, token *model.RefreshToken) error
	Update(ctx context.Context, token *model.RefreshToken) error
	GetByTokenHash(ctx context.Context, tokenHash string) (*model.RefreshToken, error)
	GetByTokenHashForUpdate(ctx context.Context, tokenHash string) (*model.RefreshToken, error)
	RevokeActiveBySessionID(ctx context.Context, sessionID string, reason string, revokedAt time.Time) error
}

type GormRefreshTokenRepository struct {
	db *gorm.DB
}

func NewRefreshTokenRepository(db *gorm.DB) *GormRefreshTokenRepository {
	return &GormRefreshTokenRepository{db: db}
}

func (r *GormRefreshTokenRepository) Create(ctx context.Context, token *model.RefreshToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

func (r *GormRefreshTokenRepository) WithTx(tx *gorm.DB) RefreshTokenRepository {
	return &GormRefreshTokenRepository{db: tx}
}

func (r *GormRefreshTokenRepository) Update(ctx context.Context, token *model.RefreshToken) error {
	return r.db.WithContext(ctx).Save(token).Error
}

func (r *GormRefreshTokenRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*model.RefreshToken, error) {
	var token model.RefreshToken
	if err := r.db.WithContext(ctx).Where("token_hash = ?", tokenHash).First(&token).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

func (r *GormRefreshTokenRepository) GetByTokenHashForUpdate(ctx context.Context, tokenHash string) (*model.RefreshToken, error) {
	var token model.RefreshToken
	if err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("token_hash = ?", tokenHash).
		First(&token).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

func (r *GormRefreshTokenRepository) RevokeActiveBySessionID(ctx context.Context, sessionID string, reason string, revokedAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&model.RefreshToken{}).
		Where("session_id = ? AND status = ?", sessionID, "active").
		Updates(map[string]any{
			"status":        "revoked",
			"revoked_at":    revokedAt,
			"revoke_reason": reason,
		}).Error
}
