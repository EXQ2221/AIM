package botdal

import (
	"context"
	"log"
	"sort"
	"strconv"
	"strings"

	bot "example.com/aim/chat-service/bot-internal/biz"
	ragpb "example.com/aim/chat-service/kitex_gen/rag"
	"example.com/aim/chat-service/kitex_gen/rag/ragservice"
	"example.com/aim/shared/errno"
)

type ConversationRAGSearcher struct {
	RAGClient       ragservice.Client
	Reranker        TextReranker
	RecallTopK      int
	ScoreThreshold  float64
}

func NewConversationRAGSearcher(ragClient ragservice.Client, reranker TextReranker, recallTopK int, scoreThreshold float64) *ConversationRAGSearcher {
	if recallTopK <= 0 {
		recallTopK = 30
	}
	if recallTopK > 500 {
		recallTopK = 500
	}
	if scoreThreshold < 0 {
		scoreThreshold = 0
	}
	if scoreThreshold > 1 {
		scoreThreshold = 1
	}
	return &ConversationRAGSearcher{
		RAGClient:      ragClient,
		Reranker:       reranker,
		RecallTopK:     recallTopK,
		ScoreThreshold: scoreThreshold,
	}
}

func (s *ConversationRAGSearcher) SearchForConversation(ctx context.Context, req bot.RAGSearchRequest) ([]bot.RAGChunk, error) {
	if s == nil || s.RAGClient == nil {
		return nil, errno.Internal("rag searcher dependencies are not complete")
	}
	if req.UserID == 0 {
		return nil, errno.Required("user id")
	}
	if req.ConversationID == 0 {
		return nil, errno.Required("conversation id")
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
	recallTopK := s.RecallTopK
	if recallTopK < topK {
		recallTopK = topK
	}
	conversationID := strconv.FormatUint(req.ConversationID, 10)

	bindingsResp, err := s.RAGClient.ListConversationKnowledgeBases(ctx, &ragpb.ListConversationKnowledgeBasesRequest{
		OperatorId:     int64(req.UserID),
		ConversationId: conversationID,
	})
	if err != nil {
		return nil, err
	}
	bindings := bindingsResp.KnowledgeBases
	if len(bindings) == 0 {
		return nil, nil
	}

	all := make([]bot.RAGChunk, 0, len(bindings)*recallTopK)
	for _, binding := range bindings {
		if binding == nil || binding.KnowledgeBaseId <= 0 {
			continue
		}
		searchResp, searchErr := s.RAGClient.SearchKnowledgeBase(ctx, &ragpb.SearchKnowledgeBaseRequest{
			OperatorId:      int64(req.UserID),
			KnowledgeBaseId: binding.KnowledgeBaseId,
			Query:           question,
			TopK:            int32Ptr(int32(recallTopK)),
		})
		if searchErr != nil {
			return nil, searchErr
		}
		for _, item := range searchResp.Chunks {
			if item == nil || strings.TrimSpace(item.Content) == "" {
				continue
			}
			all = append(all, bot.RAGChunk{
				Index:   0,
				Content: item.Content,
				Score:   item.Score,
			})
		}
	}
	if len(all) == 0 {
		return nil, nil
	}
	all = deduplicateChunksByContent(all)

	if s.Reranker != nil && len(all) > 1 {
		docs := make([]string, 0, len(all))
		for _, item := range all {
			docs = append(docs, strings.TrimSpace(item.Content))
		}
		results, err := s.Reranker.Rerank(ctx, question, docs, len(docs))
		if err != nil {
			log.Printf("rag rerank degraded: conversation=%d err=%v", req.ConversationID, err)
		} else if len(results) > 0 {
			all = applyRerankResults(all, results, s.ScoreThreshold)
			if len(all) == 0 {
				return nil, nil
			}
		}
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

func int32Ptr(value int32) *int32 {
	return &value
}

func sortByScoreDesc(chunks []bot.RAGChunk) {
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].Score > chunks[j].Score
	})
}

func deduplicateChunksByContent(chunks []bot.RAGChunk) []bot.RAGChunk {
	if len(chunks) <= 1 {
		return chunks
	}
	seen := make(map[string]struct{}, len(chunks))
	result := make([]bot.RAGChunk, 0, len(chunks))
	for _, item := range chunks {
		key := strings.TrimSpace(item.Content)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result
}

func applyRerankResults(chunks []bot.RAGChunk, results []RerankResult, scoreThreshold float64) []bot.RAGChunk {
	if len(chunks) == 0 || len(results) == 0 {
		return chunks
	}
	reranked := make([]bot.RAGChunk, 0, len(results))
	for _, item := range results {
		if item.Index < 0 || item.Index >= len(chunks) {
			continue
		}
		score := item.RelevanceScore
		if score < scoreThreshold {
			continue
		}
		next := chunks[item.Index]
		next.Score = score
		reranked = append(reranked, next)
	}
	return reranked
}

var _ bot.RAGSearcher = (*ConversationRAGSearcher)(nil)
