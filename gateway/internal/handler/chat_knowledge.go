package handler

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"example.com/aim/gateway/internal/middleware"
	"example.com/aim/gateway/internal/model/dto"
	"example.com/aim/gateway/internal/observability"
	"example.com/aim/gateway/internal/rpc"
	ragpb "example.com/aim/gateway/kitex_gen/rag"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func CreateKnowledgeBase(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	var req dto.CreateKnowledgeBaseRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(ctx, 400, "name is required")
		return
	}

	client, err := rpc.RAGClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}
	resp, err := client.CreateKnowledgeBase(ctx.Request.Context(), &ragpb.CreateKnowledgeBaseRequest{
		OperatorId:  authCtx.UserID,
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if resp.KnowledgeBase == nil {
		writeError(ctx, 500, "knowledge base creation failed")
		return
	}
	writeJSON(ctx, 200, dto.APIResponse{
		Code:    0,
		Message: "success",
		Data: dto.KnowledgeBaseInfo{
			KnowledgeBaseID: resp.KnowledgeBase.KnowledgeBaseId,
			Name:            resp.KnowledgeBase.Name,
			Description:     resp.KnowledgeBase.Description,
			Status:          resp.KnowledgeBase.Status,
		},
	})
}

func ListKnowledgeBases(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	client, err := rpc.RAGClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	resp, err := client.ListKnowledgeBases(ctx.Request.Context(), &ragpb.ListKnowledgeBasesRequest{
		OperatorId: authCtx.UserID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	items := make([]dto.KnowledgeBaseInfo, 0, len(resp.KnowledgeBases))
	for _, item := range resp.KnowledgeBases {
		items = append(items, dto.KnowledgeBaseInfo{
			KnowledgeBaseID: item.KnowledgeBaseId,
			Name:            item.Name,
			Description:     item.Description,
			Status:          item.Status,
		})
	}

	writeJSON(ctx, 200, dto.APIResponse{
		Code:    0,
		Message: "success",
		Data:    items,
	})
}

func AddKnowledgeDocumentText(ctx *gin.Context) {
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

	var req dto.AddKnowledgeDocumentTextRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		writeError(ctx, 400, "title is required")
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		writeError(ctx, 400, "content is required")
		return
	}

	kbName := fmt.Sprintf("知识库%d", kbID)
	if ragClient, ragErr := rpc.RAGClient(); ragErr == nil {
		kbName = knowledgeBaseDisplayNameV2(ctx.Request.Context(), ragClient, authCtx.UserID, kbID)
	}
	startedAt := time.Now()

	client, err := rpc.RAGClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}
	resp, err := client.AddKnowledgeDocumentText(ctx.Request.Context(), &ragpb.AddKnowledgeDocumentTextRequest{
		OperatorId:      authCtx.UserID,
		KnowledgeBaseId: kbID,
		Title:           req.Title,
		SourceType:      req.SourceType,
		Content:         req.Content,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if resp.Document == nil {
		writeError(ctx, 500, "document creation failed")
		return
	}
	if status := strings.ToUpper(strings.TrimSpace(resp.Document.Status)); status == "PENDING" || status == "PROCESSING" {
		pushKnowledgeImportStatusNotification(
			authCtx.UserID,
			"知识库文档导入已提交",
			fmt.Sprintf("知识库「%s」的文档「%s」已提交，正在后台处理。", kbName, strings.TrimSpace(resp.Document.Title)),
		)
		watchKnowledgeDocumentImport(
			authCtx.UserID,
			kbID,
			kbName,
			resp.Document.DocumentId,
			strings.TrimSpace(resp.Document.Title),
			startedAt,
		)
	}
	writeJSON(ctx, 200, dto.APIResponse{
		Code:    0,
		Message: "success",
		Data: dto.KnowledgeDocumentInfo{
			DocumentID:      resp.Document.DocumentId,
			KnowledgeBaseID: resp.Document.KnowledgeBaseId,
			Title:           resp.Document.Title,
			SourceType:      resp.Document.SourceType,
			Status:          resp.Document.Status,
			ErrorMessage:    resp.Document.ErrorMessage,
			CreatedAt:       resp.Document.CreatedAt,
		},
	})
}

func DeleteKnowledgeDocument(ctx *gin.Context) {
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

	documentIDValue := strings.TrimSpace(ctx.Param("documentId"))
	documentID, err := strconv.ParseInt(documentIDValue, 10, 64)
	if err != nil || documentID <= 0 {
		writeError(ctx, 400, "invalid documentId")
		return
	}

	client, err := rpc.RAGClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}
	resp, err := client.DeleteKnowledgeDocument(ctx.Request.Context(), &ragpb.DeleteKnowledgeDocumentRequest{
		OperatorId:      authCtx.UserID,
		KnowledgeBaseId: kbID,
		DocumentId:      documentID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if !resp.Success {
		writeError(ctx, statusFromMessage(resp.Message), presentableMessage(resp.Message))
		return
	}

	writeJSON(ctx, 200, dto.APIResponse{Code: 0, Message: "success"})
}

func AddKnowledgeDocumentFile(ctx *gin.Context) {
	logger := observability.L()
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
	logger.Info("knowledge_import.start",
		zap.Int64("user_id", authCtx.UserID),
		zap.Int64("kb_id", kbID),
	)

	fileHeader, reqErr := readUploadFile(ctx, "file", maxKnowledgeDocumentUploadBytes)
	if reqErr != nil {
		writeError(ctx, reqErr.status, reqErr.message)
		return
	}

	src, openErr := fileHeader.Open()
	if openErr != nil {
		writeError(ctx, 500, openErr.Error())
		return
	}
	defer src.Close()

	data, readErr := io.ReadAll(io.LimitReader(src, maxKnowledgeDocumentUploadBytes+1))
	if readErr != nil {
		logger.Warn("knowledge_import.read_failed",
			zap.Int64("user_id", authCtx.UserID),
			zap.Int64("kb_id", kbID),
			zap.String("filename", fileHeader.Filename),
			zap.Error(readErr),
		)
		writeError(ctx, 500, readErr.Error())
		return
	}
	if int64(len(data)) > maxKnowledgeDocumentUploadBytes {
		logger.Warn("knowledge_import.rejected_too_large",
			zap.Int64("user_id", authCtx.UserID),
			zap.Int64("kb_id", kbID),
			zap.String("filename", fileHeader.Filename),
			zap.Int("size_bytes", len(data)),
			zap.Int64("limit_bytes", maxKnowledgeDocumentUploadBytes),
		)
		writeError(ctx, 413, "file is too large")
		return
	}

	title := strings.TrimSpace(ctx.PostForm("title"))
	if title == "" {
		title = knowledgeFileTitle(fileHeader.Filename)
	}
	kbName := fmt.Sprintf("知识库%d", kbID)
	if client, err := rpc.RAGClient(); err == nil {
		kbName = knowledgeBaseDisplayNameV2(ctx.Request.Context(), client, authCtx.UserID, kbID)
	}

	startedAt := time.Now()
	pushKnowledgeImportStatusNotification(
		authCtx.UserID,
		"知识库文件导入已提交",
		fmt.Sprintf("知识库「%s」的文件「%s」已提交，正在后台解析并入库。", kbName, title),
	)
	go runKnowledgeDocumentFileImport(knowledgeDocumentFileImportTask{
		UserID:            authCtx.UserID,
		KnowledgeBaseID:   kbID,
		KnowledgeBaseName: kbName,
		Filename:          fileHeader.Filename,
		ContentType:       fileHeader.Header.Get("Content-Type"),
		Title:             title,
		Data:              data,
		StartedAt:         startedAt,
	})

	logger.Info("knowledge_import.accepted",
		zap.Int64("user_id", authCtx.UserID),
		zap.Int64("kb_id", kbID),
		zap.String("filename", fileHeader.Filename),
		zap.Int("size_bytes", len(data)),
	)
	writeJSON(ctx, 202, dto.APIResponse{
		Code:    0,
		Message: "accepted",
		Data: dto.KnowledgeDocumentInfo{
			DocumentID:      0,
			KnowledgeBaseID: kbID,
			Title:           title,
			SourceType:      "",
			Status:          "PENDING",
			ErrorMessage:    "",
			CreatedAt:       startedAt.Unix(),
		},
	})
}

func ListKnowledgeDocuments(ctx *gin.Context) {
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

	client, err := rpc.RAGClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}
	resp, err := client.ListKnowledgeDocuments(ctx.Request.Context(), &ragpb.ListKnowledgeDocumentsRequest{
		OperatorId:      authCtx.UserID,
		KnowledgeBaseId: kbID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	docs := make([]dto.KnowledgeDocumentInfo, 0, len(resp.Documents))
	for _, item := range resp.Documents {
		docs = append(docs, dto.KnowledgeDocumentInfo{
			DocumentID:      item.DocumentId,
			KnowledgeBaseID: item.KnowledgeBaseId,
			Title:           item.Title,
			SourceType:      item.SourceType,
			Status:          item.Status,
			ErrorMessage:    item.ErrorMessage,
			CreatedAt:       item.CreatedAt,
		})
	}
	writeJSON(ctx, 200, dto.APIResponse{
		Code:    0,
		Message: "success",
		Data:    docs,
	})
}

func SearchKnowledgeBase(ctx *gin.Context) {
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

	var req dto.SearchKnowledgeBaseRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Query) == "" {
		writeError(ctx, 400, "query is required")
		return
	}

	client, err := rpc.RAGClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}
	resp, err := client.SearchKnowledgeBase(ctx.Request.Context(), &ragpb.SearchKnowledgeBaseRequest{
		OperatorId:      authCtx.UserID,
		KnowledgeBaseId: kbID,
		Query:           req.Query,
		TopK:            req.TopK,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	chunks := make([]dto.KnowledgeSearchChunkInfo, 0, len(resp.Chunks))
	for _, item := range resp.Chunks {
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
		Data:    chunks,
	})
}

func BindConversationKnowledgeBase(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}
	conversationID, ok := conversationIDParam(ctx)
	if !ok {
		return
	}
	var req dto.BindConversationKnowledgeBaseRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}
	if req.KnowledgeBaseID <= 0 {
		writeError(ctx, 400, "knowledgeBaseId is required")
		return
	}

	client, err := rpc.RAGClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}
	resp, err := client.BindConversationKnowledgeBase(ctx.Request.Context(), &ragpb.BindConversationKnowledgeBaseRequest{
		OperatorId:      authCtx.UserID,
		ConversationId:  conversationID,
		KnowledgeBaseId: req.KnowledgeBaseID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if !resp.Success {
		writeError(ctx, statusFromMessage(resp.Message), presentableMessage(resp.Message))
		return
	}
	writeJSON(ctx, 200, dto.APIResponse{Code: 0, Message: "success"})
}

func ListConversationKnowledgeBases(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}
	conversationID, ok := conversationIDParam(ctx)
	if !ok {
		return
	}

	client, err := rpc.RAGClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}
	resp, err := client.ListConversationKnowledgeBases(ctx.Request.Context(), &ragpb.ListConversationKnowledgeBasesRequest{
		OperatorId:     authCtx.UserID,
		ConversationId: conversationID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}

	items := make([]dto.ConversationKnowledgeBaseInfo, 0, len(resp.KnowledgeBases))
	for _, item := range resp.KnowledgeBases {
		items = append(items, dto.ConversationKnowledgeBaseInfo{
			ID:              item.Id,
			ConversationID:  item.ConversationId,
			KnowledgeBaseID: item.KnowledgeBaseId,
			Name:            item.Name,
			Description:     item.Description,
			Status:          item.Status,
			Enabled:         item.Enabled,
		})
	}
	writeJSON(ctx, 200, dto.APIResponse{Code: 0, Message: "success", Data: items})
}

func UnbindConversationKnowledgeBase(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}
	conversationID, ok := conversationIDParam(ctx)
	if !ok {
		return
	}
	kbIDValue := strings.TrimSpace(ctx.Param("knowledgeBaseId"))
	kbID, err := strconv.ParseInt(kbIDValue, 10, 64)
	if err != nil || kbID <= 0 {
		writeError(ctx, 400, "invalid knowledgeBaseId")
		return
	}

	client, err := rpc.RAGClient()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}
	resp, err := client.UnbindConversationKnowledgeBase(ctx.Request.Context(), &ragpb.UnbindConversationKnowledgeBaseRequest{
		OperatorId:      authCtx.UserID,
		ConversationId:  conversationID,
		KnowledgeBaseId: kbID,
	})
	if err != nil {
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if !resp.Success {
		writeError(ctx, statusFromMessage(resp.Message), presentableMessage(resp.Message))
		return
	}
	writeJSON(ctx, 200, dto.APIResponse{Code: 0, Message: "success"})
}
