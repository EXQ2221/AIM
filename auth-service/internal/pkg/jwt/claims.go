package jwt

import (
	"strconv"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID       uint64 `json:"user_id"`
	AimID        string `json:"aim_id"`
	Role         string `json:"role"`
	TokenVersion uint64 `json:"token_version"`
	ExpireTime   int64  `json:"expire_time"`
	SessionID    string `json:"sid"`
	TokenID      string `json:"jti"`
	jwtv5.RegisteredClaims
}

func NewClaims(userID uint64, aimID, role string, tokenVersion uint64, sessionID, tokenID string, issuedAt, expiresAt time.Time) Claims {
	return Claims{
		UserID:       userID,
		AimID:        aimID,
		Role:         role,
		TokenVersion: tokenVersion,
		ExpireTime:   expiresAt.Unix(),
		SessionID:    sessionID,
		TokenID:      tokenID,
		RegisteredClaims: jwtv5.RegisteredClaims{
			Subject:   strconv.FormatUint(userID, 10),
			ID:        tokenID,
			IssuedAt:  jwtv5.NewNumericDate(issuedAt),
			ExpiresAt: jwtv5.NewNumericDate(expiresAt),
		},
	}
}
