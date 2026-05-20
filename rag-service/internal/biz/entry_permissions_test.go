package rag

import (
	"context"
	"errors"
	"testing"
	"time"

	"example.com/aim/rag-service/internal/dal/model"
	embedding "example.com/aim/rag-service/internal/provider"
	"example.com/aim/rag-service/internal/repository"
	"gorm.io/gorm"
)

type fakeRAGRepo struct {
	createdKB   *model.KnowledgeBase
	kbByID      map[uint64]*model.KnowledgeBase
	docByID     map[uint64]*model.KnowledgeDocument
	accessible  bool
	deleteCount int
	deletedDoc  uint64
	upsertCount int
	updateCount int
	lastUpsert  struct {
		conversationID uint64
		knowledgeBase  uint64
		createdBy      uint64
		enabled        bool
	}
	lastUpdate struct {
		conversationID uint64
		knowledgeBase  uint64
		enabled        bool
	}
}

func (f *fakeRAGRepo) WithTx(tx *gorm.DB) repository.RAGRepository { return f }
func (f *fakeRAGRepo) CreateKnowledgeBase(ctx context.Context, kb *model.KnowledgeBase) error {
	f.createdKB = kb
	if kb.ID == 0 {
		kb.ID = 1
	}
	return nil
}
func (f *fakeRAGRepo) ListKnowledgeBasesByOwner(ctx context.Context, ownerID uint64) ([]model.KnowledgeBase, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeRAGRepo) GetKnowledgeBaseByID(ctx context.Context, kbID uint64) (*model.KnowledgeBase, error) {
	item, ok := f.kbByID[kbID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	copy := *item
	return &copy, nil
}
func (f *fakeRAGRepo) CreateKnowledgeDocument(ctx context.Context, doc *model.KnowledgeDocument) error {
	return errors.New("not implemented")
}
func (f *fakeRAGRepo) UpdateKnowledgeDocument(ctx context.Context, doc *model.KnowledgeDocument) error {
	return errors.New("not implemented")
}
func (f *fakeRAGRepo) ListKnowledgeDocumentsByKBID(ctx context.Context, kbID uint64) ([]model.KnowledgeDocument, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeRAGRepo) GetKnowledgeDocumentByID(ctx context.Context, documentID uint64) (*model.KnowledgeDocument, error) {
	item, ok := f.docByID[documentID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	copy := *item
	return &copy, nil
}
func (f *fakeRAGRepo) DeleteKnowledgeDocument(ctx context.Context, documentID uint64) error {
	f.deleteCount++
	f.deletedDoc = documentID
	return nil
}
func (f *fakeRAGRepo) UpdateKnowledgeDocumentStatus(ctx context.Context, documentID uint64, status model.KnowledgeDocumentStatus, errorMessage string) error {
	return errors.New("not implemented")
}
func (f *fakeRAGRepo) ReplaceKnowledgeChunksForDocument(ctx context.Context, documentID uint64, records []repository.KnowledgeChunkRecord) error {
	return errors.New("not implemented")
}
func (f *fakeRAGRepo) SearchKnowledgeChunkCandidatesByKB(ctx context.Context, kbID uint64, query string, queryEmbedding []float32, topK int) ([]repository.KnowledgeChunkCandidate, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeRAGRepo) SearchKnowledgeChunksByKB(ctx context.Context, kbID uint64, query string, queryEmbedding []float32, topK int) ([]repository.KnowledgeSearchChunk, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeRAGRepo) IsKnowledgeBaseAccessibleByUser(ctx context.Context, kbID uint64, userID uint64) (bool, error) {
	return f.accessible, nil
}
func (f *fakeRAGRepo) UpsertConversationKnowledgeBase(ctx context.Context, conversationID uint64, knowledgeBaseID uint64, createdBy uint64, enabled bool) error {
	f.upsertCount++
	f.lastUpsert.conversationID = conversationID
	f.lastUpsert.knowledgeBase = knowledgeBaseID
	f.lastUpsert.createdBy = createdBy
	f.lastUpsert.enabled = enabled
	return nil
}
func (f *fakeRAGRepo) ListConversationKnowledgeBases(ctx context.Context, conversationID uint64) ([]model.ConversationKnowledgeBase, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeRAGRepo) GetConversationKnowledgeBase(ctx context.Context, conversationID uint64, knowledgeBaseID uint64) (*model.ConversationKnowledgeBase, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeRAGRepo) UpdateConversationKnowledgeBaseEnabled(ctx context.Context, conversationID uint64, knowledgeBaseID uint64, enabled bool) error {
	f.updateCount++
	f.lastUpdate.conversationID = conversationID
	f.lastUpdate.knowledgeBase = knowledgeBaseID
	f.lastUpdate.enabled = enabled
	return nil
}

type fakeConversationRepo struct {
	conversation *model.Conversation
	err          error
}

func (f *fakeConversationRepo) WithTx(tx *gorm.DB) repository.ConversationRepository { return f }
func (f *fakeConversationRepo) GetByConversationID(ctx context.Context, conversationID string) (*model.Conversation, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.conversation == nil {
		return nil, gorm.ErrRecordNotFound
	}
	copy := *f.conversation
	return &copy, nil
}

type fakeMemberRepo struct {
	member *model.ConversationMember
	err    error
}

func (f *fakeMemberRepo) WithTx(tx *gorm.DB) repository.MemberRepository { return f }
func (f *fakeMemberRepo) GetUserMember(ctx context.Context, conversationID, userID uint64) (*model.ConversationMember, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.member == nil {
		return nil, gorm.ErrRecordNotFound
	}
	copy := *f.member
	return &copy, nil
}

func TestCreateKnowledgeBase_AssignsOwnerToOperator(t *testing.T) {
	repo := &fakeRAGRepo{}
	service := &RAGService{Repo: repo}

	view, err := service.CreateKnowledgeBase(context.Background(), CreateKnowledgeBaseInput{
		OperatorID:  1001,
		Name:        "team notes",
		Description: "kb",
	})
	if err != nil {
		t.Fatalf("CreateKnowledgeBase returned error: %v", err)
	}
	if view == nil || view.KnowledgeBaseID == 0 {
		t.Fatalf("expected created knowledge base view, got %#v", view)
	}
	if repo.createdKB == nil {
		t.Fatal("expected repository CreateKnowledgeBase call")
	}
	if repo.createdKB.OwnerID != 1001 {
		t.Fatalf("expected owner_id=1001, got %d", repo.createdKB.OwnerID)
	}
}

func TestBindConversationKnowledgeBase_RejectsNormalMember(t *testing.T) {
	repo := &fakeRAGRepo{
		kbByID: map[uint64]*model.KnowledgeBase{
			9: {ID: 9, Status: model.KnowledgeBaseStatusActive},
		},
	}
	service := &RAGService{
		Repo: repo,
		ConversationRepo: &fakeConversationRepo{
			conversation: &model.Conversation{
				ID:             3,
				ConversationID: "c_demo",
				Type:           model.ConversationTypeGroup,
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
			},
		},
		MemberRepo: &fakeMemberRepo{
			member: &model.ConversationMember{
				ConversationID: 3,
				MemberType:     model.MemberTypeUser,
				MemberID:       2001,
				Role:           model.MemberRoleMember,
				Status:         model.MemberStatusNormal,
			},
		},
	}

	err := service.BindConversationKnowledgeBase(context.Background(), BindConversationKnowledgeBaseInput{
		OperatorID:      2001,
		ConversationID:  "c_demo",
		KnowledgeBaseID: 9,
	})
	if err != ErrAdminRequired {
		t.Fatalf("expected ErrAdminRequired, got %v", err)
	}
	if repo.upsertCount != 0 {
		t.Fatalf("expected no upsert, got %d", repo.upsertCount)
	}
}

func TestBindConversationKnowledgeBase_AllowsAdmin(t *testing.T) {
	repo := &fakeRAGRepo{
		kbByID: map[uint64]*model.KnowledgeBase{
			9: {ID: 9, Status: model.KnowledgeBaseStatusActive},
		},
	}
	service := &RAGService{
		Repo: repo,
		ConversationRepo: &fakeConversationRepo{
			conversation: &model.Conversation{
				ID:             3,
				ConversationID: "c_demo",
				Type:           model.ConversationTypeGroup,
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
			},
		},
		MemberRepo: &fakeMemberRepo{
			member: &model.ConversationMember{
				ConversationID: 3,
				MemberType:     model.MemberTypeUser,
				MemberID:       2001,
				Role:           model.MemberRoleAdmin,
				Status:         model.MemberStatusNormal,
			},
		},
	}

	err := service.BindConversationKnowledgeBase(context.Background(), BindConversationKnowledgeBaseInput{
		OperatorID:      2001,
		ConversationID:  "c_demo",
		KnowledgeBaseID: 9,
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if repo.upsertCount != 1 {
		t.Fatalf("expected 1 upsert, got %d", repo.upsertCount)
	}
	if repo.lastUpsert.conversationID != 3 || repo.lastUpsert.knowledgeBase != 9 || !repo.lastUpsert.enabled {
		t.Fatalf("unexpected upsert args: %#v", repo.lastUpsert)
	}
}

func TestUnbindConversationKnowledgeBase_RejectsNormalMember(t *testing.T) {
	repo := &fakeRAGRepo{}
	service := &RAGService{
		Repo: repo,
		ConversationRepo: &fakeConversationRepo{
			conversation: &model.Conversation{
				ID:             3,
				ConversationID: "c_demo",
				Type:           model.ConversationTypeGroup,
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
			},
		},
		MemberRepo: &fakeMemberRepo{
			member: &model.ConversationMember{
				ConversationID: 3,
				MemberType:     model.MemberTypeUser,
				MemberID:       2001,
				Role:           model.MemberRoleMember,
				Status:         model.MemberStatusNormal,
			},
		},
	}

	err := service.UnbindConversationKnowledgeBase(context.Background(), UnbindConversationKnowledgeBaseInput{
		OperatorID:      2001,
		ConversationID:  "c_demo",
		KnowledgeBaseID: 9,
	})
	if err != ErrAdminRequired {
		t.Fatalf("expected ErrAdminRequired, got %v", err)
	}
	if repo.updateCount != 0 {
		t.Fatalf("expected no update call, got %d", repo.updateCount)
	}
}

func TestDeleteKnowledgeDocument_OwnerCanDelete(t *testing.T) {
	repo := &fakeRAGRepo{
		kbByID: map[uint64]*model.KnowledgeBase{
			4: {ID: 4, OwnerID: 1001, Status: model.KnowledgeBaseStatusActive},
		},
		docByID: map[uint64]*model.KnowledgeDocument{
			12: {ID: 12, KnowledgeBaseID: 4},
		},
	}
	service := &RAGService{Repo: repo}

	err := service.DeleteKnowledgeDocument(context.Background(), DeleteKnowledgeDocumentInput{
		OperatorID:      1001,
		KnowledgeBaseID: 4,
		DocumentID:      12,
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if repo.deleteCount != 1 || repo.deletedDoc != 12 {
		t.Fatalf("expected delete document 12 once, got count=%d doc=%d", repo.deleteCount, repo.deletedDoc)
	}
}

func TestDeleteKnowledgeDocument_RejectsNonOwner(t *testing.T) {
	repo := &fakeRAGRepo{
		kbByID: map[uint64]*model.KnowledgeBase{
			4: {ID: 4, OwnerID: 1001, Status: model.KnowledgeBaseStatusActive},
		},
		docByID: map[uint64]*model.KnowledgeDocument{
			12: {ID: 12, KnowledgeBaseID: 4},
		},
	}
	service := &RAGService{Repo: repo}

	err := service.DeleteKnowledgeDocument(context.Background(), DeleteKnowledgeDocumentInput{
		OperatorID:      1002,
		KnowledgeBaseID: 4,
		DocumentID:      12,
	})
	if err != ErrKnowledgeBaseForbidden {
		t.Fatalf("expected ErrKnowledgeBaseForbidden, got %v", err)
	}
	if repo.deleteCount != 0 {
		t.Fatalf("expected no delete call, got %d", repo.deleteCount)
	}
}

func TestSearchKnowledgeBase_AllowsBoundConversationMember(t *testing.T) {
	repo := &fakeRAGRepo{
		accessible: true,
		kbByID: map[uint64]*model.KnowledgeBase{
			4: {ID: 4, OwnerID: 1001, Status: model.KnowledgeBaseStatusActive},
		},
	}
	service := &RAGService{
		Repo:            repo,
		EmbeddingClient: &stubEmbeddingClient{embeddings: [][]float32{{0.1, 0.2}}},
		DefaultTopK:     5,
		SearchTimeout:   20 * time.Second,
	}

	_, err := service.SearchKnowledgeBase(context.Background(), SearchKnowledgeBaseInput{
		OperatorID:      2002,
		KnowledgeBaseID: 4,
		Query:           "璇濆墽",
	})
	if err == nil {
		t.Fatalf("expected repository not implemented error, got nil")
	}
	if errors.Is(err, ErrKnowledgeBaseForbidden) {
		t.Fatalf("expected non-forbidden error for accessible member, got %v", err)
	}
}

func TestSearchKnowledgeBase_RejectsUnboundNonOwner(t *testing.T) {
	repo := &fakeRAGRepo{
		accessible: false,
		kbByID: map[uint64]*model.KnowledgeBase{
			4: {ID: 4, OwnerID: 1001, Status: model.KnowledgeBaseStatusActive},
		},
	}
	service := &RAGService{
		Repo:            repo,
		EmbeddingClient: &stubEmbeddingClient{embeddings: [][]float32{{0.1, 0.2}}},
	}

	_, err := service.SearchKnowledgeBase(context.Background(), SearchKnowledgeBaseInput{
		OperatorID:      2002,
		KnowledgeBaseID: 4,
		Query:           "璇濆墽",
	})
	if err != ErrKnowledgeBaseForbidden {
		t.Fatalf("expected ErrKnowledgeBaseForbidden, got %v", err)
	}
}

type stubEmbeddingClient struct {
	embeddings [][]float32
}

func (s *stubEmbeddingClient) Embed(ctx context.Context, req embedding.EmbedRequest) (*embedding.EmbedResponse, error) {
	return &embedding.EmbedResponse{Embeddings: s.embeddings}, nil
}
