package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"example.com/aim/gateway/internal/knowledgeimport"
	"example.com/aim/gateway/internal/model/dto"
	notificationx "example.com/aim/gateway/internal/notification"
	"example.com/aim/gateway/internal/observability"
	"example.com/aim/gateway/internal/rpc"
	gatewayws "example.com/aim/gateway/internal/websocket"
	chatpb "example.com/aim/gateway/kitex_gen/chat"
	ragpb "example.com/aim/gateway/kitex_gen/rag"
	"example.com/aim/gateway/kitex_gen/rag/ragservice"
	"go.uber.org/zap"
)

type knowledgeDocumentFileImportTask struct {
	UserID            int64
	KnowledgeBaseID   int64
	KnowledgeBaseName string
	Filename          string
	ContentType       string
	Title             string
	Data              []byte
	StartedAt         time.Time
}

func runKnowledgeDocumentFileImport(task knowledgeDocumentFileImportTask) {
	logger := observability.L()
	parseStart := time.Now()
	parseCtx, cancelParse := context.WithTimeout(context.Background(), knowledgeImportParseTimeout)
	parsed, parseErr := knowledgeimport.ParseViaService(
		parseCtx,
		task.Filename,
		task.ContentType,
		task.Data,
		task.Title,
	)
	cancelParse()
	if parseErr != nil {
		logger.Warn("knowledge_import.parse_failed",
			zap.Int64("user_id", task.UserID),
			zap.Int64("kb_id", task.KnowledgeBaseID),
			zap.String("filename", task.Filename),
			zap.Int("size_bytes", len(task.Data)),
			zap.Int64("parse_ms", time.Since(parseStart).Milliseconds()),
			zap.Error(parseErr),
		)
		pushKnowledgeImportFailureNotification(task, parseErr.Error())
		return
	}
	logger.Info("knowledge_import.parse_ok",
		zap.Int64("user_id", task.UserID),
		zap.Int64("kb_id", task.KnowledgeBaseID),
		zap.String("filename", task.Filename),
		zap.Int("size_bytes", len(task.Data)),
		zap.String("file_type", parsed.FileType),
		zap.String("source_type", parsed.SourceType),
		zap.Int("image_count", parsed.ImageCount),
		zap.Bool("used_vision", parsed.UsedVision),
		zap.Int64("parse_ms", time.Since(parseStart).Milliseconds()),
		zap.Int("text_chars", len([]rune(parsed.Content))),
	)

	title := task.Title
	if title == "" {
		title = parsed.Title
	}
	client, err := rpc.RAGClient()
	if err != nil {
		pushKnowledgeImportFailureNotification(task, err.Error())
		return
	}
	ragCtx, cancelRAG := context.WithTimeout(context.Background(), knowledgeImportRAGAddTimeout)
	resp, err := client.AddKnowledgeDocumentText(ragCtx, &ragpb.AddKnowledgeDocumentTextRequest{
		OperatorId:      task.UserID,
		KnowledgeBaseId: task.KnowledgeBaseID,
		Title:           title,
		SourceType:      parsed.SourceType,
		Content:         marshalDocumentImportPayload(parsed),
	})
	cancelRAG()
	if err != nil {
		logger.Warn("knowledge_import.rag_add_failed",
			zap.Int64("user_id", task.UserID),
			zap.Int64("kb_id", task.KnowledgeBaseID),
			zap.String("filename", task.Filename),
			zap.String("file_type", parsed.FileType),
			zap.Error(err),
		)
		pushKnowledgeImportFailureNotification(task, presentableMessage(err.Error()))
		return
	}
	if resp == nil || resp.Document == nil {
		logger.Warn("knowledge_import.rag_add_empty_response",
			zap.Int64("user_id", task.UserID),
			zap.Int64("kb_id", task.KnowledgeBaseID),
			zap.String("filename", task.Filename),
		)
		pushKnowledgeImportFailureNotification(task, "document creation failed")
		return
	}
	logger.Info("knowledge_import.rag_submitted",
		zap.Int64("user_id", task.UserID),
		zap.Int64("kb_id", task.KnowledgeBaseID),
		zap.String("filename", task.Filename),
		zap.String("file_type", parsed.FileType),
		zap.Int64("document_id", resp.Document.DocumentId),
		zap.String("status", resp.Document.Status),
		zap.Int64("total_ms", time.Since(task.StartedAt).Milliseconds()),
	)
	watchKnowledgeDocumentImport(task.UserID, task.KnowledgeBaseID, task.KnowledgeBaseName, resp.Document.DocumentId, resp.Document.Title, task.StartedAt)
}

func marshalDocumentImportPayload(parsed *knowledgeimport.ParsedDocument) string {
	if parsed == nil {
		return ""
	}
	type payloadChunk struct {
		Index        int    `json:"index"`
		ChunkType    string `json:"chunkType,omitempty"`
		SectionTitle string `json:"sectionTitle,omitempty"`
		Content      string `json:"content"`
	}
	type payload struct {
		Version int            `json:"version"`
		Content string         `json:"content"`
		Chunks  []payloadChunk `json:"chunks,omitempty"`
	}

	body := payload{
		Version: 1,
		Content: parsed.Content,
	}
	if len(parsed.Chunks) > 0 {
		body.Chunks = make([]payloadChunk, 0, len(parsed.Chunks))
		for _, item := range parsed.Chunks {
			content := strings.TrimSpace(item.Content)
			if content == "" {
				continue
			}
			body.Chunks = append(body.Chunks, payloadChunk{
				Index:        item.Index,
				ChunkType:    strings.TrimSpace(item.ChunkType),
				SectionTitle: strings.TrimSpace(item.SectionTitle),
				Content:      content,
			})
		}
	}
	data, err := json.Marshal(body)
	if err != nil {
		return parsed.Content
	}
	return string(data)
}

func knowledgeBaseDisplayName(ctx context.Context, client ragservice.Client, userID, knowledgeBaseID int64) string {
	if client == nil {
		return fmt.Sprintf("知识库%d", knowledgeBaseID)
	}
	resp, err := client.ListKnowledgeBases(ctx, &ragpb.ListKnowledgeBasesRequest{
		OperatorId: userID,
	})
	if err != nil || resp == nil {
		return fmt.Sprintf("知识库%d", knowledgeBaseID)
	}
	for _, item := range resp.KnowledgeBases {
		if item != nil && item.KnowledgeBaseId == knowledgeBaseID {
			name := strings.TrimSpace(item.Name)
			if name != "" {
				return name
			}
			break
		}
	}
	return fmt.Sprintf("知识库%d", knowledgeBaseID)
}

func watchKnowledgeDocumentImport(userID, knowledgeBaseID int64, knowledgeBaseName string, documentID int64, documentTitle string, startedAt time.Time) {
	if userID <= 0 || knowledgeBaseID <= 0 || documentID <= 0 {
		return
	}

	go func() {
		timeout := time.NewTimer(knowledgeImportWatchTimeout)
		defer timeout.Stop()
		ticker := time.NewTicker(knowledgeImportPollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-timeout.C:
				pushKnowledgeImportNotificationV2(userID, knowledgeBaseName, dto.KnowledgeDocumentInfo{
					DocumentID:      documentID,
					KnowledgeBaseID: knowledgeBaseID,
					Title:           documentTitle,
					Status:          "FAILED",
					ErrorMessage:    "processing timeout (>5m), circuit-break degraded",
				}, startedAt)
				return
			case <-ticker.C:
				client, err := rpc.RAGClient()
				if err != nil {
					continue
				}
				ctx, cancel := context.WithTimeout(context.Background(), knowledgeImportPollRPCDeadline)
				doc, ok := findKnowledgeDocument(ctx, client, userID, knowledgeBaseID, documentID)
				cancel()
				if !ok {
					continue
				}
				switch strings.ToUpper(strings.TrimSpace(doc.Status)) {
				case "READY", "FAILED":
					pushKnowledgeImportNotificationV2(userID, knowledgeBaseName, doc, startedAt)
					return
				}
			}
		}
	}()
}

func findKnowledgeDocument(ctx context.Context, client ragservice.Client, userID, knowledgeBaseID int64, documentID int64) (dto.KnowledgeDocumentInfo, bool) {
	resp, err := client.ListKnowledgeDocuments(ctx, &ragpb.ListKnowledgeDocumentsRequest{
		OperatorId:      userID,
		KnowledgeBaseId: knowledgeBaseID,
	})
	if err != nil || resp == nil {
		return dto.KnowledgeDocumentInfo{}, false
	}
	for _, item := range resp.Documents {
		if item == nil || item.DocumentId != documentID {
			continue
		}
		return dto.KnowledgeDocumentInfo{
			DocumentID:      item.DocumentId,
			KnowledgeBaseID: item.KnowledgeBaseId,
			Title:           item.Title,
			SourceType:      item.SourceType,
			Status:          item.Status,
			ErrorMessage:    item.ErrorMessage,
			CreatedAt:       item.CreatedAt,
		}, true
	}
	return dto.KnowledgeDocumentInfo{}, false
}

func pushKnowledgeImportNotification(userID int64, knowledgeBaseName string, doc dto.KnowledgeDocumentInfo, startedAt time.Time) {
	status := strings.ToUpper(strings.TrimSpace(doc.Status))
	title := "知识库文件导入完成"
	if status == "FAILED" {
		title = "知识库文件导入失败"
	}

	docTitle := strings.TrimSpace(doc.Title)
	if docTitle == "" {
		docTitle = fmt.Sprintf("文档 %d", doc.DocumentID)
	}
	kbName := strings.TrimSpace(knowledgeBaseName)
	if kbName == "" {
		kbName = fmt.Sprintf("知识库%d", doc.KnowledgeBaseID)
	}
	elapsed := formatKnowledgeImportDuration(time.Since(startedAt))

	content := fmt.Sprintf("知识库「%s」的文件「%s」导入成功，用时 %s。", kbName, docTitle, elapsed)
	if status == "FAILED" {
		reason := strings.TrimSpace(doc.ErrorMessage)
		if reason == "" {
			reason = "未返回具体错误"
		}
		content = fmt.Sprintf("知识库「%s」的文件「%s」导入失败，用时 %s。原因：%s", kbName, docTitle, elapsed, reason)
	}

	chatClient, err := rpc.ChatClient()
	if err == nil && chatClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), knowledgeImportNotifyRPCDeadline)
		defer cancel()
		resp, createErr := chatClient.CreateNotification(ctx, &chatpb.CreateNotificationRequest{
			OperatorId: userID,
			UserId:     userID,
			Type:       "KNOWLEDGE_IMPORT",
			Title:      title,
			Content:    content,
		})
		if createErr == nil && resp != nil && resp.Notification != nil {
			chatHub.SendToUsers([]int64{userID}, gatewayws.OutgoingEvent{
				Type: gatewayws.EventNotificationCreated,
				Data: gatewayws.NotificationCreatedData{
					Notification: gatewayws.ToNotificationInfo(resp.Notification),
					UnreadCount:  &resp.UnreadCount,
				},
			})
			return
		}
	}

	category, summary, detail := notificationx.Normalize("KNOWLEDGE_IMPORT", title, content)
	chatHub.SendToUsers([]int64{userID}, gatewayws.OutgoingEvent{
		Type: gatewayws.EventNotificationCreated,
		Data: gatewayws.NotificationCreatedData{
			Notification: gatewayws.NotificationInfo{
				ID:         time.Now().UnixMilli()*1000 + doc.DocumentID%1000,
				Type:       "KNOWLEDGE_IMPORT",
				Category:   category,
				Title:      title,
				Summary:    summary,
				Content:    content,
				Detail:     detail,
				IsRead:     false,
				CreatedAt:  time.Now().Unix(),
				Persistent: false,
			},
		},
	})
}

func formatKnowledgeImportDuration(duration time.Duration) string {
	if duration < time.Second {
		return fmt.Sprintf("%dms", duration.Milliseconds())
	}
	totalSeconds := int64(duration.Round(time.Second).Seconds())
	if totalSeconds < 60 {
		return fmt.Sprintf("%d 秒", totalSeconds)
	}
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60
	if seconds == 0 {
		return fmt.Sprintf("%d 分钟", minutes)
	}
	return fmt.Sprintf("%d 分%d 秒", minutes, seconds)
}

func knowledgeBaseDisplayNameV2(ctx context.Context, client ragservice.Client, userID, knowledgeBaseID int64) string {
	fallback := fmt.Sprintf("知识库%d", knowledgeBaseID)
	if client == nil {
		return fallback
	}
	resp, err := client.ListKnowledgeBases(ctx, &ragpb.ListKnowledgeBasesRequest{
		OperatorId: userID,
	})
	if err != nil || resp == nil {
		return fallback
	}
	for _, item := range resp.KnowledgeBases {
		if item == nil || item.KnowledgeBaseId != knowledgeBaseID {
			continue
		}
		if name := strings.TrimSpace(item.Name); name != "" {
			return name
		}
		return fallback
	}
	return fallback
}

func knowledgeFileTitle(filename string) string {
	title := strings.TrimSpace(strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename)))
	if title == "" {
		return "导入文件"
	}
	return title
}

func pushKnowledgeImportFailureNotification(task knowledgeDocumentFileImportTask, reason string) {
	docTitle := strings.TrimSpace(task.Title)
	if docTitle == "" {
		docTitle = knowledgeFileTitle(task.Filename)
	}
	pushKnowledgeImportNotificationV2(task.UserID, task.KnowledgeBaseName, dto.KnowledgeDocumentInfo{
		KnowledgeBaseID: task.KnowledgeBaseID,
		Title:           docTitle,
		Status:          "FAILED",
		ErrorMessage:    strings.TrimSpace(reason),
	}, task.StartedAt)
}

func pushKnowledgeImportNotificationV2(userID int64, knowledgeBaseName string, doc dto.KnowledgeDocumentInfo, startedAt time.Time) {
	status := strings.ToUpper(strings.TrimSpace(doc.Status))
	title := "知识库文件导入完成"
	if status == "FAILED" {
		title = "知识库文件导入失败"
	}

	docTitle := strings.TrimSpace(doc.Title)
	if docTitle == "" {
		docTitle = fmt.Sprintf("文档 %d", doc.DocumentID)
	}
	kbName := strings.TrimSpace(knowledgeBaseName)
	if kbName == "" {
		kbName = fmt.Sprintf("知识库%d", doc.KnowledgeBaseID)
	}
	elapsed := formatKnowledgeImportDurationV2(time.Since(startedAt))

	content := fmt.Sprintf("知识库「%s」的文件「%s」导入成功，用时 %s。", kbName, docTitle, elapsed)
	if status == "FAILED" {
		reason := strings.TrimSpace(doc.ErrorMessage)
		if reason == "" {
			reason = "未返回具体错误"
		}
		content = fmt.Sprintf("知识库「%s」的文件「%s」导入失败，用时 %s。原因：%s", kbName, docTitle, elapsed, reason)
	}
	pushKnowledgeImportStatusNotification(userID, title, content)
}

func pushKnowledgeImportStatusNotification(userID int64, title string, content string) {
	chatClient, err := rpc.ChatClient()
	if err == nil && chatClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), knowledgeImportNotifyRPCDeadline)
		defer cancel()
		resp, createErr := chatClient.CreateNotification(ctx, &chatpb.CreateNotificationRequest{
			OperatorId: userID,
			UserId:     userID,
			Type:       "KNOWLEDGE_IMPORT",
			Title:      title,
			Content:    content,
		})
		if createErr == nil && resp != nil && resp.Notification != nil {
			chatHub.SendToUsers([]int64{userID}, gatewayws.OutgoingEvent{
				Type: gatewayws.EventNotificationCreated,
				Data: gatewayws.NotificationCreatedData{
					Notification: gatewayws.ToNotificationInfo(resp.Notification),
					UnreadCount:  &resp.UnreadCount,
				},
			})
			return
		}
	}

	category, summary, detail := notificationx.Normalize("KNOWLEDGE_IMPORT", title, content)
	chatHub.SendToUsers([]int64{userID}, gatewayws.OutgoingEvent{
		Type: gatewayws.EventNotificationCreated,
		Data: gatewayws.NotificationCreatedData{
			Notification: gatewayws.NotificationInfo{
				ID:         time.Now().UnixMilli(),
				Type:       "KNOWLEDGE_IMPORT",
				Category:   category,
				Title:      title,
				Summary:    summary,
				Content:    content,
				Detail:     detail,
				IsRead:     false,
				CreatedAt:  time.Now().Unix(),
				Persistent: false,
			},
		},
	})
}

func formatKnowledgeImportDurationV2(duration time.Duration) string {
	if duration < time.Second {
		return fmt.Sprintf("%dms", duration.Milliseconds())
	}
	totalSeconds := int64(duration.Round(time.Second).Seconds())
	if totalSeconds < 60 {
		return fmt.Sprintf("%d 秒", totalSeconds)
	}
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60
	if seconds == 0 {
		return fmt.Sprintf("%d 分钟", minutes)
	}
	return fmt.Sprintf("%d 分%d 秒", minutes, seconds)
}
