package biz

import (
	"context"
	"errors"
	"fmt"
	"time"

	"example.com/aim/auth-service/internal/dal/model"
	browserprofile "example.com/aim/auth-service/internal/pkg/browser"
	"example.com/aim/auth-service/internal/pkg/token"
	"example.com/aim/shared/errno"
	"gorm.io/gorm"
)

func (s *AuthService) Register(ctx context.Context, input RegisterInput) (*AuthIdentity, error) {
	user, err := s.UserClient.CreateUser(ctx, input.AimID, input.Email, input.Nickname, input.Password)
	if err != nil {
		return nil, err
	}
	if !isUserAllowed(user) {
		return nil, ErrUserNotAvailable
	}
	return &AuthIdentity{
		UserID:       user.UserID,
		AimID:        user.AimID,
		Role:         user.Role,
		TokenVersion: user.TokenVersion,
	}, nil
}

func (s *AuthService) Login(ctx context.Context, input LoginInput) (*TokenPair, error) {
	userInfo, ok, err := s.UserClient.VerifyCredential(ctx, input.Email, input.Password)
	if err != nil {
		var bizErr errno.Error
		if errors.As(err, &bizErr) {
			s.recordLoginFailure(ctx, bizErr, input)
		}
		return nil, err
	}
	if !ok || !isUserAllowed(userInfo) {
		return nil, ErrInvalidCredentials
	}

	now := time.Now()
	sessionID, err := token.Generate(16)
	if err != nil {
		return nil, err
	}
	refreshTokenValue, err := NewRefreshToken()
	if err != nil {
		return nil, err
	}
	accessToken, accessJTI, accessExpiresAt, err := s.issueAccessToken(userInfo, sessionID, now)
	if err != nil {
		return nil, err
	}

	refreshExpiresAt := now.Add(s.RefreshTTL)
	browserInfo := browserprofile.Parse(input.UserAgent)
	session := &model.Session{
		SessionID:            sessionID,
		UserID:               userInfo.UserID,
		Status:               SessionStatusActive,
		DeviceID:             coalesce(input.DeviceID, sessionID),
		DeviceName:           coalesce(input.DeviceName, "unknown-device"),
		UserAgent:            input.UserAgent,
		BrowserName:          browserInfo.BrowserName,
		BrowserVersion:       browserInfo.BrowserVersion,
		OSName:               browserInfo.OSName,
		DeviceType:           browserInfo.DeviceType,
		BrowserKey:           browserInfo.Key,
		LoginIP:              input.IP,
		LastIP:               input.IP,
		LastSeenAt:           now,
		CurrentAccessJTI:     accessJTI,
		CurrentAccessExpires: accessExpiresAt,
	}

	refreshRecord := &model.RefreshToken{
		SessionID: sessionID,
		UserID:    userInfo.UserID,
		TokenHash: token.Hash(refreshTokenValue),
		Status:    RefreshStatusActive,
		ExpiresAt: refreshExpiresAt,
	}

	err = s.TxManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		sessionRepo := s.SessionRepo.WithTx(tx)
		refreshRepo := s.RefreshRepo.WithTx(tx)

		if err := sessionRepo.Create(ctx, session); err != nil {
			return err
		}
		return refreshRepo.Create(ctx, refreshRecord)
	})
	if err != nil {
		return nil, err
	}

	_ = s.cacheSession(ctx, session)
	_ = s.UserClient.UpdateLoginState(ctx, userInfo.UserID, input.IP)

	return &TokenPair{
		AccessToken:      accessToken,
		RefreshToken:     refreshTokenValue,
		SessionID:        sessionID,
		AccessExpiresAt:  accessExpiresAt.Unix(),
		RefreshExpiresAt: refreshExpiresAt.Unix(),
	}, nil
}

func (s *AuthService) recordLoginFailure(ctx context.Context, bizErr errno.Error, input LoginInput) {
	switch bizErr.Code {
	case errno.ErrPasswordWrong:
		count, redisErr := s.Cache.IncrLoginFail(ctx, bizErr.UserID)
		if redisErr != nil {
			fmt.Printf("redis incr login fail error: %v\n", redisErr)
		}
		if count >= 5 {
			_ = s.recordEvent(ctx, bizErr.UserID, "", "brute_force", input.IP, input.DeviceID, input.UserAgent, fmt.Sprintf("password wrong %d times in 5 minutes", count))
		}
	case errno.ErrUserNotFound:
		count, redisErr := s.Cache.IncrLoginFailByIP(ctx, input.IP)
		if redisErr == nil && count >= 5 {
			_ = s.recordEvent(ctx, 0, "", "brute_force", input.IP, input.DeviceID, input.UserAgent, fmt.Sprintf("user not found %d times in 5 minutes (ip-based)", count))
		}
	}
}
