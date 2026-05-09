package biz

import (
	"context"
	"errors"
	"time"

	"example.com/aim/auth-service/internal/pkg/jwt"
	"example.com/aim/auth-service/internal/pkg/token"
	"example.com/aim/auth-service/internal/repository"
	"example.com/aim/auth-service/internal/rpc"
	"gorm.io/gorm"
)

const userStatusNormal = "NORMAL"

func (s *AuthService) ValidateToken(ctx context.Context, accessToken string) (*AuthIdentity, error) {
	claims, err := jwt.Parse(accessToken, s.Secret)
	if err != nil {
		return nil, ErrInvalidAccessToken
	}

	blacklisted, err := s.Cache.IsAccessTokenBlacklisted(ctx, claims.TokenID)
	if err == nil && blacklisted {
		return nil, ErrInvalidAccessToken
	}

	entry, err := s.Cache.GetSession(ctx, claims.SessionID)
	if err != nil {
		entry = nil
	}
	if entry == nil {
		session, err := s.SessionRepo.GetBySessionID(ctx, claims.SessionID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrInvalidAccessToken
			}
			return nil, err
		}

		entry = &repository.SessionCacheEntry{
			UserID:           session.UserID,
			Status:           session.Status,
			CurrentAccessJTI: session.CurrentAccessJTI,
		}
		_ = s.Cache.SetSession(ctx, session.SessionID, *entry, s.RefreshTTL)
	}

	if entry.Status != SessionStatusActive {
		return nil, ErrSessionRevoked
	}
	if entry.UserID != claims.UserID || entry.CurrentAccessJTI != claims.TokenID {
		return nil, ErrInvalidAccessToken
	}

	user, err := s.UserClient.GetUser(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}
	if !isUserAllowed(user) {
		return nil, ErrUserNotAvailable
	}
	if user.TokenVersion != claims.TokenVersion {
		return nil, ErrInvalidAccessToken
	}

	return &AuthIdentity{
		UserID:       claims.UserID,
		AimID:        user.AimID,
		Role:         user.Role,
		TokenVersion: user.TokenVersion,
		SessionID:    claims.SessionID,
	}, nil
}

func (s *AuthService) issueAccessToken(user *rpc.UserInfo, sessionID string, now time.Time) (string, string, time.Time, error) {
	tokenID, err := token.Generate(16)
	if err != nil {
		return "", "", time.Time{}, err
	}
	expiresAt := now.Add(s.AccessTTL)
	tokenValue, err := jwt.Sign(jwt.NewClaims(
		user.UserID,
		user.AimID,
		user.Role,
		user.TokenVersion,
		sessionID,
		tokenID,
		now,
		expiresAt,
	), s.Secret)
	if err != nil {
		return "", "", time.Time{}, err
	}
	return tokenValue, tokenID, expiresAt, nil
}

func isUserAllowed(user *rpc.UserInfo) bool {
	return user != nil && user.Status == userStatusNormal
}
