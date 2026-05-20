package biz

import "example.com/aim/shared/errno"

const (
	SessionStatusActive  = "active"
	SessionStatusRevoked = "revoked"

	RefreshStatusActive  = "active"
	RefreshStatusUsed    = "used"
	RefreshStatusRevoked = "revoked"
)

var (
	ErrInvalidCredentials  = errno.Unauthorized("invalid email or password")
	ErrInvalidAccessToken  = errno.Unauthorized("invalid access token")
	ErrInvalidRefreshToken = errno.Unauthorized("invalid refresh token")
	ErrUserNotAvailable    = errno.Forbidden("user is not available")
	ErrPasswordConfirm     = errno.Unauthorized("password confirmation failed")
	ErrSessionNotFound     = errno.NotFound("session not found")
	ErrSessionRevoked      = errno.Unauthorized("session revoked")
	ErrDeviceMismatch      = errno.Unauthorized("refresh from a different device or browser")
	ErrRefreshReuse        = errno.Forbidden("refresh token reuse detected, all sessions revoked")
)

type RegisterInput struct {
	AimID    string
	Email    string
	Nickname string
	Password string
}

type LoginInput struct {
	Email      string
	Password   string
	DeviceID   string
	DeviceName string
	UserAgent  string
	IP         string
}

type RefreshInput struct {
	RefreshToken string
	DeviceID     string
	UserAgent    string
	IP           string
}

type AuthIdentity struct {
	UserID       uint64
	AimID        string
	Role         string
	TokenVersion uint64
	SessionID    string
}

type SessionView struct {
	SessionID  string
	DeviceID   string
	DeviceName string
	UserAgent  string
	LoginIP    string
	LastIP     string
	Status     string
	Current    bool
	CreatedAt  int64
	LastSeenAt int64
}
