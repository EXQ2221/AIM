# AIM P4 RAG 实现规格说明

> 版本：v1  
> 范围：为 AIM 的 Bot 引入 RAG 能力。  
> 本规格基于 PostgreSQL + pgvector。  
> 本规格不实现复杂文档解析、不实现 Redis Stream、不实现高级检索优化。  
> 本规格中的“必须”“不得”“本阶段不实现”均为实现约束。

---

## 0. 背景

AIM 当前已经完成或基本完成：

```text
1. 基础聊天能力。
2. 群聊 Bot 能力。
3. Bot 成员化。
4. Bot 非流式调用 LLM。
5. PostgreSQL 迁移。
````

P4 的目标是在现有 Bot 能力上增加 RAG：

```text
用户上传或导入知识库文本
→ 系统切分 chunk
→ 调用 embedding API
→ 写入 PostgreSQL + pgvector
→ 用户在群里 @Bot
→ Bot 根据问题检索知识库
→ 将检索结果拼入 prompt
→ LLM 基于资料回答
```

---

## 1. P4 目标

P4 必须完成最小 RAG 闭环：

```text
1. PostgreSQL 启用 pgvector。
2. 创建知识库表。
3. 支持创建知识库。
4. 支持向知识库导入纯文本 / Markdown 文档。
5. 支持文档切分 chunk。
6. 支持调用 embedding API。
7. 支持将 chunk embedding 写入 PostgreSQL。
8. 支持知识库 topK 检索接口。
9. 支持群聊绑定知识库。
10. 支持 Bot 根据 conversation_bots.permission_scope 使用 RAG。
```

---

## 2. P4 非目标

P4 不实现：

```text
PDF 解析
Word 解析
网页抓取
图片 OCR
语音转文字
Redis Stream
异步任务队列
失败重试队列
死信队列
rerank 模型
BM25 + vector 混合检索
query rewrite
GraphRAG
Agent
多知识库复杂权限继承
团队级知识库协作编辑
知识库版本管理
向量数据库 Qdrant / Milvus / Weaviate
```

P4 第一版只做：

```text
文本 / Markdown 导入
同步处理
pgvector 检索
Bot 接入
```

---

## 3. 总体架构

### 3.1 离线构建流程

用户创建知识库并导入文档后，系统必须执行：

```text
创建 knowledge_document
→ status = PROCESSING
→ 文本切分 chunk
→ 调用 embedding API
→ 写入 knowledge_chunks
→ status = READY
```

第一版允许同步执行。

如果处理失败：

```text
knowledge_document.status = FAILED
knowledge_document.error_message = 错误原因
```

### 3.2 在线检索流程

用户提问时：

```text
用户 @Bot
→ 解析目标 Bot
→ 检查 conversation_bots.permission_scope
→ 如果允许 RAG：
    读取当前 conversation 绑定的知识库
    使用用户问题生成 query embedding
    pgvector topK 检索 chunks
    拼接知识库资料
→ 拼接群聊上下文
→ 调用 LLM
→ 写入 BOT_REPLY
```

### 3.3 与现有 Bot 的关系

P4 不重写 Bot 主链路。

RAG 只能作为 Bot prompt 构造阶段的增强能力。

现有 Bot 能力必须保持可用：

```text
CONVERSATION_ONLY：
- 只使用群聊最近消息。
- 不访问知识库。

KNOWLEDGE_BASE_ONLY：
- 只使用知识库检索结果。
- 不使用群聊最近消息，除当前用户问题外。

CONVERSATION_AND_KB：
- 同时使用群聊最近消息和知识库检索结果。
```

---

## 4. PostgreSQL 与 pgvector

### 4.1 pgvector 扩展

P4 必须启用：

```sql
CREATE EXTENSION IF NOT EXISTS vector;
```

该操作应在数据库初始化或迁移阶段执行。

如果 pgvector 未安装或启用失败，RAG 初始化必须失败并输出明确错误。

### 4.2 embedding 维度

P4 必须通过配置明确 embedding 维度：

```env
EMBEDDING_DIMENSION=1536
```

如果使用的 embedding 模型不是 1536 维，必须修改该配置。

`knowledge_chunks.embedding` 的 vector 维度必须与 `EMBEDDING_DIMENSION` 一致。

### 4.3 pgvector 索引

P4 第一版可以不创建向量索引。

原因：

```text
第一版知识库数据量较小。
直接 ORDER BY embedding <=> queryVector LIMIT topK 可以跑通闭环。
```

后续数据量变大后，再单独添加：

```sql
hnsw
ivfflat
```

索引优化不属于 P4 第一版必做内容。

---

## 5. 数据模型

## 5.1 knowledge_bases

表示一个知识库。

```go
type KnowledgeBaseScope string

const (
    KnowledgeBaseScopeConversation KnowledgeBaseScope = "CONVERSATION"
)

type KnowledgeBaseStatus string

const (
    KnowledgeBaseStatusActive   KnowledgeBaseStatus = "ACTIVE"
    KnowledgeBaseStatusDisabled KnowledgeBaseStatus = "DISABLED"
)

type KnowledgeBase struct {
    ID          uint64              `gorm:"primaryKey;autoIncrement" json:"id"`
    Name        string              `gorm:"type:varchar(128);not null" json:"name"`
    Description string              `gorm:"type:text" json:"description"`
    OwnerID     uint64              `gorm:"not null;index" json:"ownerId"`
    Scope       KnowledgeBaseScope  `gorm:"type:varchar(32);not null;default:'CONVERSATION'" json:"scope"`
    Status      KnowledgeBaseStatus `gorm:"type:varchar(32);not null;default:'ACTIVE'" json:"status"`
    CreatedAt   time.Time           `json:"createdAt"`
    UpdatedAt   time.Time           `json:"updatedAt"`
}
```

P4 第一版只支持：

```text
Scope = CONVERSATION
```

---

## 5.2 knowledge_documents

表示知识库中的原始文档。

```go
type KnowledgeDocumentSourceType string

const (
    KnowledgeDocumentSourceText     KnowledgeDocumentSourceType = "TEXT"
    KnowledgeDocumentSourceMarkdown KnowledgeDocumentSourceType = "MARKDOWN"
)

type KnowledgeDocumentStatus string

const (
    KnowledgeDocumentStatusPending    KnowledgeDocumentStatus = "PENDING"
    KnowledgeDocumentStatusProcessing KnowledgeDocumentStatus = "PROCESSING"
    KnowledgeDocumentStatusReady      KnowledgeDocumentStatus = "READY"
    KnowledgeDocumentStatusFailed     KnowledgeDocumentStatus = "FAILED"
)

type KnowledgeDocument struct {
    ID              uint64                      `gorm:"primaryKey;autoIncrement" json:"id"`
    KnowledgeBaseID uint64                      `gorm:"not null;index" json:"knowledgeBaseId"`
    Title           string                      `gorm:"type:varchar(255);not null" json:"title"`
    SourceType      KnowledgeDocumentSourceType `gorm:"type:varchar(32);not null" json:"sourceType"`
    SourceURL       string                      `gorm:"type:text" json:"sourceUrl"`
    Status          KnowledgeDocumentStatus     `gorm:"type:varchar(32);not null;default:'PENDING'" json:"status"`
    ErrorMessage    string                      `gorm:"type:text" json:"errorMessage"`
    CreatedBy       uint64                      `gorm:"not null;index" json:"createdBy"`
    CreatedAt       time.Time                   `json:"createdAt"`
    UpdatedAt       time.Time                   `json:"updatedAt"`
}
```

P4 第一版只支持：

```text
TEXT
MARKDOWN
```

不得实现 PDF / Word / URL 抓取。

---

## 5.3 knowledge_chunks

表示文档切片和向量。

由于 GORM 对 pgvector 类型支持需要额外处理，P4 可以使用 raw SQL 创建该表，或使用自定义类型。

SQL 结构必须等价于：

```sql
CREATE TABLE IF NOT EXISTS knowledge_chunks (
    id BIGSERIAL PRIMARY KEY,
    knowledge_base_id BIGINT NOT NULL,
    document_id BIGINT NOT NULL,
    chunk_index INT NOT NULL,
    content TEXT NOT NULL,
    token_count INT NOT NULL DEFAULT 0,
    embedding vector(1536) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

索引：

```sql
CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_kb_id
ON knowledge_chunks (knowledge_base_id);

CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_document_id
ON knowledge_chunks (document_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_knowledge_chunks_document_index
ON knowledge_chunks (document_id, chunk_index);
```

如果 `EMBEDDING_DIMENSION` 不是 1536，`vector(1536)` 必须替换为实际维度。

---

## 5.4 conversation_knowledge_bases

表示群聊绑定知识库。

```go
type ConversationKnowledgeBase struct {
    ID              uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
    ConversationID  uint64    `gorm:"not null;index:idx_conversation_kb,unique" json:"conversationId"`
    KnowledgeBaseID uint64    `gorm:"not null;index:idx_conversation_kb,unique" json:"knowledgeBaseId"`
    Enabled         bool      `gorm:"not null;default:true" json:"enabled"`
    CreatedBy       uint64    `gorm:"not null;index" json:"createdBy"`
    CreatedAt       time.Time `json:"createdAt"`
    UpdatedAt       time.Time `json:"updatedAt"`
}
```

唯一索引：

```text
conversation_id + knowledge_base_id
```

---

## 6. Embedding Client

### 6.1 目标

系统必须新增 embedding client，用于调用 OpenAI-compatible embeddings API。

### 6.2 环境变量

必须支持：

```env
EMBEDDING_BASE_URL=https://...
EMBEDDING_API_KEY=...
EMBEDDING_MODEL=text-embedding-xxx
EMBEDDING_DIMENSION=1536
EMBEDDING_TIMEOUT_SECONDS=30
```

### 6.3 请求格式

Embedding client 必须支持批量 input：

```go
type EmbedRequest struct {
    Model string
    Input []string
}

type EmbedResponse struct {
    Embeddings [][]float32
    PromptTokens int
    TotalTokens int
}
```

OpenAI-compatible 请求体：

```json
{
  "model": "text-embedding-xxx",
  "input": [
    "第一段文本",
    "第二段文本"
  ]
}
```

### 6.4 返回校验

Embedding client 必须校验：

```text
1. 返回 embedding 数量必须等于 input 数量。
2. 每个 embedding 维度必须等于 EMBEDDING_DIMENSION。
3. API 错误必须返回明确错误信息。
4. 超时必须受 context 控制。
```

### 6.5 不得复用 Chat Completion

Embedding 不得调用：

```text
/chat/completions
```

必须调用：

```text
/embeddings
```

或供应商兼容的 embeddings endpoint。

---

## 7. Chunk 切分规则

### 7.1 P4 第一版切分策略

P4 第一版使用字符切分，不使用 tokenizer。

配置：

```env
RAG_CHUNK_SIZE=1000
RAG_CHUNK_OVERLAP=150
```

含义：

```text
RAG_CHUNK_SIZE：
- 每个 chunk 最大字符数。
- 默认 1000。

RAG_CHUNK_OVERLAP：
- 相邻 chunk 重叠字符数。
- 默认 150。
```

### 7.2 切分要求

切分器必须满足：

```text
1. 空白内容不得生成 chunk。
2. 文档内容必须 trim。
3. chunk_index 从 0 开始递增。
4. chunk 内容不得为空。
5. overlap 不得大于或等于 chunk_size。
6. 文档长度小于 chunk_size 时生成 1 个 chunk。
```

### 7.3 Markdown 处理

P4 第一版不做复杂 Markdown AST 解析。

Markdown 文本按普通文本处理。

允许保留标题、列表、代码块。

---

## 8. 检索规则

### 8.1 Query Embedding

用户查询时，系统必须：

```text
1. 使用用户问题生成 query embedding。
2. 校验 query embedding 维度。
3. 在绑定知识库中检索 topK chunks。
```

### 8.2 topK

默认：

```env
RAG_TOP_K=5
```

接口允许传入 topK，但必须限制范围：

```text
1 <= topK <= 10
```

如果未传，使用默认值。

### 8.3 相似度

P4 使用 pgvector cosine distance。

SQL 必须等价于：

```sql
SELECT
    id,
    document_id,
    content,
    embedding <=> $1 AS distance
FROM knowledge_chunks
WHERE knowledge_base_id = ANY($2)
ORDER BY embedding <=> $1
LIMIT $3;
```

响应中必须返回 `score`，并统一为：

```text
score = 1 - distance
```

语义必须固定为：

```text
score 越大，相关性越高。
```

不得在同一版本中混用 `distance` 与 `score` 语义。

### 8.4 检索范围

Bot 检索时，只能检索当前 conversation 已绑定且 enabled=true 的知识库。

不得检索未绑定知识库。

不得检索其他 conversation 的知识库。

---

## 9. API 设计

## 9.1 创建知识库

```http
POST /api/v1/knowledge-bases
```

请求：

```json
{
  "name": "AIM 项目知识库",
  "description": "存放 AIM 项目设计文档"
}
```

权限：

```text
登录用户
```

响应：

```json
{
  "knowledgeBaseId": 1,
  "name": "AIM 项目知识库",
  "description": "存放 AIM 项目设计文档",
  "status": "ACTIVE"
}
```

---

## 9.2 添加文本 / Markdown 文档

```http
POST /api/v1/knowledge-bases/{knowledgeBaseId}/documents/text
```

请求：

```json
{
  "title": "鉴权模块设计",
  "sourceType": "MARKDOWN",
  "content": "这里是一大段 Markdown 或纯文本..."
}
```

权限：

```text
知识库 owner
```

处理流程：

```text
1. 创建 knowledge_document，status=PROCESSING。
2. 切分 chunk。
3. 批量调用 embedding。
4. 写入 knowledge_chunks。
5. 更新 knowledge_document.status=READY。
6. 失败时 status=FAILED，写 error_message。
```

P4 第一版允许同步处理。

---

## 9.3 查询知识库文档

```http
GET /api/v1/knowledge-bases/{knowledgeBaseId}/documents
```

权限：

```text
知识库 owner
```

响应：

```json
{
  "documents": [
    {
      "documentId": 1,
      "title": "鉴权模块设计",
      "sourceType": "MARKDOWN",
      "status": "READY",
      "errorMessage": "",
      "createdAt": "..."
    }
  ]
}
```

---

## 9.4 测试检索

```http
POST /api/v1/knowledge-bases/{knowledgeBaseId}/search
```

请求：

```json
{
  "query": "refresh token 是怎么设计的？",
  "topK": 5
}
```

权限：

```text
知识库 owner
```

响应：

```json
{
  "chunks": [
    {
      "chunkId": 1,
      "documentId": 10,
      "score": 0.82,
      "content": "refresh token 存储在 Redis..."
    }
  ]
}
```

该接口必须先完成，再接入 Bot。

---

## 9.5 绑定知识库到群聊

```http
POST /api/v1/conversations/{conversationId}/knowledge-bases
```

请求：

```json
{
  "knowledgeBaseId": 1
}
```

权限：

```text
OWNER / ADMIN
```

行为：

```text
1. 校验 conversation 是 GROUP。
2. 校验操作者是 OWNER / ADMIN。
3. 校验 knowledgeBase 存在。
4. upsert conversation_knowledge_bases。
5. enabled=true。
```

---

## 9.6 查询群聊绑定知识库

```http
GET /api/v1/conversations/{conversationId}/knowledge-bases
```

权限：

```text
当前用户必须是 conversation USER 成员。
```

---

## 9.7 解绑知识库

```http
DELETE /api/v1/conversations/{conversationId}/knowledge-bases/{knowledgeBaseId}
```

权限：

```text
OWNER / ADMIN
```

行为：

```text
enabled=false
```

不得物理删除知识库。

---

## 10. Bot 接入 RAG

### 10.1 permission_scope

Bot 触发时必须根据 `conversation_bots.permission_scope` 决定上下文来源：

```text
CONVERSATION_ONLY：
- 只使用群聊最近消息。
- 不检索知识库。

KNOWLEDGE_BASE_ONLY：
- 只使用知识库检索结果。
- 不使用最近群聊消息。
- 仍可使用当前用户问题。

CONVERSATION_AND_KB：
- 使用群聊最近消息。
- 使用知识库检索结果。
```

兼容性约束（与现网代码对齐）：

```text
Task 1 ~ Task 4 期间允许仅支持 CONVERSATION_ONLY（保持旧行为）。
Task 5 必须完整放开并实现 KNOWLEDGE_BASE_ONLY / CONVERSATION_AND_KB。
```

### 10.2 无知识库行为

如果 scope 需要知识库，但当前 conversation 没有绑定 enabled 知识库：

```text
不得调用知识库检索。
Bot 可以继续回答，但必须在 prompt 中明确“当前会话未绑定知识库”。
```

如果是 `KNOWLEDGE_BASE_ONLY`，且没有绑定知识库，则 Bot 应回复：

```text
当前会话未绑定知识库，无法基于知识库回答。
```

### 10.3 检索失败行为

如果 embedding 或检索失败：

```text
1. 记录错误日志。
2. 写 ai_call_logs FAILED 或在现有 Bot 调用日志中记录错误。
3. 不得导致用户原始消息发送失败。
4. Bot 可以回复“知识库检索失败，请稍后再试”。
```

### 10.4 Prompt 格式

RAG prompt 必须区分：

```text
群聊上下文
知识库资料
用户问题
回答要求
```

示例：

```text
你是 AIM 群聊中的 AI 助手。
请基于“群聊上下文”和“知识库资料”回答用户问题。

要求：
1. 优先依据知识库资料。
2. 如果资料不足，请直接说明不确定。
3. 不要编造知识库中没有的信息。
4. 回答应简洁、准确。

【群聊上下文】
[USER:1001] 我们要接 RAG
[USER:1002] 先用 pgvector

【知识库资料】
[1] PostgreSQL 可以通过 pgvector 存储 embedding...
[2] RAG 流程包括文档解析、切片、embedding、检索...

【用户问题】
pgvector 怎么接？
```

### 10.5 引用格式

P4 第一版不强制精确引用来源。

但 prompt 中必须给每个 chunk 编号：

```text
[1]
[2]
[3]
```

后续可以扩展为回答中引用 `[1]`。

---

## 11. 前端最小要求

P4 前端只实现最小知识库管理能力：

```text
1. 创建知识库。
2. 添加文本 / Markdown 文档。
3. 查看文档状态。
4. 测试检索。
5. 群聊详情中绑定 / 解绑知识库。
```

P4 不实现复杂文档管理：

```text
拖拽上传
PDF 预览
chunk 可视化编辑
知识库权限协作
```

---

## 12. Task 拆分

## Task 0：RAG Spec Review

### 目标

只读审查本规格是否可以进入实现。

不得修改代码。

### 只读文件

```text
docs/specs/p4-rag-spec.md
docker-compose.yml
.env.example
chat-service/internal/bot/**
chat-service/internal/dal/model/**
chat-service/internal/repository/**
chat-service/internal/biz/**
gateway/internal/handler/**
frontend/src/**
```

### 审查内容

必须检查：

```text
1. pgvector 初始化方式是否明确。
2. embedding 维度是否明确。
3. embedding API 配置是否明确。
4. chunk 表结构是否可落地。
5. 文档导入是否同步处理。
6. 检索接口是否先于 Bot 接入。
7. conversation_knowledge_bases 权限是否明确。
8. Bot permission_scope 行为是否明确。
9. Task 是否互相影响过大。
10. 是否存在不确定措辞。
```

### 输出格式

```text
RAG Spec Review Result

1. Blocking Issues
2. Non-blocking Issues
3. Ambiguities
4. Risk Points
5. Suggested Spec Changes
6. Implementation Readiness: READY / NOT READY
```

### 输出落点

```text
Task 0 审查结果必须追加写入 output.md。
```

---

## Task 1：pgvector 初始化与 RAG 数据模型

### 目标

完成 pgvector 初始化和 RAG 表结构。

### 范围

本 Task 只处理数据库基础能力：

```text
pgvector extension
knowledge_bases
knowledge_documents
knowledge_chunks
conversation_knowledge_bases
```

### 允许修改

```text
docker-compose.yml
数据库初始化 SQL
chat-service/internal/dal/model/**
chat-service/internal/dal/postgres/**
chat-service/internal/dal/mysql/** 如果已重命名则按实际路径
chat-service 初始化迁移代码
```

### 禁止修改

```text
Bot prompt
LLM client
Embedding client
gateway handler
frontend
业务接口
```

### 要求

```text
1. PostgreSQL 启用 pgvector。
2. 创建 knowledge_bases 模型。
3. 创建 knowledge_documents 模型。
4. 创建 conversation_knowledge_bases 模型。
5. 创建 knowledge_chunks 表。
6. knowledge_chunks.embedding 使用 vector(EMBEDDING_DIMENSION)。
7. AutoMigrate 或 raw SQL 必须可重复执行。
8. 不创建 RAG 业务接口。
```

### 验收标准

```text
1. PostgreSQL 中 pgvector 可用。
2. RAG 相关表可创建。
3. 重复启动不会因表已存在而失败。
4. 不影响已有聊天 / Bot 表。
```

---

## Task 2：Embedding Client 与 Chunk 处理

### 目标

实现 embedding client、文本切分和文档处理核心逻辑。

### 范围

本 Task 不做 HTTP 接口，不做 Bot 接入，只做服务层能力：

```text
embedding client
chunk splitter
document processing service
embedding 写入 knowledge_chunks
```

### 方言差异清单（必须扫描）

实现前必须扫描并记录 PostgreSQL 与历史 MySQL 方言差异，至少覆盖：

```text
ON DUPLICATE KEY / ON CONFLICT
INSERT IGNORE / DO NOTHING
NOW() / CURRENT_TIMESTAMP / 时区函数
JSON_EXTRACT / -> / ->> / jsonb_set
LIMIT/OFFSET 与分页稳定排序
大小写敏感与 collation 差异
```

### 允许修改

```text
chat-service/internal/rag/**
chat-service/internal/embedding/**
chat-service/internal/config/**
chat-service/internal/repository/**
```

### 禁止修改

```text
gateway
frontend
Bot prompt
Bot service
IDL
```

### 要求

```text
1. 支持 EMBEDDING_BASE_URL。
2. 支持 EMBEDDING_API_KEY。
3. 支持 EMBEDDING_MODEL。
4. 支持 EMBEDDING_DIMENSION。
5. 支持 EMBEDDING_TIMEOUT_SECONDS。
6. embedding client 调用 /embeddings。
7. chunk splitter 使用 RAG_CHUNK_SIZE / RAG_CHUNK_OVERLAP。
8. 文档处理必须生成 chunks。
9. 每个 chunk 必须生成 embedding。
10. embedding 维度不匹配必须报错。
11. 处理失败必须返回明确错误。
```

### 验收标准

```text
1. 给一段文本可以切出 chunks。
2. chunks 顺序正确。
3. overlap 生效。
4. embedding client 能解析 OpenAI-compatible 响应。
5. 维度校验生效。
```

---

## Task 3：知识库 API 与检索 API

### 目标

实现知识库管理和检索 API。

### 范围

本 Task 包含后端完整链路：

```text
创建知识库
导入文本 / Markdown 文档
查询文档列表
search topK
```

### 允许修改

```text
idl/**
chat-service/internal/handler/**
chat-service/internal/biz/**
chat-service/internal/repository/**
gateway/internal/handler/**
gateway/internal/router/**
```

### 禁止修改

```text
Bot service
Bot prompt
frontend
Redis Stream
异步队列
```

### 要求

必须实现：

```http
POST /api/v1/knowledge-bases
POST /api/v1/knowledge-bases/{knowledgeBaseId}/documents/text
GET /api/v1/knowledge-bases/{knowledgeBaseId}/documents
POST /api/v1/knowledge-bases/{knowledgeBaseId}/search
```

`documents/text` 第一版允许同步处理。

`documents/text` 必须限制输入大小，推荐默认：

```text
content 最大 200000 字符（约 200KB 纯文本）
```

超限时必须返回明确错误，避免同步处理超时或 OOM。

`search` 必须：

```text
1. 对 query 生成 embedding。
2. 使用 pgvector 检索 topK chunks。
3. 返回 chunkId / documentId / score / content。
4. topK 限制在 1~10。
```

### 验收标准

```text
1. 可以创建知识库。
2. 可以导入文本或 Markdown。
3. 文档处理成功后 status=READY。
4. 处理失败后 status=FAILED。
5. search 接口能返回相关 chunks。
```

---

## Task 4：群聊绑定知识库 API

### 目标

实现 conversation 与 knowledge_base 的绑定关系。

### 范围

本 Task 只处理群聊绑定 / 解绑 / 查询，不接入 Bot。

### 允许修改

```text
idl/**
chat-service/internal/handler/**
chat-service/internal/biz/**
chat-service/internal/repository/**
gateway/internal/handler/**
gateway/internal/router/**
```

### 禁止修改

```text
Bot service
Bot prompt
frontend 复杂页面
Embedding client
文档处理逻辑
```

### 要求

必须实现：

```http
POST /api/v1/conversations/{conversationId}/knowledge-bases
GET /api/v1/conversations/{conversationId}/knowledge-bases
DELETE /api/v1/conversations/{conversationId}/knowledge-bases/{knowledgeBaseId}
```

权限：

```text
POST：OWNER / ADMIN
GET：conversation USER 成员
DELETE：OWNER / ADMIN
```

绑定要求：

```text
1. conversation 必须是 GROUP。
2. knowledge_base 必须存在且 ACTIVE。
3. upsert conversation_knowledge_bases。
4. 删除时 enabled=false。
5. 不物理删除 knowledge_base。
```

### 验收标准

```text
1. OWNER / ADMIN 可以绑定知识库。
2. MEMBER 不得绑定知识库。
3. 群成员可以查看绑定知识库。
4. OWNER / ADMIN 可以解绑知识库。
5. 解绑后 enabled=false。
```

---

## Task 5：Bot 接入 RAG

### 目标

将 RAG 检索结果接入 Bot prompt。

### 范围

本 Task 只处理 Bot 与 RAG 的集成：

```text
读取 permission_scope
读取绑定知识库
执行 query embedding
检索 topK chunks
构造 RAG prompt
调用现有 LLM
```

### 允许修改

```text
chat-service/internal/bot/**
chat-service/internal/rag/**
chat-service/internal/repository/**
chat-service/internal/biz/**
```

### 禁止修改

```text
知识库管理 API
文档导入 API
frontend
gateway
数据库模型，除非发现必要小修
```

### 要求

```text
1. CONVERSATION_ONLY 不检索知识库。
2. KNOWLEDGE_BASE_ONLY 只使用知识库和当前用户问题。
3. CONVERSATION_AND_KB 使用群聊上下文 + 知识库资料。
4. 只检索当前 conversation enabled=true 的知识库。
5. 没有绑定知识库时必须有明确行为。
6. 检索失败不得影响用户原始消息发送。
7. prompt 必须区分群聊上下文、知识库资料、用户问题。
8. topK 使用 RAG_TOP_K，默认 5。
```

### 验收标准

```text
1. Bot 在 CONVERSATION_ONLY 下行为不变。
2. Bot 在 CONVERSATION_AND_KB 下会检索知识库。
3. Bot prompt 中包含检索到的 chunk。
4. 没有绑定知识库时不会 panic。
5. 检索失败时有明确错误处理。
```

---

## Task 6：前端最小知识库管理

### 目标

实现最小可用前端知识库管理界面。

### 范围

本 Task 包含：

```text
创建知识库
导入文本 / Markdown 文档
查看文档状态
测试检索
群聊绑定 / 解绑知识库
```

### 允许修改

```text
frontend/src/**
```

### 禁止修改

```text
后端 API
数据库模型
Bot service
```

### 要求

```text
1. 提供创建知识库入口。
2. 提供文本 / Markdown 导入入口。
3. 显示文档列表和状态。
4. 提供 search 测试输入框。
5. 群聊详情中显示已绑定知识库。
6. OWNER / ADMIN 可绑定 / 解绑。
7. MEMBER 只读查看。
```

### 验收标准

```text
1. 前端可以创建知识库。
2. 前端可以导入文本。
3. 前端可以查看 READY / FAILED 状态。
4. 前端可以测试检索。
5. 群聊可以绑定知识库。
```

---

## Task 7：文档对齐与冒烟验证

### 目标

更新文档并完成 RAG 最小闭环验证。

### 允许修改

```text
README.md
docs/specs/p4-rag-spec.md
output.md
```

### 禁止修改

```text
业务代码
前端代码
数据库模型
```

### 必须验证

```text
1. PostgreSQL pgvector 可用。
2. 创建知识库成功。
3. 导入文本成功。
4. 文档 status=READY。
5. search 接口能返回 chunks。
6. 群聊绑定知识库成功。
7. Bot 在 CONVERSATION_AND_KB 下能使用知识库回答。
8. CONVERSATION_ONLY 仍保持旧行为。
```

### 输出要求

```text
Changed files
What changed
Commands run
Smoke test result
Known issues
Remaining TODOs
```

### Task7 执行注意（基于 2026-05-13 真实联调）

```text
1. 运行环境若使用普通 postgres:16 镜像，可能未内置 pgvector 扩展。
2. 若缺失 pgvector，会导致 chat-service 在创建 knowledge_chunks 时因 vector 类型不存在而健康检查失败。
3. EMBEDDING_BASE_URL 配置不得包含前导 "="，否则会生成非法 URL。
4. EMBEDDING_MODEL 必须是 OpenAI-compatible /embeddings 可用模型，否则 search/doc 处理会返回 model_not_supported。
5. Task7 冒烟允许记录“环境阻塞导致部分用例未通过”，但必须给出可复现错误与修复路径。
```

---

## Task 8：RAG 运行基线补齐（Task 0 后立即执行）

### 目标

补齐 RAG 运行前置基线，确保后续 Task1/Task2 可直接执行。

### 允许修改

```text
docker-compose.yml
.env.example
deploy/postgres/init/**
docs/specs/p4-rag-spec.md
output.md
```

### 必做项

```text
1. PostgreSQL 运行环境必须可启用 pgvector（镜像或安装方式明确）。
2. 初始化脚本必须执行 CREATE EXTENSION IF NOT EXISTS vector（至少 aim_chat）。
3. .env.example 增加 EMBEDDING_* 与 RAG_* 默认配置示例。
4. docker-compose.yml 将 EMBEDDING_* 与 RAG_* 透传给 chat-service。
```

### 验收标准

```text
1. 新环境首次启动可自动创建 vector 扩展。
2. chat-service 能读取到 EMBEDDING_* / RAG_* 环境变量。
3. 变更不影响现有非 RAG 服务启动。
```

---

## 13. 推荐执行顺序

必须按以下顺序执行：

```text
Task 0：RAG Spec Review
Task 8：RAG 运行基线补齐（Task 0 后立即执行）
Task 1：pgvector 初始化与 RAG 数据模型
Task 2：Embedding Client 与 Chunk 处理
Task 3：知识库 API 与检索 API
Task 4：群聊绑定知识库 API
Task 5：Bot 接入 RAG
Task 6：前端最小知识库管理
Task 7：文档对齐与冒烟验证
```

不得跳过 Task 0。

---

## 14. 总体验收标准

P4 完成后必须满足：

```text
1. PostgreSQL 已启用 pgvector。
2. 知识库表已创建。
3. 可以创建知识库。
4. 可以导入 TEXT / MARKDOWN 文档。
5. 文档可以切分 chunks。
6. chunks 可以生成 embedding。
7. chunks 可以写入 pgvector。
8. search 接口可以返回 topK chunks。
9. 群聊可以绑定知识库。
10. Bot 可以根据 permission_scope 使用 RAG。
11. CONVERSATION_ONLY 行为不受影响。
12. 前端可以进行最小知识库管理。
```

---

## 15. 后续 Future Work

P4 完成后，后续可继续扩展：

```text
Redis Stream 异步文档处理
失败重试
PDF / Word 解析
文件上传
chunk 可视化
RAG 引用来源展示
rerank
BM25 + vector 混合检索
HNSW / IVFFlat 索引优化
知识库权限协作
Bot 私聊 RAG
多租户知识库
```

