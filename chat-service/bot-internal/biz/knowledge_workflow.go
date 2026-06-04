package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"example.com/aim/chat-service/internal/dal/model"
	llm "example.com/aim/chat-service/llm-internal/client"
	ragpb "example.com/aim/chat-service/kitex_gen/rag"
)

type knowledgeWorkflowRequest struct {
	LLMClient        llm.Client
	ModelName        string
	ResolvedBot      resolvedBot
	MentionRequest   HandleMentionRequest
	Question         string
	PromptContent    string
	KnowledgeScope   model.BotPermissionScope
	UserDisplayNames map[uint64]string
	RecentMessages   []model.Message
	StreamMeta       *botReplyStreamMeta
	ActiveContext    *activeKnowledgeContext
}

type knowledgeWorkflowResult struct {
	Plan             workflowPlan
	Answer           string
	KnowledgeContext string
	GenerateResp     *llm.GenerateResponse
	DirectAnswer     bool
	LatencyMS        int64
	PrimaryDocID     uint64
	PrimaryDocTitle  string
	DocumentRefs     []knowledgeContextDocument
}

type workflowFamily string

const (
	workflowFamilyLookup      workflowFamily = "LOOKUP"
	workflowFamilyRead        workflowFamily = "READ"
	workflowFamilySynthesize  workflowFamily = "SYNTHESIZE"
	workflowFamilyUnsupported workflowFamily = "UNSUPPORTED"
)

type workflowOutputMode string

const (
	workflowOutputAnswer   workflowOutputMode = "answer"
	workflowOutputSummary  workflowOutputMode = "summary"
	workflowOutputCompare  workflowOutputMode = "compare"
	workflowOutputExtract  workflowOutputMode = "extract"
	workflowOutputOutline  workflowOutputMode = "outline"
	workflowOutputTable    workflowOutputMode = "table"
	workflowOutputTimeline workflowOutputMode = "timeline"
	workflowOutputQuiz     workflowOutputMode = "quiz"
	workflowOutputRewrite  workflowOutputMode = "rewrite"
)

type workflowEvidenceMode string

const (
	workflowEvidenceNone       workflowEvidenceMode = "none"
	workflowEvidenceCitation   workflowEvidenceMode = "citation"
	workflowEvidenceExactQuote workflowEvidenceMode = "exact_quote"
)

type workflowPlan struct {
	Family       workflowFamily       `json:"family"`
	OutputMode   workflowOutputMode   `json:"output_mode"`
	EvidenceMode workflowEvidenceMode `json:"evidence_mode"`
	Reason       string               `json:"reason"`
	Confidence   float64              `json:"confidence"`
}

type workflowQuoteCandidate struct {
	QuoteID       string
	DocumentTitle string
	DocumentID    int64
	ChunkID       int64
	SentenceIndex int
	PageStart     int
	PageEnd       int
	CharStart     int
	CharEnd       int
	Text          string
	Score         int
}

func (s *Service) runKnowledgeWorkflow(ctx context.Context, req knowledgeWorkflowRequest) (*knowledgeWorkflowResult, error) {
	if s == nil || s.RAGClient == nil || req.LLMClient == nil {
		return nil, nil
	}
	conversation, err := s.ConversationRepo.GetByID(ctx, req.MentionRequest.ConversationID)
	if err != nil || conversation == nil {
		return nil, err
	}

	bindingsResp, err := s.RAGClient.ListConversationKnowledgeBases(ctx, &ragpb.ListConversationKnowledgeBasesRequest{
		OperatorId:     int64(req.MentionRequest.UserID),
		ConversationId: conversation.ConversationID,
	})
	if err != nil {
		return nil, err
	}
	bindings := bindingsResp.GetKnowledgeBases()
	if len(bindings) == 0 {
		if s.KnowledgeContextStore != nil {
			s.KnowledgeContextStore.Delete(req.MentionRequest.UserID, req.MentionRequest.ConversationID, req.ResolvedBot.Bot.ID)
		}
		return &knowledgeWorkflowResult{
			Plan: workflowPlan{
				Family:       workflowFamilyUnsupported,
				OutputMode:   workflowOutputAnswer,
				EvidenceMode: workflowEvidenceNone,
				Reason:       "当前会话未绑定知识库",
				Confidence:   1,
			},
			Answer:           "当前未检索到可用的知识库资料，无法基于知识库回答。",
			KnowledgeContext: "",
			DirectAnswer:     true,
			LatencyMS:        0,
		}, nil
	}

	plan, err := planKnowledgeWorkflow(ctx, req, bindings)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, nil
	}
	switch plan.Family {
	case workflowFamilyLookup:
		result, err := s.executeKnowledgeLookup(ctx, req, *conversation, bindings, *plan)
		s.rememberKnowledgeContext(req, result)
		return result, err
	case workflowFamilyRead:
		result, err := s.executeKnowledgeRead(ctx, req, *conversation, bindings, *plan)
		s.rememberKnowledgeContext(req, result)
		return result, err
	case workflowFamilySynthesize:
		result, err := s.executeKnowledgeSynthesize(ctx, req, *conversation, bindings, *plan)
		s.rememberKnowledgeContext(req, result)
		return result, err
	default:
		return &knowledgeWorkflowResult{
			Plan:             *plan,
			Answer:           "当前知识库请求超出当前机器人支持范围。",
			KnowledgeContext: "",
			DirectAnswer:     true,
			LatencyMS:        0,
		}, nil
	}
}

func planKnowledgeWorkflow(ctx context.Context, req knowledgeWorkflowRequest, bindings []*ragpb.ConversationKnowledgeBaseInfo) (*workflowPlan, error) {
	if plan := fastPlanKnowledgeWorkflow(req, bindings); plan != nil {
		return plan, nil
	}
	client, modelName, err := newKnowledgeRouterClient()
	if err != nil {
		return fallbackPlanKnowledgeWorkflow(req, bindings), nil
	}
	systemPrompt := strings.Join([]string{
		"你是 AIM 机器人知识库路由规划器。",
		"你不回答问题，只把用户问题规划成一个 JSON 对象。",
		"只能输出 JSON，不要输出 markdown。",
		"family 只能是 LOOKUP / READ / SYNTHESIZE / UNSUPPORTED。",
		"output_mode 只能是 answer / summary / compare / extract / outline / table / timeline / quiz / rewrite。",
		"evidence_mode 只能是 none / citation / exact_quote。",
		"如果用户要求原句、原文措辞、关键句、逐字引用，evidence_mode=exact_quote。",
		"如果是局部事实问题，family=LOOKUP。",
		"如果是单文档整体理解、总结、提炼，family=READ。",
		"如果是多文档比较、综合、对照，family=SYNTHESIZE。",
		"如果无法明确文档范围但知识库里有多份文档，优先 SYNTHESIZE 而不是直接拒绝。",
		"输出格式：{\"family\":\"READ\",\"output_mode\":\"summary\",\"evidence_mode\":\"exact_quote\",\"reason\":\"...\",\"confidence\":0.9}",
	}, "\n")

	queries := buildKnowledgeQueriesForWorkflow(req)
	lines := []string{
		"当前会话绑定的知识库：",
	}
	for _, item := range bindings {
		if item == nil {
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s（kb=%d）", strings.TrimSpace(item.Name), item.KnowledgeBaseId))
	}
	lines = append(lines, "", "用户问题："+strings.TrimSpace(req.Question))
	if len(queries) > 1 {
		lines = append(lines, "上下文查询变体：")
		for _, query := range queries {
			lines = append(lines, "- "+query)
		}
	}

	resp, err := client.Generate(ctx, llm.GenerateRequest{
		Model: modelName,
		Messages: []llm.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: strings.Join(lines, "\n")},
		},
	})
	if err != nil {
		return nil, err
	}
	content := strings.TrimSpace(resp.Content)
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return nil, nil
	}
	content = content[start : end+1]
	var plan workflowPlan
	if err := json.Unmarshal([]byte(content), &plan); err != nil {
		return nil, err
	}
	plan.Family = normalizeWorkflowFamily(plan.Family)
	plan.OutputMode = normalizeWorkflowOutputMode(plan.OutputMode)
	plan.EvidenceMode = normalizeWorkflowEvidenceMode(plan.EvidenceMode)
	if plan.Family == "" {
		plan.Family = workflowFamilyRead
	}
	if plan.OutputMode == "" {
		plan.OutputMode = workflowOutputAnswer
	}
	if plan.EvidenceMode == "" {
		plan.EvidenceMode = workflowEvidenceNone
	}
	return &plan, nil
}

func fastPlanKnowledgeWorkflow(req knowledgeWorkflowRequest, bindings []*ragpb.ConversationKnowledgeBaseInfo) *workflowPlan {
	if plan := planFromActiveKnowledgeContext(req); plan != nil {
		return plan
	}
	queries := buildKnowledgeQueriesForWorkflow(req)
	text := strings.ToLower(strings.Join(queries, " "))
	if strings.TrimSpace(text) == "" {
		text = strings.ToLower(strings.TrimSpace(req.Question))
	}
	exactQuote := containsAny(text, "原句", "原文", "关键句", "摘录", "逐字", "引用")
	extract := containsAny(text, "提取", "摘录", "列出", "整理出")
	compare := containsAny(text, "比较", "对比", "区别", "异同", "不同点", "共同点", "分别")
	summary := containsAny(text, "总结", "概括", "梳理", "提纲", "主旨", "核心观点", "脉络")
	chapter := containsAny(text, "第") && containsAny(text, "章", "节", "部分")
	fact := containsAny(text, "是什么", "什么意思", "定义", "多少", "谁", "何时", "哪一页", "有没有提到")

	evidenceMode := workflowEvidenceNone
	if exactQuote {
		evidenceMode = workflowEvidenceExactQuote
	}
	outputMode := workflowOutputAnswer
	if extract {
		outputMode = workflowOutputExtract
	} else if compare {
		outputMode = workflowOutputCompare
	} else if summary {
		outputMode = workflowOutputSummary
	}

	switch {
	case compare:
		return &workflowPlan{Family: workflowFamilySynthesize, OutputMode: chooseOutput(outputMode, workflowOutputCompare), EvidenceMode: evidenceMode, Reason: "规则命中：比较类请求", Confidence: 0.98}
	case exactQuote || extract || chapter:
		return &workflowPlan{Family: workflowFamilyRead, OutputMode: chooseOutput(outputMode, workflowOutputExtract), EvidenceMode: chooseEvidence(evidenceMode, workflowEvidenceExactQuote), Reason: "规则命中：原句/提取/章节类请求", Confidence: 0.98}
	case fact && len(bindings) <= 1:
		return &workflowPlan{Family: workflowFamilyLookup, OutputMode: workflowOutputAnswer, EvidenceMode: chooseEvidence(evidenceMode, workflowEvidenceCitation), Reason: "规则命中：局部事实问题", Confidence: 0.93}
	case summary && len(bindings) <= 1:
		return &workflowPlan{Family: workflowFamilyRead, OutputMode: workflowOutputSummary, EvidenceMode: evidenceMode, Reason: "规则命中：单知识库总结请求", Confidence: 0.94}
	case summary && len(bindings) > 1:
		return &workflowPlan{Family: workflowFamilySynthesize, OutputMode: workflowOutputSummary, EvidenceMode: evidenceMode, Reason: "规则命中：多知识库总结请求", Confidence: 0.92}
	default:
		return nil
	}
}

func fallbackPlanKnowledgeWorkflow(req knowledgeWorkflowRequest, bindings []*ragpb.ConversationKnowledgeBaseInfo) *workflowPlan {
	if len(bindings) <= 1 {
		return &workflowPlan{Family: workflowFamilyRead, OutputMode: workflowOutputAnswer, EvidenceMode: workflowEvidenceNone, Reason: "回退：默认阅读路径", Confidence: 0.68}
	}
	return &workflowPlan{Family: workflowFamilySynthesize, OutputMode: workflowOutputAnswer, EvidenceMode: workflowEvidenceNone, Reason: "回退：默认综合路径", Confidence: 0.66}
}

func planFromActiveKnowledgeContext(req knowledgeWorkflowRequest) *workflowPlan {
	ctx := req.ActiveContext
	if ctx == nil {
		return nil
	}
	if isStrongKnowledgeRetarget(req.Question, *ctx) {
		return nil
	}
	output := ctx.OutputMode
	evidence := ctx.EvidenceMode
	lower := strings.ToLower(strings.TrimSpace(req.Question))
	if containsAny(lower, "原句", "原文", "关键句", "摘录", "逐字", "引用") {
		evidence = workflowEvidenceExactQuote
		output = workflowOutputExtract
	}
	if containsAny(lower, "比较", "对比", "区别", "异同", "不同点", "共同点", "分别") {
		return &workflowPlan{Family: workflowFamilySynthesize, OutputMode: workflowOutputCompare, EvidenceMode: evidence, Reason: "继承活跃知识上下文：比较追问", Confidence: 0.99}
	}
	if containsAny(lower, "总结", "概括", "梳理", "提纲", "主旨", "核心观点", "脉络") {
		if len(ctx.DocumentRefs) > 1 {
			return &workflowPlan{Family: workflowFamilySynthesize, OutputMode: workflowOutputSummary, EvidenceMode: evidence, Reason: "继承活跃知识上下文：总结追问", Confidence: 0.99}
		}
		return &workflowPlan{Family: workflowFamilyRead, OutputMode: workflowOutputSummary, EvidenceMode: evidence, Reason: "继承活跃知识上下文：总结追问", Confidence: 0.99}
	}
	switch ctx.Family {
	case workflowFamilyRead, workflowFamilyLookup:
		return &workflowPlan{Family: workflowFamilyRead, OutputMode: output, EvidenceMode: evidence, Reason: "继承活跃知识上下文", Confidence: 0.97}
	case workflowFamilySynthesize:
		return &workflowPlan{Family: workflowFamilySynthesize, OutputMode: output, EvidenceMode: evidence, Reason: "继承活跃知识上下文", Confidence: 0.97}
	default:
		return nil
	}
}

func isStrongKnowledgeRetarget(question string, ctx activeKnowledgeContext) bool {
	lower := strings.ToLower(strings.TrimSpace(question))
	if lower == "" {
		return false
	}
	if containsAny(lower, "换成", "换到", "另外", "另一", "另一个", "重新", "别的", "新的", "不要看这个", "不看这个") {
		return true
	}
	if title := normalizeDocMatchText(ctx.PrimaryDocTitle); title != "" && !strings.Contains(lower, title) {
		for _, ref := range ctx.DocumentRefs {
			next := normalizeDocMatchText(ref.Title)
			if next != "" && next != title && strings.Contains(lower, next) {
				return true
			}
		}
	}
	return false
}

func newKnowledgeRouterClient() (llm.Client, string, error) {
	cfg := llm.Config{
		BaseURL:            strings.TrimSpace(os.Getenv("LLM_BASE_URL")),
		APIKey:             strings.TrimSpace(os.Getenv("LLM_API_KEY")),
		Model:              getenvOr("KNOWLEDGE_ROUTER_MODEL", "deepseek-v4-flash"),
		Timeout:            durationEnv("KNOWLEDGE_ROUTER_TIMEOUT_SECONDS", 8*time.Second),
		InsecureSkipVerify: boolEnv("LLM_INSECURE_SKIP_VERIFY", false),
	}
	client, err := llm.NewOpenAICompatibleClient(cfg)
	if err != nil {
		return nil, "", err
	}
	return client, cfg.Model, nil
}

func containsAny(text string, tokens ...string) bool {
	for _, token := range tokens {
		if strings.Contains(text, token) {
			return true
		}
	}
	return false
}

func chooseOutput(current workflowOutputMode, fallback workflowOutputMode) workflowOutputMode {
	if current != workflowOutputAnswer && current != "" {
		return current
	}
	return fallback
}

func chooseEvidence(current workflowEvidenceMode, fallback workflowEvidenceMode) workflowEvidenceMode {
	if current != workflowEvidenceNone && current != "" {
		return current
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return fallback
}

func boolEnv(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getenvOr(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func (s *Service) executeKnowledgeLookup(
	ctx context.Context,
	req knowledgeWorkflowRequest,
	conversation model.Conversation,
	bindings []*ragpb.ConversationKnowledgeBaseInfo,
	plan workflowPlan,
) (*knowledgeWorkflowResult, error) {
	start := time.Now()
	all := make([]RAGChunk, 0, len(bindings)*5)
	docTitles := make(map[int64]string)
	for _, binding := range bindings {
		if binding == nil || binding.KnowledgeBaseId <= 0 {
			continue
		}
		searchResp, err := s.RAGClient.SearchKnowledgeBase(ctx, &ragpb.SearchKnowledgeBaseRequest{
			OperatorId:      int64(req.MentionRequest.UserID),
			KnowledgeBaseId: binding.KnowledgeBaseId,
			Query:           strings.TrimSpace(req.Question),
			TopK:            int32PtrKnowledge(5),
		})
		if err != nil {
			return nil, err
		}
		for _, item := range searchResp.GetChunks() {
			if item == nil || strings.TrimSpace(item.Content) == "" {
				continue
			}
			all = append(all, RAGChunk{Index: len(all) + 1, Content: item.Content, Score: item.Score})
			docTitles[item.DocumentId] = binding.Name
		}
	}
	if len(all) == 0 {
		return &knowledgeWorkflowResult{
			Plan:             plan,
			Answer:           "当前未检索到可用的知识库资料，无法基于知识库回答。",
			KnowledgeContext: "",
			DirectAnswer:     true,
			LatencyMS:        time.Since(start).Milliseconds(),
		}, nil
	}

	answerResp, err := s.generateKnowledgeWorkflowResponse(ctx, req, knowledgeLookupSystemPrompt(), knowledgeLookupUserPrompt(req.Question, all), plan.EvidenceMode != workflowEvidenceExactQuote)
	if err != nil {
		return nil, err
	}
	answer := strings.TrimSpace(answerResp.Content)
	chunkMap := make(map[int64][]*ragpb.KnowledgeDocumentChunkInfo)
	if plan.EvidenceMode == workflowEvidenceExactQuote {
		documents, resolveErr := s.resolveKnowledgeDocuments(ctx, req, bindings)
		if resolveErr != nil {
			return nil, resolveErr
		}
		for _, item := range documents {
			chunkMap[int64(item.DocumentID)] = item.Chunks
		}
		quotes, err := selectQuotesForLookup(ctx, req.LLMClient, req.ModelName, req.Question, answer, chunkMap)
		if err != nil {
			return nil, err
		}
		answer = appendQuotesToAnswer(answer, quotes)
	}
	return &knowledgeWorkflowResult{
		Plan:             plan,
		Answer:           answer,
		KnowledgeContext: answer,
		GenerateResp:     answerResp,
		DirectAnswer:     plan.EvidenceMode == workflowEvidenceExactQuote,
		LatencyMS:        time.Since(start).Milliseconds(),
		DocumentRefs:     collectKnowledgeContextDocumentsFromChunks(chunkMap),
	}, nil
}

func (s *Service) executeKnowledgeRead(
	ctx context.Context,
	req knowledgeWorkflowRequest,
	conversation model.Conversation,
	bindings []*ragpb.ConversationKnowledgeBaseInfo,
	plan workflowPlan,
) (*knowledgeWorkflowResult, error) {
	start := time.Now()
	queries := buildKnowledgeQueriesForWorkflow(req)
	hits, err := s.searchKnowledgeHits(ctx, req, bindings, queries, 8)
	if err != nil {
		return nil, err
	}
	document, chunks, err := s.resolveReadableKnowledgeDocument(ctx, req, bindings, queries, hits)
	if err != nil {
		return nil, err
	}
	if document == nil || len(chunks) == 0 {
		return &knowledgeWorkflowResult{
			Plan:             plan,
			Answer:           "当前知识库无法唯一确定要精读的文档，请明确文档范围。",
			KnowledgeContext: "",
			DirectAnswer:     true,
			LatencyMS:        time.Since(start).Milliseconds(),
		}, nil
	}

	answerResp, err := s.generateKnowledgeReadAnswer(ctx, req, plan.OutputMode, document.Title, chunks, plan.EvidenceMode != workflowEvidenceExactQuote)
	if err != nil {
		return nil, err
	}
	answer := strings.TrimSpace(answerResp.Content)
	if plan.EvidenceMode == workflowEvidenceExactQuote {
		quotes, err := selectQuotesFromDocument(ctx, req.LLMClient, req.ModelName, req.Question, answer, document.Title, chunks, 24)
		if err != nil {
			return nil, err
		}
		answer = appendQuotesToAnswer(answer, quotes)
	}
	return &knowledgeWorkflowResult{
		Plan:             plan,
		Answer:           answer,
		KnowledgeContext: answer,
		GenerateResp:     answerResp,
		DirectAnswer:     plan.EvidenceMode == workflowEvidenceExactQuote,
		LatencyMS:        time.Since(start).Milliseconds(),
		PrimaryDocID:     document.DocumentID,
		PrimaryDocTitle:  document.Title,
		DocumentRefs:     []knowledgeContextDocument{{DocumentID: document.DocumentID, Title: document.Title}},
	}, nil
}

func (s *Service) executeKnowledgeSynthesize(
	ctx context.Context,
	req knowledgeWorkflowRequest,
	conversation model.Conversation,
	bindings []*ragpb.ConversationKnowledgeBaseInfo,
	plan workflowPlan,
) (*knowledgeWorkflowResult, error) {
	start := time.Now()
	queries := buildKnowledgeQueriesForWorkflow(req)
	hits, err := s.searchKnowledgeHits(ctx, req, bindings, queries, 12)
	if err != nil {
		return nil, err
	}
	documents, err := s.resolveSynthesisDocuments(ctx, req, bindings, queries, hits)
	if err != nil {
		return nil, err
	}
	if len(documents) < 2 {
		return &knowledgeWorkflowResult{
			Plan:             plan,
			Answer:           "当前知识库无法稳定确定要综合的多份文档，请明确文档范围。",
			KnowledgeContext: "",
			DirectAnswer:     true,
			LatencyMS:        time.Since(start).Milliseconds(),
		}, nil
	}

	notes := make([]documentSynthesisNote, 0, len(documents))
	for _, item := range documents {
		summaryResp, err := s.generateKnowledgeReadAnswer(ctx, req, workflowOutputOutline, item.Title, item.Chunks, false)
		if err != nil {
			return nil, err
		}
		notes = append(notes, documentSynthesisNote{
			DocumentID:    item.DocumentID,
			DocumentTitle: item.Title,
			Chunks:        item.Chunks,
			Note:          strings.TrimSpace(summaryResp.Content),
		})
	}

	answerResp, err := s.generateKnowledgeWorkflowResponse(ctx, req, knowledgeSynthesisSystemPrompt(), knowledgeSynthesisUserPrompt(req.Question, plan.OutputMode, notes), plan.EvidenceMode != workflowEvidenceExactQuote)
	if err != nil {
		return nil, err
	}
	answer := strings.TrimSpace(answerResp.Content)
	if plan.EvidenceMode == workflowEvidenceExactQuote {
		quotes, err := selectQuotesForSynthesis(ctx, req.LLMClient, req.ModelName, req.Question, answer, notes)
		if err != nil {
			return nil, err
		}
		answer = appendQuotesToAnswer(answer, quotes)
	}
	return &knowledgeWorkflowResult{
		Plan:             plan,
		Answer:           answer,
		KnowledgeContext: answer,
		GenerateResp:     answerResp,
		DirectAnswer:     plan.EvidenceMode == workflowEvidenceExactQuote,
		LatencyMS:        time.Since(start).Milliseconds(),
		PrimaryDocID:     documents[0].DocumentID,
		PrimaryDocTitle:  documents[0].Title,
		DocumentRefs:     collectKnowledgeContextDocuments(documents),
	}, nil
}

type readableKnowledgeDocument struct {
	DocumentID uint64
	Title      string
	Chunks     []*ragpb.KnowledgeDocumentChunkInfo
}

type documentSynthesisNote struct {
	DocumentID    uint64
	DocumentTitle string
	Chunks        []*ragpb.KnowledgeDocumentChunkInfo
	Note          string
}

type knowledgeSearchHit struct {
	KnowledgeBaseID int64
	DocumentID      int64
	ChunkID         int64
	Score           float64
	Content         string
}

func (s *Service) resolveReadableKnowledgeDocument(ctx context.Context, req knowledgeWorkflowRequest, bindings []*ragpb.ConversationKnowledgeBaseInfo, queries []string, hits []knowledgeSearchHit) (*readableKnowledgeDocument, []*ragpb.KnowledgeDocumentChunkInfo, error) {
	documents, err := s.resolveKnowledgeDocuments(ctx, req, bindings)
	if err != nil {
		return nil, nil, err
	}
	if document := selectReadableDocumentByHits(documents, hits); document != nil {
		focused := focusDocumentChunks(document.Chunks, hitsForDocument(hits, int64(document.DocumentID)), 1)
		return document, focused, nil
	}
	if len(documents) == 1 {
		focused := focusDocumentChunks(documents[0].Chunks, hitsForDocument(hits, int64(documents[0].DocumentID)), 1)
		return &documents[0], focused, nil
	}
	queryNorms := normalizeDocMatchQueries(queries)
	if len(queryNorms) == 0 {
		return nil, nil, nil
	}
	matched := make([]readableKnowledgeDocument, 0, len(documents))
	for _, item := range documents {
		titleNorm := normalizeDocMatchText(item.Title)
		if titleNorm == "" {
			continue
		}
		for _, queryNorm := range queryNorms {
			if strings.Contains(queryNorm, titleNorm) || strings.Contains(titleNorm, queryNorm) {
				matched = append(matched, item)
				break
			}
		}
	}
	if len(matched) == 1 {
		focused := focusDocumentChunks(matched[0].Chunks, hitsForDocument(hits, int64(matched[0].DocumentID)), 1)
		return &matched[0], focused, nil
	}
	return nil, nil, nil
}

func (s *Service) resolveSynthesisDocuments(ctx context.Context, req knowledgeWorkflowRequest, bindings []*ragpb.ConversationKnowledgeBaseInfo, queries []string, hits []knowledgeSearchHit) ([]readableKnowledgeDocument, error) {
	documents, err := s.resolveKnowledgeDocuments(ctx, req, bindings)
	if err != nil {
		return nil, err
	}
	if selected := selectSynthesisDocumentsByHits(documents, hits, 4); len(selected) >= 2 {
		return selected, nil
	}
	if len(documents) <= 4 {
		return documents, nil
	}
	queryNorms := normalizeDocMatchQueries(queries)
	if len(queryNorms) > 0 {
		matched := make([]readableKnowledgeDocument, 0, 4)
		for _, item := range documents {
			titleNorm := normalizeDocMatchText(item.Title)
			if titleNorm == "" {
				continue
			}
			for _, queryNorm := range queryNorms {
				if strings.Contains(queryNorm, titleNorm) || strings.Contains(titleNorm, queryNorm) {
					matched = append(matched, item)
					break
				}
			}
		}
		if len(matched) >= 2 {
			if len(matched) > 4 {
				matched = matched[:4]
			}
			return matched, nil
		}
	}
	return documents[:4], nil
}

func (s *Service) searchKnowledgeHits(ctx context.Context, req knowledgeWorkflowRequest, bindings []*ragpb.ConversationKnowledgeBaseInfo, queries []string, topK int) ([]knowledgeSearchHit, error) {
	if topK <= 0 {
		topK = 8
	}
	if len(queries) == 0 {
		queries = []string{strings.TrimSpace(req.Question)}
	}
	hits := make([]knowledgeSearchHit, 0, len(bindings)*topK)
	for _, binding := range bindings {
		if binding == nil || binding.KnowledgeBaseId <= 0 {
			continue
		}
		for _, query := range queries {
			query = strings.TrimSpace(query)
			if query == "" {
				continue
			}
			resp, err := s.RAGClient.SearchKnowledgeBase(ctx, &ragpb.SearchKnowledgeBaseRequest{
				OperatorId:      int64(req.MentionRequest.UserID),
				KnowledgeBaseId: binding.KnowledgeBaseId,
				Query:           query,
				TopK:            int32PtrKnowledge(int32(topK)),
			})
			if err != nil {
				return nil, err
			}
			for _, item := range resp.GetChunks() {
				if item == nil || strings.TrimSpace(item.Content) == "" {
					continue
				}
				hits = append(hits, knowledgeSearchHit{
					KnowledgeBaseID: binding.KnowledgeBaseId,
					DocumentID:      item.DocumentId,
					ChunkID:         item.ChunkId,
					Score:           item.Score,
					Content:         item.Content,
				})
			}
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		return hits[i].Score > hits[j].Score
	})
	return hits, nil
}

func buildKnowledgeQueriesForWorkflow(req knowledgeWorkflowRequest) []string {
	queries := buildRAGQueries(req.Question, req.RecentMessages, req.MentionRequest.RequestMessageID)
	if extra := strings.TrimSpace(ExtractQuestion(req.PromptContent)); extra != "" {
		queries = append(queries, extra)
	}
	if req.ActiveContext != nil && !isStrongKnowledgeRetarget(req.Question, *req.ActiveContext) {
		if title := strings.TrimSpace(req.ActiveContext.PrimaryDocTitle); title != "" {
			queries = append(queries, title)
		}
		if last := strings.TrimSpace(req.ActiveContext.LastQuestion); last != "" {
			queries = append(queries, last)
		}
	}
	return deduplicateQuestions(queries)
}

func (s *Service) rememberKnowledgeContext(req knowledgeWorkflowRequest, result *knowledgeWorkflowResult) {
	if s == nil || s.KnowledgeContextStore == nil || result == nil || strings.TrimSpace(result.Answer) == "" {
		return
	}
	item := activeKnowledgeContext{
		UserID:          req.MentionRequest.UserID,
		ConversationID: req.MentionRequest.ConversationID,
		BotID:          req.ResolvedBot.Bot.ID,
		Family:         result.Plan.Family,
		OutputMode:     result.Plan.OutputMode,
		EvidenceMode:   result.Plan.EvidenceMode,
		KnowledgeScope: string(req.KnowledgeScope),
		LastQuestion:   strings.TrimSpace(req.Question),
		LastAnswer:     strings.TrimSpace(result.Answer),
	}
	switch result.Plan.Family {
	case workflowFamilyRead:
		if title, ok := inferPrimaryDocumentTitleFromAnswer(result.Answer, req.ActiveContext); ok {
			item.PrimaryDocTitle = title
		}
		if req.ActiveContext != nil && item.PrimaryDocTitle == "" {
			item.PrimaryDocTitle = req.ActiveContext.PrimaryDocTitle
			item.PrimaryDocID = req.ActiveContext.PrimaryDocID
			item.DocumentRefs = req.ActiveContext.DocumentRefs
		}
	case workflowFamilySynthesize:
		if req.ActiveContext != nil {
			item.DocumentRefs = req.ActiveContext.DocumentRefs
			item.PrimaryDocID = req.ActiveContext.PrimaryDocID
			item.PrimaryDocTitle = req.ActiveContext.PrimaryDocTitle
		}
	default:
		if req.ActiveContext != nil {
			item.PrimaryDocID = req.ActiveContext.PrimaryDocID
			item.PrimaryDocTitle = req.ActiveContext.PrimaryDocTitle
			item.DocumentRefs = req.ActiveContext.DocumentRefs
		}
	}
	if item.PrimaryDocTitle == "" && req.ActiveContext != nil {
		item.PrimaryDocTitle = req.ActiveContext.PrimaryDocTitle
	}
	s.KnowledgeContextStore.Set(item)
}

func (s *Service) registerKnowledgeSnapshot(messageID uint64, req HandleMentionRequest, botModel model.Bot, workflow *knowledgeWorkflowResult) {
	if s == nil || s.KnowledgeSnapshotStore == nil || messageID == 0 || workflow == nil || strings.TrimSpace(workflow.Answer) == "" {
		return
	}
	s.KnowledgeSnapshotStore.Set(sharedKnowledgeSnapshot{
		MessageID:       messageID,
		ConversationID:  req.ConversationID,
		BotID:           botModel.ID,
		Family:          workflow.Plan.Family,
		OutputMode:      workflow.Plan.OutputMode,
		EvidenceMode:    workflow.Plan.EvidenceMode,
		KnowledgeScope:  "",
		Question:        strings.TrimSpace(req.Content),
		Answer:          strings.TrimSpace(workflow.Answer),
		PrimaryDocID:    workflow.PrimaryDocID,
		PrimaryDocTitle: workflow.PrimaryDocTitle,
		DocumentRefs:    workflow.DocumentRefs,
	})
}

func inferPrimaryDocumentTitleFromAnswer(answer string, ctx *activeKnowledgeContext) (string, bool) {
	if ctx == nil {
		return "", false
	}
	if strings.TrimSpace(ctx.PrimaryDocTitle) != "" {
		return ctx.PrimaryDocTitle, true
	}
	if len(ctx.DocumentRefs) == 1 {
		return strings.TrimSpace(ctx.DocumentRefs[0].Title), ctx.DocumentRefs[0].Title != ""
	}
	_ = answer
	return "", false
}

func collectKnowledgeContextDocuments(documents []readableKnowledgeDocument) []knowledgeContextDocument {
	if len(documents) == 0 {
		return nil
	}
	result := make([]knowledgeContextDocument, 0, len(documents))
	for _, item := range documents {
		result = append(result, knowledgeContextDocument{
			DocumentID: item.DocumentID,
			Title:      strings.TrimSpace(item.Title),
		})
	}
	return normalizeKnowledgeContextDocuments(result)
}

func collectKnowledgeContextDocumentsFromChunks(chunks map[int64][]*ragpb.KnowledgeDocumentChunkInfo) []knowledgeContextDocument {
	if len(chunks) == 0 {
		return nil
	}
	result := make([]knowledgeContextDocument, 0, len(chunks))
	for documentID := range chunks {
		result = append(result, knowledgeContextDocument{
			DocumentID: uint64(documentID),
			Title:      "",
		})
	}
	return normalizeKnowledgeContextDocuments(result)
}

func normalizeDocMatchQueries(queries []string) []string {
	if len(queries) == 0 {
		return nil
	}
	result := make([]string, 0, len(queries))
	seen := make(map[string]struct{}, len(queries))
	for _, item := range queries {
		value := normalizeDocMatchText(item)
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

func selectReadableDocumentByHits(documents []readableKnowledgeDocument, hits []knowledgeSearchHit) *readableKnowledgeDocument {
	if len(documents) == 0 || len(hits) == 0 {
		return nil
	}
	scoreByDoc := make(map[int64]float64, len(documents))
	for _, hit := range hits {
		scoreByDoc[hit.DocumentID] += hit.Score
	}
	var (
		bestDoc   *readableKnowledgeDocument
		bestScore float64
	)
	for idx := range documents {
		score := scoreByDoc[int64(documents[idx].DocumentID)]
		if score > bestScore {
			bestScore = score
			bestDoc = &documents[idx]
		}
	}
	if bestDoc == nil || bestScore <= 0 {
		return nil
	}
	return bestDoc
}

func selectSynthesisDocumentsByHits(documents []readableKnowledgeDocument, hits []knowledgeSearchHit, limit int) []readableKnowledgeDocument {
	if len(documents) == 0 || len(hits) == 0 {
		return nil
	}
	scoreByDoc := make(map[int64]float64, len(documents))
	for _, hit := range hits {
		scoreByDoc[hit.DocumentID] += hit.Score
	}
	type docScore struct {
		doc   readableKnowledgeDocument
		score float64
	}
	scored := make([]docScore, 0, len(documents))
	for _, item := range documents {
		score := scoreByDoc[int64(item.DocumentID)]
		if score <= 0 {
			continue
		}
		item.Chunks = focusDocumentChunks(item.Chunks, hitsForDocument(hits, int64(item.DocumentID)), 1)
		scored = append(scored, docScore{doc: item, score: score})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})
	if limit > 0 && len(scored) > limit {
		scored = scored[:limit]
	}
	result := make([]readableKnowledgeDocument, 0, len(scored))
	for _, item := range scored {
		result = append(result, item.doc)
	}
	return result
}

func hitsForDocument(hits []knowledgeSearchHit, documentID int64) []knowledgeSearchHit {
	if len(hits) == 0 || documentID <= 0 {
		return nil
	}
	result := make([]knowledgeSearchHit, 0, len(hits))
	for _, item := range hits {
		if item.DocumentID == documentID {
			result = append(result, item)
		}
	}
	return result
}

func focusDocumentChunks(chunks []*ragpb.KnowledgeDocumentChunkInfo, hits []knowledgeSearchHit, window int) []*ragpb.KnowledgeDocumentChunkInfo {
	if len(chunks) == 0 || len(hits) == 0 {
		return chunks
	}
	if window < 0 {
		window = 0
	}
	byChunkID := make(map[int64]int, len(chunks))
	for idx, item := range chunks {
		if item == nil {
			continue
		}
		byChunkID[item.ChunkId] = idx
	}
	keep := make(map[int]struct{}, len(hits)*(window*2+1))
	for _, hit := range hits {
		index, ok := byChunkID[hit.ChunkID]
		if !ok {
			continue
		}
		start := index - window
		if start < 0 {
			start = 0
		}
		end := index + window
		if end >= len(chunks) {
			end = len(chunks) - 1
		}
		for i := start; i <= end; i++ {
			keep[i] = struct{}{}
		}
	}
	if len(keep) == 0 {
		return chunks
	}
	result := make([]*ragpb.KnowledgeDocumentChunkInfo, 0, len(keep))
	for idx, item := range chunks {
		if _, ok := keep[idx]; ok {
			result = append(result, item)
		}
	}
	return result
}

func (s *Service) resolveKnowledgeDocuments(ctx context.Context, req knowledgeWorkflowRequest, bindings []*ragpb.ConversationKnowledgeBaseInfo) ([]readableKnowledgeDocument, error) {
	result := make([]readableKnowledgeDocument, 0, 8)
	for _, binding := range bindings {
		if binding == nil || binding.KnowledgeBaseId <= 0 {
			continue
		}
		resp, err := s.RAGClient.ListKnowledgeDocuments(ctx, &ragpb.ListKnowledgeDocumentsRequest{
			OperatorId:      int64(req.MentionRequest.UserID),
			KnowledgeBaseId: binding.KnowledgeBaseId,
		})
		if err != nil {
			return nil, err
		}
		for _, doc := range resp.GetDocuments() {
			if doc == nil || !strings.EqualFold(strings.TrimSpace(doc.Status), "READY") {
				continue
			}
			chunkResp, err := s.RAGClient.ListKnowledgeDocumentChunks(ctx, &ragpb.ListKnowledgeDocumentChunksRequest{
				OperatorId:      int64(req.MentionRequest.UserID),
				KnowledgeBaseId: binding.KnowledgeBaseId,
				DocumentId:      doc.DocumentId,
			})
			if err != nil {
				return nil, err
			}
			chunks := filterReadableKnowledgeDocumentChunks(chunkResp.GetChunks())
			if len(chunks) == 0 {
				continue
			}
			result = append(result, readableKnowledgeDocument{
				DocumentID: uint64(doc.DocumentId),
				Title:      strings.TrimSpace(doc.Title),
				Chunks:     chunks,
			})
		}
	}
	return result, nil
}

func (s *Service) generateKnowledgeReadAnswer(ctx context.Context, req knowledgeWorkflowRequest, outputMode workflowOutputMode, title string, chunks []*ragpb.KnowledgeDocumentChunkInfo, enableStream bool) (*llm.GenerateResponse, error) {
	batches := buildKnowledgeReadBatches(chunks, 8000)
	if len(batches) == 0 {
		return &llm.GenerateResponse{Content: "当前文档暂无可读内容。"}, nil
	}
	if len(batches) == 1 {
		return s.generateKnowledgeWorkflowResponse(ctx, req, knowledgeReadSystemPrompt(), knowledgeReadUserPrompt(req.Question, outputMode, title, batches[0], ""), enableStream)
	}

	batchSummaries := make([]string, 0, len(batches))
	totalPrompt := 0
	totalCompletion := 0
	totalAll := 0
	for index, batch := range batches {
		resp, err := s.generateKnowledgeWorkflowResponse(ctx, req, knowledgeReadSystemPrompt(), knowledgeReadUserPrompt(req.Question, workflowOutputOutline, title, batch, fmt.Sprintf("这是第 %d/%d 批内容，请先提炼该批与问题最相关的阅读要点。", index+1, len(batches))), false)
		if err != nil {
			return nil, err
		}
		batchSummaries = append(batchSummaries, strings.TrimSpace(resp.Content))
		totalPrompt += resp.PromptTokens
		totalCompletion += resp.CompletionTokens
		totalAll += resp.TotalTokens
	}
	finalResp, err := s.generateKnowledgeWorkflowResponse(ctx, req, knowledgeReadAggregateSystemPrompt(), knowledgeReadAggregateUserPrompt(req.Question, outputMode, title, batchSummaries), enableStream)
	if err != nil {
		return nil, err
	}
	finalResp.PromptTokens += totalPrompt
	finalResp.CompletionTokens += totalCompletion
	finalResp.TotalTokens += totalAll
	return finalResp, nil
}

func (s *Service) generateKnowledgeWorkflowResponse(ctx context.Context, req knowledgeWorkflowRequest, systemPrompt string, userPrompt string, enableStream bool) (*llm.GenerateResponse, error) {
	if !enableStream || req.StreamMeta == nil {
		return req.LLMClient.Generate(ctx, llm.GenerateRequest{
			Model: req.ModelName,
			Messages: []llm.ChatMessage{
				{Role: "system", Content: systemPrompt},
				{Role: "user", Content: userPrompt},
			},
		})
	}
	streamer, ok := req.LLMClient.(llmStreamingClient)
	if !ok {
		return req.LLMClient.Generate(ctx, llm.GenerateRequest{
			Model: req.ModelName,
			Messages: []llm.ChatMessage{
				{Role: "system", Content: systemPrompt},
				{Role: "user", Content: userPrompt},
			},
		})
	}
	streamState := *req.StreamMeta
	streamState.info.Content = ""
	streamState.info.Done = false
	s.publishBotReplyStream(ctx, streamState)
	var (
		contentBuilder strings.Builder
		firstChunkAt   time.Time
		chunkCount     int
	)
	start := time.Now()
	resp, err := streamer.GenerateStream(ctx, llm.GenerateRequest{
		Model: req.ModelName,
		Messages: []llm.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}, func(chunk llm.StreamChunk) error {
		chunkCount++
		if firstChunkAt.IsZero() && (chunk.Content != "" || chunk.ReasoningContent != "") {
			firstChunkAt = time.Now()
		}
		if chunk.Content != "" {
			contentBuilder.WriteString(chunk.Content)
			streamState.info.Content = contentBuilder.String()
			streamState.info.Done = false
			s.publishBotReplyStream(ctx, streamState)
		}
		return nil
	})
	log.Printf(
		"knowledge workflow stream timing: conversation=%d bot=%d model=%s first_chunk_ms=%d llm_total_ms=%d chunks=%d",
		req.MentionRequest.ConversationID,
		req.ResolvedBot.Bot.ID,
		req.ModelName,
		durationMillis(start, firstChunkAt),
		time.Since(start).Milliseconds(),
		chunkCount,
	)
	if err != nil {
		return nil, err
	}
	if resp != nil {
		if resp.Content == "" {
			resp.Content = contentBuilder.String()
		}
		if strings.TrimSpace(resp.Content) != "" {
			streamState.info.Content = resp.Content
			streamState.info.Done = true
			s.publishBotReplyStream(ctx, streamState)
		}
	}
	return resp, nil
}

func knowledgeLookupSystemPrompt() string {
	return strings.Join([]string{
		"你是 AIM 的知识库问答助手。",
		"你只能依据提供的知识库片段作答，不要编造知识库中不存在的信息。",
		"如果证据不足，必须明确说“根据当前知识库资料无法确定”。",
		"如需引用证据，请使用 [1] [2] 这种编号。",
	}, "\n")
}

func knowledgeLookupUserPrompt(question string, chunks []RAGChunk) string {
	lines := []string{
		"用户问题：" + strings.TrimSpace(question),
		"",
		"知识库片段：",
	}
	for index, item := range chunks {
		lines = append(lines, fmt.Sprintf("[%d] %s", index+1, strings.TrimSpace(item.Content)))
	}
	return strings.Join(lines, "\n")
}

func knowledgeReadSystemPrompt() string {
	return strings.Join([]string{
		"你是 AIM 的长文阅读助手。",
		"你需要根据按顺序提供的文档内容进行理解，不要编造文档中不存在的信息。",
		"当前阶段不支持你自己生成原句引用，请只专注于阅读和总结。",
	}, "\n")
}

func knowledgeReadUserPrompt(question string, outputMode workflowOutputMode, title string, batch string, instruction string) string {
	task := "请阅读以下文档内容并回答用户问题。"
	switch outputMode {
	case workflowOutputSummary:
		task = "请阅读以下文档内容并输出结构化总结。"
	case workflowOutputOutline:
		task = "请阅读以下文档内容并输出提纲。"
	case workflowOutputExtract:
		task = "请阅读以下文档内容并提取关键内容。"
	case workflowOutputTable:
		task = "请阅读以下文档内容并尽量用 Markdown 表格组织答案。"
	case workflowOutputTimeline:
		task = "请阅读以下文档内容并按时间线组织答案。"
	case workflowOutputCompare:
		task = "请阅读以下文档内容并按对比视角组织答案。"
	case workflowOutputQuiz:
		task = "请阅读以下文档内容并生成测验题和答案。"
	case workflowOutputRewrite:
		task = "请阅读以下文档内容并重写成更清晰的答案。"
	}
	lines := []string{
		"文档：" + strings.TrimSpace(title),
		"任务要求：" + task,
		"用户问题：" + strings.TrimSpace(question),
	}
	if strings.TrimSpace(instruction) != "" {
		lines = append(lines, "补充说明："+strings.TrimSpace(instruction))
	}
	lines = append(lines, "", "文档内容：", batch)
	return strings.Join(lines, "\n")
}

func knowledgeReadAggregateSystemPrompt() string {
	return strings.Join([]string{
		"你是 AIM 的长文综合助手。",
		"你会收到同一文档不同批次的阅读要点，请把它们整合成最终答案。",
		"只能依据提供的阅读要点作答，不要编造额外信息。",
	}, "\n")
}

func knowledgeReadAggregateUserPrompt(question string, outputMode workflowOutputMode, title string, batchSummaries []string) string {
	task := "请综合各批次要点，回答用户问题。"
	switch outputMode {
	case workflowOutputSummary:
		task = "请综合各批次要点，输出最终总结。"
	case workflowOutputOutline:
		task = "请综合各批次要点，输出最终提纲。"
	case workflowOutputExtract:
		task = "请综合各批次要点，输出提取结果。"
	case workflowOutputTable:
		task = "请综合各批次要点，并尽量使用 Markdown 表格输出。"
	case workflowOutputTimeline:
		task = "请综合各批次要点，并按时间线输出。"
	case workflowOutputCompare:
		task = "请综合各批次要点，并按对比视角输出。"
	case workflowOutputQuiz:
		task = "请综合各批次要点，并输出测验题和答案。"
	case workflowOutputRewrite:
		task = "请综合各批次要点，并重写成更清晰的答案。"
	}
	lines := []string{
		"文档：" + strings.TrimSpace(title),
		"任务要求：" + task,
		"用户问题：" + strings.TrimSpace(question),
		"",
		"分批阅读要点：",
	}
	for index, item := range batchSummaries {
		lines = append(lines, fmt.Sprintf("第%d批要点：", index+1))
		lines = append(lines, strings.TrimSpace(item))
	}
	return strings.Join(lines, "\n")
}

func knowledgeSynthesisSystemPrompt() string {
	return strings.Join([]string{
		"你是 AIM 的多文档综合助手。",
		"你会收到多份文档各自的阅读要点，请在不编造事实的前提下进行比较、综合和归纳。",
		"如果资料不足以支撑某个结论，必须明确说明无法确定。",
	}, "\n")
}

func knowledgeSynthesisUserPrompt(question string, outputMode workflowOutputMode, notes []documentSynthesisNote) string {
	task := "请综合多份文档要点，回答用户问题。"
	switch outputMode {
	case workflowOutputSummary:
		task = "请综合多份文档要点，输出总结。"
	case workflowOutputCompare:
		task = "请重点比较多份文档的异同，并输出结构化对比结论。"
	case workflowOutputOutline:
		task = "请综合多份文档要点，按提纲形式输出。"
	case workflowOutputExtract:
		task = "请综合多份文档要点，提取关键内容。"
	case workflowOutputTable:
		task = "请综合多份文档要点，并尽量使用 Markdown 表格输出。"
	case workflowOutputTimeline:
		task = "请综合多份文档要点，并按时间线输出。"
	case workflowOutputQuiz:
		task = "请综合多份文档要点，并生成测验题和答案。"
	case workflowOutputRewrite:
		task = "请综合多份文档要点，并重写成更清晰的答案。"
	}
	lines := []string{
		"任务要求：" + task,
		"用户问题：" + strings.TrimSpace(question),
		"",
		"各文档阅读要点：",
	}
	for index, item := range notes {
		lines = append(lines, fmt.Sprintf("文档%d：《%s》", index+1, strings.TrimSpace(item.DocumentTitle)))
		lines = append(lines, strings.TrimSpace(item.Note))
	}
	return strings.Join(lines, "\n")
}

func filterReadableKnowledgeDocumentChunks(items []*ragpb.KnowledgeDocumentChunkInfo) []*ragpb.KnowledgeDocumentChunkInfo {
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

func buildKnowledgeReadBatches(chunks []*ragpb.KnowledgeDocumentChunkInfo, maxChars int) []string {
	if len(chunks) == 0 {
		return nil
	}
	if maxChars <= 0 {
		maxChars = 8000
	}
	var (
		batches []string
		builder strings.Builder
		count   int
	)
	flush := func() {
		value := strings.TrimSpace(builder.String())
		if value != "" {
			batches = append(batches, value)
		}
		builder.Reset()
		count = 0
	}
	for _, item := range chunks {
		part := formatKnowledgeChunk(item)
		size := len([]rune(part))
		if count > 0 && count+size > maxChars {
			flush()
		}
		builder.WriteString(part)
		builder.WriteString("\n\n")
		count += size + 2
	}
	flush()
	return batches
}

func formatKnowledgeChunk(item *ragpb.KnowledgeDocumentChunkInfo) string {
	lines := []string{fmt.Sprintf("【Chunk %d】", item.ChunkIndex)}
	if title := strings.TrimSpace(item.SectionTitle); title != "" {
		lines = append(lines, "章节："+title)
	}
	lines = append(lines, strings.TrimSpace(item.Content))
	return strings.Join(lines, "\n")
}

func selectQuotesForLookup(ctx context.Context, client llm.Client, modelName string, question string, answer string, chunks map[int64][]*ragpb.KnowledgeDocumentChunkInfo) ([]workflowQuoteCandidate, error) {
	candidates := make([]workflowQuoteCandidate, 0, 24)
	for documentID, items := range chunks {
		title := fmt.Sprintf("文档%d", documentID)
		for _, item := range items {
			candidates = append(candidates, splitKnowledgeChunkIntoQuoteCandidates(item, title, 6)...)
		}
	}
	return selectQuoteCandidates(ctx, client, modelName, question, answer, candidates)
}

func selectQuotesFromDocument(ctx context.Context, client llm.Client, modelName string, question string, answer string, title string, chunks []*ragpb.KnowledgeDocumentChunkInfo, limit int) ([]workflowQuoteCandidate, error) {
	candidates := make([]workflowQuoteCandidate, 0, len(chunks)*2)
	queryTokens := normalizeLowerTokens(question)
	for _, item := range chunks {
		next := splitKnowledgeChunkIntoQuoteCandidates(item, title, 0)
		for i := range next {
			next[i].Score = scoreKnowledgeSentence(next[i].Text, queryTokens)
		}
		candidates = append(candidates, next...)
	}
	candidates = normalizeKnowledgeQuoteCandidates(candidates, limit)
	return selectQuoteCandidates(ctx, client, modelName, question, answer, candidates)
}

func selectQuotesForSynthesis(ctx context.Context, client llm.Client, modelName string, question string, answer string, notes []documentSynthesisNote) ([]workflowQuoteCandidate, error) {
	candidates := make([]workflowQuoteCandidate, 0, 32)
	for _, note := range notes {
		for _, item := range note.Chunks {
			candidates = append(candidates, splitKnowledgeChunkIntoQuoteCandidates(item, note.DocumentTitle, 4)...)
		}
	}
	candidates = normalizeKnowledgeQuoteCandidates(candidates, 24)
	return selectQuoteCandidates(ctx, client, modelName, question, answer, candidates)
}

func selectQuoteCandidates(ctx context.Context, client llm.Client, modelName string, question string, answer string, candidates []workflowQuoteCandidate) ([]workflowQuoteCandidate, error) {
	if len(candidates) == 0 {
		return nil, nil
	}
	systemPrompt := strings.Join([]string{
		"你是 AIM 的原句证据选择器。",
		"你只能从候选句子中选择最能支撑答案的原句。",
		"不要改写句子，不要输出候选集中不存在的 quoteId。",
		"只返回 JSON：{\"evidence_ids\":[\"Q1\",\"Q2\"]}",
	}, "\n")
	lines := []string{
		"问题：" + strings.TrimSpace(question),
		"答案：" + strings.TrimSpace(answer),
		"",
		"候选句子：",
	}
	for _, item := range candidates {
		lines = append(lines, fmt.Sprintf("%s | %s | chunk=%d | sentence=%d", item.QuoteID, item.DocumentTitle, item.ChunkID, item.SentenceIndex))
		lines = append(lines, item.Text)
	}
	resp, err := client.Generate(ctx, llm.GenerateRequest{
		Model: modelName,
		Messages: []llm.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: strings.Join(lines, "\n")},
		},
	})
	if err != nil {
		return nil, err
	}
	content := strings.TrimSpace(resp.Content)
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return nil, nil
	}
	content = content[start : end+1]
	var parsed struct {
		EvidenceIDs []string `json:"evidence_ids"`
	}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, err
	}
	selected := make([]workflowQuoteCandidate, 0, len(parsed.EvidenceIDs))
	byID := make(map[string]workflowQuoteCandidate, len(candidates))
	for _, item := range candidates {
		byID[item.QuoteID] = item
	}
	for _, id := range normalizeTokensLocal(parsed.EvidenceIDs) {
		if candidate, ok := byID[id]; ok {
			selected = append(selected, candidate)
		}
	}
	return selected, nil
}

func splitKnowledgeChunkIntoQuoteCandidates(item *ragpb.KnowledgeDocumentChunkInfo, title string, limit int) []workflowQuoteCandidate {
	if item == nil {
		return nil
	}
	result := make([]workflowQuoteCandidate, 0, 8)
	if len(item.Sentences) > 0 {
		for _, sentence := range item.Sentences {
			if sentence == nil || strings.TrimSpace(sentence.Text) == "" {
				continue
			}
			result = append(result, workflowQuoteCandidate{
				QuoteID:       fmt.Sprintf("Q%d_%d", item.ChunkId, sentence.SentenceIndex),
				DocumentTitle: title,
				DocumentID:    item.DocumentId,
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
	sentences := splitKnowledgeSentences(item.Content)
	for idx, sentence := range sentences {
		result = append(result, workflowQuoteCandidate{
			QuoteID:       fmt.Sprintf("Q%d_%d", item.ChunkId, idx+1),
			DocumentTitle: title,
			DocumentID:    item.DocumentId,
			ChunkID:       item.ChunkId,
			SentenceIndex: idx + 1,
			PageStart:     int(item.PageStart),
			PageEnd:       int(item.PageEnd),
			CharStart:     int(item.CharStart),
			CharEnd:       int(item.CharEnd),
			Text:          sentence,
		})
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

func splitKnowledgeSentences(content string) []string {
	text := strings.TrimSpace(content)
	if text == "" {
		return nil
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	var (
		result  []string
		builder strings.Builder
	)
	flush := func() {
		value := strings.TrimSpace(builder.String())
		if value != "" {
			result = append(result, value)
		}
		builder.Reset()
	}
	for _, r := range text {
		builder.WriteRune(r)
		switch r {
		case '\n', '。', '！', '？', '；', '.', '!', '?', ';':
			flush()
		}
	}
	flush()
	return result
}

func normalizeKnowledgeQuoteCandidates(candidates []workflowQuoteCandidate, limit int) []workflowQuoteCandidate {
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
	result := make([]workflowQuoteCandidate, 0, len(candidates))
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

func scoreKnowledgeSentence(sentence string, tokens []string) int {
	value := strings.ToLower(strings.TrimSpace(sentence))
	score := 0
	for _, token := range tokens {
		if token == "" {
			continue
		}
		if strings.Contains(value, token) {
			score += 2
		}
	}
	return score
}

func appendQuotesToAnswer(answer string, quotes []workflowQuoteCandidate) string {
	if len(quotes) == 0 {
		return strings.TrimSpace(answer)
	}
	lines := []string{strings.TrimSpace(answer), "", "原文关键句："}
	for index, item := range quotes {
		page := ""
		if item.PageStart > 0 && item.PageEnd > 0 {
			if item.PageStart == item.PageEnd {
				page = fmt.Sprintf("（第%d页）", item.PageStart)
			} else {
				page = fmt.Sprintf("（第%d-%d页）", item.PageStart, item.PageEnd)
			}
		}
		lines = append(lines, fmt.Sprintf("%d. %s%s", index+1, item.Text, page))
	}
	return strings.Join(lines, "\n")
}

func normalizeLowerTokens(value string) []string {
	replacer := strings.NewReplacer("，", " ", "。", " ", "：", " ", "；", " ", "？", " ", "！", " ", ",", " ", ".", " ", ":", " ", ";", " ", "?", " ", "!", " ", "\n", " ")
	value = replacer.Replace(strings.ToLower(strings.TrimSpace(value)))
	return normalizeTokensLocal(strings.Fields(value))
}

func normalizeTokensLocal(items []string) []string {
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

func normalizeDocMatchText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	replacer := strings.NewReplacer(".pdf", "", ".md", "", ".txt", "", "《", "", "》", "", " ", "", "\t", "", "\n", "", "\r", "")
	return replacer.Replace(value)
}

func normalizeWorkflowFamily(value workflowFamily) workflowFamily {
	switch workflowFamily(strings.ToUpper(strings.TrimSpace(string(value)))) {
	case workflowFamilyLookup, workflowFamilyRead, workflowFamilySynthesize, workflowFamilyUnsupported:
		return workflowFamily(strings.ToUpper(strings.TrimSpace(string(value))))
	default:
		return ""
	}
}

func normalizeWorkflowOutputMode(value workflowOutputMode) workflowOutputMode {
	switch workflowOutputMode(strings.TrimSpace(strings.ToLower(string(value)))) {
	case workflowOutputAnswer, workflowOutputSummary, workflowOutputCompare, workflowOutputExtract, workflowOutputOutline, workflowOutputTable, workflowOutputTimeline, workflowOutputQuiz, workflowOutputRewrite:
		return workflowOutputMode(strings.TrimSpace(strings.ToLower(string(value))))
	default:
		return ""
	}
}

func normalizeWorkflowEvidenceMode(value workflowEvidenceMode) workflowEvidenceMode {
	switch workflowEvidenceMode(strings.TrimSpace(strings.ToLower(string(value)))) {
	case workflowEvidenceNone, workflowEvidenceCitation, workflowEvidenceExactQuote:
		return workflowEvidenceMode(strings.TrimSpace(strings.ToLower(string(value))))
	default:
		return ""
	}
}

func int32PtrKnowledge(value int32) *int32 {
	return &value
}
