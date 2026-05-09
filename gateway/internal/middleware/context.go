package middleware

import "github.com/gin-gonic/gin"

const authContextKey = "auth-context"

type AuthContext struct {
	UserID       int64
	AimID        string
	Role         string
	TokenVersion int64
	SessionID    string
	AccessToken  string
}

func GetAuthContext(ctx *gin.Context) (AuthContext, bool) {
	value, ok := ctx.Get(authContextKey)
	if !ok {
		return AuthContext{}, false
	}

	authCtx, ok := value.(AuthContext)
	if !ok {
		return AuthContext{}, false
	}

	return authCtx, true
}
