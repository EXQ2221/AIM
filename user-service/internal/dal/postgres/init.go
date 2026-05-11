package postgres

import (
	"time"

	"example.com/aim/user-service/internal/dal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Init(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	if err := db.AutoMigrate(&model.User{}); err != nil {
		return nil, err
	}

	if err := db.AutoMigrate(&model.FriendGroup{}, &model.FriendRelation{}, &model.FriendRequest{}); err != nil {
		return nil, err
	}

	return db, nil
}

