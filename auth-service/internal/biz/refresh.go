package biz

import (
	"context"
	"errors"
	"time"

	"example.com/aim/auth-service/internal/dal/model"
	browserprofile "example.com/aim/auth-service/internal/pkg/browser"
	"example.com/aim/auth-service/internal/pkg/token"
	"gorm.io/gorm"
)

func NewRefreshToken() (string, error) {
	return token.Generate(32)
}

func (s *AuthService) Refresh(ctx context.Context, input RefreshInput) (*TokenPair, error) {
	now := time.Now()
	refreshHash := token.Hash(input.RefreshToken)
	currentBrowser := browserprofile.Parse(input.UserAgent)

	var (
		pair         *TokenPair
		cacheSession *model.Session
		reuseRecord  *model.RefreshToken
		finalErr     error
	)

	err := s.TxManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		refreshRepo := s.RefreshRepo.WithTx(tx)
		sessionRepo := s.SessionRepo.WithTx(tx)
		eventRepo := s.EventRepo.WithTx(tx)

		record, err := refreshRepo.GetByTokenHashForUpdate(ctx, refreshHash)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				finalErr = ErrInvalidRefreshToken
				return nil
			}
			return err
		}

		session, err := sessionRepo.GetBySessionIDForUpdate(ctx, record.SessionID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				finalErr = ErrSessionNotFound
				return nil
			}
			return err
		}

		if record.Status != RefreshStatusActive {
			reuseRecord = record
			finalErr = ErrRefreshReuse
			return nil
		}
		if now.After(record.ExpiresAt) {
			record.Status = RefreshStatusRevoked
			record.RevokedAt = &now
			record.RevokeReason = "expired"
			if err := refreshRepo.Update(ctx, record); err != nil {
				return err
			}
			finalErr = ErrInvalidRefreshToken
			return nil
		}
		if session.Status != SessionStatusActive {
			finalErr = ErrSessionRevoked
			return nil
		}
		if session.DeviceID != "" && input.DeviceID != "" && session.DeviceID != input.DeviceID {
			if err := s.recordEventWithRepo(ctx, eventRepo, session.UserID, session.SessionID, "device_mismatch", input.IP, input.DeviceID, input.UserAgent, "refresh attempted from another device id"); err != nil {
				return err
			}
			if err := s.revokeSessionWithRepos(ctx, session, "device_mismatch", sessionRepo, refreshRepo); err != nil {
				return err
			}
			cacheSession = session
			finalErr = ErrDeviceMismatch
			return nil
		}
		if session.BrowserKey != "" && currentBrowser.Key != "" && session.BrowserKey != currentBrowser.Key {
			if err := s.recordEventWithRepo(ctx, eventRepo, session.UserID, session.SessionID, "browser_mismatch", input.IP, input.DeviceID, input.UserAgent, browserMismatchDetail(session.BrowserKey, currentBrowser.Key)); err != nil {
				return err
			}
			if err := s.revokeSessionWithRepos(ctx, session, "browser_mismatch", sessionRepo, refreshRepo); err != nil {
				return err
			}
			cacheSession = session
			finalErr = ErrDeviceMismatch
			return nil
		}
		if session.BrowserKey != "" && currentBrowser.Key != "" && session.BrowserKey == currentBrowser.Key &&
			session.BrowserVersion != "" && currentBrowser.BrowserVersion != "" && session.BrowserVersion != currentBrowser.BrowserVersion {
			if err := s.recordEventWithRepo(ctx, eventRepo, session.UserID, session.SessionID, "browser_version_changed", input.IP, input.DeviceID, input.UserAgent, browserVersionChangedDetail(session.BrowserName, session.BrowserVersion, currentBrowser.BrowserVersion)); err != nil {
				return err
			}
		}
		if session.LastIP != "" && input.IP != "" && session.LastIP != input.IP {
			if err := s.recordEventWithRepo(ctx, eventRepo, session.UserID, session.SessionID, "ip_changed", input.IP, input.DeviceID, input.UserAgent, "refresh token used from a new ip"); err != nil {
				return err
			}
		}

		newRefreshToken, err := NewRefreshToken()
		if err != nil {
			return err
		}
		userInfo, err := s.UserClient.GetUser(ctx, session.UserID)
		if err != nil {
			return err
		}
		if !isUserAllowed(userInfo) {
			finalErr = ErrUserNotAvailable
			return nil
		}
		accessToken, accessJTI, accessExpiresAt, err := s.issueAccessToken(userInfo, session.SessionID, now)
		if err != nil {
			return err
		}

		record.Status = RefreshStatusUsed
		record.UsedAt = &now
		record.LastUsedIP = input.IP
		record.LastUsedUserAgent = input.UserAgent
		record.RotatedTo = token.Hash(newRefreshToken)
		if err := refreshRepo.Update(ctx, record); err != nil {
			return err
		}

		refreshExpiresAt := now.Add(s.RefreshTTL)
		if err := refreshRepo.Create(ctx, &model.RefreshToken{
			SessionID: session.SessionID,
			UserID:    session.UserID,
			TokenHash: token.Hash(newRefreshToken),
			Status:    RefreshStatusActive,
			ExpiresAt: refreshExpiresAt,
		}); err != nil {
			return err
		}

		session.LastSeenAt = now
		session.LastIP = input.IP
		session.UserAgent = input.UserAgent
		if currentBrowser.BrowserName != "" {
			session.BrowserName = currentBrowser.BrowserName
		}
		if currentBrowser.BrowserVersion != "" {
			session.BrowserVersion = currentBrowser.BrowserVersion
		}
		if currentBrowser.OSName != "" {
			session.OSName = currentBrowser.OSName
		}
		if currentBrowser.DeviceType != "" {
			session.DeviceType = currentBrowser.DeviceType
		}
		if currentBrowser.Key != "" {
			session.BrowserKey = currentBrowser.Key
		}
		session.CurrentAccessJTI = accessJTI
		session.CurrentAccessExpires = accessExpiresAt
		if err := sessionRepo.Update(ctx, session); err != nil {
			return err
		}

		cacheSession = session
		pair = &TokenPair{
			AccessToken:      accessToken,
			RefreshToken:     newRefreshToken,
			SessionID:        session.SessionID,
			AccessExpiresAt:  accessExpiresAt.Unix(),
			RefreshExpiresAt: refreshExpiresAt.Unix(),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if reuseRecord != nil {
		_ = s.handleRefreshReuse(ctx, reuseRecord, input)
	}
	if cacheSession != nil {
		if cacheSession.Status == SessionStatusRevoked {
			s.afterSessionRevoked(ctx, cacheSession)
		} else {
			_ = s.cacheSession(ctx, cacheSession)
		}
	}
	if finalErr != nil {
		return nil, finalErr
	}

	return pair, nil
}

func (s *AuthService) handleRefreshReuse(ctx context.Context, record *model.RefreshToken, input RefreshInput) error {
	_ = s.recordEvent(ctx, record.UserID, record.SessionID, "refresh_token_reuse", input.IP, input.DeviceID, input.UserAgent, "used or revoked refresh token was presented again")
	if err := s.revokeAllSessions(ctx, record.UserID, "refresh_token_reuse"); err != nil {
		return err
	}
	return s.UserClient.BumpTokenVersion(ctx, record.UserID)
}

func browserMismatchDetail(expectedKey, actualKey string) string {
	return "refresh attempted from another browser identity: expected=" + expectedKey + ", actual=" + actualKey
}

func browserVersionChangedDetail(browserName, previousVersion, currentVersion string) string {
	if browserName == "" {
		browserName = "browser"
	}
	return browserName + " major version changed from " + previousVersion + " to " + currentVersion
}
