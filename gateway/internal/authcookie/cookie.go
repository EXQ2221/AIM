package authcookie

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	AccessTokenCookieName  = "access_token"
	RefreshTokenCookieName = "refresh_token"
	DeviceIDCookieName     = "device_id"

	accessTokenCookiePath  = "/"
	refreshTokenCookiePath = "/api/v1/auth/refresh"
	deviceIDCookiePath     = "/"
)

type config struct {
	domain   string
	secure   bool
	sameSite http.SameSite
}

func AccessToken(ctx *gin.Context) string {
	if token := cookieValue(ctx, AccessTokenCookieName); token != "" {
		return token
	}
	return bearerToken(ctx.GetHeader("Authorization"))
}

func RefreshToken(ctx *gin.Context) string {
	return cookieValue(ctx, RefreshTokenCookieName)
}

func DeviceID(ctx *gin.Context) string {
	return cookieValue(ctx, DeviceIDCookieName)
}

func SetSessionCookies(ctx *gin.Context, accessToken, refreshToken string, accessExpiresAt, refreshExpiresAt int64, deviceID string) {
	cfg := loadConfig()
	ctx.SetSameSite(cfg.sameSite)

	if accessToken != "" {
		setCookie(
			ctx,
			AccessTokenCookieName,
			accessToken,
			accessTokenCookiePath,
			accessExpiresAt,
			true,
			cfg,
		)
	}

	if refreshToken != "" {
		setCookie(
			ctx,
			RefreshTokenCookieName,
			refreshToken,
			refreshTokenCookiePath,
			refreshExpiresAt,
			true,
			cfg,
		)
	}

	if deviceID != "" {
		setCookie(
			ctx,
			DeviceIDCookieName,
			deviceID,
			deviceIDCookiePath,
			refreshExpiresAt,
			false,
			cfg,
		)
	}
}

func ClearSessionCookies(ctx *gin.Context) {
	cfg := loadConfig()
	ctx.SetSameSite(cfg.sameSite)
	clearCookie(ctx, AccessTokenCookieName, accessTokenCookiePath, true, cfg)
	clearCookie(ctx, RefreshTokenCookieName, refreshTokenCookiePath, true, cfg)
}

func cookieValue(ctx *gin.Context, name string) string {
	if ctx == nil {
		return ""
	}

	cookie, err := ctx.Cookie(name)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(cookie)
}

func bearerToken(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return strings.TrimSpace(parts[1])
	}

	return strings.TrimSpace(strings.TrimPrefix(header, "Bearer"))
}

func loadConfig() config {
	return config{
		domain:   strings.TrimSpace(os.Getenv("COOKIE_DOMAIN")),
		secure:   parseBoolEnv("COOKIE_SECURE"),
		sameSite: parseSameSite(os.Getenv("COOKIE_SAMESITE")),
	}
}

func parseBoolEnv(key string) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return false
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}

	return parsed
}

func parseSameSite(value string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	case "default":
		return http.SameSiteDefaultMode
	default:
		return http.SameSiteLaxMode
	}
}

func setCookie(ctx *gin.Context, name, value, path string, expiresAt int64, httpOnly bool, cfg config) {
	expireTime := time.Unix(expiresAt, 0)
	maxAge := int(time.Until(expireTime).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}

	ctx.SetCookie(name, value, maxAge, path, cfg.domain, cfg.secure, httpOnly)
}

func clearCookie(ctx *gin.Context, name, path string, httpOnly bool, cfg config) {
	ctx.SetCookie(name, "", -1, path, cfg.domain, cfg.secure, httpOnly)
}
