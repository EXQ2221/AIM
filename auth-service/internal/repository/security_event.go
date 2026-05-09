package repository

import (
	"context"

	"example.com/aim/auth-service/internal/dal/model"
	"gorm.io/gorm"
)

type SecurityEventRepository interface {
	WithTx(tx *gorm.DB) SecurityEventRepository
	Create(ctx context.Context, event *model.SecurityEvent) error
}

type GormSecurityEventRepository struct {
	db *gorm.DB
}

func NewSecurityEventRepository(db *gorm.DB) *GormSecurityEventRepository {
	return &GormSecurityEventRepository{db: db}
}

func (r *GormSecurityEventRepository) WithTx(tx *gorm.DB) SecurityEventRepository {
	return &GormSecurityEventRepository{db: tx}
}

func (r *GormSecurityEventRepository) Create(ctx context.Context, event *model.SecurityEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}
