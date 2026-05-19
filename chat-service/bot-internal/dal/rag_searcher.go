package botdal

import (
	"context"
	"errors"
	"strconv"
	"strings"

	bot "example.com/aim/chat-service/bot-internal/biz"
	ragpb "example.com/aim/chat-service/kitex_gen/rag"
	"example.com/aim/chat-service/kitex_gen/rag/ragservice"
)

type ConversationRAGSearcher struct {
	RAGClient ragservice.Client
}

func NewConversationRAGSearcher(ragClient ragservice.Client) *ConversationRAGSearcher {
	return &ConversationRAGSearcher{
		RAGClient: ragClient,
	}
}

func (s *ConversationRAGSearcher) SearchForConversation(ctx context.Context, req bot.RAGSearchRequest) ([]bot.RAGChunk, error) {
	if s == nil || s.RAGClient == nil {
		return nil, errors.New("rag searcher dependencies are not complete")
	}
	if req.UserID == 0 {
		return nil, errors.New("user id is required")
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

	all := make([]bot.RAGChunk, 0, len(bindings)*topK)
	for _, binding := range bindings {
		if binding == nil || binding.KnowledgeBaseId <= 0 {
			continue
		}
		searchResp, searchErr := s.RAGClient.SearchKnowledgeBase(ctx, &ragpb.SearchKnowledgeBaseRequest{
			OperatorId:      int64(req.UserID),
			KnowledgeBaseId: binding.KnowledgeBaseId,
			Query:           question,
			TopK:            int32Ptr(int32(topK)),
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
	for i := 0; i < len(chunks); i++ {
		for j := i + 1; j < len(chunks); j++ {
			if chunks[j].Score > chunks[i].Score {
				chunks[i], chunks[j] = chunks[j], chunks[i]
			}
		}
	}
}

var _ bot.RAGSearcher = (*ConversationRAGSearcher)(nil)
