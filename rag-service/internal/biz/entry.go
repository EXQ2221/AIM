package rag

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"example.com/aim/rag-service/internal/dal/model"
	"example.com/aim/rag-service/internal/observability"
	embedding "example.com/aim/rag-service/internal/provider"
	"example.com/aim/rag-service/internal/repository"
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
const defaultDocumentProcessTimeout = 15 * time.Minute

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

type IngestChunkInput struct {
	Index        int    `json:"index"`
	ChunkType    string `json:"chunkType,omitempty"`
	SectionTitle string `json:"sectionTitle,omitempty"`
	Content      string `json:"content"`
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
	NotificationRepo  repository.NotificationRepository
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
	notificationRepo repository.NotificationRepository,
	embedClient embedding.Client,
	processor *DocumentProcessor,
) *RAGService {
	return &RAGService{
		Repo:              repo,
		ConversationRepo:  conversationRepo,
		MemberRepo:        memberRepo,
		NotificationRepo:  notificationRepo,
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

	content, ingestChunks := parseIngestPayload(input.Content)
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

	s.processDocumentAsync(doc.ID, content, ingestChunks)

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

func (s *RAGService) processDocumentAsync(documentID uint64, content string, chunks []IngestChunkInput) {
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
			Chunks:     chunks,
			ModelName:  "",
		})
		if err != nil {
			timedOut := isTimeoutError(err) || errors.Is(ctx.Err(), context.DeadlineExceeded)
			logger.Warn(
				"knowledge_document.process_failed",
				zap.Uint64("document_id", documentID),
				zap.Int64("elapsed_ms", time.Since(start).Milliseconds()),
				zap.Bool("timed_out", timedOut),
				zap.Error(err),
			)
			s.notifyDocumentProcessFailure(documentID, err, timedOut)
			return
		}
		logger.Info(
			"knowledge_document.process_done",
			zap.Uint64("document_id", documentID),
			zap.Int64("elapsed_ms", time.Since(start).Milliseconds()),
		)
	}()
}

func (s *RAGService) notifyDocumentProcessFailure(documentID uint64, processErr error, timedOut bool) {
	if s == nil || s.Repo == nil || s.NotificationRepo == nil || documentID == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	doc, err := s.Repo.GetKnowledgeDocumentByID(ctx, documentID)
	if err != nil || doc == nil || doc.CreatedBy == 0 {
		return
	}

	docTitle := strings.TrimSpace(doc.Title)
	if docTitle == "" {
		docTitle = fmt.Sprintf("Document#%d", documentID)
	}

	title := "Knowledge document process failed"
	prefix := "processing failed"
	if timedOut {
		title = "Knowledge document process timeout"
		prefix = "processing timed out"
	}

	errText := strings.TrimSpace(processErr.Error())
	if len(errText) > 240 {
		errText = errText[:240] + "..."
	}

	content := fmt.Sprintf("Document \"%s\" %s. Error: %s", docTitle, prefix, errText)
	notification := &model.Notification{
		UserID:  doc.CreatedBy,
		Type:    model.NotificationTypeSystem,
		Title:   title,
		Content: content,
		IsRead:  false,
	}
	_ = s.NotificationRepo.Create(ctx, notification)
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var timeoutErr interface{ Timeout() bool }
	if errors.As(err, &timeoutErr) && timeoutErr.Timeout() {
		return true
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "deadline exceeded") || strings.Contains(text, "timeout")
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
		Input: []embedding.InputPart{{Type: embedding.InputPartText, Text: query}},
	})
	if err != nil {
		return nil, fmt.Errorf("knowledge search embedding failed: %w", err)
	}
	if len(embedResp.Embeddings) != 1 {
		return nil, errors.New("knowledge search embedding result is invalid")
	}

	items, err := s.Repo.SearchKnowledgeChunkCandidatesByKB(searchCtx, input.KnowledgeBaseID, query, embedResp.Embeddings[0], topK)
	if err != nil {
		return nil, err
	}
	selected := rerankCandidates(query, items, topK)
	result := make([]KnowledgeSearchChunkView, 0, len(selected))
	for _, item := range selected {
		result = append(result, KnowledgeSearchChunkView{
			ChunkID:    item.ChunkID,
			DocumentID: item.DocumentID,
			Score:      item.FinalScore,
			Content:    item.Content,
		})
	}
	return result, nil
}

type chunkMetaView struct {
	SectionTitle string
	QuestionNo   int
	QuestionText string
}

type rankedCandidate struct {
	repository.KnowledgeChunkCandidate
	Meta             chunkMetaView
	VectorScore      float64
	FullTextScore    float64
	TitleScore       float64
	SectionScore     float64
	DuplicatePenalty float64
	FinalScore       float64
}

func rerankCandidates(query string, items []repository.KnowledgeChunkCandidate, topK int) []rankedCandidate {
	if topK <= 0 {
		topK = 5
	}
	if len(items) == 0 {
		return nil
	}

	ranked := make([]rankedCandidate, 0, len(items))
	fullTextMax := 0.0
	for _, item := range items {
		if item.HasFullText && item.FullTextRank > fullTextMax {
			fullTextMax = item.FullTextRank
		}
	}
	if fullTextMax <= 0 {
		fullTextMax = 1
	}

	for _, item := range items {
		meta := decodeChunkMeta(item.Metadata)
		vectorScore := 0.0
		if item.HasVector {
			vectorScore = 1 - item.VectorDistance
			if vectorScore < 0 {
				vectorScore = 0
			}
			if vectorScore > 1 {
				vectorScore = 1
			}
		}
		fullTextScore := 0.0
		if item.HasFullText {
			fullTextScore = item.FullTextRank / fullTextMax
			if fullTextScore < 0 {
				fullTextScore = 0
			}
			if fullTextScore > 1 {
				fullTextScore = 1
			}
		}
		titleScore := calcTitleScoreSimple(query, item.DocumentTitle)
		sectionScore := calcSectionScoreSimple(query, meta)
		if item.TitleMatched && titleScore < 0.95 {
			titleScore = 0.95
		}
		if item.SectionMatched && sectionScore < 0.7 {
			sectionScore = 0.7
		}
		finalScore := vectorScore*0.62 + fullTextScore*0.28 + titleScore*0.35 + sectionScore*0.20

		ranked = append(ranked, rankedCandidate{
			KnowledgeChunkCandidate: item,
			Meta:                    meta,
			VectorScore:             vectorScore,
			FullTextScore:           fullTextScore,
			TitleScore:              titleScore,
			SectionScore:            sectionScore,
			FinalScore:              finalScore,
		})
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		if math.Abs(ranked[i].FinalScore-ranked[j].FinalScore) > 1e-9 {
			return ranked[i].FinalScore > ranked[j].FinalScore
		}
		return ranked[i].ChunkID < ranked[j].ChunkID
	})

	byQuestion := make(map[string]bool)
	keptPerDoc := make(map[uint64]int)
	for i := range ranked {
		if ranked[i].Meta.QuestionNo > 0 {
			key := fmt.Sprintf("%d#%d", ranked[i].DocumentID, ranked[i].Meta.QuestionNo)
			if byQuestion[key] {
				ranked[i].DuplicatePenalty += 0.20
			}
			byQuestion[key] = true
		}
		keptPerDoc[ranked[i].DocumentID]++
		if keptPerDoc[ranked[i].DocumentID] > 2 {
			ranked[i].DuplicatePenalty += 0.12
		}
		ranked[i].FinalScore -= ranked[i].DuplicatePenalty
		if ranked[i].FinalScore < 0 {
			ranked[i].FinalScore = 0
		}
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		if math.Abs(ranked[i].FinalScore-ranked[j].FinalScore) > 1e-9 {
			return ranked[i].FinalScore > ranked[j].FinalScore
		}
		return ranked[i].ChunkID < ranked[j].ChunkID
	})

	if len(ranked) > topK {
		ranked = ranked[:topK]
	}
	return ranked
}

func decodeChunkMeta(raw json.RawMessage) chunkMetaView {
	if len(raw) == 0 {
		return chunkMetaView{}
	}
	var meta chunkMetaView
	if err := json.Unmarshal(raw, &meta); err != nil {
		return chunkMetaView{}
	}
	return meta
}

func calcTitleScoreSimple(query string, title string) float64 {
	queryNorm := strings.ToLower(strings.TrimSpace(query))
	titleNorm := strings.ToLower(strings.TrimSpace(title))
	if titleNorm == "" || queryNorm == "" {
		return 0
	}
	if titleNorm == queryNorm {
		return 1
	}
	if strings.Contains(queryNorm, titleNorm) {
		return 0.9
	}
	if strings.Contains(titleNorm, queryNorm) || strings.Contains(queryNorm, titleNorm) {
		return 0.5
	}
	return 0
}

func calcSectionScoreSimple(query string, meta chunkMetaView) float64 {
	queryNorm := strings.ToLower(strings.TrimSpace(query))
	sectionNorm := strings.ToLower(strings.TrimSpace(meta.SectionTitle))
	questionTextNorm := strings.ToLower(strings.TrimSpace(meta.QuestionText))
	if queryNorm == "" {
		return 0
	}
	if meta.QuestionNo > 0 {
		if q := extractQuestionNo(queryNorm); q > 0 && q == meta.QuestionNo {
			return 1
		}
	}
	if sectionNorm != "" {
		if sectionNorm == queryNorm {
			return 1
		}
		if strings.Contains(sectionNorm, queryNorm) || strings.Contains(queryNorm, sectionNorm) {
			return 0.7
		}
	}
	if questionTextNorm != "" && (strings.Contains(questionTextNorm, queryNorm) || strings.Contains(queryNorm, questionTextNorm)) {
		return 0.3
	}
	return 0
}

func extractQuestionNo(query string) int {
	for _, token := range strings.FieldsFunc(query, func(r rune) bool {
		return r < '0' || r > '9'
	}) {
		if token == "" {
			continue
		}
		value, err := strconv.Atoi(token)
		if err == nil && value > 0 {
			return value
		}
	}
	return 0
}

func (s *RAGService) BindConversationKnowledgeBase(ctx context.Context, input BindConversationKnowledgeBaseInput) error {
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

func (s *RAGService) UnbindConversationKnowledgeBase(ctx context.Context, input UnbindConversationKnowledgeBaseInput) error {
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
