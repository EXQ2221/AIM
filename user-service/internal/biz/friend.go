package biz

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"example.com/aim/shared/errno"
	"example.com/aim/user-service/internal/dal/model"
	"example.com/aim/user-service/internal/realtime"
	"example.com/aim/user-service/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrFriendBadRequest       = errno.BadRequest("invalid friend request")
	ErrFriendGroupNameEmpty   = errno.Required("friend group name")
	ErrFriendGroupNotFound    = errno.NotFound("friend group not found")
	ErrFriendTargetNotFound   = errno.NotFound("target user not found")
	ErrFriendTargetBlocked    = errno.Forbidden("target user is not available")
	ErrFriendSelfAdd          = errno.BadRequest("cannot add yourself as a friend")
	ErrFriendAlreadyExists    = errno.BadRequest("friend relation already exists")
	ErrFriendNotFound         = errno.NotFound("friend relation not found")
	ErrFriendRequestPending   = errno.BadRequest("friend request is already pending")
	ErrFriendRequestExists    = errno.BadRequest("the other user has already sent you a request")
	ErrFriendRequestNotFound  = errno.NotFound("friend request not found")
	ErrFriendRequestHandled   = errno.BadRequest("friend request has already been handled")
	ErrFriendRequestForbidden = errno.Forbidden("cannot handle this friend request")
	ErrFriendRequestAction    = errno.BadRequest("invalid friend request action")
)

type FriendGroupView struct {
	ID        uint64
	Name      string
	SortOrder int32
	CreatedAt int64
	UpdatedAt int64
}

type FriendView struct {
	UserID    uint64
	AimID     string
	Nickname  string
	Avatar    string
	Remark    string
	GroupID   *uint64
	Status    string
	CreatedAt int64
	UpdatedAt int64
}

type FriendRequestView struct {
	ID        uint64
	UserID    uint64
	AimID     string
	Nickname  string
	Avatar    string
	Direction string
	Status    string
	Remark    string
	GroupID   *uint64
	CreatedAt int64
	UpdatedAt int64
}

func (s *UserService) CreateFriendGroup(ctx context.Context, userID uint64, name string) (*FriendGroupView, error) {
	name = strings.TrimSpace(name)
	if userID == 0 || name == "" {
		return nil, ErrFriendGroupNameEmpty
	}
	if len([]rune(name)) > 64 {
		return nil, errno.BadRequest("friend group name is too long")
	}

	group := &model.FriendGroup{
		UserID:    userID,
		Name:      name,
		SortOrder: 0,
	}
	if err := s.FriendGroups.Create(ctx, group); err != nil {
		return nil, err
	}

	view := friendGroupToView(group)
	s.publishFriendSync(ctx, realtime.FriendSyncEvent{
		Reason:      realtime.FriendSyncReasonGroupCreated,
		ActorUserID: int64(userID),
		UserIDs:     []int64{int64(userID)},
	})
	return view, nil
}

func (s *UserService) ListFriendGroups(ctx context.Context, userID uint64) ([]FriendGroupView, error) {
	if userID == 0 {
		return nil, ErrFriendBadRequest
	}

	groups, err := s.FriendGroups.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make([]FriendGroupView, 0, len(groups))
	for i := range groups {
		result = append(result, *friendGroupToView(&groups[i]))
	}
	return result, nil
}

func (s *UserService) AddFriend(ctx context.Context, userID uint64, targetAimID, remark string, groupID *uint64) (*FriendRequestView, error) {
	targetAimID = strings.TrimSpace(targetAimID)
	remark = strings.TrimSpace(remark)
	if userID == 0 || targetAimID == "" {
		return nil, ErrFriendBadRequest
	}
	if len([]rune(remark)) > 128 {
		return nil, errno.BadRequest("friend remark is too long")
	}
	if err := s.validateGroupOwnership(ctx, userID, groupID); err != nil {
		return nil, err
	}

	targetUser, err := s.Users.GetByAimID(ctx, targetAimID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFriendTargetNotFound
		}
		return nil, err
	}
	if targetUser.ID == userID {
		return nil, ErrFriendSelfAdd
	}
	if targetUser.Status != model.UserStatusNormal {
		return nil, ErrFriendTargetBlocked
	}

	if _, err := s.FriendRelations.GetByUserPair(ctx, userID, targetUser.ID); err == nil {
		return nil, ErrFriendAlreadyExists
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if _, err := s.FriendRelations.GetByUserPair(ctx, targetUser.ID, userID); err == nil {
		return nil, ErrFriendAlreadyExists
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if existing, err := s.FriendRequests.GetByUserPair(ctx, userID, targetUser.ID); err == nil {
		if existing.Status == model.FriendRequestStatusPending {
			return nil, ErrFriendRequestPending
		}
		existing.Remark = remark
		existing.GroupID = groupID
		existing.Status = model.FriendRequestStatusPending
		existing.UpdatedAt = time.Now()
		if err := s.FriendRequests.Update(ctx, existing); err != nil {
			return nil, err
		}
		view := friendRequestToView(*existing, *targetUser, model.FriendRequestDirectionOutgoing)
		s.publishFriendSync(ctx, realtime.FriendSyncEvent{
			Reason:       realtime.FriendSyncReasonRequestCreated,
			RequestID:    int64(existing.ID),
			Status:       existing.Status,
			ActorUserID:  int64(userID),
			FriendUserID: int64(targetUser.ID),
			UserIDs:      []int64{int64(userID), int64(targetUser.ID)},
		})
		return &view, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if inverse, err := s.FriendRequests.GetByUserPair(ctx, targetUser.ID, userID); err == nil {
		if inverse.Status == model.FriendRequestStatusPending {
			return nil, ErrFriendRequestExists
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	request := &model.FriendRequest{
		UserID:       userID,
		TargetUserID: targetUser.ID,
		Remark:       remark,
		GroupID:      groupID,
		Status:       model.FriendRequestStatusPending,
	}
	if err := s.FriendRequests.Create(ctx, request); err != nil {
		return nil, err
	}

	view := friendRequestToView(*request, *targetUser, model.FriendRequestDirectionOutgoing)
	s.publishFriendSync(ctx, realtime.FriendSyncEvent{
		Reason:       realtime.FriendSyncReasonRequestCreated,
		RequestID:    int64(request.ID),
		Status:       request.Status,
		ActorUserID:  int64(userID),
		FriendUserID: int64(targetUser.ID),
		UserIDs:      []int64{int64(userID), int64(targetUser.ID)},
	})
	return &view, nil
}

func (s *UserService) ListFriendRequests(ctx context.Context, userID uint64) ([]FriendRequestView, error) {
	if userID == 0 {
		return nil, ErrFriendBadRequest
	}

	requests, err := s.FriendRequests.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(requests) == 0 {
		return []FriendRequestView{}, nil
	}

	counterpartyIDs := make([]uint64, 0, len(requests))
	seen := make(map[uint64]struct{}, len(requests))
	for _, request := range requests {
		counterpartyID := request.TargetUserID
		if request.UserID != userID {
			counterpartyID = request.UserID
		}
		if _, ok := seen[counterpartyID]; ok {
			continue
		}
		seen[counterpartyID] = struct{}{}
		counterpartyIDs = append(counterpartyIDs, counterpartyID)
	}

	users, err := s.Users.ListByIDs(ctx, counterpartyIDs)
	if err != nil {
		return nil, err
	}
	userByID := make(map[uint64]model.User, len(users))
	for _, user := range users {
		userByID[user.ID] = user
	}

	result := make([]FriendRequestView, 0, len(requests))
	for _, request := range requests {
		direction := model.FriendRequestDirectionOutgoing
		counterpartyID := request.TargetUserID
		if request.UserID != userID {
			direction = model.FriendRequestDirectionIncoming
			counterpartyID = request.UserID
		}
		counterparty, ok := userByID[counterpartyID]
		if !ok {
			continue
		}
		result = append(result, friendRequestToView(request, counterparty, direction))
	}
	return result, nil
}

func (s *UserService) RespondFriendRequest(ctx context.Context, userID, requestID uint64, action string) (*FriendRequestView, *FriendView, error) {
	action = strings.ToUpper(strings.TrimSpace(action))
	if userID == 0 || requestID == 0 {
		return nil, nil, ErrFriendBadRequest
	}
	if action != model.FriendRequestStatusAccepted && action != model.FriendRequestStatusRejected {
		return nil, nil, ErrFriendRequestAction
	}

	request, err := s.FriendRequests.GetByID(ctx, requestID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrFriendRequestNotFound
		}
		return nil, nil, err
	}
	if request.TargetUserID != userID {
		return nil, nil, ErrFriendRequestForbidden
	}
	if request.Status != model.FriendRequestStatusPending {
		return nil, nil, ErrFriendRequestHandled
	}

	requester, err := s.Users.GetByID(ctx, request.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrFriendTargetNotFound
		}
		return nil, nil, err
	}

	if action == model.FriendRequestStatusRejected {
		request.Status = model.FriendRequestStatusRejected
		request.UpdatedAt = time.Now()
		if err := s.FriendRequests.Update(ctx, request); err != nil {
			return nil, nil, err
		}
		view := friendRequestToView(*request, *requester, model.FriendRequestDirectionIncoming)
		s.publishFriendSync(ctx, realtime.FriendSyncEvent{
			Reason:       realtime.FriendSyncReasonRequestResponded,
			RequestID:    int64(request.ID),
			Status:       request.Status,
			ActorUserID:  int64(userID),
			FriendUserID: int64(request.UserID),
			UserIDs:      []int64{int64(userID), int64(request.UserID)},
		})
		return &view, nil, nil
	}

	var friendView *FriendView
	var conversationID string
	if err := s.TxManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		friendRelations := s.FriendRelations.WithTx(tx)
		friendRequests := s.FriendRequests.WithTx(tx)

		now := time.Now()
		callerRelation := &model.FriendRelation{
			UserID:       request.UserID,
			FriendUserID: request.TargetUserID,
			Remark:       request.Remark,
			GroupID:      request.GroupID,
			Status:       model.FriendRelationStatusActive,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		targetRelation := &model.FriendRelation{
			UserID:       request.TargetUserID,
			FriendUserID: request.UserID,
			Status:       model.FriendRelationStatusActive,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := friendRelations.Create(ctx, callerRelation); err != nil {
			return err
		}
		if err := friendRelations.Create(ctx, targetRelation); err != nil {
			return err
		}

		request.Status = model.FriendRequestStatusAccepted
		request.UpdatedAt = now
		if err := friendRequests.Update(ctx, request); err != nil {
			return err
		}

		view := friendRelationToView(*targetRelation, *requester)
		friendView = &view
		return nil
	}); err != nil {
		return nil, nil, err
	}

	view := friendRequestToView(*request, *requester, model.FriendRequestDirectionIncoming)
	if s.ChatClient != nil {
		conversation, err := s.ChatClient.CreateSingleConversation(ctx, userID, request.UserID)
		if err != nil {
			log.Printf("create single conversation after friend acceptance failed: user=%d friend=%d err=%v", userID, request.UserID, err)
		} else if conversation != nil {
			conversationID = conversation.ConversationId
		}
	}
	s.publishFriendSync(ctx, realtime.FriendSyncEvent{
		Reason:         realtime.FriendSyncReasonRequestResponded,
		RequestID:      int64(request.ID),
		Status:         request.Status,
		ActorUserID:    int64(userID),
		FriendUserID:   int64(request.UserID),
		ConversationID: conversationID,
		UserIDs:        []int64{int64(userID), int64(request.UserID)},
	})
	return &view, friendView, nil
}

func (s *UserService) publishFriendSync(ctx context.Context, event realtime.FriendSyncEvent) {
	if s.FriendEvents == nil || len(event.UserIDs) == 0 || strings.TrimSpace(event.Reason) == "" {
		return
	}
	if err := s.FriendEvents.PublishFriendSync(ctx, event); err != nil {
		log.Printf("publish friend sync event failed: reason=%s request=%d err=%v", event.Reason, event.RequestID, err)
	}
}

func (s *UserService) ListFriends(ctx context.Context, userID uint64) ([]FriendView, error) {
	if userID == 0 {
		return nil, ErrFriendBadRequest
	}

	relations, err := s.FriendRelations.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(relations) == 0 {
		return []FriendView{}, nil
	}

	friendIDs := make([]uint64, 0, len(relations))
	for _, relation := range relations {
		friendIDs = append(friendIDs, relation.FriendUserID)
	}

	users, err := s.Users.ListByIDs(ctx, friendIDs)
	if err != nil {
		return nil, err
	}
	userByID := make(map[uint64]model.User, len(users))
	for _, user := range users {
		userByID[user.ID] = user
	}

	result := make([]FriendView, 0, len(relations))
	for _, relation := range relations {
		friendUser, ok := userByID[relation.FriendUserID]
		if !ok {
			continue
		}
		result = append(result, friendRelationToView(relation, friendUser))
	}
	return result, nil
}

func (s *UserService) CheckFriendRelation(ctx context.Context, userID, friendUserID uint64) (bool, error) {
	if userID == 0 || friendUserID == 0 {
		return false, ErrFriendBadRequest
	}
	relation, err := s.FriendRelations.GetByUserPair(ctx, userID, friendUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return relation.Status == model.FriendRelationStatusActive, nil
}

func (s *UserService) UpdateFriend(ctx context.Context, userID, friendUserID uint64, remark string, groupID *uint64) (*FriendView, error) {
	remark = strings.TrimSpace(remark)
	if userID == 0 || friendUserID == 0 {
		return nil, ErrFriendBadRequest
	}
	if len([]rune(remark)) > 128 {
		return nil, errno.BadRequest("friend remark is too long")
	}
	if err := s.validateGroupOwnership(ctx, userID, groupID); err != nil {
		return nil, err
	}

	relation, err := s.FriendRelations.GetByUserPair(ctx, userID, friendUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFriendNotFound
		}
		return nil, err
	}

	friendUser, err := s.Users.GetByID(ctx, friendUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFriendTargetNotFound
		}
		return nil, err
	}

	relation.Remark = remark
	relation.GroupID = groupID
	relation.UpdatedAt = time.Now()
	if err := s.FriendRelations.Update(ctx, relation); err != nil {
		return nil, err
	}

	view := friendRelationToView(*relation, *friendUser)
	s.publishFriendSync(ctx, realtime.FriendSyncEvent{
		Reason:       realtime.FriendSyncReasonFriendUpdated,
		ActorUserID:  int64(userID),
		FriendUserID: int64(friendUserID),
		UserIDs:      []int64{int64(userID)},
	})
	return &view, nil
}

func (s *UserService) DeleteFriend(ctx context.Context, userID, friendUserID uint64) error {
	if userID == 0 || friendUserID == 0 {
		return ErrFriendBadRequest
	}

	if _, err := s.FriendRelations.GetByUserPair(ctx, userID, friendUserID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFriendNotFound
		}
		return err
	}

	if err := s.TxManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		friendRelations := s.FriendRelations.WithTx(tx)
		if err := friendRelations.DeleteByUserPair(ctx, userID, friendUserID); err != nil {
			return err
		}
		return friendRelations.DeleteByUserPair(ctx, friendUserID, userID)
	}); err != nil {
		return err
	}

	s.publishFriendSync(ctx, realtime.FriendSyncEvent{
		Reason:       realtime.FriendSyncReasonFriendDeleted,
		ActorUserID:  int64(userID),
		FriendUserID: int64(friendUserID),
		UserIDs:      []int64{int64(userID), int64(friendUserID)},
	})
	return nil
}

func (s *UserService) validateGroupOwnership(ctx context.Context, userID uint64, groupID *uint64) error {
	if groupID == nil {
		return nil
	}
	if *groupID == 0 {
		return ErrFriendBadRequest
	}
	if _, err := s.FriendGroups.GetByIDAndUserID(ctx, *groupID, userID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFriendGroupNotFound
		}
		return err
	}
	return nil
}

func friendGroupToView(group *model.FriendGroup) *FriendGroupView {
	if group == nil {
		return nil
	}
	return &FriendGroupView{
		ID:        group.ID,
		Name:      group.Name,
		SortOrder: group.SortOrder,
		CreatedAt: group.CreatedAt.Unix(),
		UpdatedAt: group.UpdatedAt.Unix(),
	}
}

func friendRelationToView(relation model.FriendRelation, user model.User) FriendView {
	return FriendView{
		UserID:    user.ID,
		AimID:     user.AimID,
		Nickname:  user.Nickname,
		Avatar:    user.Avatar,
		Remark:    relation.Remark,
		GroupID:   relation.GroupID,
		Status:    relation.Status,
		CreatedAt: relation.CreatedAt.Unix(),
		UpdatedAt: relation.UpdatedAt.Unix(),
	}
}

func friendRequestToView(request model.FriendRequest, user model.User, direction string) FriendRequestView {
	return FriendRequestView{
		ID:        request.ID,
		UserID:    user.ID,
		AimID:     user.AimID,
		Nickname:  user.Nickname,
		Avatar:    user.Avatar,
		Direction: direction,
		Status:    request.Status,
		Remark:    request.Remark,
		GroupID:   request.GroupID,
		CreatedAt: request.CreatedAt.Unix(),
		UpdatedAt: request.UpdatedAt.Unix(),
	}
}

var _ repository.TxManager = (*repository.GormTxManager)(nil)
