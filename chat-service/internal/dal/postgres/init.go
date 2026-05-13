package postgres

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"example.com/aim/chat-service/internal/dal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func Init(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	if err := rebuildConversationMemberSchemaIfNeeded(db); err != nil {
		return nil, err
	}

	if err := db.AutoMigrate(
		&model.Conversation{},
		&model.GroupInfo{},
		&model.ConversationMember{},
		&model.Message{},
		&model.Bot{},
		&model.ConversationBot{},
		&model.AICallLog{},
		&model.KnowledgeBase{},
		&model.KnowledgeDocument{},
		&model.ConversationKnowledgeBase{},
	); err != nil {
		return nil, err
	}
	embeddingDim, err := embeddingDimensionFromEnv()
	if err != nil {
		return nil, err
	}
	if err := ensureKnowledgeChunksSchema(db, embeddingDim); err != nil {
		return nil, err
	}
	if err := backfillConversationIDs(db); err != nil {
		return nil, err
	}

	return db, nil
}

func embeddingDimensionFromEnv() (int, error) {
	value := strings.TrimSpace(os.Getenv("EMBEDDING_DIMENSION"))
	if value == "" {
		return 1536, nil
	}
	dimension, err := strconv.Atoi(value)
	if err != nil || dimension <= 0 {
		return 0, errors.New("EMBEDDING_DIMENSION must be a positive integer")
	}
	return dimension, nil
}

func ensureKnowledgeChunksSchema(db *gorm.DB, embeddingDim int) error {
	createTableSQL := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS knowledge_chunks (
    id BIGSERIAL PRIMARY KEY,
    knowledge_base_id BIGINT NOT NULL,
    document_id BIGINT NOT NULL,
    chunk_index INT NOT NULL,
    content TEXT NOT NULL,
    token_count INT NOT NULL DEFAULT 0,
    embedding vector(%d) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);`, embeddingDim)
	if err := db.Exec(createTableSQL).Error; err != nil {
		return fmt.Errorf("create knowledge_chunks table failed: %w", err)
	}
	if err := db.Exec(`
CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_kb_id
ON knowledge_chunks (knowledge_base_id);`).Error; err != nil {
		return fmt.Errorf("create idx_knowledge_chunks_kb_id failed: %w", err)
	}
	if err := db.Exec(`
CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_document_id
ON knowledge_chunks (document_id);`).Error; err != nil {
		return fmt.Errorf("create idx_knowledge_chunks_document_id failed: %w", err)
	}
	if err := db.Exec(`
CREATE UNIQUE INDEX IF NOT EXISTS idx_knowledge_chunks_document_index
ON knowledge_chunks (document_id, chunk_index);`).Error; err != nil {
		return fmt.Errorf("create idx_knowledge_chunks_document_index failed: %w", err)
	}
	if err := assertKnowledgeChunksEmbeddingDimension(db, embeddingDim); err != nil {
		return err
	}
	return nil
}

func assertKnowledgeChunksEmbeddingDimension(db *gorm.DB, expected int) error {
	type result struct {
		EmbeddingType string
	}
	var rows []result
	if err := db.Raw(`
SELECT format_type(a.atttypid, a.atttypmod) AS embedding_type
FROM pg_attribute a
JOIN pg_class c ON c.oid = a.attrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE c.relname = 'knowledge_chunks'
  AND n.nspname = current_schema()
  AND a.attname = 'embedding'
  AND a.attnum > 0
  AND NOT a.attisdropped
LIMIT 1;`).Scan(&rows).Error; err != nil {
		return fmt.Errorf("query knowledge_chunks.embedding type failed: %w", err)
	}
	if len(rows) == 0 {
		return errors.New("knowledge_chunks.embedding column not found")
	}
	expectedType := fmt.Sprintf("vector(%d)", expected)
	actualType := strings.TrimSpace(rows[0].EmbeddingType)
	if !strings.EqualFold(actualType, expectedType) {
		return fmt.Errorf("knowledge_chunks.embedding dimension mismatch: expected %s, got %s", expectedType, actualType)
	}
	return nil
}

func rebuildConversationMemberSchemaIfNeeded(db *gorm.DB) error {
	if !db.Migrator().HasTable(&model.ConversationMember{}) {
		return nil
	}
	if !db.Migrator().HasColumn("conversation_members", "user_id") {
		return nil
	}
	return db.Migrator().DropTable(&model.ConversationMember{})
}

func backfillConversationIDs(db *gorm.DB) error {
	var ids []uint64
	if err := db.Model(&model.Conversation{}).
		Where("conversation_id IS NULL OR conversation_id = ?", "").
		Pluck("id", &ids).Error; err != nil {
		return err
	}

	for _, id := range ids {
		if err := assignConversationID(db, id); err != nil {
			return err
		}
	}
	return nil
}

func assignConversationID(db *gorm.DB, id uint64) error {
	var lastErr error
	for i := 0; i < 5; i++ {
		conversationID, err := newConversationID()
		if err != nil {
			return err
		}
		result := db.Model(&model.Conversation{}).
			Where("id = ? AND (conversation_id IS NULL OR conversation_id = ?)", id, "").
			Update("conversation_id", conversationID)
		if result.Error != nil {
			lastErr = result.Error
			continue
		}
		return nil
	}
	if lastErr != nil {
		return lastErr
	}
	return errors.New("failed to assign conversation_id")
}

func newConversationID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	value := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:])
	return "c_" + strings.ToLower(value), nil
}

type BuiltInBotConfig struct {
	ID              uint64
	Name            string
	MentionName     string
	Aliases         []string
	Description     string
	ModelName       string
	SupportedModels []string
	SystemPrompt    string
	Avatar          string
}

func EnsureBuiltInBot(db *gorm.DB, cfg BuiltInBotConfig) error {
	if db == nil {
		return errors.New("db is nil")
	}
	if cfg.ID == 0 {
		return errors.New("built-in bot id is required")
	}
	name := strings.TrimSpace(cfg.Name)
	mentionName := strings.ToLower(strings.TrimSpace(cfg.MentionName))
	modelName := strings.TrimSpace(cfg.ModelName)
	if name == "" || mentionName == "" || modelName == "" {
		return errors.New("built-in bot name, mentionName, and modelName are required")
	}

	aliasesText, err := marshalStringList(cfg.Aliases)
	if err != nil {
		return err
	}
	supportedModelsText, err := marshalStringList(cfg.SupportedModels)
	if err != nil {
		return err
	}
	if supportedModelsText == "" {
		supportedModelsText, err = marshalStringList([]string{modelName})
		if err != nil {
			return err
		}
	}

	botModel := model.Bot{
		ID:              cfg.ID,
		Name:            name,
		MentionName:     mentionName,
		Aliases:         aliasesText,
		Avatar:          strings.TrimSpace(cfg.Avatar),
		Description:     strings.TrimSpace(cfg.Description),
		ModelName:       modelName,
		SupportedModels: supportedModelsText,
		SystemPrompt:    strings.TrimSpace(cfg.SystemPrompt),
		CreatedBy:       0,
		Status:          model.BotStatusEnabled,
	}

	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"name",
			"mention_name",
			"aliases",
			"avatar",
			"description",
			"model_name",
			"supported_models",
			"system_prompt",
			"created_by",
			"status",
			"updated_at",
		}),
	}).Create(&botModel).Error
}

func marshalStringList(values []string) (string, error) {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	if len(normalized) == 0 {
		return "", nil
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
