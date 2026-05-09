package repository

import (
	"context"

	"gorm.io/gorm"
)

type TxManager interface {
	WithinTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error
}

type GormTxManager struct {
	db *gorm.DB
}

func NewTxManager(db *gorm.DB) *GormTxManager {
	return &GormTxManager{db: db}
}

func (m *GormTxManager) WithinTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(tx)
	})
}
