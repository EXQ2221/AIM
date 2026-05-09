package handler

import (
	"net"
	"strings"

	"github.com/gin-gonic/gin"
)

func clientIP(ctx *gin.Context) string {
	if forwarded := ctx.GetHeader("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	host, _, err := net.SplitHostPort(ctx.Request.RemoteAddr)
	if err == nil {
		return host
	}
	return ctx.Request.RemoteAddr
}
