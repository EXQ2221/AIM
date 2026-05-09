package biz

import (
	"context"
	"errors"
	"testing"
	"time"

	"example.com/aim/chat-service/internal/bot"
	"example.com/aim/chat-service/internal/dal/model"
	"example.com/aim/chat-service/internal/repository"
	"example.com/aim/chat-service/internal/rpc"
	"gorm.io/gorm"
)

func TestCreateGroupCreatesRecordsInTransaction(t *testing.T) {
	conversationRepo := newFakeConversationRepo()
	groupRepo := newFakeGroupRepo()
	memberRepo := newFakeMemberRepo()
	txManager := &fakeTxManager{}
	service := NewChatService(conversationRepo, groupRepo, memberRepo, newFakeMessageRepo(), txManager, nil)

	group, err := service.CreateGroup(context.Background(), CreateGroupInput{
		OperatorID:   10001,
		Name:         "test group",
		Announcement: "hello",
		JoinPolicy:   "FREE",
	})
	if err != nil {
		t.Fatalf("CreateGroup returned error: %v", err)
	}
	if !txManager.called {
		t.Fatal("CreateGroup did not use transaction")
	}
	if group.ConversationID == "" || group.OwnerID != 10001 || group.JoinPolicy != string(model.GroupJoinFree) {
		t.Fatalf("unexpected group view: %+v", group)
	}

	member, err := memberRepo.GetUserMember(context.Background(), 1, 10001)
	if err != nil {
		t.Fatalf("owner member was not created: %v", err)
	}
	if member.Role != model.MemberRoleOwner || member.Status != model.MemberStatusNormal {
		t.Fatalf("unexpected owner member: %+v", member)
	}
}

func TestCreateSingleConversationCreatesConversationAndMembers(t *testing.T) {
	conversationRepo := newFakeConversationRepo()
	groupRepo := newFakeGroupRepo()
	memberRepo := newFakeMemberRepo()
	service := NewChatService(
		conversationRepo,
		groupRepo,
		memberRepo,
		newFakeMessageRepo(),
		&fakeTxManager{},
		&fakeUserClient{users: map[uint64]*rpc.UserInfo{
			10001: {UserID: 10001, Nickname: "alice", Status: "NORMAL"},
			10002: {UserID: 10002, Nickname: "bob", Status: "NORMAL", Avatar: "/b.png"},
		}},
	)

	conversation, err := service.CreateSingleConversation(context.Background(), CreateSingleConversationInput{
		OperatorID: 10001,
		TargetID:   10002,
	})
	if err != nil {
		t.Fatalf("CreateSingleConversation returned error: %v", err)
	}
	if conversation.Type != string(model.ConversationTypeSingle) {
		t.Fatalf("unexpected conversation type: %+v", conversation)
	}
	if conversation.Title != "bob" {
		t.Fatalf("expected single conversation title from peer, got %+v", conversation)
	}
	if _, err := memberRepo.GetUserMember(context.Background(), 1, 10001); err != nil {
		t.Fatalf("operator member missing: %v", err)
	}
	if _, err := memberRepo.GetUserMember(context.Background(), 1, 10002); err != nil {
		t.Fatalf("target member missing: %v", err)
	}
}

func TestCreateSingleConversationReturnsExistingConversation(t *testing.T) {
	conversationRepo := newFakeConversationRepo()
	memberRepo := newFakeMemberRepo()
	service := NewChatService(
		conversationRepo,
		newFakeGroupRepo(),
		memberRepo,
		newFakeMessageRepo(),
		&fakeTxManager{},
		&fakeUserClient{users: map[uint64]*rpc.UserInfo{
			10001: {UserID: 10001, Nickname: "alice", Status: "NORMAL"},
			10002: {UserID: 10002, Nickname: "bob", Status: "NORMAL"},
		}},
	)

	conversationRepo.conversations[1] = &model.Conversation{ID: 1, ConversationID: "c_single", Type: model.ConversationTypeSingle}
	_ = memberRepo.Create(context.Background(), &model.ConversationMember{
		ConversationID: 1, MemberType: model.MemberTypeUser, MemberID: 10001, Role: model.MemberRoleMember, Status: model.MemberStatusNormal, JoinedAt: time.Now(),
	})
	_ = memberRepo.Create(context.Background(), &model.ConversationMember{
		ConversationID: 1, MemberType: model.MemberTypeUser, MemberID: 10002, Role: model.MemberRoleMember, Status: model.MemberStatusNormal, JoinedAt: time.Now(),
	})

	conversation, err := service.CreateSingleConversation(context.Background(), CreateSingleConversationInput{
		OperatorID: 10001,
		TargetID:   10002,
	})
	if err != nil {
		t.Fatalf("CreateSingleConversation returned error: %v", err)
	}
	if conversation.ConversationID != "c_single" {
		t.Fatalf("expected existing conversation, got %+v", conversation)
	}
	if len(conversationRepo.conversations) != 1 {
		t.Fatalf("expected no duplicate single conversations, got %d", len(conversationRepo.conversations))
	}
}

func TestListConversationsUsesBotNameForLastBotReply(t *testing.T) {
	conversationRepo := newFakeConversationRepo()
	conversationRepo.listRows = []repository.ConversationListRow{
		{
			ConversationID:        "c_group",
			Type:                  string(model.ConversationTypeGroup),
			Title:                 "test1",
			LastMessageSenderID:   uint64Ptr(100000),
			LastMessageSenderType: string(model.SenderTypeBot),
			LastMessageContent:    "bot reply",
			UpdatedAt:             time.Now(),
		},
	}
	service := NewChatService(
		conversationRepo,
		newFakeGroupRepo(),
		newFakeMemberRepo(),
		newFakeMessageRepo(),
		&fakeTxManager{},
		&fakeUserClient{},
	)
	service.BotRepo = &fakeConversationListBotRepo{
		bots: map[uint64]*model.Bot{
			100000: {ID: 100000, Name: "AIM Bot", MentionName: "aim"},
		},
	}

	conversations, err := service.ListConversations(context.Background(), 10001)
	if err != nil {
		t.Fatalf("ListConversations returned error: %v", err)
	}
	if len(conversations) != 1 {
		t.Fatalf("expected one conversation, got %d", len(conversations))
	}
	if conversations[0].LastMessageSenderName != "AIM Bot" {
		t.Fatalf("expected bot sender name, got %+v", conversations[0])
	}
}

func TestListAICallLogsReturnsConversationLogs(t *testing.T) {
	conversationRepo := newFakeConversationRepo()
	memberRepo := newFakeMemberRepo()
	now := time.Now()
	service := NewChatService(
		conversationRepo,
		newFakeGroupRepo(),
		memberRepo,
		newFakeMessageRepo(),
		&fakeTxManager{},
		nil,
	)
	service.SetAICallLogRepository(&fakeListAICallLogRepo{
		logs: []model.AICallLog{
			{
				ID:                9,
				UserID:            10001,
				BotID:             100000,
				ConversationID:    1,
				RequestMessageID:  uint64Ptr(101),
				ResponseMessageID: uint64Ptr(102),
				ModelName:         "deepseek-chat",
				PromptTokens:      10,
				CompletionTokens:  20,
				TotalTokens:       30,
				LatencyMS:         456,
				Status:            model.AICallStatusSuccess,
				CreatedAt:         now,
			},
		},
	})
	service.BotRepo = &fakeConversationListBotRepo{
		bots: map[uint64]*model.Bot{
			100000: {ID: 100000, Name: "AIM Bot"},
		},
	}

	conversationRepo.conversations[1] = &model.Conversation{ID: 1, ConversationID: "c_test", Type: model.ConversationTypeGroup}
	_ = memberRepo.Create(context.Background(), &model.ConversationMember{
		ConversationID: 1,
		MemberType:     model.MemberTypeUser,
		MemberID:       10001,
		Role:           model.MemberRoleOwner,
		Status:         model.MemberStatusNormal,
		JoinedAt:       time.Now(),
	})

	result, err := service.ListAICallLogs(context.Background(), 10001, "c_test", nil, 30, nil, "")
	if err != nil {
		t.Fatalf("ListAICallLogs returned error: %v", err)
	}
	if len(result.Logs) != 1 {
		t.Fatalf("expected one log, got %d", len(result.Logs))
	}
	if result.Logs[0].BotName != "AIM Bot" || result.Logs[0].ConversationID != "c_test" || result.Logs[0].TotalTokens != 30 {
		t.Fatalf("unexpected log view: %+v", result.Logs[0])
	}
	if result.Quota.DailyTokenLimit != 1_000_000 || result.Quota.DailyTotalTokens != 30 {
		t.Fatalf("unexpected quota view: %+v", result.Quota)
	}
}

func TestCreateMessageRejectsNonMember(t *testing.T) {
	service := newMessageTestService(model.MemberRoleMember, model.MemberStatusNormal, false)
	_, err := service.CreateMessage(context.Background(), 20002, "c_test", "hello", nil)
	if !errors.Is(err, ErrNotMember) {
		t.Fatalf("expected ErrNotMember, got %v", err)
	}
}

func TestCreateMessageRejectsMutedMember(t *testing.T) {
	service := newMessageTestService(model.MemberRoleMember, model.MemberStatusMuted, false)
	_, err := service.CreateMessage(context.Background(), 10001, "c_test", "hello", nil)
	if !errors.Is(err, ErrMemberMuted) {
		t.Fatalf("expected ErrMemberMuted, got %v", err)
	}
}

func TestCreateMessageRejectsGroupMuteAllForMember(t *testing.T) {
	service := newMessageTestService(model.MemberRoleMember, model.MemberStatusNormal, true)
	_, err := service.CreateMessage(context.Background(), 10001, "c_test", "hello", nil)
	if !errors.Is(err, ErrGroupMutedAll) {
		t.Fatalf("expected ErrGroupMutedAll, got %v", err)
	}
}

func TestCreateMessageCreatesMessageAndUpdatesConversation(t *testing.T) {
	conversationRepo := newFakeConversationRepo()
	groupRepo := newFakeGroupRepo()
	memberRepo := newFakeMemberRepo()
	messageRepo := newFakeMessageRepo()
	service := NewChatService(conversationRepo, groupRepo, memberRepo, messageRepo, &fakeTxManager{}, nil)

	conversationRepo.conversations[1] = &model.Conversation{ID: 1, ConversationID: "c_test", Type: model.ConversationTypeGroup}
	groupRepo.groups[1] = &model.GroupInfo{ConversationID: 1, OwnerID: 10001}
	_ = memberRepo.Create(context.Background(), &model.ConversationMember{
		ConversationID: 1,
		MemberType:     model.MemberTypeUser,
		MemberID:       10001,
		Role:           model.MemberRoleOwner,
		Status:         model.MemberStatusNormal,
		JoinedAt:       time.Now(),
	})

	message, err := service.CreateMessage(context.Background(), 10001, "c_test", "hello", nil)
	if err != nil {
		t.Fatalf("CreateMessage returned error: %v", err)
	}
	if message.ID == 0 || message.Content != "hello" || message.MessageType != string(model.MessageTypeText) {
		t.Fatalf("unexpected message view: %+v", message)
	}
	if conversationRepo.lastMessageID != message.ID || conversationRepo.lastConversationID != 1 {
		t.Fatalf("conversation last message not updated, got conversation=%d message=%d", conversationRepo.lastConversationID, conversationRepo.lastMessageID)
	}
}

func TestCreateMessageSupportsSingleConversation(t *testing.T) {
	conversationRepo := newFakeConversationRepo()
	memberRepo := newFakeMemberRepo()
	messageRepo := newFakeMessageRepo()
	service := NewChatService(
		conversationRepo,
		newFakeGroupRepo(),
		memberRepo,
		messageRepo,
		&fakeTxManager{},
		&fakeUserClient{friendships: map[[2]uint64]bool{
			{10001, 10002}: true,
			{10002, 10001}: true,
		}},
	)

	conversationRepo.conversations[1] = &model.Conversation{ID: 1, ConversationID: "c_single", Type: model.ConversationTypeSingle}
	_ = memberRepo.Create(context.Background(), &model.ConversationMember{
		ConversationID: 1,
		MemberType:     model.MemberTypeUser,
		MemberID:       10001,
		Role:           model.MemberRoleMember,
		Status:         model.MemberStatusNormal,
		JoinedAt:       time.Now(),
	})
	_ = memberRepo.Create(context.Background(), &model.ConversationMember{
		ConversationID: 1,
		MemberType:     model.MemberTypeUser,
		MemberID:       10002,
		Role:           model.MemberRoleMember,
		Status:         model.MemberStatusNormal,
		JoinedAt:       time.Now(),
	})

	message, err := service.CreateMessage(context.Background(), 10001, "c_single", "hello", nil)
	if err != nil {
		t.Fatalf("CreateMessage returned error: %v", err)
	}
	if message.ConversationID != "c_single" || message.Content != "hello" {
		t.Fatalf("unexpected single message view: %+v", message)
	}
}

func TestCreateMessageRejectsSingleConversationAfterFriendDeleted(t *testing.T) {
	conversationRepo := newFakeConversationRepo()
	memberRepo := newFakeMemberRepo()
	messageRepo := newFakeMessageRepo()
	service := NewChatService(
		conversationRepo,
		newFakeGroupRepo(),
		memberRepo,
		messageRepo,
		&fakeTxManager{},
		&fakeUserClient{friendships: map[[2]uint64]bool{}},
	)

	conversationRepo.conversations[1] = &model.Conversation{ID: 1, ConversationID: "c_single", Type: model.ConversationTypeSingle}
	_ = memberRepo.Create(context.Background(), &model.ConversationMember{
		ConversationID: 1,
		MemberType:     model.MemberTypeUser,
		MemberID:       10001,
		Role:           model.MemberRoleMember,
		Status:         model.MemberStatusNormal,
		JoinedAt:       time.Now(),
	})
	_ = memberRepo.Create(context.Background(), &model.ConversationMember{
		ConversationID: 1,
		MemberType:     model.MemberTypeUser,
		MemberID:       10002,
		Role:           model.MemberRoleMember,
		Status:         model.MemberStatusNormal,
		JoinedAt:       time.Now(),
	})

	_, err := service.CreateMessage(context.Background(), 10001, "c_single", "hello", nil)
	if !errors.Is(err, ErrSingleFriendRequired) {
		t.Fatalf("expected ErrSingleFriendRequired, got %v", err)
	}
}

func TestCreateMessageTriggersBotAsyncAfterUserMessageCreated(t *testing.T) {
	service := newMessageTestService(model.MemberRoleOwner, model.MemberStatusNormal, false)
	handler := newFakeBotMentionHandler()
	service.SetBotService(handler)

	ctx, cancel := context.WithCancel(context.Background())
	message, err := service.CreateMessage(ctx, 10001, "c_test", "@AIM hello", nil)
	if err != nil {
		t.Fatalf("CreateMessage returned error: %v", err)
	}

	req := handler.waitRequest(t)
	if req.ConversationID != 1 || req.RequestMessageID != message.ID || req.UserID != 10001 || req.Content != "@AIM hello" {
		t.Fatalf("unexpected bot request: %+v", req)
	}

	cancel()
	handler.release()
	if err := handler.waitContextErr(t); err != nil {
		t.Fatalf("bot async reused canceled request ctx: %v", err)
	}
}

func TestCreateMessageDoesNotTriggerBotWhenMessageCreationFails(t *testing.T) {
	service := newMessageTestService(model.MemberRoleOwner, model.MemberStatusNormal, false)
	handler := newFakeBotMentionHandler()
	service.SetBotService(handler)

	_, err := service.CreateMessage(context.Background(), 20002, "c_test", "@AIM hello", nil)
	if !errors.Is(err, ErrNotMember) {
		t.Fatalf("expected ErrNotMember, got %v", err)
	}
	handler.assertNoRequest(t)
}

func TestCreateMessageBotFailureDoesNotFailUserMessage(t *testing.T) {
	service := newMessageTestService(model.MemberRoleOwner, model.MemberStatusNormal, false)
	handler := newFakeBotMentionHandler()
	handler.returnErr = errors.New("llm failed")
	service.SetBotService(handler)

	message, err := service.CreateMessage(context.Background(), 10001, "c_test", "@bot hello", nil)
	if err != nil {
		t.Fatalf("CreateMessage returned error: %v", err)
	}
	if message.ID == 0 || message.Content != "@bot hello" {
		t.Fatalf("unexpected message: %+v", message)
	}
	_ = handler.waitRequest(t)
	handler.release()
	_ = handler.waitContextErr(t)
}

func TestCreateMessageBotPanicIsRecovered(t *testing.T) {
	service := newMessageTestService(model.MemberRoleOwner, model.MemberStatusNormal, false)
	handler := &panicBotMentionHandler{called: make(chan struct{}, 1)}
	service.SetBotService(handler)

	if _, err := service.CreateMessage(context.Background(), 10001, "c_test", "@AIM hello", nil); err != nil {
		t.Fatalf("CreateMessage returned error: %v", err)
	}

	select {
	case <-handler.called:
	case <-time.After(time.Second):
		t.Fatal("bot handler was not called")
	}
}

func newMessageTestService(role model.ConversationMemberRole, status model.ConversationMemberStatus, muteAll bool) *ChatService {
	conversationRepo := newFakeConversationRepo()
	groupRepo := newFakeGroupRepo()
	memberRepo := newFakeMemberRepo()

	conversationRepo.conversations[1] = &model.Conversation{ID: 1, ConversationID: "c_test", Type: model.ConversationTypeGroup}
	groupRepo.groups[1] = &model.GroupInfo{ConversationID: 1, OwnerID: 99999, MuteAll: muteAll}
	_ = memberRepo.Create(context.Background(), &model.ConversationMember{
		ConversationID: 1,
		MemberType:     model.MemberTypeUser,
		MemberID:       10001,
		Role:           role,
		Status:         status,
		JoinedAt:       time.Now(),
	})

	return NewChatService(conversationRepo, groupRepo, memberRepo, newFakeMessageRepo(), &fakeTxManager{}, nil)
}

type fakeTxManager struct {
	called bool
}

func (m *fakeTxManager) WithinTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	m.called = true
	return fn(nil)
}

type fakeConversationRepo struct {
	conversations      map[uint64]*model.Conversation
	listRows           []repository.ConversationListRow
	nextID             uint64
	lastConversationID uint64
	lastMessageID      uint64
}

func newFakeConversationRepo() *fakeConversationRepo {
	return &fakeConversationRepo{
		conversations: make(map[uint64]*model.Conversation),
		nextID:        1,
	}
}

func (r *fakeConversationRepo) WithTx(tx *gorm.DB) repository.ConversationRepository {
	return r
}

func (r *fakeConversationRepo) Create(ctx context.Context, conversation *model.Conversation) error {
	conversation.ID = r.nextID
	r.nextID++
	now := time.Now()
	conversation.CreatedAt = now
	conversation.UpdatedAt = now
	r.conversations[conversation.ID] = conversation
	return nil
}

func (r *fakeConversationRepo) GetByID(ctx context.Context, id uint64) (*model.Conversation, error) {
	conversation, ok := r.conversations[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return conversation, nil
}

func (r *fakeConversationRepo) GetByConversationID(ctx context.Context, conversationID string) (*model.Conversation, error) {
	for _, conversation := range r.conversations {
		if conversation.ConversationID == conversationID {
			return conversation, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeConversationRepo) FindSingleByUsers(ctx context.Context, userID uint64, peerUserID uint64) (*model.Conversation, error) {
	for _, conversation := range r.conversations {
		if conversation.Type != model.ConversationTypeSingle {
			continue
		}
		return conversation, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeConversationRepo) ListByUserID(ctx context.Context, userID uint64) ([]repository.ConversationListRow, error) {
	return append([]repository.ConversationListRow(nil), r.listRows...), nil
}

func (r *fakeConversationRepo) UpdateLastMessage(ctx context.Context, conversationID uint64, messageID uint64, at time.Time) error {
	r.lastConversationID = conversationID
	r.lastMessageID = messageID
	return nil
}

type fakeGroupRepo struct {
	groups map[uint64]*model.GroupInfo
	nextID uint64
}

func newFakeGroupRepo() *fakeGroupRepo {
	return &fakeGroupRepo{
		groups: make(map[uint64]*model.GroupInfo),
		nextID: 1,
	}
}

func (r *fakeGroupRepo) WithTx(tx *gorm.DB) repository.GroupRepository {
	return r
}

func (r *fakeGroupRepo) Create(ctx context.Context, group *model.GroupInfo) error {
	group.ID = r.nextID
	r.nextID++
	now := time.Now()
	group.CreatedAt = now
	group.UpdatedAt = now
	r.groups[group.ConversationID] = group
	return nil
}

func (r *fakeGroupRepo) GetByConversationID(ctx context.Context, conversationID uint64) (*model.GroupInfo, error) {
	group, ok := r.groups[conversationID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return group, nil
}

type memberKey struct {
	conversationID uint64
	memberType     model.MemberType
	memberID       uint64
}

type fakeMemberRepo struct {
	members map[memberKey]*model.ConversationMember
	nextID  uint64
}

func newFakeMemberRepo() *fakeMemberRepo {
	return &fakeMemberRepo{
		members: make(map[memberKey]*model.ConversationMember),
		nextID:  1,
	}
}

func (r *fakeMemberRepo) WithTx(tx *gorm.DB) repository.MemberRepository {
	return r
}

func (r *fakeMemberRepo) Create(ctx context.Context, member *model.ConversationMember) error {
	member.ID = r.nextID
	r.nextID++
	r.members[memberKey{conversationID: member.ConversationID, memberType: member.MemberType, memberID: member.MemberID}] = member
	return nil
}

func (r *fakeMemberRepo) Update(ctx context.Context, member *model.ConversationMember) error {
	r.members[memberKey{conversationID: member.ConversationID, memberType: member.MemberType, memberID: member.MemberID}] = member
	return nil
}

func (r *fakeMemberRepo) GetUserMember(ctx context.Context, conversationID, userID uint64) (*model.ConversationMember, error) {
	member, ok := r.members[memberKey{conversationID: conversationID, memberType: model.MemberTypeUser, memberID: userID}]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return member, nil
}

func (r *fakeMemberRepo) IsUserMember(ctx context.Context, conversationID, userID uint64) (bool, error) {
	_, err := r.GetUserMember(ctx, conversationID, userID)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return false, err
}

func (r *fakeMemberRepo) ListUserMembers(ctx context.Context, conversationID uint64) ([]model.ConversationMember, error) {
	members := make([]model.ConversationMember, 0)
	for key, member := range r.members {
		if key.conversationID == conversationID && key.memberType == model.MemberTypeUser && member.Status == model.MemberStatusNormal {
			members = append(members, *member)
		}
	}
	return members, nil
}

func (r *fakeMemberRepo) ListUserMemberIDs(ctx context.Context, conversationID uint64) ([]uint64, error) {
	memberIDs := make([]uint64, 0)
	for key, member := range r.members {
		if key.conversationID == conversationID && key.memberType == model.MemberTypeUser && member.Status == model.MemberStatusNormal {
			memberIDs = append(memberIDs, key.memberID)
		}
	}
	return memberIDs, nil
}

func (r *fakeMemberRepo) GetBotMember(ctx context.Context, conversationID, botID uint64) (*model.ConversationMember, error) {
	member, ok := r.members[memberKey{conversationID: conversationID, memberType: model.MemberTypeBot, memberID: botID}]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return member, nil
}

func (r *fakeMemberRepo) IsBotMember(ctx context.Context, conversationID, botID uint64) (bool, error) {
	_, err := r.GetBotMember(ctx, conversationID, botID)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return false, err
}

func (r *fakeMemberRepo) ListBotMembers(ctx context.Context, conversationID uint64) ([]model.ConversationMember, error) {
	members := make([]model.ConversationMember, 0)
	for key, member := range r.members {
		if key.conversationID == conversationID && key.memberType == model.MemberTypeBot && member.Status == model.MemberStatusNormal {
			members = append(members, *member)
		}
	}
	return members, nil
}

func (r *fakeMemberRepo) ListByConversationID(ctx context.Context, conversationID uint64) ([]model.ConversationMember, error) {
	members := make([]model.ConversationMember, 0)
	for key, member := range r.members {
		if key.conversationID == conversationID {
			members = append(members, *member)
		}
	}
	return members, nil
}

type fakeMessageRepo struct {
	messages []*model.Message
	nextID   uint64
}

func newFakeMessageRepo() *fakeMessageRepo {
	return &fakeMessageRepo{nextID: 1}
}

func (r *fakeMessageRepo) WithTx(tx *gorm.DB) repository.MessageRepository {
	return r
}

func (r *fakeMessageRepo) Create(ctx context.Context, message *model.Message) error {
	message.ID = r.nextID
	r.nextID++
	r.messages = append(r.messages, message)
	return nil
}

func (r *fakeMessageRepo) ListByConversationID(ctx context.Context, conversationID uint64, beforeID *uint64, limit int) ([]model.Message, error) {
	messages := make([]model.Message, 0)
	for _, message := range r.messages {
		if message.ConversationID == conversationID {
			messages = append(messages, *message)
		}
	}
	return messages, nil
}

type fakeUserClient struct {
	users       map[uint64]*rpc.UserInfo
	friendships map[[2]uint64]bool
}

type fakeConversationListBotRepo struct {
	bots map[uint64]*model.Bot
}

type fakeListAICallLogRepo struct {
	logs []model.AICallLog
}

func (r *fakeConversationListBotRepo) WithTx(tx *gorm.DB) repository.BotRepository {
	return r
}

func (r *fakeConversationListBotRepo) GetByID(ctx context.Context, id uint64) (*model.Bot, error) {
	botModel, ok := r.bots[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return botModel, nil
}

func (r *fakeConversationListBotRepo) ListEnabled(ctx context.Context) ([]model.Bot, error) {
	result := make([]model.Bot, 0, len(r.bots))
	for _, botModel := range r.bots {
		result = append(result, *botModel)
	}
	return result, nil
}

func (r *fakeListAICallLogRepo) WithTx(tx *gorm.DB) repository.AICallLogRepository {
	return r
}

func (r *fakeListAICallLogRepo) Create(ctx context.Context, callLog *model.AICallLog) error {
	r.logs = append(r.logs, *callLog)
	return nil
}

func (r *fakeListAICallLogRepo) ListByConversationID(ctx context.Context, conversationID uint64, beforeID *uint64, limit int, botID *uint64, status string) ([]model.AICallLog, error) {
	result := make([]model.AICallLog, 0, len(r.logs))
	for _, item := range r.logs {
		if item.ConversationID != conversationID {
			continue
		}
		if beforeID != nil && item.ID >= *beforeID {
			continue
		}
		if botID != nil && item.BotID != *botID {
			continue
		}
		if status != "" && string(item.Status) != status {
			continue
		}
		result = append(result, item)
		if len(result) >= limit {
			break
		}
	}
	return result, nil
}

func (r *fakeListAICallLogRepo) SumTotalTokensByConversationBetween(ctx context.Context, conversationID uint64, startAt time.Time, endAt time.Time) (int64, error) {
	var total int64
	for _, item := range r.logs {
		if item.ConversationID != conversationID {
			continue
		}
		if item.CreatedAt.Before(startAt) || !item.CreatedAt.Before(endAt) {
			continue
		}
		total += int64(item.TotalTokens)
	}
	return total, nil
}

func uint64Ptr(value uint64) *uint64 {
	return &value
}

func (c *fakeUserClient) GetUser(ctx context.Context, userID uint64) (*rpc.UserInfo, error) {
	user, ok := c.users[userID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return user, nil
}

func (c *fakeUserClient) CheckFriendRelation(ctx context.Context, userID uint64, friendUserID uint64) (bool, error) {
	if c.friendships == nil {
		return false, nil
	}
	return c.friendships[[2]uint64{userID, friendUserID}], nil
}

type fakeBotMentionHandler struct {
	requests  chan bot.HandleMentionRequest
	released  chan struct{}
	ctxErrs   chan error
	returnErr error
}

func newFakeBotMentionHandler() *fakeBotMentionHandler {
	return &fakeBotMentionHandler{
		requests: make(chan bot.HandleMentionRequest, 1),
		released: make(chan struct{}),
		ctxErrs:  make(chan error, 1),
	}
}

func (h *fakeBotMentionHandler) HandleMention(ctx context.Context, req bot.HandleMentionRequest) error {
	h.requests <- req
	<-h.released
	select {
	case <-ctx.Done():
		h.ctxErrs <- ctx.Err()
	default:
		h.ctxErrs <- nil
	}
	return h.returnErr
}

func (h *fakeBotMentionHandler) waitRequest(t *testing.T) bot.HandleMentionRequest {
	t.Helper()
	select {
	case req := <-h.requests:
		return req
	case <-time.After(time.Second):
		t.Fatal("bot handler was not called")
		return bot.HandleMentionRequest{}
	}
}

func (h *fakeBotMentionHandler) assertNoRequest(t *testing.T) {
	t.Helper()
	select {
	case req := <-h.requests:
		t.Fatalf("bot handler was unexpectedly called: %+v", req)
	case <-time.After(50 * time.Millisecond):
	}
}

func (h *fakeBotMentionHandler) release() {
	close(h.released)
}

func (h *fakeBotMentionHandler) waitContextErr(t *testing.T) error {
	t.Helper()
	select {
	case err := <-h.ctxErrs:
		return err
	case <-time.After(time.Second):
		t.Fatal("bot handler did not finish")
		return nil
	}
}

type panicBotMentionHandler struct {
	called chan struct{}
}

func (h *panicBotMentionHandler) HandleMention(ctx context.Context, req bot.HandleMentionRequest) error {
	h.called <- struct{}{}
	panic("bot panic")
}
