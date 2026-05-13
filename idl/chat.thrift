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
  9: optional i64 announcement_updated_by
  10: optional i64 announcement_updated_at
}

struct CreateGroupResponse {
  1: GroupInfo group
}

struct GetGroupInfoRequest {
  1: i64 operator_id
  2: string conversation_id
}

struct GetGroupInfoResponse {
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
  14: optional bool mute_all
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
  13: optional i64 mute_until
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

struct MarkConversationReadRequest {
  1: i64 operator_id
  2: string conversation_id
  3: i64 last_read_message_id
}

struct RecallMessageRequest {
  1: i64 operator_id
  2: string conversation_id
  3: i64 message_id
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
  10: optional bool read_by_peer
  11: optional i32 read_count
  12: optional ReplyPreviewInfo reply_to
}

struct ReplyPreviewInfo {
  1: i64 message_id
  2: i64 sender_id
  3: string sender_type
  4: string message_type
  5: string content_preview
}

struct ConversationEventResponse {
  1: bool success
  2: string message
  3: optional MessageInfo event_message
  4: optional list<i64> recipient_user_ids
}

struct MessageRecalledEventInfo {
  1: i64 message_id
  2: string conversation_id
}

struct MessageRecalledEventResponse {
  1: bool success
  2: string message
  3: optional MessageRecalledEventInfo event
  4: optional list<i64> recipient_user_ids
}

struct TransferOwnerRequest {
  1: i64 operator_id
  2: string conversation_id
  3: i64 target_user_id
}

struct SetAdminRequest {
  1: i64 operator_id
  2: string conversation_id
  3: i64 target_user_id
}

struct RemoveAdminRequest {
  1: i64 operator_id
  2: string conversation_id
  3: i64 target_user_id
}

struct MuteMemberRequest {
  1: i64 operator_id
  2: string conversation_id
  3: i64 target_user_id
  4: i64 mute_until
}

struct UnmuteMemberRequest {
  1: i64 operator_id
  2: string conversation_id
  3: i64 target_user_id
}

struct RemoveMemberRequest {
  1: i64 operator_id
  2: string conversation_id
  3: i64 target_user_id
}

struct SetGroupMuteAllRequest {
  1: i64 operator_id
  2: string conversation_id
  3: bool mute_all
}

struct UpdateGroupAnnouncementRequest {
  1: i64 operator_id
  2: string conversation_id
  3: string announcement
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

struct KnowledgeBaseInfo {
  1: i64 knowledge_base_id
  2: string name
  3: string description
  4: string status
}

struct KnowledgeDocumentInfo {
  1: i64 document_id
  2: i64 knowledge_base_id
  3: string title
  4: string source_type
  5: string status
  6: string error_message
  7: i64 created_at
}

struct KnowledgeSearchChunkInfo {
  1: i64 chunk_id
  2: i64 document_id
  3: double score
  4: string content
}

struct CreateKnowledgeBaseRequest {
  1: i64 operator_id
  2: string name
  3: string description
}

struct CreateKnowledgeBaseResponse {
  1: KnowledgeBaseInfo knowledge_base
}

struct ListKnowledgeBasesRequest {
  1: i64 operator_id
}

struct ListKnowledgeBasesResponse {
  1: list<KnowledgeBaseInfo> knowledge_bases
}

struct AddKnowledgeDocumentTextRequest {
  1: i64 operator_id
  2: i64 knowledge_base_id
  3: string title
  4: string source_type
  5: string content
}

struct AddKnowledgeDocumentTextResponse {
  1: KnowledgeDocumentInfo document
}

struct ListKnowledgeDocumentsRequest {
  1: i64 operator_id
  2: i64 knowledge_base_id
}

struct ListKnowledgeDocumentsResponse {
  1: list<KnowledgeDocumentInfo> documents
}

struct SearchKnowledgeBaseRequest {
  1: i64 operator_id
  2: i64 knowledge_base_id
  3: string query
  4: optional i32 top_k
}

struct SearchKnowledgeBaseResponse {
  1: list<KnowledgeSearchChunkInfo> chunks
}

struct ConversationKnowledgeBaseInfo {
  1: i64 id
  2: string conversation_id
  3: i64 knowledge_base_id
  4: string name
  5: string description
  6: string status
  7: bool enabled
}

struct BindConversationKnowledgeBaseRequest {
  1: i64 operator_id
  2: string conversation_id
  3: i64 knowledge_base_id
}

struct ListConversationKnowledgeBasesRequest {
  1: i64 operator_id
  2: string conversation_id
}

struct ListConversationKnowledgeBasesResponse {
  1: list<ConversationKnowledgeBaseInfo> knowledge_bases
}

struct UnbindConversationKnowledgeBaseRequest {
  1: i64 operator_id
  2: string conversation_id
  3: i64 knowledge_base_id
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

struct CreateCustomBotRequest {
  1: i64 operator_id
  2: string name
  3: string mention_name
  4: list<string> aliases
  5: string description
  6: string api_base_url
  7: string api_key
  8: string model_name
  9: list<string> supported_models
  10: optional string system_prompt
}

struct CreateCustomBotResponse {
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
  5: optional string message_type
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
  GetGroupInfoResponse GetGroupInfo(1: GetGroupInfoRequest req)
  CreateSingleConversationResponse CreateSingleConversation(1: CreateSingleConversationRequest req)
  ListConversationsResponse ListConversations(1: ListConversationsRequest req)
  ConversationEventResponse JoinGroup(1: JoinGroupRequest req)
  ConversationEventResponse InviteMember(1: InviteMemberRequest req)
  ConversationEventResponse LeaveGroup(1: LeaveGroupRequest req)
  ConversationEventResponse TransferOwner(1: TransferOwnerRequest req)
  ConversationEventResponse SetAdmin(1: SetAdminRequest req)
  ConversationEventResponse RemoveAdmin(1: RemoveAdminRequest req)
  ConversationEventResponse MuteMember(1: MuteMemberRequest req)
  ConversationEventResponse UnmuteMember(1: UnmuteMemberRequest req)
  ConversationEventResponse RemoveMember(1: RemoveMemberRequest req)
  ConversationEventResponse SetGroupMuteAll(1: SetGroupMuteAllRequest req)
  ConversationEventResponse UpdateGroupAnnouncement(1: UpdateGroupAnnouncementRequest req)
  ListMembersResponse ListMembers(1: ListMembersRequest req)
  ListMessagesResponse ListMessages(1: ListMessagesRequest req)
  CommonResponse MarkConversationRead(1: MarkConversationReadRequest req)
  MessageRecalledEventResponse RecallMessage(1: RecallMessageRequest req)
  ListBotsResponse ListBots(1: ListBotsRequest req)
  CreateCustomBotResponse CreateCustomBot(1: CreateCustomBotRequest req)
  ListConversationBotsResponse ListConversationBots(1: ListConversationBotsRequest req)
  AddConversationBotResponse AddConversationBot(1: AddConversationBotRequest req)
  CommonResponse RemoveConversationBot(1: RemoveConversationBotRequest req)
  ListAICallLogsResponse ListAICallLogs(1: ListAICallLogsRequest req)
  CreateKnowledgeBaseResponse CreateKnowledgeBase(1: CreateKnowledgeBaseRequest req)
  ListKnowledgeBasesResponse ListKnowledgeBases(1: ListKnowledgeBasesRequest req)
  AddKnowledgeDocumentTextResponse AddKnowledgeDocumentText(1: AddKnowledgeDocumentTextRequest req)
  ListKnowledgeDocumentsResponse ListKnowledgeDocuments(1: ListKnowledgeDocumentsRequest req)
  SearchKnowledgeBaseResponse SearchKnowledgeBase(1: SearchKnowledgeBaseRequest req)
  CommonResponse BindConversationKnowledgeBase(1: BindConversationKnowledgeBaseRequest req)
  ListConversationKnowledgeBasesResponse ListConversationKnowledgeBases(1: ListConversationKnowledgeBasesRequest req)
  CommonResponse UnbindConversationKnowledgeBase(1: UnbindConversationKnowledgeBaseRequest req)
  CreateMessageResponse CreateMessage(1: CreateMessageRequest req)
  FindSingleByUsersResponse FindSingleByUsers(1: FindSingleByUsersRequest req)
}
