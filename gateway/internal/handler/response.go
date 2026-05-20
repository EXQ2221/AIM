package handler

import (
	"example.com/aim/shared/errno"
	"example.com/aim/gateway/internal/model/dto"
	"github.com/gin-gonic/gin"
)

func writeJSON(ctx *gin.Context, status int, data any) {
	ctx.JSON(status, data)
}

func writeError(ctx *gin.Context, status int, message string) {
	code := codeFromHTTPStatus(status)
	writeJSON(ctx, status, dto.APIResponse{
		Code:    code,
		Message: message,
	})
}

func codeFromHTTPStatus(status int) int {
	switch status {
	case 400:
		return errno.ErrBadRequest
	case 401:
		return errno.ErrUnauthorized
	case 403:
		return errno.ErrForbidden
	case 404:
		return errno.ErrNotFound
	case 409:
		return errno.ErrConflict
	default:
		return errno.ErrInternalError
	}
}
