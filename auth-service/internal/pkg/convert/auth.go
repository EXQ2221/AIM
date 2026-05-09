package convert

import (
	"example.com/aim/auth-service/internal/biz"
	authpb "example.com/aim/auth-service/kitex_gen/auth"
)

func ToTokenPair(pair *biz.TokenPair) *authpb.TokenPair {
	if pair == nil {
		return nil
	}

	return &authpb.TokenPair{
		AccessToken:      pair.AccessToken,
		RefreshToken:     pair.RefreshToken,
		SessionId:        pair.SessionID,
		AccessExpiresAt:  pair.AccessExpiresAt,
		RefreshExpiresAt: pair.RefreshExpiresAt,
	}
}

func ToUserIdentity(identity *biz.AuthIdentity) *authpb.UserIdentity {
	if identity == nil {
		return nil
	}
	return &authpb.UserIdentity{
		UserId:       int64(identity.UserID),
		AimId:        identity.AimID,
		Role:         identity.Role,
		TokenVersion: int64(identity.TokenVersion),
	}
}

func ToSessionInfos(sessions []biz.SessionView) []*authpb.SessionInfo {
	result := make([]*authpb.SessionInfo, 0, len(sessions))
	for _, session := range sessions {
		result = append(result, &authpb.SessionInfo{
			SessionId:  session.SessionID,
			DeviceId:   session.DeviceID,
			DeviceName: session.DeviceName,
			UserAgent:  session.UserAgent,
			LoginIp:    session.LoginIP,
			LastIp:     session.LastIP,
			Status:     session.Status,
			Current:    session.Current,
			CreatedAt:  session.CreatedAt,
			LastSeenAt: session.LastSeenAt,
		})
	}
	return result
}
