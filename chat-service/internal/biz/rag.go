package biz

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"example.com/aim/chat-service/internal/dal/model"
	"example.com/aim/chat-service/internal/embedding"
	"example.com/aim/chat-service/internal/rag"
	"example.com/aim/chat-service/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrKnowledgeBaseNotFound    = errors.New("not_found: knowledge base not found")
	ErrKnowledgeBaseForbidden   = errors.New("forbidden: only knowledge base owner can access")
	ErrKnowledgeBaseNameInvalid = errors.New("bad_request: knowledge base name is required")
	ErrKnowledgeDocTitleInvalid = errors.New("bad_request: document title is required")
	ErrKnowledgeDocTypeInvalid  = errors.New("bad_request: sourceType must be TEXT or MARKDOWN")
	ErrKnowledgeDocContentEmpty = errors.New("bad_request: content is required")
	ErrKnowledgeDocContentLarge = errors.New("bad_request: content exceeds 200000 characters")
	ErrKnowledgeSearchQuery     = errors.New("bad_request: query is required")
	ErrKnowledgeSearchTopK      = errors.New("bad_request: topK must be between 1 and 10")
	ErrRAGUnavailable           = errors.New("internal: rag service is unavailable")
	ErrConversationOnlyGroup    = errors.New("bad_request: conversation must be group")
)

const maxKnowledgeDocumentContentRunes = 200000

type KnowledgeBaseView struct {
	KnowledgeBaseID uint64
	Name            string
	Description     string
	Status          string
}

type KnowledgeDocumentView struct {
	DocumentID      uint64
	KnowledgeBaseID uint64
	Title           string
	SourceType      string
	Status          string
	ErrorMessage    string
	CreatedAt       int64
}

type KnowledgeSearchChunkView struct {
	ChunkID    uint64
	DocumentID uint64
	Score      float64
	Content    string
}

type CreateKnowledgeBaseInput struct {
	OperatorID  uint64
	Name        string
	Description string
}

type AddKnowledgeDocumentTextInput struct {
	OperatorID      uint64
	KnowledgeBaseID uint64
	Title           string
	SourceType      string
	Content         string
}

type ListKnowledgeDocumentsInput struct {
	OperatorID      uint64
	KnowledgeBaseID uint64
}

type SearchKnowledgeBaseInput struct {
	OperatorID      uint64
	KnowledgeBaseID uint64
	Query           string
	TopK            *int32
}

type BindConversationKnowledgeBaseInput struct {
	OperatorID      uint64
	ConversationID  string
	KnowledgeBaseID uint64
}

type UnbindConversationKnowledgeBaseInput struct {
	OperatorID      uint64
	ConversationID  string
	KnowledgeBaseID uint64
}

type ConversationKnowledgeBaseView struct {
	ID              uint64
	ConversationID  string
	KnowledgeBaseID uint64
	Name            string
	Description     string
	Status          string
	Enabled         bool
}

type RAGService struct {
	Repo              repository.RAGRepository
	EmbeddingClient   embedding.Client
	DocumentProcessor *rag.DocumentProcessor
	DefaultTopK       int
	SearchTimeout     time.Duration
}

func NewRAGService(repo repository.RAGRepository, embedClient embedding.Client, processor *rag.DocumentProcessor) *RAGService {
	return &RAGService{
		Repo:              repo,
		EmbeddingClient:   embedClient,
		DocumentProcessor: processor,
		DefaultTopK:       5,
		SearchTimeout:     20 * time.Second,
	}
}

func (s *RAGService) CreateKnowledgeBase(ctx context.Context, input CreateKnowledgeBaseInput) (*KnowledgeBaseView, error) {
	if s == nil || s.Repo == nil {
		return nil, ErrRAGUnavailable
	}
	name := strings.TrimSpace(input.Name)
	if input.OperatorID == 0 || name == "" {
		return nil, ErrKnowledgeBaseNameInvalid
	}

	kb := &model.KnowledgeBase{
		Name:        name,
		Description: strings.TrimSpace(input.Description),
		OwnerID:     input.OperatorID,
		Scope:       model.KnowledgeBaseScopeConversation,
		Status:      model.KnowledgeBaseStatusActive,
	}
	if err := s.Repo.CreateKnowledgeBase(ctx, kb); err != nil {
		return nil, err
	}
	return &KnowledgeBaseView{
		KnowledgeBaseID: kb.ID,
		Name:            kb.Name,
		Description:     kb.Description,
		Status:          string(kb.Status),
	}, nil
}

func (s *RAGService) ListKnowledgeBases(ctx context.Context, operatorID uint64) ([]KnowledgeBaseView, error) {
	if s == nil || s.Repo == nil {
		return nil, ErrRAGUnavailable
	}
	if operatorID == 0 {
		return nil, ErrBadRequest
	}
	items, err := s.Repo.ListKnowledgeBasesByOwner(ctx, operatorID)
	if err != nil {
		return nil, err
	}
	result := make([]KnowledgeBaseView, 0, len(items))
	for _, item := range items {
		result = append(result, KnowledgeBaseView{
			KnowledgeBaseID: item.ID,
			Name:            item.Name,
			Description:     item.Description,
			Status:          string(item.Status),
		})
	}
	return result, nil
}

func (s *RAGService) AddKnowledgeDocumentText(ctx context.Context, input AddKnowledgeDocumentTextInput) (*KnowledgeDocumentView, error) {
	if s == nil || s.Repo == nil || s.DocumentProcessor == nil {
		return nil, ErrRAGUnavailable
	}
	if input.OperatorID == 0 || input.KnowledgeBaseID == 0 {
		return nil, ErrBadRequest
	}

	kb, err := s.Repo.GetKnowledgeBaseByID(ctx, input.KnowledgeBaseID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrKnowledgeBaseNotFound
		}
		return nil, err
	}
	if kb.OwnerID != input.OperatorID {
		return nil, ErrKnowledgeBaseForbidden
	}
	if kb.Status != model.KnowledgeBaseStatusActive {
		return nil, errors.New("forbidden: knowledge base is disabled")
	}

	title := strings.TrimSpace(input.Title)
	if title == "" {
		return nil, ErrKnowledgeDocTitleInvalid
	}
	sourceType := strings.ToUpper(strings.TrimSpace(input.SourceType))
	if sourceType == "" {
		sourceType = string(model.KnowledgeDocumentSourceText)
	}
	if sourceType != string(model.KnowledgeDocumentSourceText) && sourceType != string(model.KnowledgeDocumentSourceMarkdown) {
		return nil, ErrKnowledgeDocTypeInvalid
	}
	content := strings.TrimSpace(input.Content)
	if content == "" {
		return nil, ErrKnowledgeDocContentEmpty
	}
	if len([]rune(content)) > maxKnowledgeDocumentContentRunes {
		return nil, ErrKnowledgeDocContentLarge
	}

	doc := &model.KnowledgeDocument{
		KnowledgeBaseID: input.KnowledgeBaseID,
		Title:           title,
		SourceType:      model.KnowledgeDocumentSourceType(sourceType),
		Status:          model.KnowledgeDocumentStatusProcessing,
		CreatedBy:       input.OperatorID,
	}
	if err := s.Repo.CreateKnowledgeDocument(ctx, doc); err != nil {
		return nil, err
	}

	processErr := s.DocumentProcessor.ProcessDocument(ctx, rag.ProcessDocumentInput{
		DocumentID: doc.ID,
		Content:    content,
		ModelName:  "",
	})
	if processErr != nil {
		doc.Status = model.KnowledgeDocumentStatusFailed
		doc.ErrorMessage = processErr.Error()
	} else {
		doc.Status = model.KnowledgeDocumentStatusReady
		doc.ErrorMessage = ""
	}
	if saveErr := s.Repo.UpdateKnowledgeDocument(ctx, doc); saveErr != nil {
		return nil, saveErr
	}
	if processErr != nil {
		// documents/text API returns document status instead of request-level error.
	}

	return &KnowledgeDocumentView{
		DocumentID:      doc.ID,
		KnowledgeBaseID: doc.KnowledgeBaseID,
		Title:           doc.Title,
		SourceType:      string(doc.SourceType),
		Status:          string(doc.Status),
		ErrorMessage:    doc.ErrorMessage,
		CreatedAt:       doc.CreatedAt.Unix(),
	}, nil
}

func (s *RAGService) ListKnowledgeDocuments(ctx context.Context, input ListKnowledgeDocumentsInput) ([]KnowledgeDocumentView, error) {
	if s == nil || s.Repo == nil {
		return nil, ErrRAGUnavailable
	}
	if input.OperatorID == 0 || input.KnowledgeBaseID == 0 {
		return nil, ErrBadRequest
	}
	kb, err := s.Repo.GetKnowledgeBaseByID(ctx, input.KnowledgeBaseID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrKnowledgeBaseNotFound
		}
		return nil, err
	}
	if kb.OwnerID != input.OperatorID {
		return nil, ErrKnowledgeBaseForbidden
	}

	docs, err := s.Repo.ListKnowledgeDocumentsByKBID(ctx, input.KnowledgeBaseID)
	if err != nil {
		return nil, err
	}
	result := make([]KnowledgeDocumentView, 0, len(docs))
	for _, doc := range docs {
		result = append(result, KnowledgeDocumentView{
			DocumentID:      doc.ID,
			KnowledgeBaseID: doc.KnowledgeBaseID,
			Title:           doc.Title,
			SourceType:      string(doc.SourceType),
			Status:          string(doc.Status),
			ErrorMessage:    doc.ErrorMessage,
			CreatedAt:       doc.CreatedAt.Unix(),
		})
	}
	return result, nil
}

func (s *RAGService) SearchKnowledgeBase(ctx context.Context, input SearchKnowledgeBaseInput) ([]KnowledgeSearchChunkView, error) {
	if s == nil || s.Repo == nil || s.EmbeddingClient == nil {
		return nil, ErrRAGUnavailable
	}
	if input.OperatorID == 0 || input.KnowledgeBaseID == 0 {
		return nil, ErrBadRequest
	}
	query := strings.TrimSpace(input.Query)
	if query == "" {
		return nil, ErrKnowledgeSearchQuery
	}

	kb, err := s.Repo.GetKnowledgeBaseByID(ctx, input.KnowledgeBaseID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrKnowledgeBaseNotFound
		}
		return nil, err
	}
	if kb.OwnerID != input.OperatorID {
		return nil, ErrKnowledgeBaseForbidden
	}
	if kb.Status != model.KnowledgeBaseStatusActive {
		return nil, errors.New("forbidden: knowledge base is disabled")
	}

	topK := s.DefaultTopK
	if topK <= 0 {
		topK = 5
	}
	if input.TopK != nil {
		value := int(*input.TopK)
		if value < 1 || value > 10 {
			return nil, ErrKnowledgeSearchTopK
		}
		topK = value
	}

	searchCtx := ctx
	timeout := s.SearchTimeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		searchCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	embedResp, err := s.EmbeddingClient.Embed(searchCtx, embedding.EmbedRequest{
		Model: "",
		Input: []embedding.InputPart{
			{
				Type: embedding.InputPartText,
				Text: query,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("knowledge search embedding failed: %w", err)
	}
	if len(embedResp.Embeddings) != 1 {
		return nil, errors.New("knowledge search embedding result is invalid")
	}

	items, err := s.Repo.SearchKnowledgeChunksByKB(searchCtx, input.KnowledgeBaseID, embedResp.Embeddings[0], topK)
	if err != nil {
		return nil, err
	}

	result := make([]KnowledgeSearchChunkView, 0, len(items))
	for _, item := range items {
		score := 1 - item.Distance
		if score < -1 {
			score = -1
		}
		if score > 1 {
			score = 1
		}
		result = append(result, KnowledgeSearchChunkView{
			ChunkID:    item.ChunkID,
			DocumentID: item.DocumentID,
			Score:      score,
			Content:    item.Content,
		})
	}
	return result, nil
}

func (s *RAGService) BindConversationKnowledgeBase(
	ctx context.Context,
	input BindConversationKnowledgeBaseInput,
	conversationRepo repository.ConversationRepository,
	memberRepo repository.MemberRepository,
) error {
	if s == nil || s.Repo == nil {
		return ErrRAGUnavailable
	}
	if input.OperatorID == 0 || input.KnowledgeBaseID == 0 || strings.TrimSpace(input.ConversationID) == "" {
		return ErrBadRequest
	}
	conversation, err := conversationRepo.GetByConversationID(ctx, input.ConversationID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrConversationNotFound
		}
		return err
	}
	if conversation.Type != model.ConversationTypeGroup {
		return ErrConversationOnlyGroup
	}
	member, err := memberRepo.GetUserMember(ctx, conversation.ID, input.OperatorID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotMember
		}
		return err
	}
	if member.Status == model.MemberStatusRemoved {
		return ErrNotMember
	}
	if member.Role != model.MemberRoleOwner && member.Role != model.MemberRoleAdmin {
		return ErrAdminRequired
	}
	kb, err := s.Repo.GetKnowledgeBaseByID(ctx, input.KnowledgeBaseID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrKnowledgeBaseNotFound
		}
		return err
	}
	if kb.Status != model.KnowledgeBaseStatusActive {
		return errors.New("forbidden: knowledge base is disabled")
	}

	return s.Repo.UpsertConversationKnowledgeBase(ctx, conversation.ID, input.KnowledgeBaseID, input.OperatorID, true)
}

func (s *RAGService) ListConversationKnowledgeBases(
	ctx context.Context,
	operatorID uint64,
	conversationID string,
	conversationRepo repository.ConversationRepository,
	memberRepo repository.MemberRepository,
) ([]ConversationKnowledgeBaseView, error) {
	if s == nil || s.Repo == nil {
		return nil, ErrRAGUnavailable
	}
	if operatorID == 0 || strings.TrimSpace(conversationID) == "" {
		return nil, ErrBadRequest
	}
	conversation, err := conversationRepo.GetByConversationID(ctx, conversationID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrConversationNotFound
		}
		return nil, err
	}
	member, err := memberRepo.GetUserMember(ctx, conversation.ID, operatorID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotMember
		}
		return nil, err
	}
	if member.Status == model.MemberStatusRemoved {
		return nil, ErrNotMember
	}

	records, err := s.Repo.ListConversationKnowledgeBases(ctx, conversation.ID)
	if err != nil {
		return nil, err
	}
	result := make([]ConversationKnowledgeBaseView, 0, len(records))
	for _, item := range records {
		kb, kbErr := s.Repo.GetKnowledgeBaseByID(ctx, item.KnowledgeBaseID)
		if kbErr != nil {
			continue
		}
		result = append(result, ConversationKnowledgeBaseView{
			ID:              item.ID,
			ConversationID:  conversation.ConversationID,
			KnowledgeBaseID: item.KnowledgeBaseID,
			Name:            kb.Name,
			Description:     kb.Description,
			Status:          string(kb.Status),
			Enabled:         item.Enabled,
		})
	}
	return result, nil
}

func (s *RAGService) UnbindConversationKnowledgeBase(
	ctx context.Context,
	input UnbindConversationKnowledgeBaseInput,
	conversationRepo repository.ConversationRepository,
	memberRepo repository.MemberRepository,
) error {
	if s == nil || s.Repo == nil {
		return ErrRAGUnavailable
	}
	if input.OperatorID == 0 || input.KnowledgeBaseID == 0 || strings.TrimSpace(input.ConversationID) == "" {
		return ErrBadRequest
	}
	conversation, err := conversationRepo.GetByConversationID(ctx, input.ConversationID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrConversationNotFound
		}
		return err
	}
	if conversation.Type != model.ConversationTypeGroup {
		return ErrConversationOnlyGroup
	}
	member, err := memberRepo.GetUserMember(ctx, conversation.ID, input.OperatorID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotMember
		}
		return err
	}
	if member.Status == model.MemberStatusRemoved {
		return ErrNotMember
	}
	if member.Role != model.MemberRoleOwner && member.Role != model.MemberRoleAdmin {
		return ErrAdminRequired
	}

	return s.Repo.UpdateConversationKnowledgeBaseEnabled(ctx, conversation.ID, input.KnowledgeBaseID, false)
}

func (s *ChatService) ensureRAGService() (*RAGService, error) {
	if s == nil || s.RAGService == nil {
		return nil, ErrRAGUnavailable
	}
	return s.RAGService, nil
}

func (s *ChatService) CreateKnowledgeBase(ctx context.Context, input CreateKnowledgeBaseInput) (*KnowledgeBaseView, error) {
	ragService, err := s.ensureRAGService()
	if err != nil {
		return nil, err
	}
	return ragService.CreateKnowledgeBase(ctx, input)
}

func (s *ChatService) ListKnowledgeBases(ctx context.Context, operatorID uint64) ([]KnowledgeBaseView, error) {
	ragService, err := s.ensureRAGService()
	if err != nil {
		return nil, err
	}
	return ragService.ListKnowledgeBases(ctx, operatorID)
}

func (s *ChatService) AddKnowledgeDocumentText(ctx context.Context, input AddKnowledgeDocumentTextInput) (*KnowledgeDocumentView, error) {
	ragService, err := s.ensureRAGService()
	if err != nil {
		return nil, err
	}
	return ragService.AddKnowledgeDocumentText(ctx, input)
}

func (s *ChatService) ListKnowledgeDocuments(ctx context.Context, input ListKnowledgeDocumentsInput) ([]KnowledgeDocumentView, error) {
	ragService, err := s.ensureRAGService()
	if err != nil {
		return nil, err
	}
	return ragService.ListKnowledgeDocuments(ctx, input)
}

func (s *ChatService) SearchKnowledgeBase(ctx context.Context, input SearchKnowledgeBaseInput) ([]KnowledgeSearchChunkView, error) {
	ragService, err := s.ensureRAGService()
	if err != nil {
		return nil, err
	}
	return ragService.SearchKnowledgeBase(ctx, input)
}

func (s *ChatService) BindConversationKnowledgeBase(ctx context.Context, input BindConversationKnowledgeBaseInput) error {
	ragService, err := s.ensureRAGService()
	if err != nil {
		return err
	}
	return ragService.BindConversationKnowledgeBase(ctx, input, s.ConversationRepo, s.MemberRepo)
}

func (s *ChatService) ListConversationKnowledgeBases(ctx context.Context, operatorID uint64, conversationID string) ([]ConversationKnowledgeBaseView, error) {
	ragService, err := s.ensureRAGService()
	if err != nil {
		return nil, err
	}
	return ragService.ListConversationKnowledgeBases(ctx, operatorID, conversationID, s.ConversationRepo, s.MemberRepo)
}

func (s *ChatService) UnbindConversationKnowledgeBase(ctx context.Context, input UnbindConversationKnowledgeBaseInput) error {
	ragService, err := s.ensureRAGService()
	if err != nil {
		return err
	}
	return ragService.UnbindConversationKnowledgeBase(ctx, input, s.ConversationRepo, s.MemberRepo)
}
