namespace go rag

struct HealthRequest {}

struct HealthResponse {
  1: bool ok
}

struct CommonResponse {
  1: bool success
  2: string message
}

struct KnowledgeBaseInfo {
  1: i64 knowledge_base_id
  2: string name
  3: string description
  4: string status
}

struct KnowledgeDocumentInfo {
  1: i64 document_id
  2: i64 knowledge_base_id
  3: string title
  4: string source_type
  5: string status
  6: string error_message
  7: i64 created_at
}

struct KnowledgeSearchChunkInfo {
  1: i64 chunk_id
  2: i64 document_id
  3: double score
  4: string content
}

struct CreateKnowledgeBaseRequest {
  1: i64 operator_id
  2: string name
  3: string description
}

struct CreateKnowledgeBaseResponse {
  1: KnowledgeBaseInfo knowledge_base
}

struct ListKnowledgeBasesRequest {
  1: i64 operator_id
}

struct ListKnowledgeBasesResponse {
  1: list<KnowledgeBaseInfo> knowledge_bases
}

struct AddKnowledgeDocumentTextRequest {
  1: i64 operator_id
  2: i64 knowledge_base_id
  3: string title
  4: string source_type
  5: string content
}

struct AddKnowledgeDocumentTextResponse {
  1: KnowledgeDocumentInfo document
}

struct ListKnowledgeDocumentsRequest {
  1: i64 operator_id
  2: i64 knowledge_base_id
}

struct ListKnowledgeDocumentsResponse {
  1: list<KnowledgeDocumentInfo> documents
}

struct DeleteKnowledgeDocumentRequest {
  1: i64 operator_id
  2: i64 knowledge_base_id
  3: i64 document_id
}

struct SearchKnowledgeBaseRequest {
  1: i64 operator_id
  2: i64 knowledge_base_id
  3: string query
  4: optional i32 top_k
}

struct SearchKnowledgeBaseResponse {
  1: list<KnowledgeSearchChunkInfo> chunks
}

struct ConversationKnowledgeBaseInfo {
  1: i64 id
  2: string conversation_id
  3: i64 knowledge_base_id
  4: string name
  5: string description
  6: string status
  7: bool enabled
}

struct BindConversationKnowledgeBaseRequest {
  1: i64 operator_id
  2: string conversation_id
  3: i64 knowledge_base_id
}

struct ListConversationKnowledgeBasesRequest {
  1: i64 operator_id
  2: string conversation_id
}

struct ListConversationKnowledgeBasesResponse {
  1: list<ConversationKnowledgeBaseInfo> knowledge_bases
}

struct UnbindConversationKnowledgeBaseRequest {
  1: i64 operator_id
  2: string conversation_id
  3: i64 knowledge_base_id
}

service RAGService {
  HealthResponse Health(1: HealthRequest req)
  CreateKnowledgeBaseResponse CreateKnowledgeBase(1: CreateKnowledgeBaseRequest req)
  ListKnowledgeBasesResponse ListKnowledgeBases(1: ListKnowledgeBasesRequest req)
  AddKnowledgeDocumentTextResponse AddKnowledgeDocumentText(1: AddKnowledgeDocumentTextRequest req)
  ListKnowledgeDocumentsResponse ListKnowledgeDocuments(1: ListKnowledgeDocumentsRequest req)
  CommonResponse DeleteKnowledgeDocument(1: DeleteKnowledgeDocumentRequest req)
  SearchKnowledgeBaseResponse SearchKnowledgeBase(1: SearchKnowledgeBaseRequest req)
  CommonResponse BindConversationKnowledgeBase(1: BindConversationKnowledgeBaseRequest req)
  ListConversationKnowledgeBasesResponse ListConversationKnowledgeBases(1: ListConversationKnowledgeBasesRequest req)
  CommonResponse UnbindConversationKnowledgeBase(1: UnbindConversationKnowledgeBaseRequest req)
}
