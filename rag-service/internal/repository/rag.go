package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"example.com/aim/rag-service/internal/dal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type KnowledgeChunkRecord struct {
	KnowledgeBaseID uint64
	DocumentID      uint64
	ChunkIndex      int
	Content         string
	Metadata        json.RawMessage
	TokenCount      int
	Embedding       []float32
}

type RAGRepository interface {
	WithTx(tx *gorm.DB) RAGRepository
	CreateKnowledgeBase(ctx context.Context, kb *model.KnowledgeBase) error
	ListKnowledgeBasesByOwner(ctx context.Context, ownerID uint64) ([]model.KnowledgeBase, error)
	GetKnowledgeBaseByID(ctx context.Context, kbID uint64) (*model.KnowledgeBase, error)
	CreateKnowledgeDocument(ctx context.Context, doc *model.KnowledgeDocument) error
	UpdateKnowledgeDocument(ctx context.Context, doc *model.KnowledgeDocument) error
	ListKnowledgeDocumentsByKBID(ctx context.Context, kbID uint64) ([]model.KnowledgeDocument, error)
	GetKnowledgeDocumentByID(ctx context.Context, documentID uint64) (*model.KnowledgeDocument, error)
	DeleteKnowledgeDocument(ctx context.Context, documentID uint64) error
	UpdateKnowledgeDocumentStatus(ctx context.Context, documentID uint64, status model.KnowledgeDocumentStatus, errorMessage string) error
	ReplaceKnowledgeChunksForDocument(ctx context.Context, documentID uint64, records []KnowledgeChunkRecord) error
	SearchKnowledgeChunkCandidatesByKB(ctx context.Context, kbID uint64, query string, queryEmbedding []float32, topK int) ([]KnowledgeChunkCandidate, error)
	SearchKnowledgeChunksByKB(ctx context.Context, kbID uint64, query string, queryEmbedding []float32, topK int) ([]KnowledgeSearchChunk, error)
	IsKnowledgeBaseAccessibleByUser(ctx context.Context, kbID uint64, userID uint64) (bool, error)
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

type KnowledgeChunkCandidate struct {
	ChunkID        uint64
	DocumentID     uint64
	DocumentTitle  string
	ChunkIndex     int
	Content        string
	Metadata       json.RawMessage
	VectorDistance float64
	HasVector      bool
	FullTextRank   float64
	HasFullText    bool
	TitleMatched   bool
	SectionMatched bool
}

var (
	stopwordCleanerV2      = regexp.MustCompile(`(?:的|是|什么|内容|一下|请问|帮我|介绍|总结|讲了什么|主要内容|大概|概括)`)
	punctuationCleanerV2   = regexp.MustCompile(`[^\p{Han}a-z0-9]+`)
	searchKeywordPatternV2 = regexp.MustCompile(`[\p{Han}]{2,}|[a-z0-9][a-z0-9._+-]{1,}`)
	reQuestionNumberV2     = regexp.MustCompile(`(?:第\s*)?(\d{1,6})\s*题`)
)

func NewRAGRepository(db *gorm.DB) *GormRAGRepository {
	return &GormRAGRepository{db: db}
}

func (r *GormRAGRepository) WithTx(tx *gorm.DB) RAGRepository {
	return &GormRAGRepository{db: tx}
}

func (r *GormRAGRepository) CreateKnowledgeBase(ctx context.Context, kb *model.KnowledgeBase) error {
	return r.db.WithContext(ctx).Create(kb).Error
}

func (r *GormRAGRepository) ListKnowledgeBasesByOwner(ctx context.Context, ownerID uint64) ([]model.KnowledgeBase, error) {
	var items []model.KnowledgeBase
	err := r.db.WithContext(ctx).
		Where("owner_id = ?", ownerID).
		Order("id DESC").
		Find(&items).Error
	return items, err
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

func (r *GormRAGRepository) DeleteKnowledgeDocument(ctx context.Context, documentID uint64) error {
	if documentID == 0 {
		return errors.New("document id is required")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM knowledge_chunks WHERE document_id = ?", documentID).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", documentID).Delete(&model.KnowledgeDocument{}).Error
	})
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
    (knowledge_base_id, document_id, chunk_index, content, metadata, token_count, embedding, created_at, updated_at)
VALUES
    (?, ?, ?, ?, ?::jsonb, ?, ?::vector, now(), now())
`, record.KnowledgeBaseID, record.DocumentID, record.ChunkIndex, record.Content, jsonOrEmptyObject(record.Metadata), record.TokenCount, embeddingLiteral).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *GormRAGRepository) SearchKnowledgeChunkCandidatesByKB(ctx context.Context, kbID uint64, query string, queryEmbedding []float32, topK int) ([]KnowledgeChunkCandidate, error) {
	if kbID == 0 {
		return nil, errors.New("knowledge base id is required")
	}
	if topK <= 0 {
		topK = 5
	}
	if topK > 30 {
		topK = 30
	}
	embeddingLiteral, err := float32SliceToVectorLiteral(queryEmbedding)
	if err != nil {
		return nil, err
	}
	query = strings.TrimSpace(query)
	titleQuery, sectionQuery, questionNoQuery := normalizeSearchQuery(query)
	if titleQuery == "" {
		titleQuery = "__no_title_match__"
	}
	if sectionQuery == "" {
		sectionQuery = "__no_section_match__"
	}
	vectorLimit := topK
	if vectorLimit < 30 {
		vectorLimit = 30
	}
	fulltextLimit := topK * 3
	if fulltextLimit < 30 {
		fulltextLimit = 30
	}
	titleLimit := topK * 4
	if titleLimit < 30 {
		titleLimit = 30
	}
	sectionLimit := topK * 4
	if sectionLimit < 30 {
		sectionLimit = 30
	}

	type row struct {
		ChunkID        uint64          `gorm:"column:chunk_id"`
		DocumentID     uint64          `gorm:"column:document_id"`
		DocumentTitle  string          `gorm:"column:document_title"`
		ChunkIndex     int             `gorm:"column:chunk_index"`
		Content        string          `gorm:"column:content"`
		Metadata       json.RawMessage `gorm:"column:metadata"`
		VectorDistance *float64        `gorm:"column:vector_distance"`
		FullTextRank   *float64        `gorm:"column:fulltext_rank"`
		TitleMatched   bool            `gorm:"column:title_matched"`
		SectionMatched bool            `gorm:"column:section_matched"`
	}

	merged := make(map[uint64]*KnowledgeChunkCandidate)
	appendRow := func(item row) {
		candidate, ok := merged[item.ChunkID]
		if !ok {
			candidate = &KnowledgeChunkCandidate{
				ChunkID:       item.ChunkID,
				DocumentID:    item.DocumentID,
				DocumentTitle: strings.TrimSpace(item.DocumentTitle),
				ChunkIndex:    item.ChunkIndex,
				Content:       item.Content,
				Metadata:      append(json.RawMessage(nil), item.Metadata...),
			}
			merged[item.ChunkID] = candidate
		}
		if candidate.DocumentTitle == "" {
			candidate.DocumentTitle = strings.TrimSpace(item.DocumentTitle)
		}
		if candidate.Content == "" {
			candidate.Content = item.Content
		}
		if len(candidate.Metadata) == 0 && len(item.Metadata) > 0 {
			candidate.Metadata = append(json.RawMessage(nil), item.Metadata...)
		}
		if item.VectorDistance != nil {
			candidate.VectorDistance = *item.VectorDistance
			candidate.HasVector = true
		}
		if item.FullTextRank != nil {
			candidate.FullTextRank = *item.FullTextRank
			candidate.HasFullText = true
		}
		if item.TitleMatched {
			candidate.TitleMatched = true
		}
		if item.SectionMatched {
			candidate.SectionMatched = true
		}
	}

	var vectorRows []row
	if err := r.db.WithContext(ctx).Raw(`
SELECT
    kc.id AS chunk_id,
    kc.document_id,
    kd.title AS document_title,
    kc.chunk_index,
    kc.content,
    kc.metadata,
    kc.embedding <=> ?::vector AS vector_distance,
    NULL::float8 AS fulltext_rank,
    FALSE AS title_matched,
    FALSE AS section_matched
FROM knowledge_chunks kc
JOIN knowledge_documents kd ON kd.id = kc.document_id
WHERE kc.knowledge_base_id = ?
  AND kd.status = ?
ORDER BY kc.embedding <=> ?::vector, kc.id DESC
LIMIT ?;
`, embeddingLiteral, kbID, model.KnowledgeDocumentStatusReady, embeddingLiteral, vectorLimit).Scan(&vectorRows).Error; err != nil {
		return nil, err
	}
	for _, item := range vectorRows {
		appendRow(item)
	}

	if query != "" {
		var fulltextRows []row
		if err := r.db.WithContext(ctx).Raw(`
SELECT
    kc.id AS chunk_id,
    kc.document_id,
    kd.title AS document_title,
    kc.chunk_index,
    kc.content,
    kc.metadata,
    NULL::float8 AS vector_distance,
    ts_rank_cd(
        setweight(to_tsvector('simple', coalesce(kd.title, '')), 'A') ||
        setweight(to_tsvector('simple', coalesce(kc.metadata->>'sectionTitle', '')), 'B') ||
        setweight(to_tsvector('simple', coalesce(kc.metadata->>'questionText', '')), 'B') ||
        setweight(to_tsvector('simple', coalesce(kc.content, '')), 'C'),
        plainto_tsquery('simple', ?)
    ) AS fulltext_rank,
    FALSE AS title_matched,
    FALSE AS section_matched
FROM knowledge_chunks kc
JOIN knowledge_documents kd ON kd.id = kc.document_id
WHERE kc.knowledge_base_id = ?
  AND kd.status = ?
  AND (
        (
            setweight(to_tsvector('simple', coalesce(kd.title, '')), 'A') ||
            setweight(to_tsvector('simple', coalesce(kc.metadata->>'sectionTitle', '')), 'B') ||
            setweight(to_tsvector('simple', coalesce(kc.metadata->>'questionText', '')), 'B') ||
            setweight(to_tsvector('simple', coalesce(kc.content, '')), 'C')
        ) @@ plainto_tsquery('simple', ?)
        OR coalesce(kd.title, '') ILIKE ('%%' || ? || '%%')
  )
ORDER BY fulltext_rank DESC, kc.id DESC
LIMIT ?;
`, query, kbID, model.KnowledgeDocumentStatusReady, query, query, fulltextLimit).Scan(&fulltextRows).Error; err != nil {
			return nil, err
		}
		for _, item := range fulltextRows {
			appendRow(item)
		}

		var titleRows []row
		if err := r.db.WithContext(ctx).Raw(`
SELECT
    kc.id AS chunk_id,
    kc.document_id,
    kd.title AS document_title,
    kc.chunk_index,
    kc.content,
    kc.metadata,
    NULL::float8 AS vector_distance,
    NULL::float8 AS fulltext_rank,
    TRUE AS title_matched,
    FALSE AS section_matched
FROM knowledge_documents kd
JOIN knowledge_chunks kc ON kc.document_id = kd.id
WHERE kd.knowledge_base_id = ?
  AND kd.status = ?
  AND (
        kd.title ILIKE ('%%' || ? || '%%')
        OR kd.title ILIKE ('%%' || ? || '%%')
  )
ORDER BY kd.id DESC, kc.chunk_index ASC
LIMIT ?;
`, kbID, model.KnowledgeDocumentStatusReady, query, titleQuery, titleLimit).Scan(&titleRows).Error; err != nil {
			return nil, err
		}
		for _, item := range titleRows {
			appendRow(item)
		}

		var sectionRows []row
		if err := r.db.WithContext(ctx).Raw(`
SELECT
    kc.id AS chunk_id,
    kc.document_id,
    kd.title AS document_title,
    kc.chunk_index,
    kc.content,
    kc.metadata,
    NULL::float8 AS vector_distance,
    NULL::float8 AS fulltext_rank,
    FALSE AS title_matched,
    TRUE AS section_matched
FROM knowledge_chunks kc
JOIN knowledge_documents kd ON kd.id = kc.document_id
WHERE kc.knowledge_base_id = ?
  AND kd.status = ?
  AND (
        COALESCE(kc.metadata->>'sectionTitle', '') ILIKE ('%%' || ? || '%%')
        OR COALESCE(kc.metadata->>'sectionTitle', '') ILIKE ('%%' || ? || '%%')
        OR COALESCE(kc.metadata->>'questionText', '') ILIKE ('%%' || ? || '%%')
        OR COALESCE(kc.metadata->>'questionText', '') ILIKE ('%%' || ? || '%%')
        OR (? <> '' AND COALESCE(kc.metadata->>'questionNo', '') = ?)
  )
ORDER BY kc.document_id DESC, kc.chunk_index ASC
LIMIT ?;
`, kbID, model.KnowledgeDocumentStatusReady, query, sectionQuery, query, sectionQuery, questionNoQuery, questionNoQuery, sectionLimit).Scan(&sectionRows).Error; err != nil {
			return nil, err
		}
		for _, item := range sectionRows {
			appendRow(item)
		}
	}

	result := make([]KnowledgeChunkCandidate, 0, len(merged))
	for _, item := range merged {
		result = append(result, *item)
	}
	return result, nil
}

func (r *GormRAGRepository) SearchKnowledgeChunksByKB(ctx context.Context, kbID uint64, query string, queryEmbedding []float32, topK int) ([]KnowledgeSearchChunk, error) {
	candidates, err := r.SearchKnowledgeChunkCandidatesByKB(ctx, kbID, query, queryEmbedding, topK)
	if err != nil {
		return nil, err
	}
	result := make([]KnowledgeSearchChunk, 0, len(candidates))
	for _, item := range candidates {
		distance := item.VectorDistance
		if !item.HasVector {
			distance = 1
		}
		result = append(result, KnowledgeSearchChunk{
			ChunkID:    item.ChunkID,
			DocumentID: item.DocumentID,
			Content:    item.Content,
			Distance:   distance,
		})
	}
	return result, nil
}

func (r *GormRAGRepository) IsKnowledgeBaseAccessibleByUser(ctx context.Context, kbID uint64, userID uint64) (bool, error) {
	if kbID == 0 || userID == 0 {
		return false, errors.New("knowledge base id and user id are required")
	}
	var count int64
	err := r.db.WithContext(ctx).
		Table("conversation_knowledge_bases AS ckb").
		Joins("JOIN conversation_members AS cm ON cm.conversation_id = ckb.conversation_id").
		Where(
			"ckb.knowledge_base_id = ? AND ckb.enabled = ? AND cm.member_type = ? AND cm.member_id = ? AND cm.status <> ?",
			kbID,
			true,
			model.MemberTypeUser,
			userID,
			model.MemberStatusRemoved,
		).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
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

func jsonOrEmptyObject(value json.RawMessage) string {
	trimmed := strings.TrimSpace(string(value))
	if trimmed == "" {
		return "{}"
	}
	return trimmed
}

func normalizeSearchQuery(query string) (string, string, string) {
	trimmed := strings.TrimSpace(strings.ToLower(query))
	if trimmed == "" {
		return "", "", ""
	}
	core := stopwordCleanerV2.ReplaceAllString(trimmed, " ")
	core = punctuationCleanerV2.ReplaceAllString(core, " ")
	tokens := searchKeywordPatternV2.FindAllString(core, -1)
	if len(tokens) > 0 {
		core = strings.Join(tokens, " ")
	} else {
		core = strings.Join(strings.Fields(core), " ")
	}
	core = strings.TrimSpace(core)
	if core == "" {
		core = trimmed
	}
	section := strings.ReplaceAll(core, " ", "")
	questionNo := ""
	if matches := reQuestionNumberV2.FindStringSubmatch(trimmed); len(matches) == 2 {
		questionNo = matches[1]
	}
	return core, section, questionNo
}

var (
	stopwordCleaner  = regexp.MustCompile(`\b(?:的|是|什么|内容|一下|请问|帮我|介绍|总结|讲了什么)\b`)
	reQuestionNumber = regexp.MustCompile(`(?:第)?\s*(\d{1,6})\s*题?`)
)
