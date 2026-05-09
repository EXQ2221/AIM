namespace go auth

struct UserIdentity {
  1: i64 user_id
  2: string aim_id
  3: string role
  4: i64 token_version
}

struct RegisterRequest {
  1: string aim_id
  2: string email
  3: string nickname
  4: string password
}

struct RegisterResponse {
  1: UserIdentity user
}

struct LoginRequest {
  1: string email
  2: string password
  3: string device_id
  4: string device_name
  5: string user_agent
  6: string ip
}

struct TokenPair {
  1: string access_token
  2: string refresh_token
  3: string session_id
  4: i64 access_expires_at
  5: i64 refresh_expires_at
}

struct RefreshTokenRequest {
  1: string refresh_token
  2: string device_id
  3: string user_agent
  4: string ip
}

struct ValidateTokenRequest {
  1: string access_token
}

struct ValidateTokenResponse {
  1: bool valid
  2: i64 user_id
  3: string session_id
  4: string reason
  5: string aim_id
  6: string role
  7: i64 token_version
}

struct CommonResponse {
  1: bool success
  2: string message
}

struct LogoutRequest {
  1: string access_token
}

struct LogoutAllRequest {
  1: i64 user_id
  2: string password
}

struct SessionInfo {
  1: string session_id
  2: string device_id
  3: string device_name
  4: string user_agent
  5: string login_ip
  6: string last_ip
  7: string status
  8: bool current
  9: i64 created_at
  10: i64 last_seen_at
}

struct ListSessionsRequest {
  1: i64 user_id
  2: string current_session_id
}

struct ListSessionsResponse {
  1: list<SessionInfo> sessions
}

struct RevokeSessionRequest {
  1: i64 user_id
  2: string session_id
  3: string password
}

service AuthService {
  RegisterResponse Register(1: RegisterRequest req)
  TokenPair Login(1: LoginRequest req)
  TokenPair RefreshToken(1: RefreshTokenRequest req)
  ValidateTokenResponse ValidateToken(1: ValidateTokenRequest req)
  CommonResponse Logout(1: LogoutRequest req)
  CommonResponse LogoutAll(1: LogoutAllRequest req)
  ListSessionsResponse ListSessions(1: ListSessionsRequest req)
  CommonResponse RevokeSession(1: RevokeSessionRequest req)
}
