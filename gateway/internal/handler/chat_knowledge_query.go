package handler

import (
	"strconv"
	"strings"

	"example.com/aim/gateway/internal/llmprofile"
	"example.com/aim/gateway/internal/middleware"
	"example.com/aim/gateway/internal/model/dto"
	"example.com/aim/gateway/internal/queryexecutor"
	"example.com/aim/gateway/internal/queryrouter"
	"example.com/aim/gateway/internal/rpc"
	"github.com/gin-gonic/gin"
)

func AskKnowledgeBase(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	kbIDValue := strings.TrimSpace(ctx.Param("knowledgeBaseId"))
	kbID, err := strconv.ParseInt(kbIDValue, 10, 64)
	if err != nil || kbID <= 0 {
		writeError(ctx, 400, "invalid knowledgeBaseId")
		return
	}

	var req dto.AskKnowledgeBaseRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Query) == "" {
		writeError(ctx, 400, "query is required")
		return
	}

	ragClient, err := rpc.RAGClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}
	profile, err := llmprofile.ResolveFromConversationBotOrEnv(ctx.Request.Context(), authCtx.UserID, req.ConversationID, req.BotID)
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}
	planner, err := queryrouter.NewHTTPPlanner(queryrouter.Config{
		BaseURL:            profile.BaseURL,
		APIKey:             profile.APIKey,
		Model:              profile.Model,
		Timeout:            profile.Timeout,
		InsecureSkipVerify: profile.InsecureSkipVerify,
	})
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	responder, responderErr := queryexecutor.NewResponder(queryexecutor.ResponderConfig{
		BaseURL:            profile.BaseURL,
		APIKey:             profile.APIKey,
		Model:              profile.Model,
		Timeout:            profile.Timeout,
		InsecureSkipVerify: profile.InsecureSkipVerify,
	})
	if responderErr != nil {
		responder = nil
	}

	service := queryexecutor.Service{
		Planner:   planner,
		RAGClient: ragClient,
		Responder: responder,
	}
	result, err := service.QueryKnowledgeBase(ctx.Request.Context(), queryexecutor.KnowledgeBaseQueryRequest{
		OperatorID:        authCtx.UserID,
		KnowledgeBaseID:   kbID,
		KnowledgeBaseName: knowledgeBaseDisplayNameV2(ctx.Request.Context(), ragClient, authCtx.UserID, kbID),
		Query:             req.Query,
		TopK:              req.TopK,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	citations := make([]dto.KnowledgeBaseQueryCitationInfo, 0, len(result.Citations))
	for _, item := range result.Citations {
		citations = append(citations, dto.KnowledgeBaseQueryCitationInfo{
			Index:         item.Index,
			ChunkID:       item.ChunkID,
			DocumentID:    item.DocumentID,
			DocumentTitle: item.DocumentTitle,
			Score:         item.Score,
			Excerpt:       item.Excerpt,
		})
	}

	quotes := make([]dto.KnowledgeBaseQueryQuoteInfo, 0, len(result.Quotes))
	for _, item := range result.Quotes {
		quotes = append(quotes, dto.KnowledgeBaseQueryQuoteInfo{
			QuoteID:       item.QuoteID,
			DocumentID:    item.DocumentID,
			DocumentTitle: item.DocumentTitle,
			ChunkID:       item.ChunkID,
			SentenceIndex: item.SentenceIndex,
			PageStart:     item.PageStart,
			PageEnd:       item.PageEnd,
			CharStart:     item.CharStart,
			CharEnd:       item.CharEnd,
			Text:          item.Text,
		})
	}

	chunks := make([]dto.KnowledgeSearchChunkInfo, 0, len(result.Chunks))
	for _, item := range result.Chunks {
		if item == nil {
			continue
		}
		chunks = append(chunks, dto.KnowledgeSearchChunkInfo{
			ChunkID:    item.ChunkId,
			DocumentID: item.DocumentId,
			Score:      item.Score,
			Content:    item.Content,
		})
	}

	writeJSON(ctx, 200, dto.APIResponse{
		Code:    0,
		Message: "success",
		Data: dto.KnowledgeBaseQueryResponse{
			Status:    string(result.Status),
			Answer:    result.Answer,
			Model:     result.Model,
			Plan:      toQueryRoutePlanDTO(result.Plan),
			Citations: citations,
			Quotes:    quotes,
			Chunks:    chunks,
		},
	})
}
