package handler

import (
	"context"

	"example.com/aim/chat-service/internal/biz"
	chatpb "example.com/aim/chat-service/kitex_gen/chat"
)

type ChatServiceImpl struct {
	Service *biz.ChatService
}

func NewChatServiceImpl(service *biz.ChatService) *ChatServiceImpl {
	return &ChatServiceImpl{Service: service}
}

func (h *ChatServiceImpl) Health(ctx context.Context, req *chatpb.HealthRequest) (*chatpb.HealthResponse, error) {
	return &chatpb.HealthResponse{Ok: true}, nil
}

func (h *ChatServiceImpl) CreateGroup(ctx context.Context, req *chatpb.CreateGroupRequest) (*chatpb.CreateGroupResponse, error) {
	group, err := h.Service.CreateGroup(ctx, biz.CreateGroupInput{
		OperatorID:   uint64(req.OperatorId),
		Name:         req.Name,
		Avatar:       req.Avatar,
		Announcement: req.Announcement,
		JoinPolicy:   req.JoinPolicy,
	})
	if err != nil {
		return nil, err
	}

	return &chatpb.CreateGroupResponse{Group: toGroupPB(group)}, nil
}

func (h *ChatServiceImpl) GetGroupInfo(ctx context.Context, req *chatpb.GetGroupInfoRequest) (*chatpb.GetGroupInfoResponse, error) {
	group, err := h.Service.GetGroupInfo(ctx, uint64(req.OperatorId), req.ConversationId)
	if err != nil {
		return nil, err
	}
	return &chatpb.GetGroupInfoResponse{Group: toGroupPB(group)}, nil
}

func (h *ChatServiceImpl) CreateSingleConversation(ctx context.Context, req *chatpb.CreateSingleConversationRequest) (*chatpb.CreateSingleConversationResponse, error) {
	conversation, err := h.Service.CreateSingleConversation(ctx, biz.CreateSingleConversationInput{
		OperatorID: uint64(req.OperatorId),
		TargetID:   uint64(req.TargetUserId),
	})
	if err != nil {
		return nil, err
	}
	return &chatpb.CreateSingleConversationResponse{Conversation: toConversationPB(*conversation)}, nil
}

func (h *ChatServiceImpl) FindSingleByUsers(ctx context.Context, req *chatpb.FindSingleByUsersRequest) (*chatpb.FindSingleByUsersResponse, error) {
	view, err := h.Service.FindSingleConversationByUsers(ctx, uint64(req.OperatorId), uint64(req.TargetUserId))
	if err != nil {
		return nil, err
	}
	if view == nil {
		return &chatpb.FindSingleByUsersResponse{Conversation: nil}, nil
	}
	pb := toConversationPB(*view)
	return &chatpb.FindSingleByUsersResponse{Conversation: pb}, nil
}

func (h *ChatServiceImpl) ListConversations(ctx context.Context, req *chatpb.ListConversationsRequest) (*chatpb.ListConversationsResponse, error) {
	conversations, err := h.Service.ListConversations(ctx, uint64(req.UserId))
	if err != nil {
		return nil, err
	}

	result := make([]*chatpb.ConversationInfo, 0, len(conversations))
	for _, conversation := range conversations {
		result = append(result, toConversationPB(conversation))
	}
	return &chatpb.ListConversationsResponse{Conversations: result}, nil
}

func (h *ChatServiceImpl) JoinGroup(ctx context.Context, req *chatpb.JoinGroupRequest) (*chatpb.ConversationEventResponse, error) {
	event, err := h.Service.JoinGroup(ctx, uint64(req.OperatorId), req.ConversationId)
	if err != nil {
		return &chatpb.ConversationEventResponse{Success: false, Message: err.Error()}, nil
	}
	return toConversationEventPB(event), nil
}

func (h *ChatServiceImpl) InviteMember(ctx context.Context, req *chatpb.InviteMemberRequest) (*chatpb.ConversationEventResponse, error) {
	event, err := h.Service.InviteMember(ctx, biz.InviteMemberInput{
		OperatorID:   uint64(req.OperatorId),
		TargetUserID: uint64(req.TargetUserId),
	}, req.ConversationId)
	if err != nil {
		return &chatpb.ConversationEventResponse{Success: false, Message: err.Error()}, nil
	}
	return toConversationEventPB(event), nil
}

func (h *ChatServiceImpl) LeaveGroup(ctx context.Context, req *chatpb.LeaveGroupRequest) (*chatpb.ConversationEventResponse, error) {
	event, err := h.Service.LeaveGroup(ctx, uint64(req.OperatorId), req.ConversationId)
	if err != nil {
		return &chatpb.ConversationEventResponse{Success: false, Message: err.Error()}, nil
	}
	return toConversationEventPB(event), nil
}

func (h *ChatServiceImpl) TransferOwner(ctx context.Context, req *chatpb.TransferOwnerRequest) (*chatpb.ConversationEventResponse, error) {
	event, err := h.Service.TransferOwner(ctx, biz.TransferOwnerInput{
		OperatorID:     uint64(req.OperatorId),
		ConversationID: req.ConversationId,
		TargetUserID:   uint64(req.TargetUserId),
	})
	if err != nil {
		return &chatpb.ConversationEventResponse{Success: false, Message: err.Error()}, nil
	}
	return toConversationEventPB(event), nil
}

func (h *ChatServiceImpl) SetAdmin(ctx context.Context, req *chatpb.SetAdminRequest) (*chatpb.ConversationEventResponse, error) {
	event, err := h.Service.SetAdmin(ctx, biz.SetAdminInput{
		OperatorID:     uint64(req.OperatorId),
		ConversationID: req.ConversationId,
		TargetUserID:   uint64(req.TargetUserId),
	})
	if err != nil {
		return &chatpb.ConversationEventResponse{Success: false, Message: err.Error()}, nil
	}
	return toConversationEventPB(event), nil
}

func (h *ChatServiceImpl) RemoveAdmin(ctx context.Context, req *chatpb.RemoveAdminRequest) (*chatpb.ConversationEventResponse, error) {
	event, err := h.Service.RemoveAdmin(ctx, biz.RemoveAdminInput{
		OperatorID:     uint64(req.OperatorId),
		ConversationID: req.ConversationId,
		TargetUserID:   uint64(req.TargetUserId),
	})
	if err != nil {
		return &chatpb.ConversationEventResponse{Success: false, Message: err.Error()}, nil
	}
	return toConversationEventPB(event), nil
}

func (h *ChatServiceImpl) MuteMember(ctx context.Context, req *chatpb.MuteMemberRequest) (*chatpb.ConversationEventResponse, error) {
	event, err := h.Service.MuteMember(ctx, biz.MuteMemberInput{
		OperatorID:     uint64(req.OperatorId),
		ConversationID: req.ConversationId,
		TargetUserID:   uint64(req.TargetUserId),
		MuteUntil:      req.MuteUntil,
	})
	if err != nil {
		return &chatpb.ConversationEventResponse{Success: false, Message: err.Error()}, nil
	}
	return toConversationEventPB(event), nil
}

func (h *ChatServiceImpl) UnmuteMember(ctx context.Context, req *chatpb.UnmuteMemberRequest) (*chatpb.ConversationEventResponse, error) {
	event, err := h.Service.UnmuteMember(ctx, biz.UnmuteMemberInput{
		OperatorID:     uint64(req.OperatorId),
		ConversationID: req.ConversationId,
		TargetUserID:   uint64(req.TargetUserId),
	})
	if err != nil {
		return &chatpb.ConversationEventResponse{Success: false, Message: err.Error()}, nil
	}
	return toConversationEventPB(event), nil
}

func (h *ChatServiceImpl) RemoveMember(ctx context.Context, req *chatpb.RemoveMemberRequest) (*chatpb.ConversationEventResponse, error) {
	event, err := h.Service.RemoveMember(ctx, biz.RemoveMemberInput{
		OperatorID:     uint64(req.OperatorId),
		ConversationID: req.ConversationId,
		TargetUserID:   uint64(req.TargetUserId),
	})
	if err != nil {
		return &chatpb.ConversationEventResponse{Success: false, Message: err.Error()}, nil
	}
	return toConversationEventPB(event), nil
}

func (h *ChatServiceImpl) SetGroupMuteAll(ctx context.Context, req *chatpb.SetGroupMuteAllRequest) (*chatpb.ConversationEventResponse, error) {
	event, err := h.Service.SetGroupMuteAll(ctx, biz.SetGroupMuteAllInput{
		OperatorID:     uint64(req.OperatorId),
		ConversationID: req.ConversationId,
		MuteAll:        req.MuteAll,
	})
	if err != nil {
		return &chatpb.ConversationEventResponse{Success: false, Message: err.Error()}, nil
	}
	return toConversationEventPB(event), nil
}

func (h *ChatServiceImpl) UpdateGroupAnnouncement(ctx context.Context, req *chatpb.UpdateGroupAnnouncementRequest) (*chatpb.ConversationEventResponse, error) {
	event, err := h.Service.UpdateGroupAnnouncement(ctx, biz.UpdateGroupAnnouncementInput{
		OperatorID:     uint64(req.OperatorId),
		ConversationID: req.ConversationId,
		Announcement:   req.Announcement,
	})
	if err != nil {
		return &chatpb.ConversationEventResponse{Success: false, Message: err.Error()}, nil
	}
	return toConversationEventPB(event), nil
}

func (h *ChatServiceImpl) ListMembers(ctx context.Context, req *chatpb.ListMembersRequest) (*chatpb.ListMembersResponse, error) {
	members, err := h.Service.ListMembers(ctx, uint64(req.OperatorId), req.ConversationId)
	if err != nil {
		return nil, err
	}

	result := make([]*chatpb.MemberInfo, 0, len(members))
	for _, member := range members {
		result = append(result, toMemberPB(member))
	}
	return &chatpb.ListMembersResponse{Members: result}, nil
}

func (h *ChatServiceImpl) ListMessages(ctx context.Context, req *chatpb.ListMessagesRequest) (*chatpb.ListMessagesResponse, error) {
	var beforeID *uint64
	if req.BeforeId != nil {
		value := uint64(*req.BeforeId)
		beforeID = &value
	}
	messages, err := h.Service.ListMessages(ctx, uint64(req.OperatorId), req.ConversationId, beforeID, int(req.Limit))
	if err != nil {
		return nil, err
	}

	result := make([]*chatpb.MessageInfo, 0, len(messages))
	for _, message := range messages {
		result = append(result, toMessagePB(message))
	}
	return &chatpb.ListMessagesResponse{Messages: result}, nil
}

func (h *ChatServiceImpl) MarkConversationRead(ctx context.Context, req *chatpb.MarkConversationReadRequest) (*chatpb.CommonResponse, error) {
	if err := h.Service.MarkConversationRead(ctx, biz.MarkConversationReadInput{
		OperatorID:        uint64(req.OperatorId),
		ConversationID:    req.ConversationId,
		LastReadMessageID: uint64(req.LastReadMessageId),
	}); err != nil {
		return &chatpb.CommonResponse{Success: false, Message: err.Error()}, nil
	}
	return &chatpb.CommonResponse{Success: true, Message: "ok"}, nil
}

func (h *ChatServiceImpl) RecallMessage(ctx context.Context, req *chatpb.RecallMessageRequest) (*chatpb.MessageRecalledEventResponse, error) {
	event, err := h.Service.RecallMessage(ctx, biz.RecallMessageInput{
		OperatorID:     uint64(req.OperatorId),
		ConversationID: req.ConversationId,
		MessageID:      uint64(req.MessageId),
	})
	if err != nil {
		return &chatpb.MessageRecalledEventResponse{Success: false, Message: err.Error()}, nil
	}
	return toMessageRecalledEventPB(event), nil
}

func (h *ChatServiceImpl) ListBots(ctx context.Context, req *chatpb.ListBotsRequest) (*chatpb.ListBotsResponse, error) {
	bots, err := h.Service.ListBots(ctx, uint64(req.OperatorId))
	if err != nil {
		return nil, err
	}

	result := make([]*chatpb.BotInfo, 0, len(bots))
	for _, item := range bots {
		result = append(result, toBotPB(item))
	}
	return &chatpb.ListBotsResponse{Bots: result}, nil
}

func (h *ChatServiceImpl) CreateCustomBot(ctx context.Context, req *chatpb.CreateCustomBotRequest) (*chatpb.CreateCustomBotResponse, error) {
	item, err := h.Service.CreateCustomBot(ctx, biz.CreateCustomBotInput{
		OperatorID:      uint64(req.OperatorId),
		Name:            req.Name,
		MentionName:     req.MentionName,
		Aliases:         req.Aliases,
		Description:     req.Description,
		APIBaseURL:      req.ApiBaseUrl,
		APIKey:          req.ApiKey,
		ModelName:       req.ModelName,
		SupportedModels: req.SupportedModels,
		SystemPrompt:    req.GetSystemPrompt(),
	})
	if err != nil {
		return nil, err
	}
	return &chatpb.CreateCustomBotResponse{Bot: toBotPB(*item)}, nil
}

func (h *ChatServiceImpl) ListConversationBots(ctx context.Context, req *chatpb.ListConversationBotsRequest) (*chatpb.ListConversationBotsResponse, error) {
	bots, err := h.Service.ListConversationBots(ctx, uint64(req.OperatorId), req.ConversationId)
	if err != nil {
		return nil, err
	}

	result := make([]*chatpb.BotInfo, 0, len(bots))
	for _, item := range bots {
		result = append(result, toBotPB(item))
	}
	return &chatpb.ListConversationBotsResponse{Bots: result}, nil
}

func (h *ChatServiceImpl) AddConversationBot(ctx context.Context, req *chatpb.AddConversationBotRequest) (*chatpb.AddConversationBotResponse, error) {
	item, err := h.Service.AddConversationBot(ctx, biz.AddConversationBotInput{
		OperatorID:          uint64(req.OperatorId),
		ConversationID:      req.ConversationId,
		BotID:               uint64(req.BotId),
		DisplayNameOverride: req.DisplayNameOverride,
		MentionNameOverride: req.MentionNameOverride,
		AliasesOverride:     req.AliasesOverride,
		PermissionScope:     req.PermissionScope,
		ModelNameOverride:   req.ModelNameOverride,
	})
	if err != nil {
		return nil, err
	}
	return &chatpb.AddConversationBotResponse{Bot: toBotPB(*item)}, nil
}

func (h *ChatServiceImpl) RemoveConversationBot(ctx context.Context, req *chatpb.RemoveConversationBotRequest) (*chatpb.CommonResponse, error) {
	if err := h.Service.RemoveConversationBot(ctx, uint64(req.OperatorId), req.ConversationId, uint64(req.BotId)); err != nil {
		return &chatpb.CommonResponse{Success: false, Message: err.Error()}, nil
	}
	return &chatpb.CommonResponse{Success: true, Message: "ok"}, nil
}

func (h *ChatServiceImpl) ListAICallLogs(ctx context.Context, req *chatpb.ListAICallLogsRequest) (*chatpb.ListAICallLogsResponse, error) {
	var beforeID *uint64
	if req.BeforeId != nil {
		value := uint64(*req.BeforeId)
		beforeID = &value
	}
	var botID *uint64
	if req.BotId != nil {
		value := uint64(*req.BotId)
		botID = &value
	}
	status := ""
	if req.Status != nil {
		status = *req.Status
	}

	result, err := h.Service.ListAICallLogs(
		ctx,
		uint64(req.OperatorId),
		req.ConversationId,
		beforeID,
		int(req.Limit),
		botID,
		status,
	)
	if err != nil {
		return nil, err
	}

	logs := make([]*chatpb.AICallLogInfo, 0, len(result.Logs))
	for _, item := range result.Logs {
		logs = append(logs, toAICallLogPB(item))
	}
	return &chatpb.ListAICallLogsResponse{
		Logs: logs,
		Quota: &chatpb.AICallLogQuotaInfo{
			DailyTotalTokens: result.Quota.DailyTotalTokens,
			DailyTokenLimit:  result.Quota.DailyTokenLimit,
			RemainingTokens:  result.Quota.RemainingTokens,
		},
	}, nil
}

func (h *ChatServiceImpl) CreateKnowledgeBase(ctx context.Context, req *chatpb.CreateKnowledgeBaseRequest) (*chatpb.CreateKnowledgeBaseResponse, error) {
	item, err := h.Service.CreateKnowledgeBase(ctx, biz.CreateKnowledgeBaseInput{
		OperatorID:  uint64(req.OperatorId),
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		return nil, err
	}
	return &chatpb.CreateKnowledgeBaseResponse{
		KnowledgeBase: &chatpb.KnowledgeBaseInfo{
			KnowledgeBaseId: int64(item.KnowledgeBaseID),
			Name:            item.Name,
			Description:     item.Description,
			Status:          item.Status,
		},
	}, nil
}

func (h *ChatServiceImpl) AddKnowledgeDocumentText(ctx context.Context, req *chatpb.AddKnowledgeDocumentTextRequest) (*chatpb.AddKnowledgeDocumentTextResponse, error) {
	item, err := h.Service.AddKnowledgeDocumentText(ctx, biz.AddKnowledgeDocumentTextInput{
		OperatorID:      uint64(req.OperatorId),
		KnowledgeBaseID: uint64(req.KnowledgeBaseId),
		Title:           req.Title,
		SourceType:      req.SourceType,
		Content:         req.Content,
	})
	if err != nil {
		return nil, err
	}
	return &chatpb.AddKnowledgeDocumentTextResponse{
		Document: &chatpb.KnowledgeDocumentInfo{
			DocumentId:      int64(item.DocumentID),
			KnowledgeBaseId: int64(item.KnowledgeBaseID),
			Title:           item.Title,
			SourceType:      item.SourceType,
			Status:          item.Status,
			ErrorMessage:    item.ErrorMessage,
			CreatedAt:       item.CreatedAt,
		},
	}, nil
}

func (h *ChatServiceImpl) ListKnowledgeDocuments(ctx context.Context, req *chatpb.ListKnowledgeDocumentsRequest) (*chatpb.ListKnowledgeDocumentsResponse, error) {
	items, err := h.Service.ListKnowledgeDocuments(ctx, biz.ListKnowledgeDocumentsInput{
		OperatorID:      uint64(req.OperatorId),
		KnowledgeBaseID: uint64(req.KnowledgeBaseId),
	})
	if err != nil {
		return nil, err
	}
	docs := make([]*chatpb.KnowledgeDocumentInfo, 0, len(items))
	for _, item := range items {
		docs = append(docs, &chatpb.KnowledgeDocumentInfo{
			DocumentId:      int64(item.DocumentID),
			KnowledgeBaseId: int64(item.KnowledgeBaseID),
			Title:           item.Title,
			SourceType:      item.SourceType,
			Status:          item.Status,
			ErrorMessage:    item.ErrorMessage,
			CreatedAt:       item.CreatedAt,
		})
	}
	return &chatpb.ListKnowledgeDocumentsResponse{Documents: docs}, nil
}

func (h *ChatServiceImpl) SearchKnowledgeBase(ctx context.Context, req *chatpb.SearchKnowledgeBaseRequest) (*chatpb.SearchKnowledgeBaseResponse, error) {
	items, err := h.Service.SearchKnowledgeBase(ctx, biz.SearchKnowledgeBaseInput{
		OperatorID:      uint64(req.OperatorId),
		KnowledgeBaseID: uint64(req.KnowledgeBaseId),
		Query:           req.Query,
		TopK:            req.TopK,
	})
	if err != nil {
		return nil, err
	}
	chunks := make([]*chatpb.KnowledgeSearchChunkInfo, 0, len(items))
	for _, item := range items {
		chunks = append(chunks, &chatpb.KnowledgeSearchChunkInfo{
			ChunkId:    int64(item.ChunkID),
			DocumentId: int64(item.DocumentID),
			Score:      item.Score,
			Content:    item.Content,
		})
	}
	return &chatpb.SearchKnowledgeBaseResponse{Chunks: chunks}, nil
}

func (h *ChatServiceImpl) BindConversationKnowledgeBase(ctx context.Context, req *chatpb.BindConversationKnowledgeBaseRequest) (*chatpb.CommonResponse, error) {
	if err := h.Service.BindConversationKnowledgeBase(ctx, biz.BindConversationKnowledgeBaseInput{
		OperatorID:      uint64(req.OperatorId),
		ConversationID:  req.ConversationId,
		KnowledgeBaseID: uint64(req.KnowledgeBaseId),
	}); err != nil {
		return &chatpb.CommonResponse{Success: false, Message: err.Error()}, nil
	}
	return &chatpb.CommonResponse{Success: true, Message: "ok"}, nil
}

func (h *ChatServiceImpl) ListConversationKnowledgeBases(ctx context.Context, req *chatpb.ListConversationKnowledgeBasesRequest) (*chatpb.ListConversationKnowledgeBasesResponse, error) {
	items, err := h.Service.ListConversationKnowledgeBases(ctx, uint64(req.OperatorId), req.ConversationId)
	if err != nil {
		return nil, err
	}
	result := make([]*chatpb.ConversationKnowledgeBaseInfo, 0, len(items))
	for _, item := range items {
		result = append(result, &chatpb.ConversationKnowledgeBaseInfo{
			Id:              int64(item.ID),
			ConversationId:  item.ConversationID,
			KnowledgeBaseId: int64(item.KnowledgeBaseID),
			Name:            item.Name,
			Description:     item.Description,
			Status:          item.Status,
			Enabled:         item.Enabled,
		})
	}
	return &chatpb.ListConversationKnowledgeBasesResponse{KnowledgeBases: result}, nil
}

func (h *ChatServiceImpl) UnbindConversationKnowledgeBase(ctx context.Context, req *chatpb.UnbindConversationKnowledgeBaseRequest) (*chatpb.CommonResponse, error) {
	if err := h.Service.UnbindConversationKnowledgeBase(ctx, biz.UnbindConversationKnowledgeBaseInput{
		OperatorID:      uint64(req.OperatorId),
		ConversationID:  req.ConversationId,
		KnowledgeBaseID: uint64(req.KnowledgeBaseId),
	}); err != nil {
		return &chatpb.CommonResponse{Success: false, Message: err.Error()}, nil
	}
	return &chatpb.CommonResponse{Success: true, Message: "ok"}, nil
}

func (h *ChatServiceImpl) CreateMessage(ctx context.Context, req *chatpb.CreateMessageRequest) (*chatpb.CreateMessageResponse, error) {
	var replyToID *uint64
	if req.ReplyToId != nil {
		value := uint64(*req.ReplyToId)
		replyToID = &value
	}
	message, err := h.Service.CreateMessage(ctx, uint64(req.OperatorId), req.ConversationId, req.Content, replyToID, req.GetMessageType())
	if err != nil {
		return nil, err
	}
	return &chatpb.CreateMessageResponse{Message: toMessagePB(*message)}, nil
}

func toGroupPB(group *biz.GroupView) *chatpb.GroupInfo {
	if group == nil {
		return nil
	}
	pb := &chatpb.GroupInfo{
		ConversationId: group.ConversationID,
		Type:           group.Type,
		Name:           group.Name,
		Avatar:         group.Avatar,
		Announcement:   group.Announcement,
		OwnerId:        int64(group.OwnerID),
		JoinPolicy:     group.JoinPolicy,
		CreatedAt:      group.CreatedAt,
	}
	if group.AnnouncementUpdatedBy != nil {
		value := int64(*group.AnnouncementUpdatedBy)
		pb.AnnouncementUpdatedBy = &value
	}
	if group.AnnouncementUpdatedAt != nil {
		value := *group.AnnouncementUpdatedAt
		pb.AnnouncementUpdatedAt = &value
	}
	return pb
}

func toBotPB(item biz.BotView) *chatpb.BotInfo {
	return &chatpb.BotInfo{
		BotId:           int64(item.BotID),
		MemberType:      item.MemberType,
		MemberId:        int64(item.MemberID),
		Name:            item.Name,
		DisplayName:     item.DisplayName,
		MentionName:     item.MentionName,
		Aliases:         item.Aliases,
		Avatar:          item.Avatar,
		Description:     item.Description,
		Enabled:         item.Enabled,
		PermissionScope: item.PermissionScope,
		MemberStatus:    item.MemberStatus,
		ModelName:       item.ModelName,
		SupportedModels: item.SupportedModels,
	}
}

func toAICallLogPB(item biz.AICallLogView) *chatpb.AICallLogInfo {
	pb := &chatpb.AICallLogInfo{
		Id:               int64(item.ID),
		ConversationId:   item.ConversationID,
		UserId:           int64(item.UserID),
		BotId:            int64(item.BotID),
		BotName:          item.BotName,
		ModelName:        item.ModelName,
		PromptTokens:     int32(item.PromptTokens),
		CompletionTokens: int32(item.CompletionTokens),
		TotalTokens:      int32(item.TotalTokens),
		LatencyMs:        item.LatencyMS,
		Status:           item.Status,
		ErrorMessage:     item.ErrorMessage,
		CreatedAt:        item.CreatedAt,
	}
	if item.RequestMessageID != nil {
		value := int64(*item.RequestMessageID)
		pb.RequestMessageId = &value
	}
	if item.ResponseMessageID != nil {
		value := int64(*item.ResponseMessageID)
		pb.ResponseMessageId = &value
	}
	return pb
}

func toConversationPB(conversation biz.ConversationView) *chatpb.ConversationInfo {
	var lastMessageID *int64
	if conversation.LastMessageID != nil {
		value := int64(*conversation.LastMessageID)
		lastMessageID = &value
	}
	var lastMessageSenderID *int64
	if conversation.LastMessageSenderID != nil {
		value := int64(*conversation.LastMessageSenderID)
		lastMessageSenderID = &value
	}
	return &chatpb.ConversationInfo{
		ConversationId:        conversation.ConversationID,
		Type:                  conversation.Type,
		Title:                 conversation.Title,
		Avatar:                conversation.Avatar,
		LastMessageId:         lastMessageID,
		LastMessageAt:         conversation.LastMessageAt,
		Role:                  conversation.Role,
		IsPinned:              conversation.IsPinned,
		IsMuted:               conversation.IsMuted,
		UpdatedAt:             conversation.UpdatedAt,
		LastMessageSenderId:   lastMessageSenderID,
		LastMessageSenderName: conversation.LastMessageSenderName,
		LastMessageContent:    conversation.LastMessageContent,
		MuteAll:               conversation.MuteAll,
	}
}

func toMemberPB(member biz.MemberListView) *chatpb.MemberInfo {
	pb := &chatpb.MemberInfo{
		UserId:     int64(member.UserID),
		Nickname:   member.Nickname,
		Avatar:     member.Avatar,
		Role:       member.Role,
		Status:     member.Status,
		JoinedAt:   member.JoinedAt,
		MemberType: member.MemberType,
	}
	if member.BotID > 0 {
		pb.BotId = &[]int64{int64(member.BotID)}[0]
	}
	if member.MentionName != "" {
		pb.MentionName = &member.MentionName
	}
	if len(member.Aliases) > 0 {
		pb.Aliases = member.Aliases
	}
	if member.Enabled != nil {
		val := *member.Enabled
		pb.Enabled = &val
	}
	if member.PermissionScope != "" {
		pb.PermissionScope = &member.PermissionScope
	}
	if member.MuteUntil != nil {
		value := *member.MuteUntil
		pb.MuteUntil = &value
	}
	return pb
}

func toMessagePB(message biz.MessageView) *chatpb.MessageInfo {
	var replyToID *int64
	if message.ReplyToID != nil {
		value := int64(*message.ReplyToID)
		replyToID = &value
	}
	var replyTo *chatpb.ReplyPreviewInfo
	if message.ReplyTo != nil {
		replyTo = &chatpb.ReplyPreviewInfo{
			MessageId:      int64(message.ReplyTo.MessageID),
			SenderId:       int64(message.ReplyTo.SenderID),
			SenderType:     message.ReplyTo.SenderType,
			MessageType:    message.ReplyTo.MessageType,
			ContentPreview: message.ReplyTo.ContentPreview,
		}
	}
	pb := &chatpb.MessageInfo{
		Id:             int64(message.ID),
		ConversationId: message.ConversationID,
		SenderId:       int64(message.SenderID),
		SenderType:     message.SenderType,
		MessageType:    message.MessageType,
		Content:        message.Content,
		ReplyToId:      replyToID,
		ReplyTo:        replyTo,
		Status:         message.Status,
		CreatedAt:      message.CreatedAt,
	}
	if message.ReadByPeer != nil {
		value := *message.ReadByPeer
		pb.ReadByPeer = &value
	}
	if message.ReadCount != nil {
		value := *message.ReadCount
		pb.ReadCount = &value
	}
	return pb
}

func toConversationEventPB(event *biz.ConversationEventView) *chatpb.ConversationEventResponse {
	resp := &chatpb.ConversationEventResponse{
		Success: true,
		Message: "ok",
	}
	if event == nil {
		return resp
	}
	if event.Message != nil {
		resp.EventMessage = toMessagePB(*event.Message)
	}
	if len(event.RecipientUserIDs) > 0 {
		recipients := make([]int64, 0, len(event.RecipientUserIDs))
		for _, userID := range event.RecipientUserIDs {
			recipients = append(recipients, int64(userID))
		}
		resp.RecipientUserIds = recipients
	}
	return resp
}

func toMessageRecalledEventPB(event *biz.MessageRecalledEventView) *chatpb.MessageRecalledEventResponse {
	resp := &chatpb.MessageRecalledEventResponse{
		Success: true,
		Message: "ok",
	}
	if event == nil {
		return resp
	}
	resp.Event = &chatpb.MessageRecalledEventInfo{
		MessageId:      int64(event.MessageID),
		ConversationId: event.ConversationID,
	}
	if len(event.RecipientUserIDs) > 0 {
		recipients := make([]int64, 0, len(event.RecipientUserIDs))
		for _, userID := range event.RecipientUserIDs {
			recipients = append(recipients, int64(userID))
		}
		resp.RecipientUserIds = recipients
	}
	return resp
}
