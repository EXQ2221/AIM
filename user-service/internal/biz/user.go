package biz

import (
	"context"
	"errors"
	"strings"
	"time"

	"example.com/aim/shared/errno"
	"example.com/aim/user-service/internal/dal/model"
	"example.com/aim/user-service/internal/pkg/password"
	"example.com/aim/user-service/internal/realtime"
	"example.com/aim/user-service/internal/repository"
	"example.com/aim/user-service/internal/rpc"
	"gorm.io/gorm"
)

type UserService struct {
	Users           repository.UserRepository
	FriendGroups    repository.FriendGroupRepository
	FriendRelations repository.FriendRelationRepository
	FriendRequests  repository.FriendRequestRepository
	TxManager       repository.TxManager
	ChatClient      rpc.ChatClient
	FriendEvents    realtime.FriendSyncPublisher
}

func NewUserService(
	userRepo repository.UserRepository,
	friendGroupRepo repository.FriendGroupRepository,
	friendRelationRepo repository.FriendRelationRepository,
	friendRequestRepo repository.FriendRequestRepository,
	txManager repository.TxManager,
) *UserService {
	return &UserService{
		Users:           userRepo,
		FriendGroups:    friendGroupRepo,
		FriendRelations: friendRelationRepo,
		FriendRequests:  friendRequestRepo,
		TxManager:       txManager,
	}
}

func (s *UserService) SetChatClient(chatClient rpc.ChatClient) {
	s.ChatClient = chatClient
}

func (s *UserService) SetFriendEventPublisher(publisher realtime.FriendSyncPublisher) {
	s.FriendEvents = publisher
}

func (s *UserService) GetByID(ctx context.Context, id uint64) (*model.User, error) {
	return s.Users.GetByID(ctx, id)
}

func (s *UserService) GetByAimID(ctx context.Context, aimID string) (*model.User, error) {
	return s.Users.GetByAimID(ctx, strings.TrimSpace(aimID))
}

func (s *UserService) CreateUser(ctx context.Context, aimID, email, nickname, rawPassword string) (*model.User, error) {
	aimID = strings.TrimSpace(aimID)
	email = strings.ToLower(strings.TrimSpace(email))
	nickname = strings.TrimSpace(nickname)

	if aimID == "" || email == "" || nickname == "" || rawPassword == "" {
		return nil, errno.New(errno.ErrBadRequest, "aim_id, email, nickname and password are required")
	}
	if strings.ContainsAny(aimID, " \t\r\n") {
		return nil, errno.New(errno.ErrBadRequest, "aim_id cannot contain whitespace")
	}

	hash := password.HashPassword(rawPassword)
	if hash == "" {
		return nil, errors.New("failed to hash password")
	}

	user := &model.User{
		AimID:        aimID,
		Email:        email,
		Nickname:     nickname,
		PasswordHash: hash,
		Status:       model.UserStatusNormal,
		Role:         model.UserRoleUser,
		TokenVersion: 1,
	}

	if err := s.Users.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *UserService) VerifyCredential(ctx context.Context, email, rawPassword string) (*model.User, bool, error) {
	user, err := s.Users.GetByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, errno.New(errno.ErrUserNotFound, "user not found")
		}
		return nil, false, err
	}

	if user.Status != model.UserStatusNormal {
		return user, false, errno.NewWithUser(errno.ErrUserNotAvailable, "user is not available", user.ID)
	}

	if !password.ComparePassword(rawPassword, user.PasswordHash) {
		return user, false, errno.NewWithUser(errno.ErrPasswordWrong, "password incorrect", user.ID)
	}

	return user, true, nil
}

func (s *UserService) CheckPassword(ctx context.Context, userID uint64, rawPassword string) (bool, error) {
	user, err := s.Users.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}

	return password.ComparePassword(rawPassword, user.PasswordHash), nil
}

func (s *UserService) UpdateLoginState(ctx context.Context, userID uint64, ip string) error {
	return s.Users.UpdateLoginState(ctx, userID, strings.TrimSpace(ip), time.Now())
}

func (s *UserService) BumpTokenVersion(ctx context.Context, userID uint64) error {
	return s.Users.BumpTokenVersion(ctx, userID)
}

func (s *UserService) UpdateAvatar(ctx context.Context, userID uint64, avatar string) (*model.User, error) {
	avatar = strings.TrimSpace(avatar)
	if userID == 0 || avatar == "" {
		return nil, errno.New(errno.ErrBadRequest, "user_id and avatar are required")
	}
	if len(avatar) > 512 {
		return nil, errno.New(errno.ErrBadRequest, "avatar is too long")
	}

	if err := s.Users.UpdateAvatar(ctx, userID, avatar); err != nil {
		return nil, err
	}
	return s.Users.GetByID(ctx, userID)
}

func (s *UserService) SeedDemoUser(ctx context.Context) error {
	_, err := s.Users.GetByEmail(ctx, "demo@example.com")
	switch {
	case err == nil:
		return nil
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return err
	}

	_, err = s.CreateUser(ctx, "aim_demo", "demo@example.com", "demo-user", "Password123!")
	return err
}
