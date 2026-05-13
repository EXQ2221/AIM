package bot

import (
	"context"
	"errors"
	"strings"

	"example.com/aim/chat-service/internal/embedding"
	"example.com/aim/chat-service/internal/repository"
)

type ConversationRAGSearcher struct {
	EmbeddingClient embedding.Client
	RAGRepo         repository.RAGRepository
}

func NewConversationRAGSearcher(embedClient embedding.Client, ragRepo repository.RAGRepository) *ConversationRAGSearcher {
	return &ConversationRAGSearcher{
		EmbeddingClient: embedClient,
		RAGRepo:         ragRepo,
	}
}

func (s *ConversationRAGSearcher) SearchForConversation(ctx context.Context, req RAGSearchRequest) ([]RAGChunk, error) {
	if s == nil || s.EmbeddingClient == nil || s.RAGRepo == nil {
		return nil, errors.New("rag searcher dependencies are not complete")
	}
	if req.ConversationID == 0 {
		return nil, errors.New("conversation id is required")
	}
	question := strings.TrimSpace(req.Question)
	if question == "" {
		return nil, nil
	}
	topK := req.TopK
	if topK <= 0 {
		topK = 5
	}
	if topK > 10 {
		topK = 10
	}

	bindings, err := s.RAGRepo.ListConversationKnowledgeBases(ctx, req.ConversationID)
	if err != nil {
		return nil, err
	}
	if len(bindings) == 0 {
		return nil, nil
	}

	embedResp, err := s.EmbeddingClient.Embed(ctx, embedding.EmbedRequest{
		Model: "",
		Input: []embedding.InputPart{
			{
				Type: embedding.InputPartText,
				Text: question,
			},
		},
	})
	if err != nil {
		return nil, err
	}
	if len(embedResp.Embeddings) != 1 {
		return nil, errors.New("invalid embedding result")
	}
	queryEmbedding := embedResp.Embeddings[0]

	all := make([]RAGChunk, 0, len(bindings)*topK)
	for _, binding := range bindings {
		chunks, searchErr := s.RAGRepo.SearchKnowledgeChunksByKB(ctx, binding.KnowledgeBaseID, queryEmbedding, topK)
		if searchErr != nil {
			return nil, searchErr
		}
		for _, item := range chunks {
			score := 1 - item.Distance
			all = append(all, RAGChunk{
				Index:   0,
				Content: item.Content,
				Score:   score,
			})
		}
	}
	if len(all) == 0 {
		return nil, nil
	}

	// Keep topK chunks globally across all bound KBs.
	sortByScoreDesc(all)
	if len(all) > topK {
		all = all[:topK]
	}
	for idx := range all {
		all[idx].Index = idx + 1
	}
	return all, nil
}

func sortByScoreDesc(chunks []RAGChunk) {
	for i := 0; i < len(chunks); i++ {
		for j := i + 1; j < len(chunks); j++ {
			if chunks[j].Score > chunks[i].Score {
				chunks[i], chunks[j] = chunks[j], chunks[i]
			}
		}
	}
}

var _ RAGSearcher = (*ConversationRAGSearcher)(nil)
