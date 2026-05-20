package handler

import (
	"context"

	ragbiz "example.com/aim/rag-service/internal/biz"
	ragpb "example.com/aim/rag-service/kitex_gen/rag"
)

type RAGServiceImpl struct {
	Service *ragbiz.RAGService
}

func NewRAGServiceImpl(service *ragbiz.RAGService) *RAGServiceImpl {
	return &RAGServiceImpl{Service: service}
}

func (h *RAGServiceImpl) Health(ctx context.Context, req *ragpb.HealthRequest) (*ragpb.HealthResponse, error) {
	return &ragpb.HealthResponse{Ok: true}, nil
}

func (h *RAGServiceImpl) CreateKnowledgeBase(ctx context.Context, req *ragpb.CreateKnowledgeBaseRequest) (*ragpb.CreateKnowledgeBaseResponse, error) {
	item, err := h.Service.CreateKnowledgeBase(ctx, ragbiz.CreateKnowledgeBaseInput{
		OperatorID:  uint64(req.OperatorId),
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		return nil, err
	}
	return &ragpb.CreateKnowledgeBaseResponse{
		KnowledgeBase: &ragpb.KnowledgeBaseInfo{
			KnowledgeBaseId: int64(item.KnowledgeBaseID),
			Name:            item.Name,
			Description:     item.Description,
			Status:          item.Status,
		},
	}, nil
}

func (h *RAGServiceImpl) ListKnowledgeBases(ctx context.Context, req *ragpb.ListKnowledgeBasesRequest) (*ragpb.ListKnowledgeBasesResponse, error) {
	items, err := h.Service.ListKnowledgeBases(ctx, uint64(req.OperatorId))
	if err != nil {
		return nil, err
	}
	result := make([]*ragpb.KnowledgeBaseInfo, 0, len(items))
	for _, item := range items {
		result = append(result, &ragpb.KnowledgeBaseInfo{
			KnowledgeBaseId: int64(item.KnowledgeBaseID),
			Name:            item.Name,
			Description:     item.Description,
			Status:          item.Status,
		})
	}
	return &ragpb.ListKnowledgeBasesResponse{KnowledgeBases: result}, nil
}

func (h *RAGServiceImpl) AddKnowledgeDocumentText(ctx context.Context, req *ragpb.AddKnowledgeDocumentTextRequest) (*ragpb.AddKnowledgeDocumentTextResponse, error) {
	item, err := h.Service.AddKnowledgeDocumentText(ctx, ragbiz.AddKnowledgeDocumentTextInput{
		OperatorID:      uint64(req.OperatorId),
		KnowledgeBaseID: uint64(req.KnowledgeBaseId),
		Title:           req.Title,
		SourceType:      req.SourceType,
		Content:         req.Content,
	})
	if err != nil {
		return nil, err
	}
	return &ragpb.AddKnowledgeDocumentTextResponse{
		Document: &ragpb.KnowledgeDocumentInfo{
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

func (h *RAGServiceImpl) ListKnowledgeDocuments(ctx context.Context, req *ragpb.ListKnowledgeDocumentsRequest) (*ragpb.ListKnowledgeDocumentsResponse, error) {
	items, err := h.Service.ListKnowledgeDocuments(ctx, ragbiz.ListKnowledgeDocumentsInput{
		OperatorID:      uint64(req.OperatorId),
		KnowledgeBaseID: uint64(req.KnowledgeBaseId),
	})
	if err != nil {
		return nil, err
	}
	docs := make([]*ragpb.KnowledgeDocumentInfo, 0, len(items))
	for _, item := range items {
		docs = append(docs, &ragpb.KnowledgeDocumentInfo{
			DocumentId:      int64(item.DocumentID),
			KnowledgeBaseId: int64(item.KnowledgeBaseID),
			Title:           item.Title,
			SourceType:      item.SourceType,
			Status:          item.Status,
			ErrorMessage:    item.ErrorMessage,
			CreatedAt:       item.CreatedAt,
		})
	}
	return &ragpb.ListKnowledgeDocumentsResponse{Documents: docs}, nil
}

func (h *RAGServiceImpl) DeleteKnowledgeDocument(ctx context.Context, req *ragpb.DeleteKnowledgeDocumentRequest) (*ragpb.CommonResponse, error) {
	if err := h.Service.DeleteKnowledgeDocument(ctx, ragbiz.DeleteKnowledgeDocumentInput{
		OperatorID:      uint64(req.OperatorId),
		KnowledgeBaseID: uint64(req.KnowledgeBaseId),
		DocumentID:      uint64(req.DocumentId),
	}); err != nil {
		return &ragpb.CommonResponse{Success: false, Message: err.Error()}, nil
	}
	return &ragpb.CommonResponse{Success: true, Message: "ok"}, nil
}

func (h *RAGServiceImpl) SearchKnowledgeBase(ctx context.Context, req *ragpb.SearchKnowledgeBaseRequest) (*ragpb.SearchKnowledgeBaseResponse, error) {
	items, err := h.Service.SearchKnowledgeBase(ctx, ragbiz.SearchKnowledgeBaseInput{
		OperatorID:      uint64(req.OperatorId),
		KnowledgeBaseID: uint64(req.KnowledgeBaseId),
		Query:           req.Query,
		TopK:            req.TopK,
	})
	if err != nil {
		return nil, err
	}
	chunks := make([]*ragpb.KnowledgeSearchChunkInfo, 0, len(items))
	for _, item := range items {
		chunks = append(chunks, &ragpb.KnowledgeSearchChunkInfo{
			ChunkId:    int64(item.ChunkID),
			DocumentId: int64(item.DocumentID),
			Score:      item.Score,
			Content:    item.Content,
		})
	}
	return &ragpb.SearchKnowledgeBaseResponse{Chunks: chunks}, nil
}

func (h *RAGServiceImpl) BindConversationKnowledgeBase(ctx context.Context, req *ragpb.BindConversationKnowledgeBaseRequest) (*ragpb.CommonResponse, error) {
	if err := h.Service.BindConversationKnowledgeBase(ctx, ragbiz.BindConversationKnowledgeBaseInput{
		OperatorID:      uint64(req.OperatorId),
		ConversationID:  req.ConversationId,
		KnowledgeBaseID: uint64(req.KnowledgeBaseId),
	}); err != nil {
		return &ragpb.CommonResponse{Success: false, Message: err.Error()}, nil
	}
	return &ragpb.CommonResponse{Success: true, Message: "ok"}, nil
}

func (h *RAGServiceImpl) ListConversationKnowledgeBases(ctx context.Context, req *ragpb.ListConversationKnowledgeBasesRequest) (*ragpb.ListConversationKnowledgeBasesResponse, error) {
	items, err := h.Service.ListConversationKnowledgeBases(ctx, uint64(req.OperatorId), req.ConversationId)
	if err != nil {
		return nil, err
	}
	result := make([]*ragpb.ConversationKnowledgeBaseInfo, 0, len(items))
	for _, item := range items {
		result = append(result, &ragpb.ConversationKnowledgeBaseInfo{
			Id:              int64(item.ID),
			ConversationId:  item.ConversationID,
			KnowledgeBaseId: int64(item.KnowledgeBaseID),
			Name:            item.Name,
			Description:     item.Description,
			Status:          item.Status,
			Enabled:         item.Enabled,
		})
	}
	return &ragpb.ListConversationKnowledgeBasesResponse{KnowledgeBases: result}, nil
}

func (h *RAGServiceImpl) UnbindConversationKnowledgeBase(ctx context.Context, req *ragpb.UnbindConversationKnowledgeBaseRequest) (*ragpb.CommonResponse, error) {
	if err := h.Service.UnbindConversationKnowledgeBase(ctx, ragbiz.UnbindConversationKnowledgeBaseInput{
		OperatorID:      uint64(req.OperatorId),
		ConversationID:  req.ConversationId,
		KnowledgeBaseID: uint64(req.KnowledgeBaseId),
	}); err != nil {
		return &ragpb.CommonResponse{Success: false, Message: err.Error()}, nil
	}
	return &ragpb.CommonResponse{Success: true, Message: "ok"}, nil
}
