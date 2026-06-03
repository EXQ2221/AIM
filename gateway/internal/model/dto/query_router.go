package dto

type QueryRouteTarget struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
}

type QueryRouteAvailableSpaces struct {
	Conversation      bool `json:"conversation"`
	KnowledgeBase     bool `json:"knowledge_base"`
	SelectedDocuments bool `json:"selected_documents"`
	AllDocuments      bool `json:"all_documents"`
	Metadata          bool `json:"metadata"`
	Mixed             bool `json:"mixed"`
}

type QueryRouteCapabilities struct {
	CanLookup                  bool `json:"can_lookup"`
	CanFullReadDocument        bool `json:"can_full_read_document"`
	CanSynthesizeMultiDocument bool `json:"can_synthesize_multi_document"`
	CanExtractExactQuote       bool `json:"can_extract_exact_quote"`
	CanControlBindings         bool `json:"can_control_bindings"`
	CanUseExternalWeb          bool `json:"can_use_external_web"`
}

type QueryRouteContextHints struct {
	ConversationID     string   `json:"conversation_id"`
	CurrentDocumentIDs []string `json:"current_document_ids"`
	CurrentKBIDs       []string `json:"current_kb_ids"`
}

type QueryRoutePlanRequest struct {
	UserQuery       string                    `json:"userQuery"`
	ConversationID  string                    `json:"conversationId"`
	BotID           *int64                    `json:"botId,omitempty"`
	SelectedTargets []QueryRouteTarget        `json:"selectedTargets"`
	AvailableSpaces QueryRouteAvailableSpaces `json:"availableSpaces"`
	Capabilities    QueryRouteCapabilities    `json:"capabilities"`
	ContextHints    QueryRouteContextHints    `json:"contextHints"`
}

type QueryRouteConstraints struct {
	MustGroundInSources bool `json:"must_ground_in_sources"`
	AllowExternalWeb    bool `json:"allow_external_web"`
	StrictQuoteRequired bool `json:"strict_quote_required"`
}

type QueryRoutePlanInfo struct {
	PlanVersion   string                 `json:"plan_version"`
	Family        string                 `json:"family"`
	SourceSpace   string                 `json:"source_space"`
	Scope         string                 `json:"scope"`
	ReadDepth     string                 `json:"read_depth"`
	OutputMode    string                 `json:"output_mode"`
	EvidenceMode  string                 `json:"evidence_mode"`
	Targets       []string               `json:"targets"`
	Constraints   QueryRouteConstraints  `json:"constraints"`
	Confidence    float64                `json:"confidence"`
	FallbackFamily string                `json:"fallback_family"`
	Reason        string                 `json:"reason"`
}
