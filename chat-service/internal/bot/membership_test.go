package bot

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"example.com/aim/chat-service/internal/dal/model"
	"example.com/aim/chat-service/internal/repository"
	"gorm.io/gorm"
)

func TestMembershipServiceAddBotToConversationCreatesMemberAndConversationBot(t *testing.T) {
	txManager := &fakeMembershipTxManager{}
	conversationRepo := &fakeMembershipConversationRepo{
		conversations: map[uint64]*model.Conversation{
			10: {ID: 10, Type: model.ConversationTypeGroup},
		},
	}
	memberRepo := newFakeMembershipMemberRepo()
	botRepo := &fakeBotRepo{
		bots: map[uint64]*model.Bot{
			7: {ID: 7, Name: "AIM"},
		},
	}
	conversationBotRepo := newFakeConversationBotRepo()
	service := NewMembershipService(txManager, conversationRepo, memberRepo, botRepo, conversationBotRepo)

	if err := service.AddBotToConversation(context.Background(), 10, 7); err != nil {
		t.Fatalf("AddBotToConversation returned error: %v", err)
	}
	if !txManager.called {
		t.Fatal("AddBotToConversation did not use transaction")
	}

	member, err := memberRepo.GetBotMember(context.Background(), 10, 7)
	if err != nil {
		t.Fatalf("bot member missing: %v", err)
	}
	if member.MemberType != model.MemberTypeBot || member.MemberID != 7 || member.Role != model.MemberRoleBot || member.Status != model.MemberStatusNormal {
		t.Fatalf("unexpected bot member: %+v", member)
	}

	conversationBot, err := conversationBotRepo.GetByConversationAndBotID(context.Background(), 10, 7)
	if err != nil {
		t.Fatalf("conversation bot missing: %v", err)
	}
	if !conversationBot.Enabled || conversationBot.PermissionScope != model.BotScopeConversationOnly {
		t.Fatalf("unexpected conversation bot: %+v", conversationBot)
	}
}

func TestMembershipServiceAddBotToConversationRestoresExistingRecords(t *testing.T) {
	txManager := &fakeMembershipTxManager{}
	conversationRepo := &fakeMembershipConversationRepo{
		conversations: map[uint64]*model.Conversation{
			10: {ID: 10, Type: model.ConversationTypeGroup},
		},
	}
	memberRepo := newFakeMembershipMemberRepo()
	_ = memberRepo.Create(context.Background(), &model.ConversationMember{
		ConversationID: 10,
		MemberType:     model.MemberTypeBot,
		MemberID:       7,
		Role:           model.MemberRoleMember,
		Status:         model.MemberStatusRemoved,
		JoinedAt:       time.Now().Add(-time.Hour),
	})
	botRepo := &fakeBotRepo{
		bots: map[uint64]*model.Bot{
			7: {ID: 7, Name: "AIM"},
		},
	}
	conversationBotRepo := newFakeConversationBotRepo()
	_ = conversationBotRepo.Create(context.Background(), &model.ConversationBot{
		ConversationID:  10,
		BotID:           7,
		Enabled:         false,
		PermissionScope: "",
	})
	service := NewMembershipService(txManager, conversationRepo, memberRepo, botRepo, conversationBotRepo)

	if err := service.AddBotToConversation(context.Background(), 10, 7); err != nil {
		t.Fatalf("AddBotToConversation returned error: %v", err)
	}

	member, err := memberRepo.GetBotMember(context.Background(), 10, 7)
	if err != nil {
		t.Fatalf("bot member missing: %v", err)
	}
	if member.Role != model.MemberRoleBot || member.Status != model.MemberStatusNormal {
		t.Fatalf("bot member was not restored: %+v", member)
	}

	conversationBot, err := conversationBotRepo.GetByConversationAndBotID(context.Background(), 10, 7)
	if err != nil {
		t.Fatalf("conversation bot missing: %v", err)
	}
	if !conversationBot.Enabled || conversationBot.PermissionScope != model.BotScopeConversationOnly {
		t.Fatalf("conversation bot was not restored: %+v", conversationBot)
	}
}

func TestMembershipServiceAddBotToConversationWithConfigPersistsOverrides(t *testing.T) {
	txManager := &fakeMembershipTxManager{}
	conversationRepo := &fakeMembershipConversationRepo{
		conversations: map[uint64]*model.Conversation{
			10: {ID: 10, Type: model.ConversationTypeGroup},
		},
	}
	memberRepo := newFakeMembershipMemberRepo()
	botRepo := &fakeBotRepo{
		bots: map[uint64]*model.Bot{
			7: {ID: 7, Name: "AIM"},
		},
	}
	conversationBotRepo := newFakeConversationBotRepo()
	service := NewMembershipService(txManager, conversationRepo, memberRepo, botRepo, conversationBotRepo)

	err := service.AddBotToConversationWithConfig(context.Background(), 10, 7, ConversationBotConfig{
		DisplayNameOverride: "群助手",
		MentionNameOverride: "helper",
		AliasesOverride:     "[\"aim-helper\"]",
		PermissionScope:     model.BotScopeConversationOnly,
	})
	if err != nil {
		t.Fatalf("AddBotToConversationWithConfig returned error: %v", err)
	}

	conversationBot, err := conversationBotRepo.GetByConversationAndBotID(context.Background(), 10, 7)
	if err != nil {
		t.Fatalf("conversation bot missing: %v", err)
	}
	if conversationBot.DisplayNameOverride != "群助手" ||
		conversationBot.MentionNameOverride != "helper" ||
		conversationBot.AliasesOverride != "[\"aim-helper\"]" ||
		conversationBot.PermissionScope != model.BotScopeConversationOnly {
		t.Fatalf("unexpected conversation bot config: %+v", conversationBot)
	}
}

func TestMembershipServiceRemoveBotFromConversationDisablesBothTables(t *testing.T) {
	txManager := &fakeMembershipTxManager{}
	memberRepo := newFakeMembershipMemberRepo()
	_ = memberRepo.Create(context.Background(), &model.ConversationMember{
		ConversationID: 10,
		MemberType:     model.MemberTypeBot,
		MemberID:       7,
		Role:           model.MemberRoleBot,
		Status:         model.MemberStatusNormal,
		JoinedAt:       time.Now(),
	})
	conversationBotRepo := newFakeConversationBotRepo()
	_ = conversationBotRepo.Create(context.Background(), &model.ConversationBot{
		ConversationID:  10,
		BotID:           7,
		Enabled:         true,
		PermissionScope: model.BotScopeConversationOnly,
	})
	service := NewMembershipService(txManager, &fakeMembershipConversationRepo{}, memberRepo, &fakeBotRepo{}, conversationBotRepo)

	if err := service.RemoveBotFromConversation(context.Background(), 10, 7); err != nil {
		t.Fatalf("RemoveBotFromConversation returned error: %v", err)
	}
	if !txManager.called {
		t.Fatal("RemoveBotFromConversation did not use transaction")
	}

	member, err := memberRepo.GetBotMember(context.Background(), 10, 7)
	if err != nil {
		t.Fatalf("bot member missing: %v", err)
	}
	if member.Status != model.MemberStatusRemoved {
		t.Fatalf("bot member was not removed: %+v", member)
	}

	conversationBot, err := conversationBotRepo.GetByConversationAndBotID(context.Background(), 10, 7)
	if err != nil {
		t.Fatalf("conversation bot missing: %v", err)
	}
	if conversationBot.Enabled {
		t.Fatalf("conversation bot was not disabled: %+v", conversationBot)
	}
}

func TestMembershipServiceAddBotToConversationRejectsNonGroupConversation(t *testing.T) {
	service := NewMembershipService(
		&fakeMembershipTxManager{},
		&fakeMembershipConversationRepo{
			conversations: map[uint64]*model.Conversation{
				10: {ID: 10, Type: model.ConversationTypeSingle},
			},
		},
		newFakeMembershipMemberRepo(),
		&fakeBotRepo{bots: map[uint64]*model.Bot{7: {ID: 7}}},
		newFakeConversationBotRepo(),
	)

	err := service.AddBotToConversation(context.Background(), 10, 7)
	if !errors.Is(err, ErrConversationTypeNotSupported) {
		t.Fatalf("expected ErrConversationTypeNotSupported, got %v", err)
	}
}

func TestMembershipServiceAddBotToConversationReturnsConversationBotWriteError(t *testing.T) {
	txManager := &fakeMembershipTxManager{}
	service := NewMembershipService(
		txManager,
		&fakeMembershipConversationRepo{
			conversations: map[uint64]*model.Conversation{
				10: {ID: 10, Type: model.ConversationTypeGroup},
			},
		},
		newFakeMembershipMemberRepo(),
		&fakeBotRepo{bots: map[uint64]*model.Bot{7: {ID: 7}}},
		&fakeConversationBotRepo{
			items:     make(map[string]*model.ConversationBot),
			nextID:    1,
			createErr: errors.New("write conversation bot failed"),
		},
	)

	err := service.AddBotToConversation(context.Background(), 10, 7)
	if err == nil || err.Error() != "write conversation bot failed" {
		t.Fatalf("expected conversation bot error, got %v", err)
	}
	if !txManager.called {
		t.Fatal("transaction was not used")
	}
}

func TestMembershipServiceRemoveBotFromConversationReturnsErrWhenNotAttached(t *testing.T) {
	service := NewMembershipService(
		&fakeMembershipTxManager{},
		&fakeMembershipConversationRepo{},
		newFakeMembershipMemberRepo(),
		&fakeBotRepo{},
		newFakeConversationBotRepo(),
	)

	err := service.RemoveBotFromConversation(context.Background(), 10, 7)
	if !errors.Is(err, ErrBotNotInConversation) {
		t.Fatalf("expected ErrBotNotInConversation, got %v", err)
	}
}

type fakeMembershipTxManager struct {
	called bool
}

func (m *fakeMembershipTxManager) WithinTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	m.called = true
	return fn(nil)
}

type fakeMembershipConversationRepo struct {
	conversations map[uint64]*model.Conversation
}

func (r *fakeMembershipConversationRepo) WithTx(tx *gorm.DB) repository.ConversationRepository {
	return r
}

func (r *fakeMembershipConversationRepo) Create(ctx context.Context, conversation *model.Conversation) error {
	return nil
}

func (r *fakeMembershipConversationRepo) GetByID(ctx context.Context, id uint64) (*model.Conversation, error) {
	if conversation, ok := r.conversations[id]; ok {
		return conversation, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeMembershipConversationRepo) GetByConversationID(ctx context.Context, conversationID string) (*model.Conversation, error) {
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeMembershipConversationRepo) FindSingleByUsers(ctx context.Context, userID uint64, peerUserID uint64) (*model.Conversation, error) {
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeMembershipConversationRepo) ListByUserID(ctx context.Context, userID uint64) ([]repository.ConversationListRow, error) {
	return nil, nil
}

func (r *fakeMembershipConversationRepo) UpdateLastMessage(ctx context.Context, conversationID uint64, messageID uint64, at time.Time) error {
	return nil
}

type fakeMembershipMemberRepo struct {
	members map[string]*model.ConversationMember
	nextID  uint64
}

func newFakeMembershipMemberRepo() *fakeMembershipMemberRepo {
	return &fakeMembershipMemberRepo{
		members: make(map[string]*model.ConversationMember),
		nextID:  1,
	}
}

func (r *fakeMembershipMemberRepo) WithTx(tx *gorm.DB) repository.MemberRepository {
	return r
}

func (r *fakeMembershipMemberRepo) Create(ctx context.Context, member *model.ConversationMember) error {
	member.ID = r.nextID
	r.nextID++
	memberCopy := *member
	r.members[membershipKey(member.ConversationID, member.MemberType, member.MemberID)] = &memberCopy
	return nil
}

func (r *fakeMembershipMemberRepo) Update(ctx context.Context, member *model.ConversationMember) error {
	memberCopy := *member
	r.members[membershipKey(member.ConversationID, member.MemberType, member.MemberID)] = &memberCopy
	return nil
}

func (r *fakeMembershipMemberRepo) UpdateLastReadMessageID(ctx context.Context, conversationID, userID, lastReadMessageID uint64) error {
	member, err := r.GetUserMember(ctx, conversationID, userID)
	if err != nil {
		return err
	}
	member.LastReadMessageID = &[]uint64{lastReadMessageID}[0]
	return r.Update(ctx, member)
}

func (r *fakeMembershipMemberRepo) GetDB() *gorm.DB {
	return nil
}

func (r *fakeMembershipMemberRepo) GetUserMember(ctx context.Context, conversationID, userID uint64) (*model.ConversationMember, error) {
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeMembershipMemberRepo) IsUserMember(ctx context.Context, conversationID, userID uint64) (bool, error) {
	return false, nil
}

func (r *fakeMembershipMemberRepo) ListUserMembers(ctx context.Context, conversationID uint64) ([]model.ConversationMember, error) {
	return nil, nil
}

func (r *fakeMembershipMemberRepo) ListUserMemberIDs(ctx context.Context, conversationID uint64) ([]uint64, error) {
	return nil, nil
}

func (r *fakeMembershipMemberRepo) GetBotMember(ctx context.Context, conversationID, botID uint64) (*model.ConversationMember, error) {
	member, ok := r.members[membershipKey(conversationID, model.MemberTypeBot, botID)]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	memberCopy := *member
	return &memberCopy, nil
}

func (r *fakeMembershipMemberRepo) IsBotMember(ctx context.Context, conversationID, botID uint64) (bool, error) {
	_, err := r.GetBotMember(ctx, conversationID, botID)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return false, err
}

func (r *fakeMembershipMemberRepo) ListBotMembers(ctx context.Context, conversationID uint64) ([]model.ConversationMember, error) {
	members := make([]model.ConversationMember, 0)
	for _, member := range r.members {
		if member.ConversationID == conversationID && member.MemberType == model.MemberTypeBot && member.Status == model.MemberStatusNormal {
			members = append(members, *member)
		}
	}
	return members, nil
}

func (r *fakeMembershipMemberRepo) ListByConversationID(ctx context.Context, conversationID uint64) ([]model.ConversationMember, error) {
	return nil, nil
}

type fakeBotRepo struct {
	bots map[uint64]*model.Bot
}

func (r *fakeBotRepo) WithTx(tx *gorm.DB) repository.BotRepository {
	return r
}

func (r *fakeBotRepo) GetByID(ctx context.Context, id uint64) (*model.Bot, error) {
	if bot, ok := r.bots[id]; ok {
		return bot, nil
	}
	return nil, gorm.ErrRecordNotFound
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

func membershipKey(conversationID uint64, memberType model.MemberType, memberID uint64) string {
	return fmt.Sprintf("%d:%s:%d", conversationID, memberType, memberID)
}

func conversationBotKey(conversationID, botID uint64) string {
	return fmt.Sprintf("%d:%d", conversationID, botID)
}
