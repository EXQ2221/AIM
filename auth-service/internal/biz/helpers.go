package biz

import (
	"context"
	"time"

	"example.com/aim/auth-service/internal/dal/model"
	"example.com/aim/auth-service/internal/repository"
)

func (s *AuthService) cacheSession(ctx context.Context, session *model.Session) error {
	return s.Cache.SetSession(ctx, session.SessionID, repository.SessionCacheEntry{
		UserID:           session.UserID,
		Status:           session.Status,
		CurrentAccessJTI: session.CurrentAccessJTI,
	}, s.RefreshTTL)
}

func (s *AuthService) recordEvent(ctx context.Context, userID uint64, sessionID, eventType, ip, deviceID, userAgent, detail string) error {
	return s.recordEventWithRepo(ctx, s.EventRepo, userID, sessionID, eventType, ip, deviceID, userAgent, detail)
}

func (s *AuthService) recordEventWithRepo(ctx context.Context, eventRepo repository.SecurityEventRepository, userID uint64, sessionID, eventType, ip, deviceID, userAgent, detail string) error {
	return eventRepo.Create(ctx, &model.SecurityEvent{
		UserID:    userID,
		SessionID: sessionID,
		EventType: eventType,
		IP:        ip,
		DeviceID:  deviceID,
		UserAgent: userAgent,
		Detail:    detail,
		CreatedAt: time.Now(),
	})
}

func (s *AuthService) revokeSessionWithRepos(ctx context.Context, session *model.Session, reason string, sessionRepo repository.SessionRepository, refreshRepo repository.RefreshTokenRepository) error {
	now := time.Now()
	session.Status = SessionStatusRevoked
	session.RevokedAt = &now
	session.RevokeReason = reason

	if err := sessionRepo.Update(ctx, session); err != nil {
		return err
	}
	return refreshRepo.RevokeActiveBySessionID(ctx, session.SessionID, reason, now)
}

func (s *AuthService) afterSessionRevoked(ctx context.Context, session *model.Session) {
	if ttl := time.Until(session.CurrentAccessExpires); ttl > 0 {
		_ = s.Cache.BlacklistAccessToken(ctx, session.CurrentAccessJTI, ttl)
	}
	_ = s.cacheSession(ctx, session)
}

func coalesce(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
