package model

type RegisterRequest struct {
	AimID    string `json:"aim_id"`
	Email    string `json:"email"`
	Nickname string `json:"nickname"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	DeviceName string `json:"device_name"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token,omitempty"`
}

type PasswordRequest struct {
	Password string `json:"password"`
}

type RevokeSessionRequest struct {
	SessionID string `json:"session_id"`
	Password  string `json:"password"`
}

type AuthSessionResponse struct {
	SessionID        string `json:"session_id"`
	DeviceID         string `json:"device_id"`
	AccessExpiresAt  int64  `json:"access_expires_at"`
	RefreshExpiresAt int64  `json:"refresh_expires_at"`
}

type SessionInfo struct {
	SessionID  string `json:"session_id"`
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	UserAgent  string `json:"user_agent"`
	LoginIP    string `json:"login_ip"`
	LastIP     string `json:"last_ip"`
	Status     string `json:"status"`
	Current    bool   `json:"current"`
	CreatedAt  int64  `json:"created_at"`
	LastSeenAt int64  `json:"last_seen_at"`
}

type UserInfo struct {
	UserID       int64  `json:"user_id"`
	AimID        string `json:"aim_id"`
	Email        string `json:"email"`
	Nickname     string `json:"nickname"`
	Avatar       string `json:"avatar"`
	Status       string `json:"status"`
	Role         string `json:"role"`
	TokenVersion int64  `json:"token_version"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
}

type UploadedFileInfo struct {
	URL         string `json:"url"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

type UploadAvatarResponse struct {
	Avatar string           `json:"avatar"`
	File   UploadedFileInfo `json:"file"`
	User   UserInfo         `json:"user"`
}

type UploadMediaResponse struct {
	File UploadedFileInfo `json:"file"`
}
