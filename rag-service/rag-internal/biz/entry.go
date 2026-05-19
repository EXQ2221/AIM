package rag

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"example.com/aim/rag-service/internal/dal/model"
	"example.com/aim/rag-service/internal/observability"
	"example.com/aim/rag-service/internal/repository"
	embedding "example.com/aim/rag-service/rag-internal/client"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	ErrBadRequest                = errors.New("bad_request: invalid request")
	ErrConversationNotFound      = errors.New("not_found: conversation not found")
	ErrNotMember                 = errors.New("forbidden: user is not a member of this conversation")
	ErrAdminRequired             = errors.New("forbidden: owner/admin is required")
	ErrKnowledgeBaseNotFound     = errors.New("not_found: knowledge base not found")
	ErrKnowledgeDocumentNotFound = errors.New("not_found: knowledge document not found")
	ErrKnowledgeBaseForbidden    = errors.New("forbidden: only knowledge base owner can access")
	ErrKnowledgeBaseNameInvalid  = errors.New("bad_request: knowledge base name is required")
	ErrKnowledgeDocTitleInvalid  = errors.New("bad_request: document title is required")
	ErrKnowledgeDocTypeInvalid   = errors.New("bad_request: sourceType must be TEXT or MARKDOWN")
	ErrKnowledgeDocContentEmpty  = errors.New("bad_request: content is required")
	ErrKnowledgeDocContentLarge  = errors.New("bad_request: content exceeds 200000 characters")
	ErrKnowledgeSearchQuery      = errors.New("bad_request: query is required")
	ErrKnowledgeSearchTopK       = errors.New("bad_request: topK must be between 1 and 10")
	ErrRAGUnavailable            = errors.New("internal: rag service is unavailable")
	ErrConversationOnlyGroup     = errors.New("bad_request: conversation must be group")
)

const maxKnowledgeDocumentContentRunes = 200000
const defaultDocumentProcessTimeout = 5 * time.Minute

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

type DeleteKnowledgeDocumentInput struct {
	OperatorID      uint64
	KnowledgeBaseID uint64
	DocumentID      uint64
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
	ConversationRepo  repository.ConversationRepository
	MemberRepo        repository.MemberRepository
	EmbeddingClient   embedding.Client
	DocumentProcessor *DocumentProcessor
	DefaultTopK       int
	SearchTimeout     time.Duration
	ProcessTimeout    time.Duration
}

func NewRAGService(
	repo repository.RAGRepository,
	conversationRepo repository.ConversationRepository,
	memberRepo repository.MemberRepository,
	embedClient embedding.Client,
	processor *DocumentProcessor,
) *RAGService {
	return &RAGService{
		Repo:              repo,
		ConversationRepo:  conversationRepo,
		MemberRepo:        memberRepo,
		EmbeddingClient:   embedClient,
		DocumentProcessor: processor,
		DefaultTopK:       5,
		SearchTimeout:     40 * time.Second,
		ProcessTimeout:    defaultDocumentProcessTimeout,
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
		accessible, accessErr := s.Repo.IsKnowledgeBaseAccessibleByUser(ctx, input.KnowledgeBaseID, input.OperatorID)
		if accessErr != nil {
			return nil, accessErr
		}
		if !accessible {
			return nil, ErrKnowledgeBaseForbidden
		}
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
		Status:          model.KnowledgeDocumentStatusPending,
		CreatedBy:       input.OperatorID,
	}
	if err := s.Repo.CreateKnowledgeDocument(ctx, doc); err != nil {
		return nil, err
	}

	s.processDocumentAsync(doc.ID, content)

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

func (s *RAGService) processDocumentAsync(documentID uint64, content string) {
	timeout := s.ProcessTimeout
	if timeout <= 0 {
		timeout = defaultDocumentProcessTimeout
	}
	go func() {
		logger := observability.L()
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		logger.Info(
			"knowledge_document.process_start",
			zap.Uint64("document_id", documentID),
			zap.Int("content_chars", len([]rune(content))),
			zap.Int64("timeout_ms", timeout.Milliseconds()),
		)

		err := s.DocumentProcessor.ProcessDocument(ctx, ProcessDocumentInput{
			DocumentID: documentID,
			Content:    content,
			ModelName:  "",
		})
		if err != nil {
			logger.Warn(
				"knowledge_document.process_failed",
				zap.Uint64("document_id", documentID),
				zap.Int64("elapsed_ms", time.Since(start).Milliseconds()),
				zap.Error(err),
			)
			return
		}
		logger.Info(
			"knowledge_document.process_done",
			zap.Uint64("document_id", documentID),
			zap.Int64("elapsed_ms", time.Since(start).Milliseconds()),
		)
	}()
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
		accessible, accessErr := s.Repo.IsKnowledgeBaseAccessibleByUser(ctx, input.KnowledgeBaseID, input.OperatorID)
		if accessErr != nil {
			return nil, accessErr
		}
		if !accessible {
			return nil, ErrKnowledgeBaseForbidden
		}
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

func (s *RAGService) DeleteKnowledgeDocument(ctx context.Context, input DeleteKnowledgeDocumentInput) error {
	if s == nil || s.Repo == nil {
		return ErrRAGUnavailable
	}
	if input.OperatorID == 0 || input.KnowledgeBaseID == 0 || input.DocumentID == 0 {
		return ErrBadRequest
	}
	kb, err := s.Repo.GetKnowledgeBaseByID(ctx, input.KnowledgeBaseID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrKnowledgeBaseNotFound
		}
		return err
	}
	if kb.OwnerID != input.OperatorID {
		return ErrKnowledgeBaseForbidden
	}

	doc, err := s.Repo.GetKnowledgeDocumentByID(ctx, input.DocumentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrKnowledgeDocumentNotFound
		}
		return err
	}
	if doc.KnowledgeBaseID != input.KnowledgeBaseID {
		return ErrKnowledgeDocumentNotFound
	}

	return s.Repo.DeleteKnowledgeDocument(ctx, input.DocumentID)
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
		accessible, accessErr := s.Repo.IsKnowledgeBaseAccessibleByUser(ctx, input.KnowledgeBaseID, input.OperatorID)
		if accessErr != nil {
			return nil, accessErr
		}
		if !accessible {
			return nil, ErrKnowledgeBaseForbidden
		}
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
		timeout = 40 * time.Second
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
) error {
	if s == nil || s.Repo == nil || s.ConversationRepo == nil || s.MemberRepo == nil {
		return ErrRAGUnavailable
	}
	if input.OperatorID == 0 || input.KnowledgeBaseID == 0 || strings.TrimSpace(input.ConversationID) == "" {
		return ErrBadRequest
	}
	conversation, err := s.ConversationRepo.GetByConversationID(ctx, input.ConversationID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrConversationNotFound
		}
		return err
	}
	if conversation.Type != model.ConversationTypeGroup {
		return ErrConversationOnlyGroup
	}
	member, err := s.MemberRepo.GetUserMember(ctx, conversation.ID, input.OperatorID)
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
) ([]ConversationKnowledgeBaseView, error) {
	if s == nil || s.Repo == nil || s.ConversationRepo == nil || s.MemberRepo == nil {
		return nil, ErrRAGUnavailable
	}
	if operatorID == 0 || strings.TrimSpace(conversationID) == "" {
		return nil, ErrBadRequest
	}
	conversation, err := s.ConversationRepo.GetByConversationID(ctx, conversationID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrConversationNotFound
		}
		return nil, err
	}
	member, err := s.MemberRepo.GetUserMember(ctx, conversation.ID, operatorID)
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
) error {
	if s == nil || s.Repo == nil || s.ConversationRepo == nil || s.MemberRepo == nil {
		return ErrRAGUnavailable
	}
	if input.OperatorID == 0 || input.KnowledgeBaseID == 0 || strings.TrimSpace(input.ConversationID) == "" {
		return ErrBadRequest
	}
	conversation, err := s.ConversationRepo.GetByConversationID(ctx, input.ConversationID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrConversationNotFound
		}
		return err
	}
	if conversation.Type != model.ConversationTypeGroup {
		return ErrConversationOnlyGroup
	}
	member, err := s.MemberRepo.GetUserMember(ctx, conversation.ID, input.OperatorID)
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
