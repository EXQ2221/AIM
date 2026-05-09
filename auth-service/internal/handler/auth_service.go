package handler

import (
	"context"

	"example.com/aim/auth-service/internal/biz"
	"example.com/aim/auth-service/internal/pkg/convert"
	authpb "example.com/aim/auth-service/kitex_gen/auth"
)

type AuthServiceImpl struct {
	Service *biz.AuthService
}

func NewAuthServiceImpl(service *biz.AuthService) *AuthServiceImpl {
	return &AuthServiceImpl{Service: service}
}

func (h *AuthServiceImpl) Register(ctx context.Context, req *authpb.RegisterRequest) (*authpb.RegisterResponse, error) {
	identity, err := h.Service.Register(ctx, biz.RegisterInput{
		AimID:    req.AimId,
		Email:    req.Email,
		Nickname: req.Nickname,
		Password: req.Password,
	})
	if err != nil {
		return nil, err
	}

	return &authpb.RegisterResponse{User: convert.ToUserIdentity(identity)}, nil
}

func (h *AuthServiceImpl) Login(ctx context.Context, req *authpb.LoginRequest) (*authpb.TokenPair, error) {
	pair, err := h.Service.Login(ctx, biz.LoginInput{
		Email:      req.Email,
		Password:   req.Password,
		DeviceID:   req.DeviceId,
		DeviceName: req.DeviceName,
		UserAgent:  req.UserAgent,
		IP:         req.Ip,
	})
	if err != nil {
		return nil, err
	}

	return convert.ToTokenPair(pair), nil
}

func (h *AuthServiceImpl) RefreshToken(ctx context.Context, req *authpb.RefreshTokenRequest) (*authpb.TokenPair, error) {
	pair, err := h.Service.Refresh(ctx, biz.RefreshInput{
		RefreshToken: req.RefreshToken,
		DeviceID:     req.DeviceId,
		UserAgent:    req.UserAgent,
		IP:           req.Ip,
	})
	if err != nil {
		return nil, err
	}

	return convert.ToTokenPair(pair), nil
}

func (h *AuthServiceImpl) ValidateToken(ctx context.Context, req *authpb.ValidateTokenRequest) (*authpb.ValidateTokenResponse, error) {
	identity, err := h.Service.ValidateToken(ctx, req.AccessToken)
	if err != nil {
		return &authpb.ValidateTokenResponse{
			Valid:  false,
			Reason: err.Error(),
		}, nil
	}

	return &authpb.ValidateTokenResponse{
		Valid:        true,
		UserId:       int64(identity.UserID),
		SessionId:    identity.SessionID,
		AimId:        identity.AimID,
		Role:         identity.Role,
		TokenVersion: int64(identity.TokenVersion),
	}, nil
}

func (h *AuthServiceImpl) Logout(ctx context.Context, req *authpb.LogoutRequest) (*authpb.CommonResponse, error) {
	if err := h.Service.Logout(ctx, req.AccessToken); err != nil {
		return &authpb.CommonResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &authpb.CommonResponse{Success: true, Message: "ok"}, nil
}

func (h *AuthServiceImpl) LogoutAll(ctx context.Context, req *authpb.LogoutAllRequest) (*authpb.CommonResponse, error) {
	if err := h.Service.LogoutAll(ctx, uint64(req.UserId), req.Password); err != nil {
		return &authpb.CommonResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &authpb.CommonResponse{Success: true, Message: "ok"}, nil
}

func (h *AuthServiceImpl) ListSessions(ctx context.Context, req *authpb.ListSessionsRequest) (*authpb.ListSessionsResponse, error) {
	sessions, err := h.Service.ListSessions(ctx, uint64(req.UserId), req.CurrentSessionId)
	if err != nil {
		return nil, err
	}

	return &authpb.ListSessionsResponse{
		Sessions: convert.ToSessionInfos(sessions),
	}, nil
}

func (h *AuthServiceImpl) RevokeSession(ctx context.Context, req *authpb.RevokeSessionRequest) (*authpb.CommonResponse, error) {
	if err := h.Service.RevokeSession(ctx, uint64(req.UserId), req.SessionId, req.Password); err != nil {
		return &authpb.CommonResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &authpb.CommonResponse{Success: true, Message: "ok"}, nil
}
