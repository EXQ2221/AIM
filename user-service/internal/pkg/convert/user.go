package convert

import (
	"example.com/aim/user-service/internal/biz"
	"example.com/aim/user-service/internal/dal/model"
	userpb "example.com/aim/user-service/kitex_gen/user"
)

func ToUserInfo(user *model.User) *userpb.UserInfo {
	if user == nil {
		return nil
	}

	return &userpb.UserInfo{
		UserId:       int64(user.ID),
		AimId:        user.AimID,
		Email:        user.Email,
		Nickname:     user.Nickname,
		Avatar:       user.Avatar,
		Status:       user.Status,
		Role:         user.Role,
		TokenVersion: int64(user.TokenVersion),
		CreatedAt:    user.CreatedAt.Unix(),
		UpdatedAt:    user.UpdatedAt.Unix(),
	}
}

func ToFriendGroupInfo(group *biz.FriendGroupView) *userpb.FriendGroupInfo {
	if group == nil {
		return nil
	}

	return &userpb.FriendGroupInfo{
		Id:        int64(group.ID),
		Name:      group.Name,
		SortOrder: group.SortOrder,
		CreatedAt: group.CreatedAt,
		UpdatedAt: group.UpdatedAt,
	}
}

func ToFriendInfo(friend *biz.FriendView) *userpb.FriendInfo {
	if friend == nil {
		return nil
	}

	var groupID *int64
	if friend.GroupID != nil {
		value := int64(*friend.GroupID)
		groupID = &value
	}

	return &userpb.FriendInfo{
		UserId:    int64(friend.UserID),
		AimId:     friend.AimID,
		Nickname:  friend.Nickname,
		Avatar:    friend.Avatar,
		Remark:    friend.Remark,
		GroupId:   groupID,
		Status:    friend.Status,
		CreatedAt: friend.CreatedAt,
		UpdatedAt: friend.UpdatedAt,
	}
}

func ToFriendRequestInfo(request *biz.FriendRequestView) *userpb.FriendRequestInfo {
	if request == nil {
		return nil
	}

	var groupID *int64
	if request.GroupID != nil {
		value := int64(*request.GroupID)
		groupID = &value
	}

	return &userpb.FriendRequestInfo{
		Id:        int64(request.ID),
		UserId:    int64(request.UserID),
		AimId:     request.AimID,
		Nickname:  request.Nickname,
		Avatar:    request.Avatar,
		Direction: request.Direction,
		Status:    request.Status,
		Remark:    request.Remark,
		GroupId:   groupID,
		CreatedAt: request.CreatedAt,
		UpdatedAt: request.UpdatedAt,
	}
}
