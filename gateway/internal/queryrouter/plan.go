package queryrouter

import (
	"fmt"
	"strings"
)

type Family string

const (
	FamilyMeta        Family = "META"
	FamilyControl     Family = "CONTROL"
	FamilyLookup      Family = "LOOKUP"
	FamilyRead        Family = "READ"
	FamilySynthesize  Family = "SYNTHESIZE"
	FamilyUnsupported Family = "UNSUPPORTED"
)

type SourceSpace string

const (
	SourceSpaceConversation      SourceSpace = "conversation"
	SourceSpaceKnowledgeBase     SourceSpace = "knowledge_base"
	SourceSpaceSelectedDocuments SourceSpace = "selected_documents"
	SourceSpaceAllDocuments      SourceSpace = "all_documents"
	SourceSpaceMetadata          SourceSpace = "metadata"
	SourceSpaceMixed             SourceSpace = "mixed"
)

type Scope string

const (
	ScopeChunk         Scope = "chunk"
	ScopeSection       Scope = "section"
	ScopeDocument      Scope = "document"
	ScopeMultiDocument Scope = "multi_document"
	ScopeNotebook      Scope = "notebook"
)

type ReadDepth string

const (
	ReadDepthRetrieve    ReadDepth = "retrieve"
	ReadDepthFocusedRead ReadDepth = "focused_read"
	ReadDepthFullRead    ReadDepth = "full_read"
)

type OutputMode string

const (
	OutputModeAnswer   OutputMode = "answer"
	OutputModeSummary  OutputMode = "summary"
	OutputModeCompare  OutputMode = "compare"
	OutputModeExtract  OutputMode = "extract"
	OutputModeOutline  OutputMode = "outline"
	OutputModeTable    OutputMode = "table"
	OutputModeTimeline OutputMode = "timeline"
	OutputModeQuiz     OutputMode = "quiz"
	OutputModeRewrite  OutputMode = "rewrite"
)

type EvidenceMode string

const (
	EvidenceModeNone       EvidenceMode = "none"
	EvidenceModeCitation   EvidenceMode = "citation"
	EvidenceModeExactQuote EvidenceMode = "exact_quote"
)

type Target struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
}

type AvailableSpaces struct {
	Conversation      bool `json:"conversation"`
	KnowledgeBase     bool `json:"knowledge_base"`
	SelectedDocuments bool `json:"selected_documents"`
	AllDocuments      bool `json:"all_documents"`
	Metadata          bool `json:"metadata"`
	Mixed             bool `json:"mixed"`
}

type Capabilities struct {
	CanLookup                  bool `json:"can_lookup"`
	CanFullReadDocument        bool `json:"can_full_read_document"`
	CanSynthesizeMultiDocument bool `json:"can_synthesize_multi_document"`
	CanExtractExactQuote       bool `json:"can_extract_exact_quote"`
	CanControlBindings         bool `json:"can_control_bindings"`
	CanUseExternalWeb          bool `json:"can_use_external_web"`
}

type ContextHints struct {
	ConversationID    string   `json:"conversation_id"`
	CurrentDocumentIDs []string `json:"current_document_ids"`
	CurrentKBIDs      []string `json:"current_kb_ids"`
}

type PlanningInput struct {
	UserQuery       string          `json:"user_query"`
	SelectedTargets []Target        `json:"selected_targets"`
	AvailableSpaces AvailableSpaces `json:"available_spaces"`
	Capabilities    Capabilities    `json:"capabilities"`
	ContextHints    ContextHints    `json:"context_hints"`
}

type Constraints struct {
	MustGroundInSources bool `json:"must_ground_in_sources"`
	AllowExternalWeb    bool `json:"allow_external_web"`
	StrictQuoteRequired bool `json:"strict_quote_required"`
}

type Plan struct {
	PlanVersion  string       `json:"plan_version"`
	Family       Family       `json:"family"`
	SourceSpace  SourceSpace  `json:"source_space"`
	Scope        Scope        `json:"scope"`
	ReadDepth    ReadDepth    `json:"read_depth"`
	OutputMode   OutputMode   `json:"output_mode"`
	EvidenceMode EvidenceMode `json:"evidence_mode"`
	Targets      []string     `json:"targets"`
	Constraints  Constraints  `json:"constraints"`
	Confidence   float64      `json:"confidence"`
	FallbackFamily Family     `json:"fallback_family"`
	Reason       string       `json:"reason"`
}

func (in PlanningInput) Normalized() PlanningInput {
	in.UserQuery = strings.TrimSpace(in.UserQuery)
	in.SelectedTargets = normalizeTargets(in.SelectedTargets)
	in.AvailableSpaces = in.AvailableSpaces.normalized()
	in.Capabilities = in.Capabilities.normalized()
	in.ContextHints.ConversationID = strings.TrimSpace(in.ContextHints.ConversationID)
	in.ContextHints.CurrentDocumentIDs = normalizeStringList(in.ContextHints.CurrentDocumentIDs)
	in.ContextHints.CurrentKBIDs = normalizeStringList(in.ContextHints.CurrentKBIDs)
	return in
}

func (p Plan) Normalized(input PlanningInput) Plan {
	input = input.Normalized()

	p.PlanVersion = "v1"
	p.Family = normalizeFamily(p.Family)
	p.SourceSpace = normalizeSourceSpace(p.SourceSpace)
	p.Scope = normalizeScope(p.Scope)
	p.ReadDepth = normalizeReadDepth(p.ReadDepth)
	p.OutputMode = normalizeOutputMode(p.OutputMode)
	p.EvidenceMode = normalizeEvidenceMode(p.EvidenceMode)
	p.Targets = normalizeStringList(p.Targets)
	p.Confidence = clampConfidence(p.Confidence)
	p.Reason = strings.TrimSpace(p.Reason)

	if p.Family == "" {
		p.Family = FamilyRead
	}
	if p.SourceSpace == "" {
		p.SourceSpace = inferSourceSpace(input)
	}
	if p.Scope == "" {
		p.Scope = defaultScopeForFamily(p.Family)
	}
	if p.ReadDepth == "" {
		p.ReadDepth = defaultReadDepthForFamily(p.Family, p.Scope)
	}
	if p.OutputMode == "" {
		p.OutputMode = OutputModeAnswer
	}
	if p.EvidenceMode == "" {
		p.EvidenceMode = EvidenceModeNone
	}
	if len(p.Targets) == 0 {
		p.Targets = defaultTargets(input, p.SourceSpace)
	}

	p.Constraints.MustGroundInSources = true
	p.Constraints.AllowExternalWeb = input.Capabilities.CanUseExternalWeb
	p.Constraints.StrictQuoteRequired = p.EvidenceMode == EvidenceModeExactQuote

	if p.FallbackFamily == "" {
		p.FallbackFamily = defaultFallbackFamily(p.Family)
	} else {
		p.FallbackFamily = normalizeFamily(p.FallbackFamily)
		if p.FallbackFamily == "" {
			p.FallbackFamily = defaultFallbackFamily(p.Family)
		}
	}

	if p.Confidence == 0 {
		p.Confidence = defaultConfidenceForFamily(p.Family)
	}

	if reason := unsupportedReason(input, p); reason != "" {
		p.Family = FamilyUnsupported
		p.SourceSpace = SourceSpaceMetadata
		p.Scope = ScopeNotebook
		p.ReadDepth = ReadDepthRetrieve
		p.FallbackFamily = FamilyUnsupported
		p.Confidence = 1
		p.Reason = reason
		return p
	}

	if p.Reason == "" {
		p.Reason = defaultReason(p)
	}
	return p
}

func normalizeTargets(items []Target) []Target {
	if len(items) == 0 {
		return nil
	}
	result := make([]Target, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		item.ID = strings.TrimSpace(item.ID)
		item.Type = strings.TrimSpace(strings.ToLower(item.Type))
		item.Title = strings.TrimSpace(item.Title)
		if item.ID == "" {
			continue
		}
		key := item.Type + "#" + item.ID
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result
}

func normalizeStringList(items []string) []string {
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

func (spaces AvailableSpaces) normalized() AvailableSpaces {
	if spaces.hasAny() {
		return spaces
	}
	return AvailableSpaces{
		Conversation:      true,
		KnowledgeBase:     true,
		SelectedDocuments: true,
		AllDocuments:      true,
		Metadata:          true,
		Mixed:             true,
	}
}

func (spaces AvailableSpaces) hasAny() bool {
	return spaces.Conversation ||
		spaces.KnowledgeBase ||
		spaces.SelectedDocuments ||
		spaces.AllDocuments ||
		spaces.Metadata ||
		spaces.Mixed
}

func (capabilities Capabilities) normalized() Capabilities {
	if capabilities.hasAny() {
		return capabilities
	}
	return Capabilities{
		CanLookup:                  true,
		CanFullReadDocument:        true,
		CanSynthesizeMultiDocument: true,
		CanExtractExactQuote:       false,
		CanControlBindings:         true,
		CanUseExternalWeb:          false,
	}
}

func (capabilities Capabilities) hasAny() bool {
	return capabilities.CanLookup ||
		capabilities.CanFullReadDocument ||
		capabilities.CanSynthesizeMultiDocument ||
		capabilities.CanExtractExactQuote ||
		capabilities.CanControlBindings ||
		capabilities.CanUseExternalWeb
}

func normalizeFamily(value Family) Family {
	switch Family(strings.ToUpper(strings.TrimSpace(string(value)))) {
	case FamilyMeta, FamilyControl, FamilyLookup, FamilyRead, FamilySynthesize, FamilyUnsupported:
		return Family(strings.ToUpper(strings.TrimSpace(string(value))))
	default:
		return ""
	}
}

func normalizeSourceSpace(value SourceSpace) SourceSpace {
	switch SourceSpace(strings.TrimSpace(strings.ToLower(string(value)))) {
	case SourceSpaceConversation, SourceSpaceKnowledgeBase, SourceSpaceSelectedDocuments, SourceSpaceAllDocuments, SourceSpaceMetadata, SourceSpaceMixed:
		return SourceSpace(strings.TrimSpace(strings.ToLower(string(value))))
	default:
		return ""
	}
}

func normalizeScope(value Scope) Scope {
	switch Scope(strings.TrimSpace(strings.ToLower(string(value)))) {
	case ScopeChunk, ScopeSection, ScopeDocument, ScopeMultiDocument, ScopeNotebook:
		return Scope(strings.TrimSpace(strings.ToLower(string(value))))
	default:
		return ""
	}
}

func normalizeReadDepth(value ReadDepth) ReadDepth {
	switch ReadDepth(strings.TrimSpace(strings.ToLower(string(value)))) {
	case ReadDepthRetrieve, ReadDepthFocusedRead, ReadDepthFullRead:
		return ReadDepth(strings.TrimSpace(strings.ToLower(string(value))))
	default:
		return ""
	}
}

func normalizeOutputMode(value OutputMode) OutputMode {
	switch OutputMode(strings.TrimSpace(strings.ToLower(string(value)))) {
	case OutputModeAnswer, OutputModeSummary, OutputModeCompare, OutputModeExtract, OutputModeOutline, OutputModeTable, OutputModeTimeline, OutputModeQuiz, OutputModeRewrite:
		return OutputMode(strings.TrimSpace(strings.ToLower(string(value))))
	default:
		return ""
	}
}

func normalizeEvidenceMode(value EvidenceMode) EvidenceMode {
	switch EvidenceMode(strings.TrimSpace(strings.ToLower(string(value)))) {
	case EvidenceModeNone, EvidenceModeCitation, EvidenceModeExactQuote:
		return EvidenceMode(strings.TrimSpace(strings.ToLower(string(value))))
	default:
		return ""
	}
}

func inferSourceSpace(input PlanningInput) SourceSpace {
	if len(input.SelectedTargets) == 0 {
		if input.AvailableSpaces.Metadata && !input.AvailableSpaces.Conversation && !input.AvailableSpaces.KnowledgeBase && !input.AvailableSpaces.SelectedDocuments && !input.AvailableSpaces.AllDocuments && !input.AvailableSpaces.Mixed {
			return SourceSpaceMetadata
		}
		if len(input.ContextHints.CurrentDocumentIDs) > 0 {
			return SourceSpaceSelectedDocuments
		}
		if len(input.ContextHints.CurrentKBIDs) > 0 {
			return SourceSpaceKnowledgeBase
		}
		if input.ContextHints.ConversationID != "" {
			return SourceSpaceConversation
		}
		if input.AvailableSpaces.SelectedDocuments {
			return SourceSpaceSelectedDocuments
		}
		if input.AvailableSpaces.KnowledgeBase {
			return SourceSpaceKnowledgeBase
		}
		if input.AvailableSpaces.Conversation {
			return SourceSpaceConversation
		}
		return SourceSpaceMetadata
	}

	hasDoc := false
	hasKB := false
	hasConversation := false
	for _, item := range input.SelectedTargets {
		switch item.Type {
		case "document":
			hasDoc = true
		case "knowledge_base":
			hasKB = true
		case "conversation":
			hasConversation = true
		}
	}
	switch {
	case hasConversation && (hasDoc || hasKB):
		return SourceSpaceMixed
	case hasConversation:
		return SourceSpaceConversation
	case hasDoc && hasKB:
		return SourceSpaceMixed
	case hasDoc:
		return SourceSpaceSelectedDocuments
	case hasKB:
		return SourceSpaceKnowledgeBase
	default:
		return SourceSpaceMetadata
	}
}

func defaultScopeForFamily(family Family) Scope {
	switch family {
	case FamilyMeta, FamilyControl, FamilyUnsupported:
		return ScopeNotebook
	case FamilyLookup:
		return ScopeChunk
	case FamilySynthesize:
		return ScopeMultiDocument
	default:
		return ScopeDocument
	}
}

func defaultReadDepthForFamily(family Family, scope Scope) ReadDepth {
	switch family {
	case FamilyMeta, FamilyControl, FamilyUnsupported, FamilyLookup:
		return ReadDepthRetrieve
	case FamilySynthesize:
		return ReadDepthFocusedRead
	case FamilyRead:
		if scope == ScopeChunk || scope == ScopeSection {
			return ReadDepthFocusedRead
		}
		return ReadDepthFullRead
	default:
		return ReadDepthRetrieve
	}
}

func defaultTargets(input PlanningInput, sourceSpace SourceSpace) []string {
	if len(input.SelectedTargets) == 0 {
		return nil
	}
	targets := make([]string, 0, len(input.SelectedTargets))
	for _, item := range input.SelectedTargets {
		switch sourceSpace {
		case SourceSpaceSelectedDocuments:
			if item.Type != "document" {
				continue
			}
		case SourceSpaceKnowledgeBase:
			if item.Type != "knowledge_base" {
				continue
			}
		case SourceSpaceConversation:
			if item.Type != "conversation" {
				continue
			}
		case SourceSpaceMetadata:
			continue
		}
		targets = append(targets, item.ID)
	}
	return normalizeStringList(targets)
}

func defaultFallbackFamily(family Family) Family {
	switch family {
	case FamilyLookup:
		return FamilyRead
	case FamilyRead:
		return FamilySynthesize
	case FamilyMeta:
		return FamilyMeta
	case FamilyControl:
		return FamilyControl
	case FamilyUnsupported:
		return FamilyUnsupported
	default:
		return FamilySynthesize
	}
}

func defaultConfidenceForFamily(family Family) float64 {
	switch family {
	case FamilyMeta, FamilyControl:
		return 0.95
	case FamilyLookup:
		return 0.80
	case FamilyRead:
		return 0.78
	case FamilySynthesize:
		return 0.75
	case FamilyUnsupported:
		return 0.90
	default:
		return 0.60
	}
}

func unsupportedReason(input PlanningInput, plan Plan) string {
	switch plan.Family {
	case FamilyLookup:
		if !input.Capabilities.CanLookup {
			return "当前未提供局部检索能力，无法执行 LOOKUP 路径"
		}
	case FamilyRead:
		if plan.ReadDepth != ReadDepthRetrieve && !input.Capabilities.CanFullReadDocument {
			return "当前未提供长文精读能力，无法执行 READ 路径"
		}
	case FamilySynthesize:
		if !input.Capabilities.CanSynthesizeMultiDocument {
			return "当前未提供跨文档综合能力，无法执行 SYNTHESIZE 路径"
		}
	case FamilyControl:
		if !input.Capabilities.CanControlBindings {
			return "当前未提供控制类执行能力，无法执行 CONTROL 路径"
		}
	}

	if plan.EvidenceMode == EvidenceModeExactQuote && !input.Capabilities.CanExtractExactQuote {
		return "当前未提供严格原句抽取能力，无法满足 exact_quote 要求"
	}
	if plan.Constraints.AllowExternalWeb && !input.Capabilities.CanUseExternalWeb {
		return "当前未提供外部联网能力，无法满足外部检索要求"
	}
	return ""
}

func defaultReason(plan Plan) string {
	switch plan.Family {
	case FamilyMeta:
		return "状态或元数据查询"
	case FamilyControl:
		return "控制类请求"
	case FamilyLookup:
		return "局部事实检索"
	case FamilyRead:
		return "单源精读理解"
	case FamilySynthesize:
		return "多源综合分析"
	case FamilyUnsupported:
		return "当前能力无法满足请求"
	default:
		return "已回退到最近的方法族"
	}
}

func clampConfidence(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func (p Plan) Validate() error {
	if normalizeFamily(p.Family) == "" {
		return fmt.Errorf("invalid family")
	}
	if normalizeSourceSpace(p.SourceSpace) == "" {
		return fmt.Errorf("invalid source_space")
	}
	if normalizeScope(p.Scope) == "" {
		return fmt.Errorf("invalid scope")
	}
	if normalizeReadDepth(p.ReadDepth) == "" {
		return fmt.Errorf("invalid read_depth")
	}
	if normalizeOutputMode(p.OutputMode) == "" {
		return fmt.Errorf("invalid output_mode")
	}
	if normalizeEvidenceMode(p.EvidenceMode) == "" {
		return fmt.Errorf("invalid evidence_mode")
	}
	if normalizeFamily(p.FallbackFamily) == "" {
		return fmt.Errorf("invalid fallback_family")
	}
	if strings.TrimSpace(p.PlanVersion) == "" {
		return fmt.Errorf("missing plan_version")
	}
	return nil
}
