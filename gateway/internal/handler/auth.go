package handler

import (
	"errors"
	"io"
	"strings"

	"example.com/aim/gateway/internal/authcookie"
	"example.com/aim/gateway/internal/middleware"
	"example.com/aim/gateway/internal/model/dto"
	"example.com/aim/gateway/internal/rpc"
	authpb "example.com/aim/gateway/kitex_gen/auth"
	"github.com/gin-gonic/gin"
)

func Register(ctx *gin.Context) {
	var req dto.RegisterRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}
	if strings.TrimSpace(req.AimID) == "" || strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.Nickname) == "" || req.Password == "" {
		writeError(ctx, 400, "aim_id, email, nickname and password are required")
		return
	}
	if strings.ContainsAny(strings.TrimSpace(req.AimID), " \t\r\n") {
		writeError(ctx, 400, "aim_id cannot contain whitespace")
		return
	}

	client, err := rpc.AuthClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.Register(ctx.Request.Context(), &authpb.RegisterRequest{
		AimId:    req.AimID,
		Email:    req.Email,
		Nickname: req.Nickname,
		Password: req.Password,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	writeJSON(ctx, 200, dto.APIResponse{
		Code:    0,
		Message: "success",
		Data: dto.UserInfo{
			UserID:       resp.User.UserId,
			AimID:        resp.User.AimId,
			Role:         resp.User.Role,
			TokenVersion: resp.User.TokenVersion,
		},
	})
}

func Login(ctx *gin.Context) {
	var req dto.LoginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}

	client, err := rpc.AuthClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	deviceID := authcookie.DeviceID(ctx)
	if deviceID == "" {
		deviceID = newID()
	}
	if req.DeviceName == "" {
		req.DeviceName = "web-client"
	}

	resp, err := client.Login(ctx.Request.Context(), &authpb.LoginRequest{
		Email:      req.Email,
		Password:   req.Password,
		DeviceId:   deviceID,
		DeviceName: req.DeviceName,
		UserAgent:  ctx.Request.UserAgent(),
		Ip:         clientIP(ctx),
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	authcookie.SetSessionCookies(
		ctx,
		resp.AccessToken,
		resp.RefreshToken,
		resp.AccessExpiresAt,
		resp.RefreshExpiresAt,
		deviceID,
	)

	writeJSON(ctx, 200, dto.APIResponse{
		Code:    0,
		Message: "success",
		Data: dto.AuthSessionResponse{
			SessionID:        resp.SessionId,
			DeviceID:         deviceID,
			AccessExpiresAt:  resp.AccessExpiresAt,
			RefreshExpiresAt: resp.RefreshExpiresAt,
		},
	})
}

func Refresh(ctx *gin.Context) {
	var req dto.RefreshRequest
	if err := bindOptionalJSON(ctx, &req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}

	if req.RefreshToken == "" {
		req.RefreshToken = authcookie.RefreshToken(ctx)
	}
	deviceID := authcookie.DeviceID(ctx)
	if req.RefreshToken == "" {
		writeError(ctx, 401, "missing refresh token")
		return
	}
	if deviceID == "" {
		writeError(ctx, 400, "missing device_id")
		return
	}

	client, err := rpc.AuthClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.RefreshToken(ctx.Request.Context(), &authpb.RefreshTokenRequest{
		RefreshToken: req.RefreshToken,
		DeviceId:     deviceID,
		UserAgent:    ctx.Request.UserAgent(),
		Ip:           clientIP(ctx),
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	authcookie.SetSessionCookies(
		ctx,
		resp.AccessToken,
		resp.RefreshToken,
		resp.AccessExpiresAt,
		resp.RefreshExpiresAt,
		deviceID,
	)

	writeJSON(ctx, 200, dto.APIResponse{
		Code:    0,
		Message: "success",
		Data: dto.AuthSessionResponse{
			SessionID:        resp.SessionId,
			DeviceID:         deviceID,
			AccessExpiresAt:  resp.AccessExpiresAt,
			RefreshExpiresAt: resp.RefreshExpiresAt,
		},
	})
}

func Logout(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	client, err := rpc.AuthClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.Logout(ctx.Request.Context(), &authpb.LogoutRequest{
		AccessToken: authCtx.AccessToken,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if !resp.Success {
		writeError(ctx, statusFromMessage(resp.Message), presentableMessage(resp.Message))
		return
	}

	authcookie.ClearSessionCookies(ctx)
	writeJSON(ctx, 200, dto.APIResponse{Code: 0, Message: "success"})
}

func LogoutAll(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	var req dto.PasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}

	client, err := rpc.AuthClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.LogoutAll(ctx.Request.Context(), &authpb.LogoutAllRequest{
		UserId:   authCtx.UserID,
		Password: req.Password,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if !resp.Success {
		writeError(ctx, statusFromMessage(resp.Message), presentableMessage(resp.Message))
		return
	}

	authcookie.ClearSessionCookies(ctx)
	writeJSON(ctx, 200, dto.APIResponse{Code: 0, Message: "success"})
}

func ListSessions(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	client, err := rpc.AuthClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.ListSessions(ctx.Request.Context(), &authpb.ListSessionsRequest{
		UserId:           authCtx.UserID,
		CurrentSessionId: authCtx.SessionID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	sessions := make([]dto.SessionInfo, 0, len(resp.Sessions))
	for _, session := range resp.Sessions {
		sessions = append(sessions, dto.SessionInfo{
			SessionID:  session.SessionId,
			DeviceID:   session.DeviceId,
			DeviceName: session.DeviceName,
			UserAgent:  session.UserAgent,
			LoginIP:    session.LoginIp,
			LastIP:     session.LastIp,
			Status:     session.Status,
			Current:    session.Current,
			CreatedAt:  session.CreatedAt,
			LastSeenAt: session.LastSeenAt,
		})
	}

	writeJSON(ctx, 200, dto.APIResponse{
		Code:    0,
		Message: "success",
		Data:    sessions,
	})
}

func RevokeSession(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	var req dto.RevokeSessionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}

	client, err := rpc.AuthClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.RevokeSession(ctx.Request.Context(), &authpb.RevokeSessionRequest{
		UserId:    authCtx.UserID,
		SessionId: req.SessionID,
		Password:  req.Password,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if !resp.Success {
		writeError(ctx, statusFromMessage(resp.Message), presentableMessage(resp.Message))
		return
	}

	if req.SessionID == authCtx.SessionID {
		authcookie.ClearSessionCookies(ctx)
	}
	writeJSON(ctx, 200, dto.APIResponse{Code: 0, Message: "success"})
}

func bindOptionalJSON(ctx *gin.Context, target any) error {
	if ctx.Request == nil || ctx.Request.Body == nil || ctx.Request.ContentLength == 0 {
		return nil
	}

	err := ctx.ShouldBindJSON(target)
	if errors.Is(err, io.EOF) {
		return nil
	}

	return err
}
