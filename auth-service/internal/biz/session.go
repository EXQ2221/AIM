package biz

import "errors"

const (
	SessionStatusActive  = "active"
	SessionStatusRevoked = "revoked"

	RefreshStatusActive  = "active"
	RefreshStatusUsed    = "used"
	RefreshStatusRevoked = "revoked"
)

var (
	ErrInvalidCredentials  = errors.New("unauthorized: invalid email or password")
	ErrInvalidAccessToken  = errors.New("unauthorized: invalid access token")
	ErrInvalidRefreshToken = errors.New("unauthorized: invalid refresh token")
	ErrUserNotAvailable    = errors.New("forbidden: user is not available")
	ErrPasswordConfirm     = errors.New("unauthorized: password confirmation failed")
	ErrSessionNotFound     = errors.New("not_found: session not found")
	ErrSessionRevoked      = errors.New("unauthorized: session revoked")
	ErrDeviceMismatch      = errors.New("unauthorized: refresh from a different device or browser")
	ErrRefreshReuse        = errors.New("forbidden: refresh token reuse detected, all sessions revoked")
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
