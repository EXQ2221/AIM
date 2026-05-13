package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"example.com/aim/chat-service/internal/dal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type KnowledgeChunkRecord struct {
	KnowledgeBaseID uint64
	DocumentID      uint64
	ChunkIndex      int
	Content         string
	TokenCount      int
	Embedding       []float32
}

type RAGRepository interface {
	WithTx(tx *gorm.DB) RAGRepository
	CreateKnowledgeBase(ctx context.Context, kb *model.KnowledgeBase) error
	GetKnowledgeBaseByID(ctx context.Context, kbID uint64) (*model.KnowledgeBase, error)
	CreateKnowledgeDocument(ctx context.Context, doc *model.KnowledgeDocument) error
	UpdateKnowledgeDocument(ctx context.Context, doc *model.KnowledgeDocument) error
	ListKnowledgeDocumentsByKBID(ctx context.Context, kbID uint64) ([]model.KnowledgeDocument, error)
	GetKnowledgeDocumentByID(ctx context.Context, documentID uint64) (*model.KnowledgeDocument, error)
	UpdateKnowledgeDocumentStatus(ctx context.Context, documentID uint64, status model.KnowledgeDocumentStatus, errorMessage string) error
	ReplaceKnowledgeChunksForDocument(ctx context.Context, documentID uint64, records []KnowledgeChunkRecord) error
	SearchKnowledgeChunksByKB(ctx context.Context, kbID uint64, queryEmbedding []float32, topK int) ([]KnowledgeSearchChunk, error)
	UpsertConversationKnowledgeBase(ctx context.Context, conversationID uint64, knowledgeBaseID uint64, createdBy uint64, enabled bool) error
	ListConversationKnowledgeBases(ctx context.Context, conversationID uint64) ([]model.ConversationKnowledgeBase, error)
	GetConversationKnowledgeBase(ctx context.Context, conversationID uint64, knowledgeBaseID uint64) (*model.ConversationKnowledgeBase, error)
	UpdateConversationKnowledgeBaseEnabled(ctx context.Context, conversationID uint64, knowledgeBaseID uint64, enabled bool) error
}

type GormRAGRepository struct {
	db *gorm.DB
}

type KnowledgeSearchChunk struct {
	ChunkID    uint64
	DocumentID uint64
	Content    string
	Distance   float64
}

func NewRAGRepository(db *gorm.DB) *GormRAGRepository {
	return &GormRAGRepository{db: db}
}

func (r *GormRAGRepository) WithTx(tx *gorm.DB) RAGRepository {
	return &GormRAGRepository{db: tx}
}

func (r *GormRAGRepository) CreateKnowledgeBase(ctx context.Context, kb *model.KnowledgeBase) error {
	return r.db.WithContext(ctx).Create(kb).Error
}

func (r *GormRAGRepository) GetKnowledgeBaseByID(ctx context.Context, kbID uint64) (*model.KnowledgeBase, error) {
	var kb model.KnowledgeBase
	if err := r.db.WithContext(ctx).First(&kb, kbID).Error; err != nil {
		return nil, err
	}
	return &kb, nil
}

func (r *GormRAGRepository) CreateKnowledgeDocument(ctx context.Context, doc *model.KnowledgeDocument) error {
	return r.db.WithContext(ctx).Create(doc).Error
}

func (r *GormRAGRepository) UpdateKnowledgeDocument(ctx context.Context, doc *model.KnowledgeDocument) error {
	return r.db.WithContext(ctx).Save(doc).Error
}

func (r *GormRAGRepository) ListKnowledgeDocumentsByKBID(ctx context.Context, kbID uint64) ([]model.KnowledgeDocument, error) {
	var docs []model.KnowledgeDocument
	err := r.db.WithContext(ctx).
		Where("knowledge_base_id = ?", kbID).
		Order("id DESC").
		Find(&docs).Error
	return docs, err
}

func (r *GormRAGRepository) GetKnowledgeDocumentByID(ctx context.Context, documentID uint64) (*model.KnowledgeDocument, error) {
	var document model.KnowledgeDocument
	if err := r.db.WithContext(ctx).First(&document, documentID).Error; err != nil {
		return nil, err
	}
	return &document, nil
}

func (r *GormRAGRepository) UpdateKnowledgeDocumentStatus(ctx context.Context, documentID uint64, status model.KnowledgeDocumentStatus, errorMessage string) error {
	return r.db.WithContext(ctx).
		Model(&model.KnowledgeDocument{}).
		Where("id = ?", documentID).
		Updates(map[string]interface{}{
			"status":        status,
			"error_message": strings.TrimSpace(errorMessage),
			"updated_at":    time.Now(),
		}).Error
}

func (r *GormRAGRepository) ReplaceKnowledgeChunksForDocument(ctx context.Context, documentID uint64, records []KnowledgeChunkRecord) error {
	if documentID == 0 {
		return errors.New("document id is required")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM knowledge_chunks WHERE document_id = ?", documentID).Error; err != nil {
			return err
		}
		for _, record := range records {
			if strings.TrimSpace(record.Content) == "" {
				continue
			}
			embeddingLiteral, err := float32SliceToVectorLiteral(record.Embedding)
			if err != nil {
				return err
			}
			if err := tx.Exec(`
INSERT INTO knowledge_chunks
    (knowledge_base_id, document_id, chunk_index, content, token_count, embedding, created_at, updated_at)
VALUES
    (?, ?, ?, ?, ?, ?::vector, now(), now())
`, record.KnowledgeBaseID, record.DocumentID, record.ChunkIndex, record.Content, record.TokenCount, embeddingLiteral).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *GormRAGRepository) SearchKnowledgeChunksByKB(ctx context.Context, kbID uint64, queryEmbedding []float32, topK int) ([]KnowledgeSearchChunk, error) {
	if kbID == 0 {
		return nil, errors.New("knowledge base id is required")
	}
	if topK <= 0 {
		topK = 5
	}
	if topK > 10 {
		topK = 10
	}
	embeddingLiteral, err := float32SliceToVectorLiteral(queryEmbedding)
	if err != nil {
		return nil, err
	}

	type row struct {
		ChunkID    uint64  `gorm:"column:chunk_id"`
		DocumentID uint64  `gorm:"column:document_id"`
		Content    string  `gorm:"column:content"`
		Distance   float64 `gorm:"column:distance"`
	}
	var rows []row
	if err := r.db.WithContext(ctx).Raw(`
SELECT
    id AS chunk_id,
    document_id,
    content,
    embedding <=> ?::vector AS distance
FROM knowledge_chunks
WHERE knowledge_base_id = ?
ORDER BY embedding <=> ?::vector
LIMIT ?;
`, embeddingLiteral, kbID, embeddingLiteral, topK).Scan(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]KnowledgeSearchChunk, 0, len(rows))
	for _, item := range rows {
		result = append(result, KnowledgeSearchChunk{
			ChunkID:    item.ChunkID,
			DocumentID: item.DocumentID,
			Content:    item.Content,
			Distance:   item.Distance,
		})
	}
	return result, nil
}

func (r *GormRAGRepository) UpsertConversationKnowledgeBase(ctx context.Context, conversationID uint64, knowledgeBaseID uint64, createdBy uint64, enabled bool) error {
	record := model.ConversationKnowledgeBase{
		ConversationID:  conversationID,
		KnowledgeBaseID: knowledgeBaseID,
		Enabled:         enabled,
		CreatedBy:       createdBy,
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "conversation_id"},
			{Name: "knowledge_base_id"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"enabled":    enabled,
			"updated_at": time.Now(),
		}),
	}).Create(&record).Error
}

func (r *GormRAGRepository) ListConversationKnowledgeBases(ctx context.Context, conversationID uint64) ([]model.ConversationKnowledgeBase, error) {
	var records []model.ConversationKnowledgeBase
	err := r.db.WithContext(ctx).
		Where("conversation_id = ? AND enabled = ?", conversationID, true).
		Order("id DESC").
		Find(&records).Error
	return records, err
}

func (r *GormRAGRepository) GetConversationKnowledgeBase(ctx context.Context, conversationID uint64, knowledgeBaseID uint64) (*model.ConversationKnowledgeBase, error) {
	var record model.ConversationKnowledgeBase
	err := r.db.WithContext(ctx).
		Where("conversation_id = ? AND knowledge_base_id = ?", conversationID, knowledgeBaseID).
		First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *GormRAGRepository) UpdateConversationKnowledgeBaseEnabled(ctx context.Context, conversationID uint64, knowledgeBaseID uint64, enabled bool) error {
	return r.db.WithContext(ctx).
		Model(&model.ConversationKnowledgeBase{}).
		Where("conversation_id = ? AND knowledge_base_id = ?", conversationID, knowledgeBaseID).
		Updates(map[string]interface{}{
			"enabled":    enabled,
			"updated_at": time.Now(),
		}).Error
}

func float32SliceToVectorLiteral(vector []float32) (string, error) {
	if len(vector) == 0 {
		return "", errors.New("embedding vector is empty")
	}
	builder := strings.Builder{}
	builder.Grow(len(vector) * 8)
	builder.WriteByte('[')
	for i, value := range vector {
		if i > 0 {
			builder.WriteByte(',')
		}
		builder.WriteString(strconv.FormatFloat(float64(value), 'f', -1, 32))
	}
	builder.WriteByte(']')
	literal := builder.String()
	if len(literal) < 3 {
		return "", fmt.Errorf("invalid embedding vector literal: %q", literal)
	}
	return literal, nil
}
