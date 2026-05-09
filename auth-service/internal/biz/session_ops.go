package biz

import (
	"context"
	"errors"

	"example.com/aim/auth-service/internal/dal/model"
	"example.com/aim/auth-service/internal/pkg/jwt"
	"gorm.io/gorm"
)

func (s *AuthService) Logout(ctx context.Context, accessToken string) error {
	claims, err := jwt.Parse(accessToken, s.Secret)
	if err != nil {
		return ErrInvalidAccessToken
	}

	session, err := s.SessionRepo.GetBySessionID(ctx, claims.SessionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrSessionNotFound
		}
		return err
	}

	return s.revokeSession(ctx, session, "logout")
}

func (s *AuthService) LogoutAll(ctx context.Context, userID uint64, password string) error {
	ok, err := s.UserClient.CheckPassword(ctx, userID, password)
	if err != nil {
		return err
	}
	if !ok {
		return ErrPasswordConfirm
	}
	if err := s.revokeAllSessions(ctx, userID, "logout_all"); err != nil {
		return err
	}
	return s.UserClient.BumpTokenVersion(ctx, userID)
}

func (s *AuthService) ListSessions(ctx context.Context, userID uint64, currentSessionID string) ([]SessionView, error) {
	sessions, err := s.SessionRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make([]SessionView, 0, len(sessions))
	for _, session := range sessions {
		if session.Status == SessionStatusRevoked {
			continue
		}
		result = append(result, SessionView{
			SessionID:  session.SessionID,
			DeviceID:   session.DeviceID,
			DeviceName: session.DeviceName,
			UserAgent:  session.UserAgent,
			LoginIP:    session.LoginIP,
			LastIP:     session.LastIP,
			Status:     session.Status,
			Current:    session.SessionID == currentSessionID,
			CreatedAt:  session.CreatedAt.Unix(),
			LastSeenAt: session.LastSeenAt.Unix(),
		})
	}

	return result, nil
}

func (s *AuthService) RevokeSession(ctx context.Context, userID uint64, sessionID, password string) error {
	ok, err := s.UserClient.CheckPassword(ctx, userID, password)
	if err != nil {
		return err
	}
	if !ok {
		return ErrPasswordConfirm
	}

	session, err := s.SessionRepo.GetBySessionID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrSessionNotFound
		}
		return err
	}
	if session.UserID != userID {
		return ErrSessionNotFound
	}

	return s.revokeSession(ctx, session, "revoke_session")
}

func (s *AuthService) revokeAllSessions(ctx context.Context, userID uint64, reason string) error {
	sessions, err := s.SessionRepo.ListByUserID(ctx, userID)
	if err != nil {
		return err
	}

	for i := range sessions {
		if sessions[i].Status == SessionStatusRevoked {
			continue
		}
		if err := s.revokeSession(ctx, &sessions[i], reason); err != nil {
			return err
		}
	}

	return nil
}

func (s *AuthService) revokeSession(ctx context.Context, session *model.Session, reason string) error {
	err := s.TxManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		sessionRepo := s.SessionRepo.WithTx(tx)
		refreshRepo := s.RefreshRepo.WithTx(tx)
		return s.revokeSessionWithRepos(ctx, session, reason, sessionRepo, refreshRepo)
	})
	if err != nil {
		return err
	}

	s.afterSessionRevoked(ctx, session)
	return nil
}
