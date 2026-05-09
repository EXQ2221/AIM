package middleware

import (
	"example.com/aim/gateway/internal/authcookie"
	"example.com/aim/gateway/internal/rpc"
	authpb "example.com/aim/gateway/kitex_gen/auth"
	"github.com/gin-gonic/gin"
)

func Auth() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		token := authcookie.AccessToken(ctx)
		if token == "" {
			ctx.AbortWithStatusJSON(401, gin.H{
				"code":    401,
				"message": "missing access token",
			})
			return
		}

		client, err := rpc.AuthClient()
		if err != nil {
			ctx.AbortWithStatusJSON(500, gin.H{
				"code":    500,
				"message": err.Error(),
			})
			return
		}

		resp, err := client.ValidateToken(ctx.Request.Context(), &authpb.ValidateTokenRequest{
			AccessToken: token,
		})

		if err != nil {
			ctx.AbortWithStatusJSON(500, gin.H{
				"code":    500,
				"message": err.Error(),
			})
			return
		}
		if !resp.Valid {
			ctx.AbortWithStatusJSON(401, gin.H{
				"code":    401,
				"message": resp.Reason,
			})
			return
		}

		ctx.Set(authContextKey, AuthContext{
			UserID:       resp.UserId,
			AimID:        resp.AimId,
			Role:         resp.Role,
			TokenVersion: resp.TokenVersion,
			SessionID:    resp.SessionId,
			AccessToken:  token,
		})
		ctx.Next()
	}
}
