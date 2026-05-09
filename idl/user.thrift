namespace go user

struct UserInfo {
  1: i64 user_id
  2: string aim_id
  3: string email
  4: string nickname
  5: string avatar
  6: string status
  7: string role
  8: i64 token_version
  9: i64 created_at
  10: i64 updated_at
}

struct CreateUserRequest {
  1: string aim_id
  2: string email
  3: string nickname
  4: string password
}

struct CreateUserResponse {
  1: UserInfo user
}

struct GetUserRequest {
  1: i64 user_id
}

struct GetUserResponse {
  1: UserInfo user
}

struct GetUserByAimIDRequest {
  1: string aim_id
}

struct GetUserByAimIDResponse {
  1: UserInfo user
}

struct VerifyCredentialRequest {
  1: string email
  2: string password
}

struct VerifyCredentialResponse {
  1: bool ok
  2: UserInfo user
  3: string reason
}

struct CheckPasswordRequest {
  1: i64 user_id
  2: string password
}

struct CheckPasswordResponse {
  1: bool ok
}

struct UpdateLoginStateRequest {
  1: i64 user_id
  2: string last_login_ip
}

struct CommonResponse {
  1: bool success
  2: string message
}

struct BumpTokenVersionRequest {
  1: i64 user_id
}

struct UpdateAvatarRequest {
  1: i64 user_id
  2: string avatar
}

struct UpdateAvatarResponse {
  1: UserInfo user
}

struct FriendGroupInfo {
  1: i64 id
  2: string name
  3: i32 sort_order
  4: i64 created_at
  5: i64 updated_at
}

struct CreateFriendGroupRequest {
  1: i64 user_id
  2: string name
}

struct CreateFriendGroupResponse {
  1: FriendGroupInfo group
}

struct ListFriendGroupsRequest {
  1: i64 user_id
}

struct ListFriendGroupsResponse {
  1: list<FriendGroupInfo> groups
}

struct FriendInfo {
  1: i64 user_id
  2: string aim_id
  3: string nickname
  4: string avatar
  5: string remark
  6: optional i64 group_id
  7: string status
  8: i64 created_at
  9: i64 updated_at
}

struct FriendRequestInfo {
  1: i64 id
  2: i64 user_id
  3: string aim_id
  4: string nickname
  5: string avatar
  6: string direction
  7: string status
  8: string remark
  9: optional i64 group_id
  10: i64 created_at
  11: i64 updated_at
}

struct AddFriendRequest {
  1: i64 user_id
  2: string target_aim_id
  3: string remark
  4: optional i64 group_id
}

struct AddFriendResponse {
  1: FriendRequestInfo request
}

struct ListFriendsRequest {
  1: i64 user_id
}

struct ListFriendsResponse {
  1: list<FriendInfo> friends
}

struct UpdateFriendRequest {
  1: i64 user_id
  2: i64 friend_user_id
  3: string remark
  4: optional i64 group_id
}

struct UpdateFriendResponse {
  1: FriendInfo friend
}

struct DeleteFriendRequest {
  1: i64 user_id
  2: i64 friend_user_id
}

struct CheckFriendRelationRequest {
  1: i64 user_id
  2: i64 friend_user_id
}

struct CheckFriendRelationResponse {
  1: bool is_friend
}

struct ListFriendRequestsRequest {
  1: i64 user_id
}

struct ListFriendRequestsResponse {
  1: list<FriendRequestInfo> requests
}

struct RespondFriendRequestRequest {
  1: i64 user_id
  2: i64 request_id
  3: string action
}

struct RespondFriendRequestResponse {
  1: FriendRequestInfo request
  2: optional FriendInfo friend
}

service UserService {
  CreateUserResponse CreateUser(1: CreateUserRequest req)
  GetUserResponse GetUser(1: GetUserRequest req)
  GetUserByAimIDResponse GetUserByAimID(1: GetUserByAimIDRequest req)
  VerifyCredentialResponse VerifyCredential(1: VerifyCredentialRequest req)
  CheckPasswordResponse CheckPassword(1: CheckPasswordRequest req)
  CommonResponse UpdateLoginState(1: UpdateLoginStateRequest req)
  CommonResponse BumpTokenVersion(1: BumpTokenVersionRequest req)
  UpdateAvatarResponse UpdateAvatar(1: UpdateAvatarRequest req)
  CreateFriendGroupResponse CreateFriendGroup(1: CreateFriendGroupRequest req)
  ListFriendGroupsResponse ListFriendGroups(1: ListFriendGroupsRequest req)
  AddFriendResponse AddFriend(1: AddFriendRequest req)
  ListFriendsResponse ListFriends(1: ListFriendsRequest req)
  CheckFriendRelationResponse CheckFriendRelation(1: CheckFriendRelationRequest req)
  UpdateFriendResponse UpdateFriend(1: UpdateFriendRequest req)
  CommonResponse DeleteFriend(1: DeleteFriendRequest req)
  ListFriendRequestsResponse ListFriendRequests(1: ListFriendRequestsRequest req)
  RespondFriendRequestResponse RespondFriendRequest(1: RespondFriendRequestRequest req)
}
