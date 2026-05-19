package bot

import (
	"context"
	"fmt"

	"example.com/aim/chat-service/internal/dal/model"
	"example.com/aim/chat-service/internal/repository"
	"gorm.io/gorm"
)

type fakeBotRepo struct {
	bots map[uint64]*model.Bot
}

func (r *fakeBotRepo) WithTx(tx *gorm.DB) repository.BotRepository {
	return r
}

func (r *fakeBotRepo) Create(ctx context.Context, bot *model.Bot) error {
	if r.bots == nil {
		r.bots = make(map[uint64]*model.Bot)
	}
	botCopy := *bot
	r.bots[bot.ID] = &botCopy
	return nil
}

func (r *fakeBotRepo) GetByID(ctx context.Context, id uint64) (*model.Bot, error) {
	if bot, ok := r.bots[id]; ok {
		return bot, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeBotRepo) ListEnabledByOwner(ctx context.Context, ownerID uint64) ([]model.Bot, error) {
	result := make([]model.Bot, 0, len(r.bots))
	for _, item := range r.bots {
		if item.Status == "" || item.Status == model.BotStatusEnabled {
			if item.CreatedBy == 0 || item.CreatedBy == ownerID {
				result = append(result, *item)
			}
		}
	}
	return result, nil
}

func (r *fakeBotRepo) ListEnabled(ctx context.Context) ([]model.Bot, error) {
	result := make([]model.Bot, 0, len(r.bots))
	for _, item := range r.bots {
		if item.Status == "" || item.Status == model.BotStatusEnabled {
			result = append(result, *item)
		}
	}
	return result, nil
}

type fakeConversationBotRepo struct {
	items     map[string]*model.ConversationBot
	nextID    uint64
	createErr error
}

func newFakeConversationBotRepo() *fakeConversationBotRepo {
	return &fakeConversationBotRepo{
		items:  make(map[string]*model.ConversationBot),
		nextID: 1,
	}
}

func (r *fakeConversationBotRepo) WithTx(tx *gorm.DB) repository.ConversationBotRepository {
	return r
}

func (r *fakeConversationBotRepo) Create(ctx context.Context, conversationBot *model.ConversationBot) error {
	if r.createErr != nil {
		return r.createErr
	}
	conversationBot.ID = r.nextID
	r.nextID++
	itemCopy := *conversationBot
	r.items[conversationBotKey(conversationBot.ConversationID, conversationBot.BotID)] = &itemCopy
	return nil
}

func (r *fakeConversationBotRepo) Update(ctx context.Context, conversationBot *model.ConversationBot) error {
	itemCopy := *conversationBot
	r.items[conversationBotKey(conversationBot.ConversationID, conversationBot.BotID)] = &itemCopy
	return nil
}

func (r *fakeConversationBotRepo) GetByConversationAndBotID(ctx context.Context, conversationID, botID uint64) (*model.ConversationBot, error) {
	item, ok := r.items[conversationBotKey(conversationID, botID)]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	itemCopy := *item
	return &itemCopy, nil
}

func (r *fakeConversationBotRepo) ListByConversationID(ctx context.Context, conversationID uint64) ([]model.ConversationBot, error) {
	items := make([]model.ConversationBot, 0)
	for _, item := range r.items {
		if item.ConversationID == conversationID {
			items = append(items, *item)
		}
	}
	return items, nil
}

func conversationBotKey(conversationID, botID uint64) string {
	return fmt.Sprintf("%d:%d", conversationID, botID)
}
