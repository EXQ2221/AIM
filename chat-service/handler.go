package main

import (
	"context"
	chat "example.com/aim/chat-service/kitex_gen/chat"
)

// ChatServiceImpl implements the last service interface defined in the IDL.
type ChatServiceImpl struct{}

// Health implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) Health(ctx context.Context, req *chat.HealthRequest) (resp *chat.HealthResponse, err error) {
	// TODO: Your code here...
	return
}

// CreateGroup implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) CreateGroup(ctx context.Context, req *chat.CreateGroupRequest) (resp *chat.CreateGroupResponse, err error) {
	// TODO: Your code here...
	return
}

// GetGroupInfo implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) GetGroupInfo(ctx context.Context, req *chat.GetGroupInfoRequest) (resp *chat.GetGroupInfoResponse, err error) {
	// TODO: Your code here...
	return
}

// CreateSingleConversation implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) CreateSingleConversation(ctx context.Context, req *chat.CreateSingleConversationRequest) (resp *chat.CreateSingleConversationResponse, err error) {
	// TODO: Your code here...
	return
}

// ListConversations implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) ListConversations(ctx context.Context, req *chat.ListConversationsRequest) (resp *chat.ListConversationsResponse, err error) {
	// TODO: Your code here...
	return
}

// JoinGroup implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) JoinGroup(ctx context.Context, req *chat.JoinGroupRequest) (resp *chat.ConversationEventResponse, err error) {
	// TODO: Your code here...
	return
}

// InviteMember implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) InviteMember(ctx context.Context, req *chat.InviteMemberRequest) (resp *chat.ConversationEventResponse, err error) {
	// TODO: Your code here...
	return
}

// LeaveGroup implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) LeaveGroup(ctx context.Context, req *chat.LeaveGroupRequest) (resp *chat.ConversationEventResponse, err error) {
	// TODO: Your code here...
	return
}

// TransferOwner implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) TransferOwner(ctx context.Context, req *chat.TransferOwnerRequest) (resp *chat.ConversationEventResponse, err error) {
	// TODO: Your code here...
	return
}

// SetAdmin implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) SetAdmin(ctx context.Context, req *chat.SetAdminRequest) (resp *chat.ConversationEventResponse, err error) {
	// TODO: Your code here...
	return
}

// RemoveAdmin implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) RemoveAdmin(ctx context.Context, req *chat.RemoveAdminRequest) (resp *chat.ConversationEventResponse, err error) {
	// TODO: Your code here...
	return
}

// MuteMember implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) MuteMember(ctx context.Context, req *chat.MuteMemberRequest) (resp *chat.ConversationEventResponse, err error) {
	// TODO: Your code here...
	return
}

// UnmuteMember implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) UnmuteMember(ctx context.Context, req *chat.UnmuteMemberRequest) (resp *chat.ConversationEventResponse, err error) {
	// TODO: Your code here...
	return
}

// RemoveMember implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) RemoveMember(ctx context.Context, req *chat.RemoveMemberRequest) (resp *chat.ConversationEventResponse, err error) {
	// TODO: Your code here...
	return
}

// SetGroupMuteAll implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) SetGroupMuteAll(ctx context.Context, req *chat.SetGroupMuteAllRequest) (resp *chat.ConversationEventResponse, err error) {
	// TODO: Your code here...
	return
}

// UpdateGroupAnnouncement implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) UpdateGroupAnnouncement(ctx context.Context, req *chat.UpdateGroupAnnouncementRequest) (resp *chat.ConversationEventResponse, err error) {
	// TODO: Your code here...
	return
}

// ListMembers implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) ListMembers(ctx context.Context, req *chat.ListMembersRequest) (resp *chat.ListMembersResponse, err error) {
	// TODO: Your code here...
	return
}

// ListMessages implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) ListMessages(ctx context.Context, req *chat.ListMessagesRequest) (resp *chat.ListMessagesResponse, err error) {
	// TODO: Your code here...
	return
}

// MarkConversationRead implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) MarkConversationRead(ctx context.Context, req *chat.MarkConversationReadRequest) (resp *chat.CommonResponse, err error) {
	// TODO: Your code here...
	return
}

// RecallMessage implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) RecallMessage(ctx context.Context, req *chat.RecallMessageRequest) (resp *chat.MessageRecalledEventResponse, err error) {
	// TODO: Your code here...
	return
}

// ListBots implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) ListBots(ctx context.Context, req *chat.ListBotsRequest) (resp *chat.ListBotsResponse, err error) {
	// TODO: Your code here...
	return
}

// CreateCustomBot implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) CreateCustomBot(ctx context.Context, req *chat.CreateCustomBotRequest) (resp *chat.CreateCustomBotResponse, err error) {
	// TODO: Your code here...
	return
}

// ListConversationBots implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) ListConversationBots(ctx context.Context, req *chat.ListConversationBotsRequest) (resp *chat.ListConversationBotsResponse, err error) {
	// TODO: Your code here...
	return
}

// AddConversationBot implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) AddConversationBot(ctx context.Context, req *chat.AddConversationBotRequest) (resp *chat.AddConversationBotResponse, err error) {
	// TODO: Your code here...
	return
}

// RemoveConversationBot implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) RemoveConversationBot(ctx context.Context, req *chat.RemoveConversationBotRequest) (resp *chat.CommonResponse, err error) {
	// TODO: Your code here...
	return
}

// ListAICallLogs implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) ListAICallLogs(ctx context.Context, req *chat.ListAICallLogsRequest) (resp *chat.ListAICallLogsResponse, err error) {
	// TODO: Your code here...
	return
}

// CreateMessage implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) CreateMessage(ctx context.Context, req *chat.CreateMessageRequest) (resp *chat.CreateMessageResponse, err error) {
	// TODO: Your code here...
	return
}

// FindSingleByUsers implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) FindSingleByUsers(ctx context.Context, req *chat.FindSingleByUsersRequest) (resp *chat.FindSingleByUsersResponse, err error) {
	// TODO: Your code here...
	return
}

// CreateKnowledgeBase implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) CreateKnowledgeBase(ctx context.Context, req *chat.CreateKnowledgeBaseRequest) (resp *chat.CreateKnowledgeBaseResponse, err error) {
	// TODO: Your code here...
	return
}

// AddKnowledgeDocumentText implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) AddKnowledgeDocumentText(ctx context.Context, req *chat.AddKnowledgeDocumentTextRequest) (resp *chat.AddKnowledgeDocumentTextResponse, err error) {
	// TODO: Your code here...
	return
}

// ListKnowledgeDocuments implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) ListKnowledgeDocuments(ctx context.Context, req *chat.ListKnowledgeDocumentsRequest) (resp *chat.ListKnowledgeDocumentsResponse, err error) {
	// TODO: Your code here...
	return
}

// SearchKnowledgeBase implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) SearchKnowledgeBase(ctx context.Context, req *chat.SearchKnowledgeBaseRequest) (resp *chat.SearchKnowledgeBaseResponse, err error) {
	// TODO: Your code here...
	return
}

// BindConversationKnowledgeBase implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) BindConversationKnowledgeBase(ctx context.Context, req *chat.BindConversationKnowledgeBaseRequest) (resp *chat.CommonResponse, err error) {
	// TODO: Your code here...
	return
}

// ListConversationKnowledgeBases implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) ListConversationKnowledgeBases(ctx context.Context, req *chat.ListConversationKnowledgeBasesRequest) (resp *chat.ListConversationKnowledgeBasesResponse, err error) {
	// TODO: Your code here...
	return
}

// UnbindConversationKnowledgeBase implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) UnbindConversationKnowledgeBase(ctx context.Context, req *chat.UnbindConversationKnowledgeBaseRequest) (resp *chat.CommonResponse, err error) {
	// TODO: Your code here...
	return
}

// ListKnowledgeBases implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) ListKnowledgeBases(ctx context.Context, req *chat.ListKnowledgeBasesRequest) (resp *chat.ListKnowledgeBasesResponse, err error) {
	// TODO: Your code here...
	return
}
