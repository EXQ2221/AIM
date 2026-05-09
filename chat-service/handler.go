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
func (s *ChatServiceImpl) JoinGroup(ctx context.Context, req *chat.JoinGroupRequest) (resp *chat.CommonResponse, err error) {
	// TODO: Your code here...
	return
}

// LeaveGroup implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) LeaveGroup(ctx context.Context, req *chat.LeaveGroupRequest) (resp *chat.CommonResponse, err error) {
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

// CreateMessage implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) CreateMessage(ctx context.Context, req *chat.CreateMessageRequest) (resp *chat.CreateMessageResponse, err error) {
	// TODO: Your code here...
	return
}

// ListBots implements the ChatServiceImpl interface.
func (s *ChatServiceImpl) ListBots(ctx context.Context, req *chat.ListBotsRequest) (resp *chat.ListBotsResponse, err error) {
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
