package rag

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"example.com/aim/rag-service/internal/dal/model"
	"example.com/aim/rag-service/internal/repository"
	embedding "example.com/aim/rag-service/rag-internal/client"
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
	documentTitle := strings.TrimSpace(document.Title)

	p.updateDocumentStatus(ctx, input.DocumentID, model.KnowledgeDocumentStatusProcessing, "")

	chunks, err := SplitText(input.Content, p.SplitterConfig)
	if err != nil {
		p.updateDocumentStatus(ctx, input.DocumentID, model.KnowledgeDocumentStatusFailed, err.Error())
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
			chunkText := strings.TrimSpace(chunk.Content)
			if documentTitle != "" {
				chunkText = fmt.Sprintf("【文档标题】%s\n%s", documentTitle, chunkText)
			}
			inputParts = append(inputParts, embedding.InputPart{
				Type: embedding.InputPartText,
				Text: chunkText,
			})
		}

		resp, embedErr := p.EmbeddingClient.Embed(ctx, embedding.EmbedRequest{
			Model: input.ModelName,
			Input: inputParts,
		})
		if embedErr != nil {
			p.updateDocumentStatus(ctx, input.DocumentID, model.KnowledgeDocumentStatusFailed, embedErr.Error())
			return fmt.Errorf("embed chunks failed: %w", embedErr)
		}
		if len(resp.Embeddings) != len(batch) {
			err = fmt.Errorf("embedding count mismatch: expected=%d got=%d", len(batch), len(resp.Embeddings))
			p.updateDocumentStatus(ctx, input.DocumentID, model.KnowledgeDocumentStatusFailed, err.Error())
			return err
		}

		for index, item := range batch {
			chunkText := strings.TrimSpace(item.Content)
			if documentTitle != "" {
				chunkText = fmt.Sprintf("【文档标题】%s\n%s", documentTitle, chunkText)
			}
			records = append(records, repository.KnowledgeChunkRecord{
				KnowledgeBaseID: document.KnowledgeBaseID,
				DocumentID:      document.ID,
				ChunkIndex:      item.Index,
				Content:         chunkText,
				TokenCount:      0,
				Embedding:       resp.Embeddings[index],
			})
		}
	}

	if err := p.RAGRepo.ReplaceKnowledgeChunksForDocument(ctx, document.ID, records); err != nil {
		p.updateDocumentStatus(ctx, input.DocumentID, model.KnowledgeDocumentStatusFailed, err.Error())
		return err
	}
	if err := p.RAGRepo.UpdateKnowledgeDocumentStatus(ctx, input.DocumentID, model.KnowledgeDocumentStatusReady, ""); err != nil {
		return err
	}
	return nil
}

func (p *DocumentProcessor) updateDocumentStatus(
	ctx context.Context,
	documentID uint64,
	status model.KnowledgeDocumentStatus,
	errMsg string,
) {
	if p == nil || p.RAGRepo == nil || documentID == 0 {
		return
	}
	if err := p.RAGRepo.UpdateKnowledgeDocumentStatus(ctx, documentID, status, errMsg); err == nil {
		return
	}
	if ctx == nil || ctx.Err() == nil {
		return
	}
	rescueCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = p.RAGRepo.UpdateKnowledgeDocumentStatus(rescueCtx, documentID, status, errMsg)
}
