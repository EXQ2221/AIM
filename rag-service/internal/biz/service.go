package rag

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"example.com/aim/rag-service/internal/dal/model"
	embedding "example.com/aim/rag-service/internal/provider"
	"example.com/aim/rag-service/internal/repository"
)

type DocumentProcessor struct {
	EmbeddingClient embedding.Client
	RAGRepo         repository.RAGRepository
	SplitterConfig  SplitterConfig
	EmbedBatchSize  int
}

const maxEmbeddingBatchSize = 10

func NewDocumentProcessor(client embedding.Client, ragRepo repository.RAGRepository, splitterCfg SplitterConfig) *DocumentProcessor {
	return &DocumentProcessor{
		EmbeddingClient: client,
		RAGRepo:         ragRepo,
		SplitterConfig:  splitterCfg,
		EmbedBatchSize:  maxEmbeddingBatchSize,
	}
}

type ProcessDocumentInput struct {
	DocumentID uint64
	Content    string
	Chunks     []IngestChunkInput
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

	chunks, err := p.buildChunks(input)
	if err != nil {
		p.updateDocumentStatus(ctx, input.DocumentID, model.KnowledgeDocumentStatusFailed, err.Error())
		return err
	}

	batchSize := p.EmbedBatchSize
	if batchSize <= 0 {
		batchSize = maxEmbeddingBatchSize
	}
	if batchSize > maxEmbeddingBatchSize {
		batchSize = maxEmbeddingBatchSize
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
			metadataBytes, marshalErr := json.Marshal(item.Metadata)
			if marshalErr != nil {
				metadataBytes = []byte("{}")
			}
			records = append(records, repository.KnowledgeChunkRecord{
				KnowledgeBaseID: document.KnowledgeBaseID,
				DocumentID:      document.ID,
				ChunkIndex:      item.Index,
				Content:         chunkText,
				Metadata:        metadataBytes,
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

func (p *DocumentProcessor) buildChunks(input ProcessDocumentInput) ([]Chunk, error) {
	if len(input.Chunks) > 0 {
		if chunks, err := p.fromProvidedChunks(input.Chunks); err == nil && len(chunks) > 0 {
			return chunks, nil
		}
	}
	return SplitText(input.Content, p.SplitterConfig)
}

func (p *DocumentProcessor) fromProvidedChunks(items []IngestChunkInput) ([]Chunk, error) {
	chunks := make([]Chunk, 0, len(items))
	for idx, item := range items {
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		chunkType := strings.ToUpper(strings.TrimSpace(item.ChunkType))
		documentType := DocumentTypePlainText
		switch chunkType {
		case string(DocumentTypeQuestionBank):
			documentType = DocumentTypeQuestionBank
		case string(DocumentTypeScript):
			documentType = DocumentTypeScript
		case string(DocumentTypeMarkdown):
			documentType = DocumentTypeMarkdown
		default:
			documentType = DocumentTypePlainText
		}

		chunkIndex := item.Index
		if chunkIndex < 0 {
			chunkIndex = idx
		}
		sectionTitle := strings.TrimSpace(item.SectionTitle)
		if sectionTitle == "" {
			sectionTitle = fmt.Sprintf("Chunk %d", idx+1)
		}

		meta := ChunkMetadata{
			DocumentType: documentType,
			SectionTitle: sectionTitle,
		}
		if documentType == DocumentTypeQuestionBank {
			meta.QuestionNo = questionNoFromSection(sectionTitle)
			meta.QuestionText = firstLine(content)
		}
		chunks = append(chunks, Chunk{
			Index:    chunkIndex,
			Content:  content,
			Metadata: meta,
		})
	}
	if len(chunks) == 0 {
		return nil, errors.New("document content is empty")
	}
	return chunks, nil
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
