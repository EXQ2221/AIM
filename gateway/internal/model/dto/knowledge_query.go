package dto

type AskKnowledgeBaseRequest struct {
	Query          string `json:"query"`
	TopK           *int32 `json:"topK,omitempty"`
	ConversationID string `json:"conversationId,omitempty"`
	BotID          *int64 `json:"botId,omitempty"`
}

type KnowledgeBaseQueryCitationInfo struct {
	Index         int     `json:"index"`
	ChunkID       int64   `json:"chunkId"`
	DocumentID    int64   `json:"documentId"`
	DocumentTitle string  `json:"documentTitle"`
	Score         float64 `json:"score"`
	Excerpt       string  `json:"excerpt"`
}

type KnowledgeBaseQueryQuoteInfo struct {
	QuoteID       string `json:"quoteId"`
	DocumentID    int64  `json:"documentId"`
	DocumentTitle string `json:"documentTitle"`
	ChunkID       int64  `json:"chunkId"`
	SentenceIndex int    `json:"sentenceIndex"`
	PageStart     int    `json:"pageStart"`
	PageEnd       int    `json:"pageEnd"`
	CharStart     int    `json:"charStart"`
	CharEnd       int    `json:"charEnd"`
	Text          string `json:"text"`
}

type KnowledgeBaseQueryResponse struct {
	Status    string                         `json:"status"`
	Answer    string                         `json:"answer"`
	Model     string                         `json:"model,omitempty"`
	Plan      QueryRoutePlanInfo             `json:"plan"`
	Citations []KnowledgeBaseQueryCitationInfo `json:"citations,omitempty"`
	Quotes    []KnowledgeBaseQueryQuoteInfo  `json:"quotes,omitempty"`
	Chunks    []KnowledgeSearchChunkInfo     `json:"chunks,omitempty"`
}
