package handler

import (
	"context"
	"errors"

	"example.com/aim/shared/errno"
	"example.com/aim/user-service/internal/biz"
	"example.com/aim/user-service/internal/pkg/convert"
	userpb "example.com/aim/user-service/kitex_gen/user"
)

type UserServiceImpl struct {
	Service *biz.UserService
}

func NewUserServiceImpl(service *biz.UserService) *UserServiceImpl {
	return &UserServiceImpl{Service: service}
}

func (h *UserServiceImpl) CreateUser(ctx context.Context, req *userpb.CreateUserRequest) (*userpb.CreateUserResponse, error) {
	user, err := h.Service.CreateUser(ctx, req.AimId, req.Email, req.Nickname, req.Password)
	if err != nil {
		return nil, err
	}

	return &userpb.CreateUserResponse{
		User: convert.ToUserInfo(user),
	}, nil
}

func (h *UserServiceImpl) GetUser(ctx context.Context, req *userpb.GetUserRequest) (*userpb.GetUserResponse, error) {
	user, err := h.Service.GetByID(ctx, uint64(req.UserId))
	if err != nil {
		return nil, err
	}

	return &userpb.GetUserResponse{
		User: convert.ToUserInfo(user),
	}, nil
}

func (h *UserServiceImpl) GetUserByAimID(ctx context.Context, req *userpb.GetUserByAimIDRequest) (*userpb.GetUserByAimIDResponse, error) {
	user, err := h.Service.GetByAimID(ctx, req.AimId)
	if err != nil {
		return nil, err
	}

	return &userpb.GetUserByAimIDResponse{
		User: convert.ToUserInfo(user),
	}, nil
}

func (h *UserServiceImpl) VerifyCredential(ctx context.Context, req *userpb.VerifyCredentialRequest) (*userpb.VerifyCredentialResponse, error) {
	user, ok, err := h.Service.VerifyCredential(ctx, req.Email, req.Password)
	resp := &userpb.VerifyCredentialResponse{
		Ok:   ok,
		User: convert.ToUserInfo(user),
	}

	if err != nil {
		var bizErr errno.Error
		if errors.As(err, &bizErr) {
			resp.Reason = err.Error()
			return resp, nil
		}
		return nil, err
	}

	return resp, nil
}

func (h *UserServiceImpl) CheckPassword(ctx context.Context, req *userpb.CheckPasswordRequest) (*userpb.CheckPasswordResponse, error) {
	ok, err := h.Service.CheckPassword(ctx, uint64(req.UserId), req.Password)
	if err != nil {
		return nil, err
	}

	return &userpb.CheckPasswordResponse{Ok: ok}, nil
}

func (h *UserServiceImpl) UpdateLoginState(ctx context.Context, req *userpb.UpdateLoginStateRequest) (*userpb.CommonResponse, error) {
	if err := h.Service.UpdateLoginState(ctx, uint64(req.UserId), req.LastLoginIp); err != nil {
		return &userpb.CommonResponse{Success: false, Message: err.Error()}, nil
	}
	return &userpb.CommonResponse{Success: true, Message: "ok"}, nil
}

func (h *UserServiceImpl) BumpTokenVersion(ctx context.Context, req *userpb.BumpTokenVersionRequest) (*userpb.CommonResponse, error) {
	if err := h.Service.BumpTokenVersion(ctx, uint64(req.UserId)); err != nil {
		return &userpb.CommonResponse{Success: false, Message: err.Error()}, nil
	}
	return &userpb.CommonResponse{Success: true, Message: "ok"}, nil
}

func (h *UserServiceImpl) UpdateAvatar(ctx context.Context, req *userpb.UpdateAvatarRequest) (*userpb.UpdateAvatarResponse, error) {
	user, err := h.Service.UpdateAvatar(ctx, uint64(req.UserId), req.Avatar)
	if err != nil {
		return nil, err
	}
	return &userpb.UpdateAvatarResponse{
		User: convert.ToUserInfo(user),
	}, nil
}

func (h *UserServiceImpl) CreateFriendGroup(ctx context.Context, req *userpb.CreateFriendGroupRequest) (*userpb.CreateFriendGroupResponse, error) {
	group, err := h.Service.CreateFriendGroup(ctx, uint64(req.UserId), req.Name)
	if err != nil {
		return nil, err
	}

	return &userpb.CreateFriendGroupResponse{
		Group: convert.ToFriendGroupInfo(group),
	}, nil
}

func (h *UserServiceImpl) ListFriendGroups(ctx context.Context, req *userpb.ListFriendGroupsRequest) (*userpb.ListFriendGroupsResponse, error) {
	groups, err := h.Service.ListFriendGroups(ctx, uint64(req.UserId))
	if err != nil {
		return nil, err
	}

	result := make([]*userpb.FriendGroupInfo, 0, len(groups))
	for i := range groups {
		group := groups[i]
		result = append(result, convert.ToFriendGroupInfo(&group))
	}
	return &userpb.ListFriendGroupsResponse{Groups: result}, nil
}

func (h *UserServiceImpl) AddFriend(ctx context.Context, req *userpb.AddFriendRequest) (*userpb.AddFriendResponse, error) {
	var groupID *uint64
	if req.GroupId != nil {
		value := uint64(*req.GroupId)
		groupID = &value
	}

	friendRequest, err := h.Service.AddFriend(ctx, uint64(req.UserId), req.TargetAimId, req.Remark, groupID)
	if err != nil {
		return nil, err
	}
	return &userpb.AddFriendResponse{
		Request: convert.ToFriendRequestInfo(friendRequest),
	}, nil
}

func (h *UserServiceImpl) ListFriends(ctx context.Context, req *userpb.ListFriendsRequest) (*userpb.ListFriendsResponse, error) {
	friends, err := h.Service.ListFriends(ctx, uint64(req.UserId))
	if err != nil {
		return nil, err
	}

	result := make([]*userpb.FriendInfo, 0, len(friends))
	for i := range friends {
		friend := friends[i]
		result = append(result, convert.ToFriendInfo(&friend))
	}
	return &userpb.ListFriendsResponse{Friends: result}, nil
}

func (h *UserServiceImpl) CheckFriendRelation(ctx context.Context, req *userpb.CheckFriendRelationRequest) (*userpb.CheckFriendRelationResponse, error) {
	isFriend, err := h.Service.CheckFriendRelation(ctx, uint64(req.UserId), uint64(req.FriendUserId))
	if err != nil {
		return nil, err
	}
	return &userpb.CheckFriendRelationResponse{IsFriend: isFriend}, nil
}

func (h *UserServiceImpl) UpdateFriend(ctx context.Context, req *userpb.UpdateFriendRequest) (*userpb.UpdateFriendResponse, error) {
	var groupID *uint64
	if req.GroupId != nil {
		value := uint64(*req.GroupId)
		groupID = &value
	}

	friend, err := h.Service.UpdateFriend(ctx, uint64(req.UserId), uint64(req.FriendUserId), req.Remark, groupID)
	if err != nil {
		return nil, err
	}
	return &userpb.UpdateFriendResponse{
		Friend: convert.ToFriendInfo(friend),
	}, nil
}

func (h *UserServiceImpl) DeleteFriend(ctx context.Context, req *userpb.DeleteFriendRequest) (*userpb.CommonResponse, error) {
	if err := h.Service.DeleteFriend(ctx, uint64(req.UserId), uint64(req.FriendUserId)); err != nil {
		return &userpb.CommonResponse{Success: false, Message: err.Error()}, nil
	}
	return &userpb.CommonResponse{Success: true, Message: "ok"}, nil
}

func (h *UserServiceImpl) ListFriendRequests(ctx context.Context, req *userpb.ListFriendRequestsRequest) (*userpb.ListFriendRequestsResponse, error) {
	requests, err := h.Service.ListFriendRequests(ctx, uint64(req.UserId))
	if err != nil {
		return nil, err
	}

	result := make([]*userpb.FriendRequestInfo, 0, len(requests))
	for i := range requests {
		request := requests[i]
		result = append(result, convert.ToFriendRequestInfo(&request))
	}
	return &userpb.ListFriendRequestsResponse{Requests: result}, nil
}

func (h *UserServiceImpl) RespondFriendRequest(ctx context.Context, req *userpb.RespondFriendRequestRequest) (*userpb.RespondFriendRequestResponse, error) {
	request, friend, err := h.Service.RespondFriendRequest(ctx, uint64(req.UserId), uint64(req.RequestId), req.Action)
	if err != nil {
		return nil, err
	}

	return &userpb.RespondFriendRequestResponse{
		Request: convert.ToFriendRequestInfo(request),
		Friend:  convert.ToFriendInfo(friend),
	}, nil
}
