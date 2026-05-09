package biz

import (
	"time"

	"example.com/aim/auth-service/internal/repository"
	"example.com/aim/auth-service/internal/rpc"
)

type TokenPair struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	SessionID        string `json:"session_id"`
	AccessExpiresAt  int64  `json:"access_expires_at"`
	RefreshExpiresAt int64  `json:"refresh_expires_at"`
}

type AuthService struct {
	SessionRepo repository.SessionRepository
	RefreshRepo repository.RefreshTokenRepository
	EventRepo   repository.SecurityEventRepository
	TxManager   repository.TxManager
	Cache       repository.AuthCache
	UserClient  rpc.UserClient
	Secret      string
	AccessTTL   time.Duration
	RefreshTTL  time.Duration
}

func NewAuthService(
	sessionRepo repository.SessionRepository,
	refreshRepo repository.RefreshTokenRepository,
	eventRepo repository.SecurityEventRepository,
	txManager repository.TxManager,
	cache repository.AuthCache,
	userClient rpc.UserClient,
	secret string,
	accessTTL time.Duration,
	refreshTTL time.Duration,
) *AuthService {
	return &AuthService{
		SessionRepo: sessionRepo,
		RefreshRepo: refreshRepo,
		EventRepo:   eventRepo,
		TxManager:   txManager,
		Cache:       cache,
		UserClient:  userClient,
		Secret:      secret,
		AccessTTL:   accessTTL,
		RefreshTTL:  refreshTTL,
	}
}
