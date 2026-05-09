package model

type CreateFriendGroupRequest struct {
	Name string `json:"name"`
}

type FriendGroupInfo struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	SortOrder int32  `json:"sort_order"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

type AddFriendRequest struct {
	TargetAimID string `json:"target_aim_id"`
	Remark      string `json:"remark"`
	GroupID     *int64 `json:"group_id,omitempty"`
}

type RespondFriendRequest struct {
	Action string `json:"action"`
}

type UpdateFriendRequest struct {
	Remark  string `json:"remark"`
	GroupID *int64 `json:"group_id,omitempty"`
}

type FriendInfo struct {
	UserID    int64  `json:"user_id"`
	AimID     string `json:"aim_id"`
	Nickname  string `json:"nickname"`
	Avatar    string `json:"avatar"`
	Remark    string `json:"remark"`
	GroupID   *int64 `json:"group_id,omitempty"`
	Status    string `json:"status"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

type FriendRequestInfo struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"user_id"`
	AimID     string `json:"aim_id"`
	Nickname  string `json:"nickname"`
	Avatar    string `json:"avatar"`
	Direction string `json:"direction"`
	Status    string `json:"status"`
	Remark    string `json:"remark"`
	GroupID   *int64 `json:"group_id,omitempty"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}
