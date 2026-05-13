package rag

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"example.com/aim/chat-service/internal/dal/model"
	"example.com/aim/chat-service/internal/embedding"
	"example.com/aim/chat-service/internal/repository"
)

type DocumentProcessor struct {
	EmbeddingClient embedding.Client
	RAGRepo         repository.RAGRepository
	SplitterConfig  SplitterConfig
	EmbedBatchSize  int
}

func NewDocumentProcessor(client embedding.Client, ragRepo repository.RAGRepository, splitterCfg SplitterConfig) *DocumentProcessor {
	return &DocumentProcessor{
		EmbeddingClient: client,
		RAGRepo:         ragRepo,
		SplitterConfig:  splitterCfg,
		EmbedBatchSize:  16,
	}
}

type ProcessDocumentInput struct {
	DocumentID uint64
	Content    string
	ModelName  string
}

func (p *DocumentProcessor) ProcessDocument(ctx context.Context, input ProcessDocumentInput) error {
	if p == nil || p.EmbeddingClient == nil || p.RAGRepo == nil {
		return errors.New("rag document processor is not initialized")
	}
	if input.DocumentID == 0 {
		return errors.New("document id is required")
	}
	if strings.TrimSpace(input.Content) == "" {
		return errors.New("document content is empty")
	}

	document, err := p.RAGRepo.GetKnowledgeDocumentByID(ctx, input.DocumentID)
	if err != nil {
		return err
	}

	_ = p.RAGRepo.UpdateKnowledgeDocumentStatus(ctx, input.DocumentID, model.KnowledgeDocumentStatusProcessing, "")

	chunks, err := SplitText(input.Content, p.SplitterConfig)
	if err != nil {
		_ = p.RAGRepo.UpdateKnowledgeDocumentStatus(ctx, input.DocumentID, model.KnowledgeDocumentStatusFailed, err.Error())
		return err
	}

	batchSize := p.EmbedBatchSize
	if batchSize <= 0 {
		batchSize = 16
	}

	records := make([]repository.KnowledgeChunkRecord, 0, len(chunks))
	for start := 0; start < len(chunks); start += batchSize {
		end := start + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}
		batch := chunks[start:end]

		inputParts := make([]embedding.InputPart, 0, len(batch))
		for _, chunk := range batch {
			inputParts = append(inputParts, embedding.InputPart{
				Type: embedding.InputPartText,
				Text: chunk.Content,
			})
		}

		resp, embedErr := p.EmbeddingClient.Embed(ctx, embedding.EmbedRequest{
			Model: input.ModelName,
			Input: inputParts,
		})
		if embedErr != nil {
			_ = p.RAGRepo.UpdateKnowledgeDocumentStatus(ctx, input.DocumentID, model.KnowledgeDocumentStatusFailed, embedErr.Error())
			return fmt.Errorf("embed chunks failed: %w", embedErr)
		}
		if len(resp.Embeddings) != len(batch) {
			err = fmt.Errorf("embedding count mismatch: expected=%d got=%d", len(batch), len(resp.Embeddings))
			_ = p.RAGRepo.UpdateKnowledgeDocumentStatus(ctx, input.DocumentID, model.KnowledgeDocumentStatusFailed, err.Error())
			return err
		}

		for index, item := range batch {
			records = append(records, repository.KnowledgeChunkRecord{
				KnowledgeBaseID: document.KnowledgeBaseID,
				DocumentID:      document.ID,
				ChunkIndex:      item.Index,
				Content:         item.Content,
				TokenCount:      0,
				Embedding:       resp.Embeddings[index],
			})
		}
	}

	if err := p.RAGRepo.ReplaceKnowledgeChunksForDocument(ctx, document.ID, records); err != nil {
		_ = p.RAGRepo.UpdateKnowledgeDocumentStatus(ctx, input.DocumentID, model.KnowledgeDocumentStatusFailed, err.Error())
		return err
	}
	if err := p.RAGRepo.UpdateKnowledgeDocumentStatus(ctx, input.DocumentID, model.KnowledgeDocumentStatusReady, ""); err != nil {
		return err
	}
	return nil
}
