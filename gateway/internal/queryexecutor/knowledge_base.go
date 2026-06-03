package queryexecutor

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"example.com/aim/gateway/internal/queryrouter"
	ragpb "example.com/aim/gateway/kitex_gen/rag"
	"example.com/aim/gateway/kitex_gen/rag/ragservice"
)

type KnowledgeBaseQueryRequest struct {
	OperatorID      int64
	KnowledgeBaseID int64
	KnowledgeBaseName string
	Query           string
	TopK            *int32
}

type KnowledgeBaseQueryStatus string

const (
	KnowledgeBaseQueryStatusAnswered    KnowledgeBaseQueryStatus = "ANSWERED"
	KnowledgeBaseQueryStatusNoHit       KnowledgeBaseQueryStatus = "NO_HIT"
	KnowledgeBaseQueryStatusUnsupported KnowledgeBaseQueryStatus = "UNSUPPORTED"
)

type Citation struct {
	Index         int
	ChunkID       int64
	DocumentID    int64
	DocumentTitle string
	Score         float64
	Excerpt       string
}

type ExactQuote struct {
	QuoteID       string
	DocumentID    int64
	DocumentTitle string
	ChunkID       int64
	SentenceIndex int
	PageStart     int
	PageEnd       int
	CharStart     int
	CharEnd       int
	Text          string
}

type KnowledgeBaseQueryResult struct {
	Plan      queryrouter.Plan
	Status    KnowledgeBaseQueryStatus
	Answer    string
	Model     string
	Citations []Citation
	Quotes    []ExactQuote
	Chunks    []*ragpb.KnowledgeSearchChunkInfo
}

type Service struct {
	Planner   queryrouter.Planner
	RAGClient ragservice.Client
	Responder *Responder
}

func (s *Service) QueryKnowledgeBase(ctx context.Context, req KnowledgeBaseQueryRequest) (*KnowledgeBaseQueryResult, error) {
	if s == nil {
		return nil, fmt.Errorf("query executor service is nil")
	}
	if s.Planner == nil {
		return nil, fmt.Errorf("query planner is nil")
	}
	if s.RAGClient == nil {
		return nil, fmt.Errorf("rag client is nil")
	}
	query := strings.TrimSpace(req.Query)
	if req.OperatorID <= 0 || req.KnowledgeBaseID <= 0 || query == "" {
		return nil, fmt.Errorf("operator id, knowledge base id and query are required")
	}

	plan, err := s.Planner.Plan(ctx, queryrouter.PlanningInput{
		UserQuery: query,
		SelectedTargets: []queryrouter.Target{
			{
				ID:    strconv.FormatInt(req.KnowledgeBaseID, 10),
				Type:  "knowledge_base",
				Title: strings.TrimSpace(req.KnowledgeBaseName),
			},
		},
		AvailableSpaces: queryrouter.AvailableSpaces{
			KnowledgeBase: true,
			Metadata:      true,
		},
	Capabilities: queryrouter.Capabilities{
		CanLookup:                  true,
		CanFullReadDocument:        true,
		CanSynthesizeMultiDocument: true,
		CanExtractExactQuote:       true,
		CanControlBindings:         false,
		CanUseExternalWeb:          false,
		},
		ContextHints: queryrouter.ContextHints{
			CurrentKBIDs: []string{strconv.FormatInt(req.KnowledgeBaseID, 10)},
		},
	})
	if err != nil {
		return nil, err
	}

	result := &KnowledgeBaseQueryResult{Plan: *plan}
	switch plan.Family {
	case queryrouter.FamilyLookup:
		return s.executeLookup(ctx, req, result)
	case queryrouter.FamilyRead:
		return s.executeRead(ctx, req, result)
	case queryrouter.FamilySynthesize:
		return s.executeSynthesize(ctx, req, result)
	case queryrouter.FamilyUnsupported:
		result.Status = KnowledgeBaseQueryStatusUnsupported
		result.Answer = strings.TrimSpace(plan.Reason)
		if result.Answer == "" {
			result.Answer = "当前请求超出知识库问答入口支持范围"
		}
		return result, nil
	default:
		result.Status = KnowledgeBaseQueryStatusUnsupported
		result.Answer = fmt.Sprintf("当前知识库问答入口暂不支持 %s 路径。%s", plan.Family, strings.TrimSpace(plan.Reason))
		return result, nil
	}
}

func (s *Service) executeSynthesize(ctx context.Context, req KnowledgeBaseQueryRequest, result *KnowledgeBaseQueryResult) (*KnowledgeBaseQueryResult, error) {
	docResp, err := s.RAGClient.ListKnowledgeDocuments(ctx, &ragpb.ListKnowledgeDocumentsRequest{
		OperatorId:      req.OperatorID,
		KnowledgeBaseId: req.KnowledgeBaseID,
	})
	if err != nil {
		return nil, err
	}

	searchResp, err := s.RAGClient.SearchKnowledgeBase(ctx, &ragpb.SearchKnowledgeBaseRequest{
		OperatorId:      req.OperatorID,
		KnowledgeBaseId: req.KnowledgeBaseID,
		Query:           strings.TrimSpace(req.Query),
		TopK:            int32Ptr(8),
	})
	if err == nil {
		result.Chunks = filterKnowledgeChunks(searchResp.GetChunks())
		docTitles, titleErr := loadDocumentTitles(ctx, s.RAGClient, req.OperatorID, req.KnowledgeBaseID)
		if titleErr == nil {
			result.Citations = buildCitations(result.Chunks, docTitles)
		}
	}

	documents, reason := selectSynthesisDocuments(strings.TrimSpace(req.Query), docResp.GetDocuments(), result.Chunks)
	if len(documents) < 2 {
		result.Status = KnowledgeBaseQueryStatusUnsupported
		if reason == "" {
			reason = "当前知识库无法稳定确定要综合的多份文档，请明确文档范围。"
		}
		result.Answer = reason
		return result, nil
	}

	if s.Responder == nil {
		result.Status = KnowledgeBaseQueryStatusUnsupported
		result.Answer = "当前未配置回答模型，无法执行多文档综合。"
		return result, nil
	}

	docNotes := make([]DocumentSynthesisNote, 0, len(documents))
	for _, document := range documents {
		chunkResp, chunkErr := s.RAGClient.ListKnowledgeDocumentChunks(ctx, &ragpb.ListKnowledgeDocumentChunksRequest{
			OperatorId:      req.OperatorID,
			KnowledgeBaseId: req.KnowledgeBaseID,
			DocumentId:      document.DocumentId,
		})
		if chunkErr != nil {
			return nil, chunkErr
		}
		readChunks := filterReadableDocumentChunks(chunkResp.GetChunks())
		if len(readChunks) == 0 {
			continue
		}
		note, noteErr := s.Responder.SummarizeKnowledgeDocumentForSynthesis(ctx, KnowledgeReadPromptInput{
			Query:             req.Query,
			OutputMode:        queryrouter.OutputModeOutline,
			EvidenceMode:      result.Plan.EvidenceMode,
			KnowledgeBaseName: req.KnowledgeBaseName,
			DocumentTitle:     strings.TrimSpace(document.Title),
			Chunks:            readChunks,
		})
		if noteErr != nil {
			return nil, noteErr
		}
		docNotes = append(docNotes, DocumentSynthesisNote{
			DocumentID:    document.DocumentId,
			DocumentTitle: strings.TrimSpace(document.Title),
			Note:          note,
		})
	}
	if len(docNotes) < 2 {
		result.Status = KnowledgeBaseQueryStatusUnsupported
		result.Answer = "可用于综合分析的文档数量不足，请明确至少两份可读文档。"
		return result, nil
	}

	answer, modelName, err := s.Responder.AnswerKnowledgeSynthesis(ctx, KnowledgeSynthesisPromptInput{
		Query:             req.Query,
		OutputMode:        result.Plan.OutputMode,
		EvidenceMode:      result.Plan.EvidenceMode,
		KnowledgeBaseName: req.KnowledgeBaseName,
		Documents:         docNotes,
	})
	if err != nil {
		return nil, err
	}
	result.Status = KnowledgeBaseQueryStatusAnswered
	result.Answer = answer
	result.Model = modelName
	if result.Plan.EvidenceMode == queryrouter.EvidenceModeExactQuote {
		docChunks := make(map[int64][]*ragpb.KnowledgeDocumentChunkInfo, len(documents))
		for _, document := range documents {
			chunkResp, chunkErr := s.RAGClient.ListKnowledgeDocumentChunks(ctx, &ragpb.ListKnowledgeDocumentChunksRequest{
				OperatorId:      req.OperatorID,
				KnowledgeBaseId: req.KnowledgeBaseID,
				DocumentId:      document.DocumentId,
			})
			if chunkErr != nil {
				return nil, chunkErr
			}
			docChunks[document.DocumentId] = filterReadableDocumentChunks(chunkResp.GetChunks())
		}
		quotes, quoteErr := s.resolveExactQuotesForSynthesis(ctx, req, result, documents, docChunks)
		if quoteErr != nil {
			return nil, quoteErr
		}
		result.Quotes = quotes
	}
	return result, nil
}

func (s *Service) executeRead(ctx context.Context, req KnowledgeBaseQueryRequest, result *KnowledgeBaseQueryResult) (*KnowledgeBaseQueryResult, error) {
	docResp, err := s.RAGClient.ListKnowledgeDocuments(ctx, &ragpb.ListKnowledgeDocumentsRequest{
		OperatorId:      req.OperatorID,
		KnowledgeBaseId: req.KnowledgeBaseID,
	})
	if err != nil {
		return nil, err
	}

	document, reason := selectReadableDocument(strings.TrimSpace(req.Query), docResp.GetDocuments())
	if document == nil {
		result.Status = KnowledgeBaseQueryStatusUnsupported
		if reason == "" {
			reason = "当前知识库无法唯一确定要精读的文档，请明确文档范围。"
		}
		result.Answer = reason
		return result, nil
	}

	chunkResp, err := s.RAGClient.ListKnowledgeDocumentChunks(ctx, &ragpb.ListKnowledgeDocumentChunksRequest{
		OperatorId:      req.OperatorID,
		KnowledgeBaseId: req.KnowledgeBaseID,
		DocumentId:      document.DocumentId,
	})
	if err != nil {
		return nil, err
	}
	readChunks := filterReadableDocumentChunks(chunkResp.GetChunks())
	if len(readChunks) == 0 {
		result.Status = KnowledgeBaseQueryStatusNoHit
		result.Answer = "目标文档暂无可读取内容。"
		return result, nil
	}

	if s.Responder == nil {
		result.Status = KnowledgeBaseQueryStatusUnsupported
		result.Answer = "当前未配置回答模型，无法执行全文精读。"
		return result, nil
	}

	answer, modelName, err := s.Responder.AnswerKnowledgeRead(ctx, KnowledgeReadPromptInput{
		Query:             req.Query,
		OutputMode:        result.Plan.OutputMode,
		EvidenceMode:      result.Plan.EvidenceMode,
		KnowledgeBaseName: req.KnowledgeBaseName,
		DocumentTitle:     strings.TrimSpace(document.Title),
		Chunks:            readChunks,
	})
	if err != nil {
		return nil, err
	}

	result.Status = KnowledgeBaseQueryStatusAnswered
	result.Answer = answer
	result.Model = modelName
	if result.Plan.EvidenceMode == queryrouter.EvidenceModeExactQuote {
		quotes, quoteErr := s.resolveExactQuotesForRead(ctx, req, result, document, readChunks)
		if quoteErr != nil {
			return nil, quoteErr
		}
		result.Quotes = quotes
	}
	return result, nil
}

func (s *Service) executeLookup(ctx context.Context, req KnowledgeBaseQueryRequest, result *KnowledgeBaseQueryResult) (*KnowledgeBaseQueryResult, error) {
	searchResp, err := s.RAGClient.SearchKnowledgeBase(ctx, &ragpb.SearchKnowledgeBaseRequest{
		OperatorId:      req.OperatorID,
		KnowledgeBaseId: req.KnowledgeBaseID,
		Query:           strings.TrimSpace(req.Query),
		TopK:            req.TopK,
	})
	if err != nil {
		return nil, err
	}

	chunks := filterKnowledgeChunks(searchResp.GetChunks())
	result.Chunks = chunks
	if len(chunks) == 0 {
		result.Status = KnowledgeBaseQueryStatusNoHit
		result.Answer = "未检索到相关资料，无法基于当前知识库回答。"
		return result, nil
	}

	docTitles, err := loadDocumentTitles(ctx, s.RAGClient, req.OperatorID, req.KnowledgeBaseID)
	if err != nil {
		return nil, err
	}
	result.Citations = buildCitations(chunks, docTitles)

	if s.Responder == nil {
		result.Status = KnowledgeBaseQueryStatusAnswered
		result.Answer = "已检索到相关片段，请结合引用查看。"
		return result, nil
	}

	answer, modelName, err := s.Responder.AnswerKnowledgeLookup(ctx, KnowledgeLookupPromptInput{
		Query:          req.Query,
		OutputMode:     result.Plan.OutputMode,
		EvidenceMode:   result.Plan.EvidenceMode,
		KnowledgeBaseName: req.KnowledgeBaseName,
		Citations:      result.Citations,
	})
	if err != nil {
		return nil, err
	}
	result.Status = KnowledgeBaseQueryStatusAnswered
	result.Answer = answer
	result.Model = modelName
	if result.Plan.EvidenceMode == queryrouter.EvidenceModeExactQuote {
		quotes, quoteErr := s.resolveExactQuotesForLookup(ctx, req, result, docTitles)
		if quoteErr != nil {
			return nil, quoteErr
		}
		result.Quotes = quotes
	}
	return result, nil
}

func (s *Service) resolveExactQuotesForLookup(ctx context.Context, req KnowledgeBaseQueryRequest, result *KnowledgeBaseQueryResult, docTitles map[int64]string) ([]ExactQuote, error) {
	if s.Responder == nil || len(result.Chunks) == 0 {
		return nil, nil
	}
	candidates, err := s.buildLookupExactQuoteCandidates(ctx, req, docTitles, result.Chunks)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	selectedIDs, err := s.Responder.SelectExactQuoteIDs(ctx, ExactQuoteSelectionInput{
		Query:        req.Query,
		Answer:       result.Answer,
		DocumentHint: req.KnowledgeBaseName,
		Candidates:   candidates,
	})
	if err != nil {
		return nil, err
	}
	return materializeExactQuotes(candidates, selectedIDs), nil
}

func (s *Service) buildLookupExactQuoteCandidates(
	ctx context.Context,
	req KnowledgeBaseQueryRequest,
	docTitles map[int64]string,
	hits []*ragpb.KnowledgeSearchChunkInfo,
) ([]ExactQuoteCandidate, error) {
	if len(hits) == 0 {
		return nil, nil
	}
	docChunks := make(map[int64]map[int64]*ragpb.KnowledgeDocumentChunkInfo)
	for _, hit := range hits {
		if hit == nil || hit.DocumentId <= 0 {
			continue
		}
		if _, exists := docChunks[hit.DocumentId]; exists {
			continue
		}
		chunkResp, err := s.RAGClient.ListKnowledgeDocumentChunks(ctx, &ragpb.ListKnowledgeDocumentChunksRequest{
			OperatorId:      req.OperatorID,
			KnowledgeBaseId: req.KnowledgeBaseID,
			DocumentId:      hit.DocumentId,
		})
		if err != nil {
			return nil, err
		}
		byChunkID := make(map[int64]*ragpb.KnowledgeDocumentChunkInfo, len(chunkResp.GetChunks()))
		for _, item := range chunkResp.GetChunks() {
			if item == nil {
				continue
			}
			byChunkID[item.ChunkId] = item
		}
		docChunks[hit.DocumentId] = byChunkID
	}

	candidates := make([]ExactQuoteCandidate, 0, 24)
	for _, hit := range hits {
		if hit == nil {
			continue
		}
		chunk := docChunks[hit.DocumentId][hit.ChunkId]
		if chunk == nil {
			continue
		}
		title := strings.TrimSpace(docTitles[hit.DocumentId])
		if title == "" {
			title = fmt.Sprintf("文档%d", hit.DocumentId)
		}
		next := splitDocumentChunkIntoCandidates(chunk, title, 6)
		candidates = append(candidates, next...)
	}
	return normalizeExactQuoteCandidates(candidates, 24), nil
}

func (s *Service) resolveExactQuotesForRead(ctx context.Context, req KnowledgeBaseQueryRequest, result *KnowledgeBaseQueryResult, document *ragpb.KnowledgeDocumentInfo, chunks []*ragpb.KnowledgeDocumentChunkInfo) ([]ExactQuote, error) {
	if s.Responder == nil || document == nil {
		return nil, nil
	}
	candidates := buildExactQuoteCandidatesFromDocumentChunks(chunks, strings.TrimSpace(document.Title), req.Query, 24)
	if len(candidates) == 0 {
		return nil, nil
	}
	selectedIDs, err := s.Responder.SelectExactQuoteIDs(ctx, ExactQuoteSelectionInput{
		Query:        req.Query,
		Answer:       result.Answer,
		DocumentHint: strings.TrimSpace(document.Title),
		Candidates:   candidates,
	})
	if err != nil {
		return nil, err
	}
	return materializeExactQuotes(candidates, selectedIDs), nil
}

func (s *Service) resolveExactQuotesForSynthesis(
	ctx context.Context,
	req KnowledgeBaseQueryRequest,
	result *KnowledgeBaseQueryResult,
	documents []*ragpb.KnowledgeDocumentInfo,
	docChunks map[int64][]*ragpb.KnowledgeDocumentChunkInfo,
) ([]ExactQuote, error) {
	if s.Responder == nil {
		return nil, nil
	}
	candidates := make([]ExactQuoteCandidate, 0, 32)
	for _, document := range documents {
		if document == nil {
			continue
		}
		docCandidates := buildExactQuoteCandidatesFromDocumentChunks(
			docChunks[document.DocumentId],
			strings.TrimSpace(document.Title),
			req.Query,
			8,
		)
		candidates = append(candidates, docCandidates...)
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	selectedIDs, err := s.Responder.SelectExactQuoteIDs(ctx, ExactQuoteSelectionInput{
		Query:        req.Query,
		Answer:       result.Answer,
		DocumentHint: req.KnowledgeBaseName,
		Candidates:   candidates,
	})
	if err != nil {
		return nil, err
	}
	return materializeExactQuotes(candidates, selectedIDs), nil
}

func filterKnowledgeChunks(items []*ragpb.KnowledgeSearchChunkInfo) []*ragpb.KnowledgeSearchChunkInfo {
	if len(items) == 0 {
		return nil
	}
	result := make([]*ragpb.KnowledgeSearchChunkInfo, 0, len(items))
	for _, item := range items {
		if item == nil || strings.TrimSpace(item.Content) == "" {
			continue
		}
		result = append(result, item)
	}
	return result
}

func loadDocumentTitles(ctx context.Context, client ragservice.Client, operatorID, knowledgeBaseID int64) (map[int64]string, error) {
	resp, err := client.ListKnowledgeDocuments(ctx, &ragpb.ListKnowledgeDocumentsRequest{
		OperatorId:      operatorID,
		KnowledgeBaseId: knowledgeBaseID,
	})
	if err != nil {
		return nil, err
	}
	result := make(map[int64]string, len(resp.GetDocuments()))
	for _, item := range resp.GetDocuments() {
		if item == nil || item.DocumentId <= 0 {
			continue
		}
		result[item.DocumentId] = strings.TrimSpace(item.Title)
	}
	return result, nil
}

func buildCitations(chunks []*ragpb.KnowledgeSearchChunkInfo, docTitles map[int64]string) []Citation {
	if len(chunks) == 0 {
		return nil
	}
	result := make([]Citation, 0, len(chunks))
	for index, item := range chunks {
		title := strings.TrimSpace(docTitles[item.DocumentId])
		if title == "" {
			title = fmt.Sprintf("文档%d", item.DocumentId)
		}
		result = append(result, Citation{
			Index:         index + 1,
			ChunkID:       item.ChunkId,
			DocumentID:    item.DocumentId,
			DocumentTitle: title,
			Score:         item.Score,
			Excerpt:       excerpt(item.Content, 220),
		})
	}
	return result
}

func selectReadableDocument(query string, docs []*ragpb.KnowledgeDocumentInfo) (*ragpb.KnowledgeDocumentInfo, string) {
	readyDocs := make([]*ragpb.KnowledgeDocumentInfo, 0, len(docs))
	for _, item := range docs {
		if item == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(item.Status), "READY") {
			readyDocs = append(readyDocs, item)
		}
	}
	if len(readyDocs) == 0 {
		return nil, "当前知识库没有可精读的就绪文档。"
	}
	if len(readyDocs) == 1 {
		return readyDocs[0], ""
	}

	queryNorm := normalizeDocMatchText(query)
	if queryNorm == "" {
		return nil, "当前知识库包含多份文档，请明确指出要精读的文档。"
	}
	var matched []*ragpb.KnowledgeDocumentInfo
	for _, item := range readyDocs {
		titleNorm := normalizeDocMatchText(item.Title)
		if titleNorm == "" {
			continue
		}
		if strings.Contains(queryNorm, titleNorm) || strings.Contains(titleNorm, queryNorm) {
			matched = append(matched, item)
		}
	}
	if len(matched) == 1 {
		return matched[0], ""
	}
	if len(matched) > 1 {
		return nil, "匹配到多份文档，请进一步缩小文档范围。"
	}
	return nil, "当前知识库包含多份文档，请明确指出要精读的文档。"
}

func normalizeDocMatchText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	replacer := strings.NewReplacer(".pdf", "", ".md", "", ".txt", "", "《", "", "》", "", " ", "", "\t", "", "\n", "", "\r", "")
	return replacer.Replace(value)
}

func filterReadableDocumentChunks(items []*ragpb.KnowledgeDocumentChunkInfo) []*ragpb.KnowledgeDocumentChunkInfo {
	if len(items) == 0 {
		return nil
	}
	result := make([]*ragpb.KnowledgeDocumentChunkInfo, 0, len(items))
	for _, item := range items {
		if item == nil || strings.TrimSpace(item.Content) == "" {
			continue
		}
		result = append(result, item)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].ChunkIndex == result[j].ChunkIndex {
			return result[i].ChunkId < result[j].ChunkId
		}
		return result[i].ChunkIndex < result[j].ChunkIndex
	})
	return result
}

func selectSynthesisDocuments(query string, docs []*ragpb.KnowledgeDocumentInfo, hits []*ragpb.KnowledgeSearchChunkInfo) ([]*ragpb.KnowledgeDocumentInfo, string) {
	readyDocs := make([]*ragpb.KnowledgeDocumentInfo, 0, len(docs))
	byID := make(map[int64]*ragpb.KnowledgeDocumentInfo, len(docs))
	for _, item := range docs {
		if item == nil || !strings.EqualFold(strings.TrimSpace(item.Status), "READY") {
			continue
		}
		readyDocs = append(readyDocs, item)
		byID[item.DocumentId] = item
	}
	if len(readyDocs) < 2 {
		return nil, "当前知识库中可综合的就绪文档不足两份。"
	}
	if len(readyDocs) == 2 {
		return readyDocs, ""
	}

	queryNorm := normalizeDocMatchText(query)
	if queryNorm != "" {
		matched := make([]*ragpb.KnowledgeDocumentInfo, 0, 4)
		for _, item := range readyDocs {
			titleNorm := normalizeDocMatchText(item.Title)
			if titleNorm == "" {
				continue
			}
			if strings.Contains(queryNorm, titleNorm) || strings.Contains(titleNorm, queryNorm) {
				matched = append(matched, item)
			}
		}
		if len(matched) >= 2 {
			if len(matched) > 4 {
				matched = matched[:4]
			}
			return matched, ""
		}
	}

	selected := make([]*ragpb.KnowledgeDocumentInfo, 0, 4)
	seen := make(map[int64]struct{}, 4)
	for _, hit := range hits {
		if hit == nil || hit.DocumentId <= 0 {
			continue
		}
		if _, exists := seen[hit.DocumentId]; exists {
			continue
		}
		document := byID[hit.DocumentId]
		if document == nil {
			continue
		}
		seen[hit.DocumentId] = struct{}{}
		selected = append(selected, document)
		if len(selected) >= 4 {
			break
		}
	}
	if len(selected) >= 2 {
		return selected, ""
	}
	return nil, "当前知识库包含多份文档，请明确列出要综合的文档名称。"
}

func excerpt(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 {
		limit = 220
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "..."
}

func int32Ptr(value int32) *int32 {
	return &value
}

type Responder struct {
	config     ResponderConfig
	httpClient *http.Client
}

type ResponderConfig struct {
	BaseURL            string
	APIKey             string
	Model              string
	Timeout            time.Duration
	InsecureSkipVerify bool
}

type KnowledgeLookupPromptInput struct {
	Query             string
	OutputMode        queryrouter.OutputMode
	EvidenceMode      queryrouter.EvidenceMode
	KnowledgeBaseName string
	Citations         []Citation
}

type KnowledgeReadPromptInput struct {
	Query             string
	OutputMode        queryrouter.OutputMode
	EvidenceMode      queryrouter.EvidenceMode
	KnowledgeBaseName string
	DocumentTitle     string
	Chunks            []*ragpb.KnowledgeDocumentChunkInfo
}

type DocumentSynthesisNote struct {
	DocumentID    int64
	DocumentTitle string
	Note          string
}

type KnowledgeSynthesisPromptInput struct {
	Query             string
	OutputMode        queryrouter.OutputMode
	EvidenceMode      queryrouter.EvidenceMode
	KnowledgeBaseName string
	Documents         []DocumentSynthesisNote
}

type ExactQuoteCandidate struct {
	QuoteID       string
	DocumentID    int64
	DocumentTitle string
	ChunkID       int64
	SentenceIndex int
	PageStart     int
	PageEnd       int
	CharStart     int
	CharEnd       int
	Text          string
	Score         int
}

type ExactQuoteSelectionInput struct {
	Query        string
	Answer       string
	DocumentHint string
	Candidates   []ExactQuoteCandidate
}

func NewResponder(config ResponderConfig) (*Responder, error) {
	config.BaseURL = strings.TrimSpace(config.BaseURL)
	config.APIKey = strings.TrimSpace(config.APIKey)
	config.Model = strings.TrimSpace(config.Model)
	if config.BaseURL == "" {
		return nil, fmt.Errorf("query executor llm base url is empty")
	}
	if config.APIKey == "" {
		return nil, fmt.Errorf("query executor llm api key is empty")
	}
	if config.Model == "" {
		return nil, fmt.Errorf("query executor llm model is empty")
	}
	if config.Timeout <= 0 {
		config.Timeout = 25 * time.Second
	}
	return &Responder{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: config.InsecureSkipVerify}, //nolint:gosec
			},
		},
	}, nil
}

func (r *Responder) AnswerKnowledgeLookup(ctx context.Context, input KnowledgeLookupPromptInput) (string, string, error) {
	if r == nil {
		return "", "", fmt.Errorf("query executor responder is nil")
	}
	systemPrompt, userPrompt := buildKnowledgeLookupPrompts(input)
	payload := map[string]any{
		"model": r.config.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"stream":      false,
		"temperature": 0.2,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, r.config.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+r.config.APIKey)

	resp, err := r.httpClient.Do(httpReq)
	if err != nil {
		return "", "", fmt.Errorf("query executor llm request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return "", "", fmt.Errorf("query executor llm response read failed: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", "", fmt.Errorf("query executor llm request failed: status=%d %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", "", fmt.Errorf("query executor llm response parse failed: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", "", fmt.Errorf("query executor llm response empty")
	}
	answer := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if answer == "" {
		return "", "", fmt.Errorf("query executor llm answer is empty")
	}
	return answer, r.config.Model, nil
}

func (r *Responder) AnswerKnowledgeRead(ctx context.Context, input KnowledgeReadPromptInput) (string, string, error) {
	if r == nil {
		return "", "", fmt.Errorf("query executor responder is nil")
	}
	batches := buildDocumentReadBatches(input.Chunks, 8000)
	if len(batches) == 0 {
		return "", "", fmt.Errorf("document read batches are empty")
	}

	if len(batches) == 1 {
		systemPrompt, userPrompt := buildKnowledgeReadPrompts(input, batches[0], "")
		answer, err := r.completeText(ctx, systemPrompt, userPrompt)
		if err != nil {
			return "", "", err
		}
		return answer, r.config.Model, nil
	}

	batchSummaries := make([]string, 0, len(batches))
	for index, batch := range batches {
		systemPrompt, userPrompt := buildKnowledgeReadPrompts(input, batch, fmt.Sprintf("这是第 %d/%d 批内容，请先产出该批的阅读要点。", index+1, len(batches)))
		summary, err := r.completeText(ctx, systemPrompt, userPrompt)
		if err != nil {
			return "", "", err
		}
		batchSummaries = append(batchSummaries, strings.TrimSpace(summary))
	}

	finalSystem, finalUser := buildKnowledgeReadAggregatePrompts(input, batchSummaries)
	answer, err := r.completeText(ctx, finalSystem, finalUser)
	if err != nil {
		return "", "", err
	}
	return answer, r.config.Model, nil
}

func (r *Responder) SummarizeKnowledgeDocumentForSynthesis(ctx context.Context, input KnowledgeReadPromptInput) (string, error) {
	if r == nil {
		return "", fmt.Errorf("query executor responder is nil")
	}
	batches := buildDocumentReadBatches(input.Chunks, 8000)
	if len(batches) == 0 {
		return "", fmt.Errorf("document read batches are empty")
	}

	if len(batches) == 1 {
		systemPrompt, userPrompt := buildKnowledgeReadPrompts(input, batches[0], "请针对用户问题提炼这份文档的阅读要点，便于后续跨文档综合。")
		return r.completeText(ctx, systemPrompt, userPrompt)
	}

	batchSummaries := make([]string, 0, len(batches))
	for index, batch := range batches {
		systemPrompt, userPrompt := buildKnowledgeReadPrompts(input, batch, fmt.Sprintf("这是第 %d/%d 批内容，请提炼该批与用户问题最相关的阅读要点。", index+1, len(batches)))
		summary, err := r.completeText(ctx, systemPrompt, userPrompt)
		if err != nil {
			return "", err
		}
		batchSummaries = append(batchSummaries, strings.TrimSpace(summary))
	}
	finalSystem, finalUser := buildKnowledgeReadAggregatePrompts(input, batchSummaries)
	return r.completeText(ctx, finalSystem, finalUser)
}

func (r *Responder) AnswerKnowledgeSynthesis(ctx context.Context, input KnowledgeSynthesisPromptInput) (string, string, error) {
	if r == nil {
		return "", "", fmt.Errorf("query executor responder is nil")
	}
	systemPrompt, userPrompt := buildKnowledgeSynthesisPrompts(input)
	answer, err := r.completeText(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", "", err
	}
	return answer, r.config.Model, nil
}

func (r *Responder) SelectExactQuoteIDs(ctx context.Context, input ExactQuoteSelectionInput) ([]string, error) {
	if r == nil {
		return nil, fmt.Errorf("query executor responder is nil")
	}
	systemPrompt, userPrompt := buildExactQuoteSelectionPrompts(input)
	content, err := r.completeText(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}
	jsonBody, err := extractJSONObject(content)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		EvidenceIDs []string `json:"evidence_ids"`
	}
	if err := json.Unmarshal([]byte(jsonBody), &parsed); err != nil {
		return nil, fmt.Errorf("parse exact quote selection failed: %w", err)
	}
	return normalizeTokens(parsed.EvidenceIDs), nil
}

func (r *Responder) completeText(ctx context.Context, systemPrompt string, userPrompt string) (string, error) {
	payload := map[string]any{
		"model": r.config.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"stream":      false,
		"temperature": 0.2,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, r.config.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+r.config.APIKey)

	resp, err := r.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("query executor llm request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return "", fmt.Errorf("query executor llm response read failed: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("query executor llm request failed: status=%d %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("query executor llm response parse failed: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("query executor llm response empty")
	}
	answer := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if answer == "" {
		return "", fmt.Errorf("query executor llm answer is empty")
	}
	return answer, nil
}

func buildKnowledgeLookupPrompts(input KnowledgeLookupPromptInput) (string, string) {
	systemPrompt := strings.Join([]string{
		"你是 AIM 的知识库问答助手。",
		"你只能依据提供的知识库片段作答，不要编造知识库中不存在的信息。",
		"如果证据不足，必须明确说“根据当前知识库资料无法确定”。",
		"如果引用证据，请使用 [1] [2] 这种编号引用，不要编造编号。",
	}, "\n")

	taskInstruction := "请直接回答问题，并在关键结论后附上引用编号。"
	switch input.OutputMode {
	case queryrouter.OutputModeSummary:
		taskInstruction = "请基于给定片段做简洁总结，并在关键结论后附上引用编号。"
	case queryrouter.OutputModeOutline:
		taskInstruction = "请按提纲形式组织答案，并在关键结论后附上引用编号。"
	case queryrouter.OutputModeExtract:
		taskInstruction = "请从给定片段中提取与问题最相关的信息，按要点列出，并在每条后附上引用编号。"
	case queryrouter.OutputModeRewrite:
		taskInstruction = "请在不改变原意的前提下重写答案，并在关键结论后附上引用编号。"
	case queryrouter.OutputModeTable:
		taskInstruction = "请尽量用 Markdown 表格组织答案，并在表格单元格中保留引用编号。"
	case queryrouter.OutputModeTimeline:
		taskInstruction = "请按时间线组织答案，并在每条后附上引用编号。"
	case queryrouter.OutputModeCompare:
		taskInstruction = "请按对比形式组织答案，并在关键结论后附上引用编号。"
	case queryrouter.OutputModeQuiz:
		taskInstruction = "请根据给定片段生成简短测验题，并在答案或解析后附上引用编号。"
	}
	if input.EvidenceMode == queryrouter.EvidenceModeCitation {
		taskInstruction += " 需要保持引用可追溯。"
	}

	lines := make([]string, 0, len(input.Citations)+8)
	if name := strings.TrimSpace(input.KnowledgeBaseName); name != "" {
		lines = append(lines, "知识库："+name)
	}
	lines = append(lines, "任务要求："+taskInstruction)
	lines = append(lines, "用户问题："+strings.TrimSpace(input.Query))
	lines = append(lines, "", "知识库证据：")

	citations := append([]Citation(nil), input.Citations...)
	sort.SliceStable(citations, func(i, j int) bool {
		return citations[i].Index < citations[j].Index
	})
	for _, item := range citations {
		lines = append(lines, fmt.Sprintf("[%d] 文档：%s（documentId=%d, chunkId=%d, score=%.3f）", item.Index, item.DocumentTitle, item.DocumentID, item.ChunkID, item.Score))
		lines = append(lines, item.Excerpt)
		lines = append(lines, "")
	}
	return systemPrompt, strings.Join(lines, "\n")
}

func buildKnowledgeReadPrompts(input KnowledgeReadPromptInput, batch string, instruction string) (string, string) {
	systemPrompt := strings.Join([]string{
		"你是 AIM 的长文阅读助手。",
		"你需要基于按顺序提供的文档内容进行阅读和理解，不要编造文档中不存在的信息。",
		"如果证据不足，必须明确说“根据当前文档内容无法确定”。",
		"当前阶段不支持逐字原句抽取，因此不要假装输出逐字引文。",
	}, "\n")

	taskInstruction := "请阅读以下文档内容，并回答用户问题。"
	switch input.OutputMode {
	case queryrouter.OutputModeSummary:
		taskInstruction = "请阅读以下文档内容，并输出简洁、结构化的总结。"
	case queryrouter.OutputModeOutline:
		taskInstruction = "请阅读以下文档内容，并按提纲形式输出核心结构与要点。"
	case queryrouter.OutputModeExtract:
		taskInstruction = "请阅读以下文档内容，并提取与用户问题最相关的关键信息。"
	case queryrouter.OutputModeRewrite:
		taskInstruction = "请阅读以下文档内容，并在不改变原意的前提下组织更清晰的答案。"
	case queryrouter.OutputModeTable:
		taskInstruction = "请阅读以下文档内容，并尽量用 Markdown 表格组织答案。"
	case queryrouter.OutputModeTimeline:
		taskInstruction = "请阅读以下文档内容，并按时间线组织答案。"
	case queryrouter.OutputModeCompare:
		taskInstruction = "请阅读以下文档内容，并按对比视角组织答案。"
	case queryrouter.OutputModeQuiz:
		taskInstruction = "请阅读以下文档内容，并生成简短测验题及答案。"
	}

	lines := []string{
		"知识库：" + strings.TrimSpace(input.KnowledgeBaseName),
		"文档：" + strings.TrimSpace(input.DocumentTitle),
		"任务要求：" + taskInstruction,
		"用户问题：" + strings.TrimSpace(input.Query),
	}
	if instruction = strings.TrimSpace(instruction); instruction != "" {
		lines = append(lines, "补充说明："+instruction)
	}
	lines = append(lines, "", "文档内容：", batch)
	return systemPrompt, strings.Join(lines, "\n")
}

func buildKnowledgeReadAggregatePrompts(input KnowledgeReadPromptInput, batchSummaries []string) (string, string) {
	systemPrompt := strings.Join([]string{
		"你是 AIM 的长文综合助手。",
		"你会收到同一文档不同批次的阅读要点，请把它们整合成最终答案。",
		"只能依据提供的阅读要点作答，不要编造额外信息。",
	}, "\n")
	taskInstruction := "请综合各批次要点，回答用户问题。"
	switch input.OutputMode {
	case queryrouter.OutputModeSummary:
		taskInstruction = "请综合各批次要点，输出最终总结。"
	case queryrouter.OutputModeOutline:
		taskInstruction = "请综合各批次要点，输出最终提纲。"
	case queryrouter.OutputModeExtract:
		taskInstruction = "请综合各批次要点，输出最终抽取结果。"
	case queryrouter.OutputModeTable:
		taskInstruction = "请综合各批次要点，并尽量用 Markdown 表格输出。"
	case queryrouter.OutputModeTimeline:
		taskInstruction = "请综合各批次要点，并按时间线输出。"
	case queryrouter.OutputModeCompare:
		taskInstruction = "请综合各批次要点，并按对比视角输出。"
	case queryrouter.OutputModeQuiz:
		taskInstruction = "请综合各批次要点，输出测验题及答案。"
	}

	lines := []string{
		"知识库：" + strings.TrimSpace(input.KnowledgeBaseName),
		"文档：" + strings.TrimSpace(input.DocumentTitle),
		"任务要求：" + taskInstruction,
		"用户问题：" + strings.TrimSpace(input.Query),
		"",
		"分批阅读要点：",
	}
	for index, item := range batchSummaries {
		lines = append(lines, fmt.Sprintf("第%d批要点：", index+1))
		lines = append(lines, strings.TrimSpace(item))
		lines = append(lines, "")
	}
	return systemPrompt, strings.Join(lines, "\n")
}

func buildKnowledgeSynthesisPrompts(input KnowledgeSynthesisPromptInput) (string, string) {
	systemPrompt := strings.Join([]string{
		"你是 AIM 的多文档综合助手。",
		"你会收到多份文档各自的阅读要点，请在不编造事实的前提下进行比较、综合和归纳。",
		"如果资料不足以支撑某个结论，必须明确说明无法确定。",
		"当前阶段不支持逐字原句抽取，因此不要假装给出逐字原文。",
	}, "\n")

	taskInstruction := "请综合多份文档要点，回答用户问题。"
	switch input.OutputMode {
	case queryrouter.OutputModeSummary:
		taskInstruction = "请综合多份文档要点，输出总结。"
	case queryrouter.OutputModeCompare:
		taskInstruction = "请重点比较多份文档的异同，并输出结构化对比结论。"
	case queryrouter.OutputModeOutline:
		taskInstruction = "请综合多份文档要点，按提纲形式输出。"
	case queryrouter.OutputModeExtract:
		taskInstruction = "请综合多份文档要点，提取与用户问题最相关的信息。"
	case queryrouter.OutputModeTable:
		taskInstruction = "请综合多份文档要点，并尽量使用 Markdown 表格输出。"
	case queryrouter.OutputModeTimeline:
		taskInstruction = "请综合多份文档要点，并按时间线输出。"
	case queryrouter.OutputModeQuiz:
		taskInstruction = "请综合多份文档要点，生成测验题及答案。"
	case queryrouter.OutputModeRewrite:
		taskInstruction = "请综合多份文档要点，重写成更清晰的答案。"
	}

	lines := []string{
		"知识库：" + strings.TrimSpace(input.KnowledgeBaseName),
		"任务要求：" + taskInstruction,
		"用户问题：" + strings.TrimSpace(input.Query),
		"",
		"各文档阅读要点：",
	}
	for index, item := range input.Documents {
		title := strings.TrimSpace(item.DocumentTitle)
		if title == "" {
			title = fmt.Sprintf("文档%d", item.DocumentID)
		}
		lines = append(lines, fmt.Sprintf("文档%d：《%s》", index+1, title))
		lines = append(lines, strings.TrimSpace(item.Note))
		lines = append(lines, "")
	}
	return systemPrompt, strings.Join(lines, "\n")
}

func buildExactQuoteSelectionPrompts(input ExactQuoteSelectionInput) (string, string) {
	systemPrompt := strings.Join([]string{
		"你是 AIM 的原句证据选择器。",
		"你只能从提供的候选句子里选择最能支撑答案的原句。",
		"不要改写句子，不要输出候选集中不存在的 quoteId。",
		"只返回一个 JSON 对象，格式为 {\"evidence_ids\":[\"Q1\",\"Q2\"]}。",
		"如果没有合适的原句，返回 {\"evidence_ids\":[]}。",
	}, "\n")

	lines := []string{
		"任务：为已有答案选择最合适的原句证据。",
		"问题：" + strings.TrimSpace(input.Query),
		"答案：" + strings.TrimSpace(input.Answer),
	}
	if hint := strings.TrimSpace(input.DocumentHint); hint != "" {
		lines = append(lines, "范围提示："+hint)
	}
	lines = append(lines, "", "候选原句：")
	for _, item := range input.Candidates {
		lines = append(lines, fmt.Sprintf("%s | 文档：%s | chunkId=%d | sentenceIndex=%d", item.QuoteID, item.DocumentTitle, item.ChunkID, item.SentenceIndex))
		lines = append(lines, item.Text)
		lines = append(lines, "")
	}
	return systemPrompt, strings.Join(lines, "\n")
}

func buildDocumentReadBatches(chunks []*ragpb.KnowledgeDocumentChunkInfo, maxChars int) []string {
	if len(chunks) == 0 {
		return nil
	}
	if maxChars <= 0 {
		maxChars = 8000
	}
	batches := make([]string, 0, len(chunks))
	var builder strings.Builder
	currentChars := 0

	flush := func() {
		content := strings.TrimSpace(builder.String())
		if content != "" {
			batches = append(batches, content)
		}
		builder.Reset()
		currentChars = 0
	}

	for _, item := range chunks {
		if item == nil {
			continue
		}
		part := formatDocumentChunk(item)
		partLen := len([]rune(part))
		if currentChars > 0 && currentChars+partLen > maxChars {
			flush()
		}
		builder.WriteString(part)
		if !strings.HasSuffix(part, "\n") {
			builder.WriteByte('\n')
		}
		builder.WriteByte('\n')
		currentChars += partLen + 2
	}
	flush()
	return batches
}

func buildExactQuoteCandidatesFromDocumentChunks(chunks []*ragpb.KnowledgeDocumentChunkInfo, documentTitle string, query string, limit int) []ExactQuoteCandidate {
	candidates := make([]ExactQuoteCandidate, 0, len(chunks)*2)
	queryTokens := scoreTokens(query)
	for _, item := range chunks {
		if item == nil {
			continue
		}
		next := splitDocumentChunkIntoCandidates(item, documentTitle, 0)
		for idx := range next {
			next[idx].Score = sentenceScore(next[idx].Text, queryTokens)
		}
		candidates = append(candidates, next...)
	}
	return normalizeExactQuoteCandidates(candidates, limit)
}

func splitDocumentChunkIntoCandidates(item *ragpb.KnowledgeDocumentChunkInfo, documentTitle string, limit int) []ExactQuoteCandidate {
	if item == nil {
		return nil
	}
	if len(item.Sentences) > 0 {
		result := make([]ExactQuoteCandidate, 0, len(item.Sentences))
		for _, sentence := range item.Sentences {
			if sentence == nil || strings.TrimSpace(sentence.Text) == "" {
				continue
			}
			result = append(result, ExactQuoteCandidate{
				QuoteID:       fmt.Sprintf("Q%d_%d", item.ChunkId, sentence.SentenceIndex),
				DocumentID:    item.DocumentId,
				DocumentTitle: documentTitle,
				ChunkID:       item.ChunkId,
				SentenceIndex: int(sentence.SentenceIndex),
				PageStart:     int(sentence.PageStart),
				PageEnd:       int(sentence.PageEnd),
				CharStart:     int(sentence.CharStart),
				CharEnd:       int(sentence.CharEnd),
				Text:          strings.TrimSpace(sentence.Text),
			})
			if limit > 0 && len(result) >= limit {
				break
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	sentences := splitSentencesWithOffsets(item.Content)
	if len(sentences) == 0 {
		return nil
	}
	result := make([]ExactQuoteCandidate, 0, len(sentences))
	for index, sentence := range sentences {
		text := strings.TrimSpace(sentence.Text)
		if text == "" {
			continue
		}
		result = append(result, ExactQuoteCandidate{
			QuoteID:       fmt.Sprintf("Q%d_%d", item.ChunkId, index+1),
			DocumentID:    item.DocumentId,
			DocumentTitle: documentTitle,
			ChunkID:       item.ChunkId,
			SentenceIndex: index + 1,
			PageStart:     int(item.PageStart),
			PageEnd:       int(item.PageEnd),
			CharStart:     int(item.CharStart) + sentence.Start,
			CharEnd:       int(item.CharStart) + sentence.End,
			Text:          text,
		})
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

type sentenceSpan struct {
	Text  string
	Start int
	End   int
}

func splitSentencesWithOffsets(content string) []sentenceSpan {
	text := strings.TrimSpace(content)
	if text == "" {
		return nil
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	var (
		result  []sentenceSpan
		builder strings.Builder
		start   = -1
		cursor  = 0
	)
	flush := func() {
		value := strings.TrimSpace(builder.String())
		if value != "" {
			end := cursor
			result = append(result, sentenceSpan{Text: value, Start: maxInt(start, 0), End: end})
		}
		builder.Reset()
		start = -1
	}
	for _, r := range text {
		if start < 0 {
			start = cursor
		}
		builder.WriteRune(r)
		cursor++
		switch r {
		case '\n', '。', '！', '？', '；', '.', '!', '?', ';':
			flush()
		}
	}
	flush()
	return result
}

func scoreTokens(query string) []string {
	normalized := strings.ToLower(strings.TrimSpace(query))
	if normalized == "" {
		return nil
	}
	replacer := strings.NewReplacer("，", " ", "。", " ", "：", " ", "；", " ", "？", " ", "！", " ", ",", " ", ".", " ", ":", " ", ";", " ", "?", " ", "!", " ", "\n", " ")
	normalized = replacer.Replace(normalized)
	fields := strings.Fields(normalized)
	return normalizeTokens(fields)
}

func sentenceScore(sentence string, queryTokens []string) int {
	if len(queryTokens) == 0 {
		return 0
	}
	value := strings.ToLower(strings.TrimSpace(sentence))
	score := 0
	for _, token := range queryTokens {
		if token == "" {
			continue
		}
		if strings.Contains(value, token) {
			score += 2
		}
	}
	return score
}

func normalizeExactQuoteCandidates(candidates []ExactQuoteCandidate, limit int) []ExactQuoteCandidate {
	if len(candidates) == 0 {
		return nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			if candidates[i].DocumentID == candidates[j].DocumentID {
				if candidates[i].ChunkID == candidates[j].ChunkID {
					return candidates[i].SentenceIndex < candidates[j].SentenceIndex
				}
				return candidates[i].ChunkID < candidates[j].ChunkID
			}
			return candidates[i].DocumentID < candidates[j].DocumentID
		}
		return candidates[i].Score > candidates[j].Score
	})
	seen := make(map[string]struct{}, len(candidates))
	result := make([]ExactQuoteCandidate, 0, len(candidates))
	for _, item := range candidates {
		key := strings.TrimSpace(item.Text)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

func materializeExactQuotes(candidates []ExactQuoteCandidate, selectedIDs []string) []ExactQuote {
	if len(candidates) == 0 || len(selectedIDs) == 0 {
		return nil
	}
	byID := make(map[string]ExactQuoteCandidate, len(candidates))
	for _, item := range candidates {
		byID[item.QuoteID] = item
	}
	result := make([]ExactQuote, 0, len(selectedIDs))
	seen := make(map[string]struct{}, len(selectedIDs))
	for _, id := range selectedIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		item, ok := byID[id]
		if !ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, ExactQuote{
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
	return result
}

func extractJSONObject(content string) (string, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```JSON")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return "", fmt.Errorf("exact quote selector did not return a json object")
	}
	return content[start : end+1], nil
}

func normalizeTokens(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	result := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}

func formatDocumentChunk(item *ragpb.KnowledgeDocumentChunkInfo) string {
	lines := []string{
		fmt.Sprintf("【Chunk %d】", item.ChunkIndex),
	}
	if title := strings.TrimSpace(item.SectionTitle); title != "" {
		lines = append(lines, "章节："+title)
	}
	if chunkType := strings.TrimSpace(item.ChunkType); chunkType != "" {
		lines = append(lines, "类型："+chunkType)
	}
	lines = append(lines, strings.TrimSpace(item.Content))
	return strings.Join(lines, "\n")
}
