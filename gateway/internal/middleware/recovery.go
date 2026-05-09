package middleware

import (
	"log"

	"github.com/gin-gonic/gin"
)

func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(ctx *gin.Context, rec any) {
		log.Printf("panic recovered: %v", rec)
		ctx.AbortWithStatusJSON(500, gin.H{
			"code":    500,
			"message": "internal server error",
		})
	})
}
