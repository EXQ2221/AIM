namespace go chat

struct HealthRequest {}

struct HealthResponse {
  1: bool ok
}

struct CommonResponse {
  1: bool success
  2: string message
}

struct CreateGroupRequest {
  1: i64 operator_id
  2: string name
  3: string avatar
  4: string announcement
  5: string join_policy
}

struct GroupInfo {
  1: string conversation_id
  2: string type
  3: string name
  4: string avatar
  5: string announcement
  6: i64 owner_id
  7: string join_policy
  8: i64 created_at
}

struct CreateGroupResponse {
  1: GroupInfo group
}

struct CreateSingleConversationRequest {
  1: i64 operator_id
  2: i64 target_user_id
}

struct CreateSingleConversationResponse {
  1: ConversationInfo conversation
}

struct ListConversationsRequest {
  1: i64 user_id
}

struct ConversationInfo {
  1: string conversation_id
  2: string type
  3: string title
  4: string avatar
  5: optional i64 last_message_id
  6: optional i64 last_message_at
  7: string role
  8: bool is_pinned
  9: bool is_muted
  10: i64 updated_at
  11: optional i64 last_message_sender_id
  12: string last_message_sender_name
  13: string last_message_content
}

struct ListConversationsResponse {
  1: list<ConversationInfo> conversations
}

struct JoinGroupRequest {
  1: i64 operator_id
  2: string conversation_id
}

struct InviteMemberRequest {
  1: i64 operator_id
  2: string conversation_id
  3: i64 target_user_id
}

struct LeaveGroupRequest {
  1: i64 operator_id
  2: string conversation_id
}

struct ListMembersRequest {
  1: i64 operator_id
  2: string conversation_id
}

struct MemberInfo {
  1: i64 user_id
  2: string nickname
  3: string avatar
  4: string role
  5: string status
  6: i64 joined_at
  7: string member_type
  8: optional i64 bot_id
  9: optional string mention_name
  10: optional list<string> aliases
  11: optional bool enabled
  12: optional string permission_scope
}

struct ListMembersResponse {
  1: list<MemberInfo> members
}

struct ListMessagesRequest {
  1: i64 operator_id
  2: string conversation_id
  3: optional i64 before_id
  4: i32 limit
}

struct MessageInfo {
  1: i64 id
  2: string conversation_id
  3: i64 sender_id
  4: string sender_type
  5: string message_type
  6: string content
  7: optional i64 reply_to_id
  8: string status
  9: i64 created_at
}

struct BotInfo {
  1: i64 bot_id
  2: string member_type
  3: i64 member_id
  4: string name
  5: string display_name
  6: string mention_name
  7: list<string> aliases
  8: string avatar
  9: string description
  10: bool enabled
  11: string permission_scope
  12: string member_status
  13: string model_name
  14: list<string> supported_models
}

struct AICallLogInfo {
  1: i64 id
  2: string conversation_id
  3: i64 user_id
  4: i64 bot_id
  5: string bot_name
  6: optional i64 request_message_id
  7: optional i64 response_message_id
  8: string model_name
  9: i32 prompt_tokens
  10: i32 completion_tokens
  11: i32 total_tokens
  12: i64 latency_ms
  13: string status
  14: string error_message
  15: i64 created_at
}

struct AICallLogQuotaInfo {
  1: i64 daily_total_tokens
  2: i64 daily_token_limit
  3: i64 remaining_tokens
}

struct ListMessagesResponse {
  1: list<MessageInfo> messages
}

struct ListBotsRequest {
  1: i64 operator_id
}

struct ListBotsResponse {
  1: list<BotInfo> bots
}

struct ListConversationBotsRequest {
  1: i64 operator_id
  2: string conversation_id
}

struct ListConversationBotsResponse {
  1: list<BotInfo> bots
}

struct AddConversationBotRequest {
  1: i64 operator_id
  2: string conversation_id
  3: i64 bot_id
  4: string display_name_override
  5: string mention_name_override
  6: list<string> aliases_override
  7: string permission_scope
  8: string model_name_override
}

struct AddConversationBotResponse {
  1: BotInfo bot
}

struct RemoveConversationBotRequest {
  1: i64 operator_id
  2: string conversation_id
  3: i64 bot_id
}

struct ListAICallLogsRequest {
  1: i64 operator_id
  2: string conversation_id
  3: optional i64 before_id
  4: i32 limit
  5: optional i64 bot_id
  6: optional string status
}

struct ListAICallLogsResponse {
  1: list<AICallLogInfo> logs
  2: AICallLogQuotaInfo quota
}

struct CreateMessageRequest {
  1: i64 operator_id
  2: string conversation_id
  3: string content
  4: optional i64 reply_to_id
}

struct CreateMessageResponse {
  1: MessageInfo message
}

struct FindSingleByUsersRequest {
  1: i64 operator_id
  2: i64 target_user_id
}

struct FindSingleByUsersResponse {
  1: optional ConversationInfo conversation
}

service ChatService {
  HealthResponse Health(1: HealthRequest req)
  CreateGroupResponse CreateGroup(1: CreateGroupRequest req)
  CreateSingleConversationResponse CreateSingleConversation(1: CreateSingleConversationRequest req)
  ListConversationsResponse ListConversations(1: ListConversationsRequest req)
  CommonResponse JoinGroup(1: JoinGroupRequest req)
  CommonResponse InviteMember(1: InviteMemberRequest req)
  CommonResponse LeaveGroup(1: LeaveGroupRequest req)
  ListMembersResponse ListMembers(1: ListMembersRequest req)
  ListMessagesResponse ListMessages(1: ListMessagesRequest req)
  ListBotsResponse ListBots(1: ListBotsRequest req)
  ListConversationBotsResponse ListConversationBots(1: ListConversationBotsRequest req)
  AddConversationBotResponse AddConversationBot(1: AddConversationBotRequest req)
  CommonResponse RemoveConversationBot(1: RemoveConversationBotRequest req)
  ListAICallLogsResponse ListAICallLogs(1: ListAICallLogsRequest req)
  CreateMessageResponse CreateMessage(1: CreateMessageRequest req)
  FindSingleByUsersResponse FindSingleByUsers(1: FindSingleByUsersRequest req)
}
