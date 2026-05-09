package repository

import (
	"context"

	"example.com/aim/chat-service/internal/dal/model"
	"gorm.io/gorm"
)

type BotRepository interface {
	WithTx(tx *gorm.DB) BotRepository
	GetByID(ctx context.Context, id uint64) (*model.Bot, error)
	ListEnabled(ctx context.Context) ([]model.Bot, error)
}

type ConversationBotRepository interface {
	WithTx(tx *gorm.DB) ConversationBotRepository
	Create(ctx context.Context, conversationBot *model.ConversationBot) error
	Update(ctx context.Context, conversationBot *model.ConversationBot) error
	GetByConversationAndBotID(ctx context.Context, conversationID, botID uint64) (*model.ConversationBot, error)
	ListByConversationID(ctx context.Context, conversationID uint64) ([]model.ConversationBot, error)
}

type GormBotRepository struct {
	db *gorm.DB
}

func NewBotRepository(db *gorm.DB) *GormBotRepository {
	return &GormBotRepository{db: db}
}

func (r *GormBotRepository) WithTx(tx *gorm.DB) BotRepository {
	return &GormBotRepository{db: tx}
}

func (r *GormBotRepository) GetByID(ctx context.Context, id uint64) (*model.Bot, error) {
	var bot model.Bot
	if err := r.db.WithContext(ctx).First(&bot, id).Error; err != nil {
		return nil, err
	}
	return &bot, nil
}

func (r *GormBotRepository) ListEnabled(ctx context.Context) ([]model.Bot, error) {
	var bots []model.Bot
	err := r.db.WithContext(ctx).
		Where("status = ? AND created_by = ?", model.BotStatusEnabled, 0).
		Order("id ASC").
		Find(&bots).Error
	return bots, err
}

type GormConversationBotRepository struct {
	db *gorm.DB
}

func NewConversationBotRepository(db *gorm.DB) *GormConversationBotRepository {
	return &GormConversationBotRepository{db: db}
}

func (r *GormConversationBotRepository) WithTx(tx *gorm.DB) ConversationBotRepository {
	return &GormConversationBotRepository{db: tx}
}

func (r *GormConversationBotRepository) Create(ctx context.Context, conversationBot *model.ConversationBot) error {
	return r.db.WithContext(ctx).Create(conversationBot).Error
}

func (r *GormConversationBotRepository) Update(ctx context.Context, conversationBot *model.ConversationBot) error {
	return r.db.WithContext(ctx).Save(conversationBot).Error
}

func (r *GormConversationBotRepository) GetByConversationAndBotID(ctx context.Context, conversationID, botID uint64) (*model.ConversationBot, error) {
	var conversationBot model.ConversationBot
	err := r.db.WithContext(ctx).
		Where("conversation_id = ? AND bot_id = ?", conversationID, botID).
		First(&conversationBot).Error
	if err != nil {
		return nil, err
	}
	return &conversationBot, nil
}

func (r *GormConversationBotRepository) ListByConversationID(ctx context.Context, conversationID uint64) ([]model.ConversationBot, error) {
	var conversationBots []model.ConversationBot
	err := r.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Order("id ASC").
		Find(&conversationBots).Error
	return conversationBots, err
}
