package biz

import (
	"context"
	"errors"
	"testing"
	"time"

	chatpb "example.com/aim/chat-service/kitex_gen/chat"
	"example.com/aim/user-service/internal/dal/model"
	"example.com/aim/user-service/internal/realtime"
	"example.com/aim/user-service/internal/repository"
	"gorm.io/gorm"
)

func TestAddFriendCreatesPendingRequest(t *testing.T) {
	userRepo := newFakeUserRepo()
	friendGroupRepo := newFakeFriendGroupRepo()
	friendRelationRepo := newFakeFriendRelationRepo()
	friendRequestRepo := newFakeFriendRequestRepo()
	eventPublisher := &fakeFriendEventPublisher{}

	userRepo.users[10001] = &model.User{ID: 10001, AimID: "aim_a", Nickname: "A", Status: model.UserStatusNormal}
	userRepo.users[10002] = &model.User{ID: 10002, AimID: "aim_b", Nickname: "B", Status: model.UserStatusNormal}
	userRepo.usersByAimID["aim_a"] = userRepo.users[10001]
	userRepo.usersByAimID["aim_b"] = userRepo.users[10002]

	service := NewUserService(userRepo, friendGroupRepo, friendRelationRepo, friendRequestRepo, &fakeUserTxManager{})
	service.SetFriendEventPublisher(eventPublisher)

	request, err := service.AddFriend(context.Background(), 10001, "aim_b", "teammate", nil)
	if err != nil {
		t.Fatalf("AddFriend returned error: %v", err)
	}
	if request == nil || request.UserID != 10002 || request.Remark != "teammate" || request.Status != model.FriendRequestStatusPending {
		t.Fatalf("unexpected request view: %+v", request)
	}

	if _, err := friendRequestRepo.GetByUserPair(context.Background(), 10001, 10002); err != nil {
		t.Fatalf("friend request missing: %v", err)
	}
	if _, err := friendRelationRepo.GetByUserPair(context.Background(), 10001, 10002); err != gorm.ErrRecordNotFound {
		t.Fatalf("expected no relation before approval, got %v", err)
	}
	if len(eventPublisher.events) != 1 {
		t.Fatalf("expected one friend event, got %d", len(eventPublisher.events))
	}
	if eventPublisher.events[0].Reason != realtime.FriendSyncReasonRequestCreated ||
		eventPublisher.events[0].Status != model.FriendRequestStatusPending ||
		len(eventPublisher.events[0].UserIDs) != 2 ||
		eventPublisher.events[0].UserIDs[0] != 10001 ||
		eventPublisher.events[0].UserIDs[1] != 10002 {
		t.Fatalf("unexpected friend event: %+v", eventPublisher.events[0])
	}
}

func TestCreateFriendGroupPublishesRealtimeEvent(t *testing.T) {
	userRepo := newFakeUserRepo()
	friendGroupRepo := newFakeFriendGroupRepo()
	friendRelationRepo := newFakeFriendRelationRepo()
	friendRequestRepo := newFakeFriendRequestRepo()
	eventPublisher := &fakeFriendEventPublisher{}

	service := NewUserService(userRepo, friendGroupRepo, friendRelationRepo, friendRequestRepo, &fakeUserTxManager{})
	service.SetFriendEventPublisher(eventPublisher)

	group, err := service.CreateFriendGroup(context.Background(), 10001, "team")
	if err != nil {
		t.Fatalf("CreateFriendGroup returned error: %v", err)
	}
	if group == nil || group.Name != "team" {
		t.Fatalf("unexpected group: %+v", group)
	}
	if len(eventPublisher.events) != 1 {
		t.Fatalf("expected one friend event, got %d", len(eventPublisher.events))
	}
	if eventPublisher.events[0].Reason != realtime.FriendSyncReasonGroupCreated ||
		len(eventPublisher.events[0].UserIDs) != 1 ||
		eventPublisher.events[0].UserIDs[0] != 10001 {
		t.Fatalf("unexpected group event: %+v", eventPublisher.events[0])
	}
}

func TestRespondFriendRequestAcceptCreatesBidirectionalRelations(t *testing.T) {
	userRepo := newFakeUserRepo()
	friendGroupRepo := newFakeFriendGroupRepo()
	friendRelationRepo := newFakeFriendRelationRepo()
	friendRequestRepo := newFakeFriendRequestRepo()
	eventPublisher := &fakeFriendEventPublisher{}

	userRepo.users[10001] = &model.User{ID: 10001, AimID: "aim_a", Nickname: "A", Status: model.UserStatusNormal}
	userRepo.users[10002] = &model.User{ID: 10002, AimID: "aim_b", Nickname: "B", Status: model.UserStatusNormal}
	userRepo.usersByAimID["aim_a"] = userRepo.users[10001]
	userRepo.usersByAimID["aim_b"] = userRepo.users[10002]

	service := NewUserService(userRepo, friendGroupRepo, friendRelationRepo, friendRequestRepo, &fakeUserTxManager{})
	chatClient := &fakeChatClient{}
	service.SetChatClient(chatClient)
	service.SetFriendEventPublisher(eventPublisher)
	if _, err := service.AddFriend(context.Background(), 10001, "aim_b", "teammate", nil); err != nil {
		t.Fatalf("AddFriend returned error: %v", err)
	}

	request, _ := friendRequestRepo.GetByUserPair(context.Background(), 10001, 10002)
	requestView, friendView, err := service.RespondFriendRequest(context.Background(), 10002, request.ID, model.FriendRequestStatusAccepted)
	if err != nil {
		t.Fatalf("RespondFriendRequest returned error: %v", err)
	}
	if requestView == nil || requestView.Status != model.FriendRequestStatusAccepted {
		t.Fatalf("unexpected request view: %+v", requestView)
	}
	if friendView == nil || friendView.UserID != 10001 {
		t.Fatalf("unexpected friend view: %+v", friendView)
	}

	if _, err := friendRelationRepo.GetByUserPair(context.Background(), 10001, 10002); err != nil {
		t.Fatalf("caller relation missing: %v", err)
	}
	if _, err := friendRelationRepo.GetByUserPair(context.Background(), 10002, 10001); err != nil {
		t.Fatalf("target relation missing: %v", err)
	}
	if len(chatClient.calls) != 1 {
		t.Fatalf("expected one single-conversation call, got %d", len(chatClient.calls))
	}
	if chatClient.calls[0].operatorID != 10002 || chatClient.calls[0].targetUserID != 10001 {
		t.Fatalf("unexpected single-conversation call: %+v", chatClient.calls[0])
	}
	if len(eventPublisher.events) != 2 {
		t.Fatalf("expected two friend events, got %d", len(eventPublisher.events))
	}
	lastEvent := eventPublisher.events[1]
	if lastEvent.Reason != realtime.FriendSyncReasonRequestResponded ||
		lastEvent.Status != model.FriendRequestStatusAccepted ||
		lastEvent.ConversationID != "c_test" {
		t.Fatalf("unexpected accepted friend event: %+v", lastEvent)
	}
}

func TestRespondFriendRequestRejectDoesNotCreateRelations(t *testing.T) {
	userRepo := newFakeUserRepo()
	friendGroupRepo := newFakeFriendGroupRepo()
	friendRelationRepo := newFakeFriendRelationRepo()
	friendRequestRepo := newFakeFriendRequestRepo()

	userRepo.users[10001] = &model.User{ID: 10001, AimID: "aim_a", Nickname: "A", Status: model.UserStatusNormal}
	userRepo.users[10002] = &model.User{ID: 10002, AimID: "aim_b", Nickname: "B", Status: model.UserStatusNormal}
	userRepo.usersByAimID["aim_a"] = userRepo.users[10001]
	userRepo.usersByAimID["aim_b"] = userRepo.users[10002]

	service := NewUserService(userRepo, friendGroupRepo, friendRelationRepo, friendRequestRepo, &fakeUserTxManager{})
	if _, err := service.AddFriend(context.Background(), 10001, "aim_b", "", nil); err != nil {
		t.Fatalf("AddFriend returned error: %v", err)
	}

	request, _ := friendRequestRepo.GetByUserPair(context.Background(), 10001, 10002)
	requestView, friendView, err := service.RespondFriendRequest(context.Background(), 10002, request.ID, model.FriendRequestStatusRejected)
	if err != nil {
		t.Fatalf("RespondFriendRequest returned error: %v", err)
	}
	if requestView == nil || requestView.Status != model.FriendRequestStatusRejected {
		t.Fatalf("unexpected request view: %+v", requestView)
	}
	if friendView != nil {
		t.Fatalf("expected no friend view on reject, got %+v", friendView)
	}
	if _, err := friendRelationRepo.GetByUserPair(context.Background(), 10001, 10002); err != gorm.ErrRecordNotFound {
		t.Fatalf("expected no relation after reject, got %v", err)
	}
}

func TestRespondFriendRequestAcceptStillSucceedsWhenSingleConversationCreationFails(t *testing.T) {
	userRepo := newFakeUserRepo()
	friendGroupRepo := newFakeFriendGroupRepo()
	friendRelationRepo := newFakeFriendRelationRepo()
	friendRequestRepo := newFakeFriendRequestRepo()

	userRepo.users[10001] = &model.User{ID: 10001, AimID: "aim_a", Nickname: "A", Status: model.UserStatusNormal}
	userRepo.users[10002] = &model.User{ID: 10002, AimID: "aim_b", Nickname: "B", Status: model.UserStatusNormal}
	userRepo.usersByAimID["aim_a"] = userRepo.users[10001]
	userRepo.usersByAimID["aim_b"] = userRepo.users[10002]

	service := NewUserService(userRepo, friendGroupRepo, friendRelationRepo, friendRequestRepo, &fakeUserTxManager{})
	service.SetChatClient(&fakeChatClient{err: errors.New("chat offline")})
	if _, err := service.AddFriend(context.Background(), 10001, "aim_b", "teammate", nil); err != nil {
		t.Fatalf("AddFriend returned error: %v", err)
	}

	request, _ := friendRequestRepo.GetByUserPair(context.Background(), 10001, 10002)
	requestView, friendView, err := service.RespondFriendRequest(context.Background(), 10002, request.ID, model.FriendRequestStatusAccepted)
	if err != nil {
		t.Fatalf("RespondFriendRequest returned error: %v", err)
	}
	if requestView == nil || friendView == nil {
		t.Fatalf("expected accepted friend request and friend view, got request=%+v friend=%+v", requestView, friendView)
	}
	if _, err := friendRelationRepo.GetByUserPair(context.Background(), 10001, 10002); err != nil {
		t.Fatalf("caller relation missing: %v", err)
	}
	if _, err := friendRelationRepo.GetByUserPair(context.Background(), 10002, 10001); err != nil {
		t.Fatalf("target relation missing: %v", err)
	}
}

func TestDeleteFriendRemovesBidirectionalRelations(t *testing.T) {
	userRepo := newFakeUserRepo()
	friendGroupRepo := newFakeFriendGroupRepo()
	friendRelationRepo := newFakeFriendRelationRepo()
	friendRequestRepo := newFakeFriendRequestRepo()
	eventPublisher := &fakeFriendEventPublisher{}
	service := NewUserService(userRepo, friendGroupRepo, friendRelationRepo, friendRequestRepo, &fakeUserTxManager{})
	service.SetFriendEventPublisher(eventPublisher)

	now := time.Now()
	_ = friendRelationRepo.Create(context.Background(), &model.FriendRelation{
		UserID:       10001,
		FriendUserID: 10002,
		Status:       model.FriendRelationStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	_ = friendRelationRepo.Create(context.Background(), &model.FriendRelation{
		UserID:       10002,
		FriendUserID: 10001,
		Status:       model.FriendRelationStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	})

	if err := service.DeleteFriend(context.Background(), 10001, 10002); err != nil {
		t.Fatalf("DeleteFriend returned error: %v", err)
	}

	if _, err := friendRelationRepo.GetByUserPair(context.Background(), 10001, 10002); err != gorm.ErrRecordNotFound {
		t.Fatalf("expected caller relation to be deleted, got %v", err)
	}
	if _, err := friendRelationRepo.GetByUserPair(context.Background(), 10002, 10001); err != gorm.ErrRecordNotFound {
		t.Fatalf("expected target relation to be deleted, got %v", err)
	}
	if len(eventPublisher.events) != 1 {
		t.Fatalf("expected one friend event, got %d", len(eventPublisher.events))
	}
	if eventPublisher.events[0].Reason != realtime.FriendSyncReasonFriendDeleted ||
		len(eventPublisher.events[0].UserIDs) != 2 {
		t.Fatalf("unexpected delete event: %+v", eventPublisher.events[0])
	}
}

func TestUpdateFriendPublishesRealtimeEvent(t *testing.T) {
	userRepo := newFakeUserRepo()
	friendGroupRepo := newFakeFriendGroupRepo()
	friendRelationRepo := newFakeFriendRelationRepo()
	friendRequestRepo := newFakeFriendRequestRepo()
	eventPublisher := &fakeFriendEventPublisher{}
	service := NewUserService(userRepo, friendGroupRepo, friendRelationRepo, friendRequestRepo, &fakeUserTxManager{})
	service.SetFriendEventPublisher(eventPublisher)

	userRepo.users[10002] = &model.User{ID: 10002, AimID: "aim_b", Nickname: "B", Status: model.UserStatusNormal}
	userRepo.usersByAimID["aim_b"] = userRepo.users[10002]

	now := time.Now()
	_ = friendRelationRepo.Create(context.Background(), &model.FriendRelation{
		UserID:       10001,
		FriendUserID: 10002,
		Status:       model.FriendRelationStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	})

	friend, err := service.UpdateFriend(context.Background(), 10001, 10002, "buddy", nil)
	if err != nil {
		t.Fatalf("UpdateFriend returned error: %v", err)
	}
	if friend == nil || friend.Remark != "buddy" {
		t.Fatalf("unexpected friend: %+v", friend)
	}
	if len(eventPublisher.events) != 1 {
		t.Fatalf("expected one friend event, got %d", len(eventPublisher.events))
	}
	if eventPublisher.events[0].Reason != realtime.FriendSyncReasonFriendUpdated ||
		len(eventPublisher.events[0].UserIDs) != 1 ||
		eventPublisher.events[0].UserIDs[0] != 10001 {
		t.Fatalf("unexpected update event: %+v", eventPublisher.events[0])
	}
}

type fakeUserTxManager struct{}

func (m *fakeUserTxManager) WithinTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return fn(nil)
}

type fakeUserRepo struct {
	users        map[uint64]*model.User
	usersByAimID map[string]*model.User
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{
		users:        make(map[uint64]*model.User),
		usersByAimID: make(map[string]*model.User),
	}
}

func (r *fakeUserRepo) Create(ctx context.Context, user *model.User) error {
	r.users[user.ID] = user
	r.usersByAimID[user.AimID] = user
	return nil
}

func (r *fakeUserRepo) GetByID(ctx context.Context, id uint64) (*model.User, error) {
	user, ok := r.users[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return user, nil
}

func (r *fakeUserRepo) ListByIDs(ctx context.Context, ids []uint64) ([]model.User, error) {
	users := make([]model.User, 0, len(ids))
	for _, id := range ids {
		if user, ok := r.users[id]; ok {
			users = append(users, *user)
		}
	}
	return users, nil
}

func (r *fakeUserRepo) GetByAimID(ctx context.Context, aimID string) (*model.User, error) {
	user, ok := r.usersByAimID[aimID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return user, nil
}

func (r *fakeUserRepo) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeUserRepo) UpdateLoginState(ctx context.Context, userID uint64, ip string, loginAt time.Time) error {
	return nil
}

func (r *fakeUserRepo) BumpTokenVersion(ctx context.Context, userID uint64) error {
	return nil
}

func (r *fakeUserRepo) UpdateAvatar(ctx context.Context, userID uint64, avatar string) error {
	return nil
}

type fakeChatClient struct {
	calls []fakeChatClientCall
	err   error
}

type fakeFriendEventPublisher struct {
	events []realtime.FriendSyncEvent
}

func (p *fakeFriendEventPublisher) PublishFriendSync(ctx context.Context, event realtime.FriendSyncEvent) error {
	p.events = append(p.events, event)
	return nil
}

type fakeChatClientCall struct {
	operatorID   uint64
	targetUserID uint64
}

func (c *fakeChatClient) CreateSingleConversation(ctx context.Context, operatorID, targetUserID uint64) (*chatpb.ConversationInfo, error) {
	c.calls = append(c.calls, fakeChatClientCall{operatorID: operatorID, targetUserID: targetUserID})
	if c.err != nil {
		return nil, c.err
	}
	return &chatpb.ConversationInfo{ConversationId: "c_test", Type: "SINGLE"}, nil
}

type fakeFriendGroupRepo struct {
	groups map[uint64]*model.FriendGroup
	nextID uint64
}

func newFakeFriendGroupRepo() *fakeFriendGroupRepo {
	return &fakeFriendGroupRepo{
		groups: make(map[uint64]*model.FriendGroup),
		nextID: 1,
	}
}

func (r *fakeFriendGroupRepo) WithTx(tx *gorm.DB) repository.FriendGroupRepository {
	return r
}

func (r *fakeFriendGroupRepo) Create(ctx context.Context, group *model.FriendGroup) error {
	group.ID = r.nextID
	r.nextID++
	now := time.Now()
	group.CreatedAt = now
	group.UpdatedAt = now
	r.groups[group.ID] = group
	return nil
}

func (r *fakeFriendGroupRepo) ListByUserID(ctx context.Context, userID uint64) ([]model.FriendGroup, error) {
	result := make([]model.FriendGroup, 0)
	for _, group := range r.groups {
		if group.UserID == userID {
			result = append(result, *group)
		}
	}
	return result, nil
}

func (r *fakeFriendGroupRepo) GetByIDAndUserID(ctx context.Context, id, userID uint64) (*model.FriendGroup, error) {
	group, ok := r.groups[id]
	if !ok || group.UserID != userID {
		return nil, gorm.ErrRecordNotFound
	}
	return group, nil
}

type fakeFriendRelationRepo struct {
	relations map[[2]uint64]*model.FriendRelation
	nextID    uint64
}

func newFakeFriendRelationRepo() *fakeFriendRelationRepo {
	return &fakeFriendRelationRepo{
		relations: make(map[[2]uint64]*model.FriendRelation),
		nextID:    1,
	}
}

func (r *fakeFriendRelationRepo) WithTx(tx *gorm.DB) repository.FriendRelationRepository {
	return r
}

func (r *fakeFriendRelationRepo) Create(ctx context.Context, relation *model.FriendRelation) error {
	relation.ID = r.nextID
	r.nextID++
	r.relations[[2]uint64{relation.UserID, relation.FriendUserID}] = relation
	return nil
}

func (r *fakeFriendRelationRepo) GetByUserPair(ctx context.Context, userID, friendUserID uint64) (*model.FriendRelation, error) {
	relation, ok := r.relations[[2]uint64{userID, friendUserID}]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return relation, nil
}

func (r *fakeFriendRelationRepo) ListByUserID(ctx context.Context, userID uint64) ([]model.FriendRelation, error) {
	result := make([]model.FriendRelation, 0)
	for _, relation := range r.relations {
		if relation.UserID == userID {
			result = append(result, *relation)
		}
	}
	return result, nil
}

func (r *fakeFriendRelationRepo) Update(ctx context.Context, relation *model.FriendRelation) error {
	r.relations[[2]uint64{relation.UserID, relation.FriendUserID}] = relation
	return nil
}

func (r *fakeFriendRelationRepo) DeleteByUserPair(ctx context.Context, userID, friendUserID uint64) error {
	delete(r.relations, [2]uint64{userID, friendUserID})
	return nil
}

type fakeFriendRequestRepo struct {
	requests map[[2]uint64]*model.FriendRequest
	byID     map[uint64]*model.FriendRequest
	nextID   uint64
}

func newFakeFriendRequestRepo() *fakeFriendRequestRepo {
	return &fakeFriendRequestRepo{
		requests: make(map[[2]uint64]*model.FriendRequest),
		byID:     make(map[uint64]*model.FriendRequest),
		nextID:   1,
	}
}

func (r *fakeFriendRequestRepo) WithTx(tx *gorm.DB) repository.FriendRequestRepository {
	return r
}

func (r *fakeFriendRequestRepo) Create(ctx context.Context, request *model.FriendRequest) error {
	request.ID = r.nextID
	r.nextID++
	now := time.Now()
	if request.CreatedAt.IsZero() {
		request.CreatedAt = now
	}
	request.UpdatedAt = now
	r.requests[[2]uint64{request.UserID, request.TargetUserID}] = request
	r.byID[request.ID] = request
	return nil
}

func (r *fakeFriendRequestRepo) GetByID(ctx context.Context, id uint64) (*model.FriendRequest, error) {
	request, ok := r.byID[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return request, nil
}

func (r *fakeFriendRequestRepo) GetByUserPair(ctx context.Context, userID, targetUserID uint64) (*model.FriendRequest, error) {
	request, ok := r.requests[[2]uint64{userID, targetUserID}]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return request, nil
}

func (r *fakeFriendRequestRepo) ListByUserID(ctx context.Context, userID uint64) ([]model.FriendRequest, error) {
	result := make([]model.FriendRequest, 0)
	for _, request := range r.requests {
		if request.UserID == userID || request.TargetUserID == userID {
			result = append(result, *request)
		}
	}
	return result, nil
}

func (r *fakeFriendRequestRepo) Update(ctx context.Context, request *model.FriendRequest) error {
	r.requests[[2]uint64{request.UserID, request.TargetUserID}] = request
	r.byID[request.ID] = request
	return nil
}
