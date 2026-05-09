# AIM Bot 非流式接入 Spec v4

> **⚠️ 历史背景文档**
>
> 本文档是 P3 之前的 Bot 接入原始 spec（v4），记录了早期设计决策和完整实现细节。
>
> **P3 阶段已重写成员模型和 Bot 架构**，当前以以下文档为准：
> - `docs/specs/tasks/*.md` — 各 task 的具体要求
> - `docs/specs/tasks/p3-ai-bot-overview.md` — P3 总纲
> - `docs/specs/gorm-model-spec.md` — 模型演进记录（具体实现以代码为准）
>
> 本 spec 中与 P3 task spec 冲突的内容（如旧 user_id 字段、无 member_type、无 mention_name/aliases 等）已被 P3 覆盖。

## 0. 给 Codex 的执行约束

本任务目标是：**在 chat-service 内部新增 Bot/LLM 模块，实现 @AIM 触发的非流式 AI 回复闭环**。

核心策略：

```text
先让用户消息正常发送成功；
再异步触发 Bot；
Bot 回复也写入 message 表；
实时广播问题单独处理，避免第一阶段改动过大。
```

### 0.1 只读文件

为了节省 Codex 额度，开始任务前只允许优先读取以下文件：

```text
AGENTS.md
docs/specs/gorm-model-spec.md
docs/specs/message-spec.md
docs/specs/websocket-spec.md
chat-service/internal/dal/model/chat.go
chat-service/internal/dal/mysql/init.go
chat-service/internal/repository/chat.go
chat-service/internal/repository/tx.go
chat-service/internal/biz/chat.go
chat-service/internal/biz/dto.go
chat-service/internal/handler/chat_service.go
chat-service/cmd/server/main.go
```

如果上述文件不足以完成任务，可以再读取：

```text
idl/chat.thrift
gateway/internal/websocket/event.go
gateway/internal/websocket/client.go
gateway/internal/websocket/hub.go
gateway/internal/handler/chat.go
docker-compose.yml
```

除非任务明确要求，否则不要全仓库搜索，不要扫描 frontend，不要读取无关服务。

### 0.2 第一阶段禁止修改的内容

第一阶段不要修改：

```text
idl/chat.thrift
gateway/**
frontend/**
auth-service/**
user-service/**
docker-compose.yml
```

第一阶段只允许修改：

```text
chat-service/internal/dal/model/**
chat-service/internal/dal/mysql/init.go
chat-service/internal/repository/**
chat-service/internal/biz/**
chat-service/internal/bot/**
chat-service/internal/llm/**
chat-service/internal/pkg/**    # 如确实需要
```

如确实需要修改 IDL、gateway、frontend 或 docker-compose，必须先输出原因和修改计划，等待人工确认。

### 0.3 代码生成约束

第一阶段不要重新生成 Kitex 代码。

原因：

- 当前 Bot 回复可以复用已有 `Message` 表和内部业务逻辑；
- 不需要新增对外 RPC；
- 避免 thrift 变更引发 gateway/chat-service 多处联动，节省额度。

### 0.4 输出要求

每次执行后必须输出：

```text
Changed files
What changed
Tests run
Tests not run
Remaining TODOs
```

并将同样内容追加到：

```text
output.md
```

如果本次任务不允许修改 `output.md`，则只在对话中输出记录内容，等待人工复制。

---

## 1. 背景

AIM 目前已有基础聊天系统：

```text
用户发消息
→ gateway WebSocket
→ chat-service CreateMessage
→ message 落库
→ gateway 广播 NEW_MESSAGE
```

现在需要接入 AI Bot，但第一版不新增独立 bot-service，不改 WebSocket 协议，不做流式回复。

第一版目标是：

```text
用户在群聊中发送 @AIM 消息
→ 用户消息正常落库并广播
→ chat-service 内部异步触发 Bot
→ Bot 调用 OpenAI-compatible LLM
→ Bot 回复作为一条普通 message 落库
→ 后续再解决 Bot 回复实时广播
```

---

## 2. 总体设计原则

### 2.1 只做非流式回复

当前版本只支持非流式 Bot 回复。

原因：

- AIM 是 IM 群聊产品，消息单位是完整 message；
- 非流式可以复用现有 message 持久化和 `NEW_MESSAGE` 广播；
- 群聊中逐字输出会增加噪音；
- 流式需要新增 `BOT_REPLY_START / BOT_REPLY_DELTA / BOT_REPLY_DONE` 等事件，当前阶段不做。

### 2.2 Bot 回复也是 Message

Bot 回复必须写入现有 `messages` 表，而不是只通过 WebSocket 临时推送。

Bot 回复建议字段：

```text
sender_type = BOT
message_type = BOT_REPLY
content = AI 生成的完整文本
conversation_id = 当前会话内部自增 ID
```

### 2.3 不要阻塞用户发消息

用户发送 `@AIM` 消息时，用户消息应该先正常成功。

Bot 回复异步执行：

```text
用户消息 MESSAGE_ACK / NEW_MESSAGE
→ 后台调用 LLM
→ 稍后生成 Bot 回复 message
```

不要让用户发送消息等待 LLM 完整返回。

### 2.4 Chat、Bot、LLM 分层

不要把大模型 HTTP 调用直接写进 `chat.go` 的 `CreateMessage` 主逻辑。

推荐边界：

```text
chat-service/internal/biz
- 负责普通消息创建
- 负责成员权限、禁言、最近消息更新
- 只在用户消息创建成功后触发 Bot

chat-service/internal/bot
- 负责判断是否触发 Bot
- 负责获取上下文
- 负责构造 prompt
- 负责调用 llm.Client
- 负责写入 Bot 回复
- 负责写入 AI 调用日志

chat-service/internal/llm
- 负责读取环境变量
- 负责调用 OpenAI-compatible API
- 负责解析返回文本和 token 用量
```

---

## 3. 异步触发 Bot 的实现逻辑

### 3.1 为什么要异步

同步触发流程是：

```text
用户发送 @AIM
→ chat-service 保存用户消息
→ chat-service 调用 AI
→ 等 AI 回复
→ 保存 Bot 回复
→ 返回给 gateway
```

问题：

```text
AI 可能耗时 3 秒、10 秒，甚至超时；
用户发送消息会被阻塞；
WebSocket SEND_MESSAGE 的 ACK 会变慢；
模型失败会影响用户消息发送体验。
```

异步触发流程是：

```text
用户发送 @AIM
→ chat-service 保存用户消息
→ 立即返回用户消息创建成功
→ 后台 goroutine 调用 AI
→ AI 回复生成后再写入 message 表
```

即：

```text
用户消息 message_id = 101，立即成功
Bot 回复 message_id = 102，稍后生成
```

### 3.2 CreateMessage 中的触发点

Bot 触发必须发生在**用户消息成功落库之后**。

伪代码：

```go
func (s *ChatBiz) CreateMessage(ctx context.Context, input CreateMessageInput) (*MessageView, error) {
    msg, err := s.createUserMessage(ctx, input)
    if err != nil {
        return nil, err
    }

    if s.botService != nil && bot.ShouldTriggerBot(msg) {
        req := bot.HandleMentionRequest{
            ConversationID:   msg.ConversationID,   // 内部自增 ID
            RequestMessageID: msg.ID,
            UserID:           msg.SenderID,
            Content:          msg.Content,
        }

        go s.handleBotAsync(req)
    }

    return msg, nil
}
```

要求：

```text
1. 用户消息创建失败时，不触发 Bot。
2. 只有 USER + TEXT 消息可以触发 Bot。
3. Bot 回复、系统消息、文件消息不能触发 Bot。
4. Bot 异步失败不能影响用户消息的成功返回。
```

### 3.3 不要直接复用原始 ctx

不要这样写：

```go
go s.botService.HandleMention(ctx, req)
```

原因：

```text
CreateMessage 返回后，RPC/HTTP 请求上下文可能被取消；
Bot 调用 LLM 可能还没完成就收到 context canceled；
异步任务生命周期不应该绑定用户发消息请求的 ctx。
```

应该在异步任务里创建新的 timeout context：

```go
func (s *ChatBiz) handleBotAsync(req bot.HandleMentionRequest) {
    defer func() {
        if r := recover(); r != nil {
            log.Printf("bot async panic: %v", r)
        }
    }()

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := s.botService.HandleMention(ctx, req); err != nil {
        log.Printf("bot async failed: %v", err)
    }
}
```

### 3.4 goroutine 内必须 recover

goroutine 内的 panic 不能影响 chat-service 主进程。

必须包含：

```go
defer func() {
    if r := recover(); r != nil {
        log.Printf("bot async panic: %v", r)
    }
}()
```

要求：

```text
1. recover 中不要打印 API Key。
2. panic 只记录日志，不向用户消息发送流程返回错误。
3. 如果已有项目统一 logger，应使用项目 logger，不直接使用 fmt.Println。
```

### 3.5 BotService.HandleMention 的职责

`HandleMention` 负责完整 Bot 生成流程。

推荐流程：

```text
HandleMention
  ↓
查询触发消息和会话信息
  ↓
查询当前会话最近 20 条消息
  ↓
构造 prompt
  ↓
创建或准备 ai_call_log
  ↓
调用 LLM
  ↓
成功：创建 Bot 回复 message
  ↓
成功：更新 conversation.last_message_id / last_message_at
  ↓
成功：更新 ai_call_log 为 SUCCESS
  ↓
失败：更新 ai_call_log 为 FAILED
```

伪代码：

```go
func (s *Service) HandleMention(ctx context.Context, req HandleMentionRequest) error {
    recentMessages, err := s.messageRepo.ListRecent(ctx, req.ConversationID, 20)
    if err != nil {
        return err
    }

    prompt := BuildPrompt(recentMessages, req.Content)

    start := time.Now()

    resp, err := s.llm.Generate(ctx, llm.GenerateRequest{
        Model: s.modelName,
        Messages: []llm.ChatMessage{
            {
                Role:    "system",
                Content: s.systemPrompt,
            },
            {
                Role:    "user",
                Content: prompt,
            },
        },
    })

    latency := time.Since(start).Milliseconds()

    if err != nil {
        return s.aiLogRepo.CreateFailed(ctx, req, err.Error(), latency)
    }

    botMsg, err := s.messageRepo.Create(ctx, model.Message{
        ConversationID: req.ConversationID,
        SenderID:       s.botID,
        SenderType:     model.SenderTypeBot,
        MessageType:    model.MessageTypeBotReply,
        Content:        resp.Content,
        Status:         model.MessageStatusNormal,
    })
    if err != nil {
        return err
    }

    if err := s.conversationRepo.UpdateLastMessage(ctx, req.ConversationID, botMsg.ID); err != nil {
        return err
    }

    return s.aiLogRepo.CreateSuccess(ctx, req, botMsg.ID, resp, latency)
}
```

### 3.6 异步失败时的用户体验

第一版建议：

```text
LLM 调用失败时：
- 写入 ai_call_logs FAILED；
- 不一定要生成一条 Bot 错误消息；
- 用户原始消息保持成功。
```

可选优化：

```text
生成一条 Bot 消息：
“AI 助手暂时无法回复，请稍后再试。”
```

如果加这条错误消息，也应该写入 message 表，且 `sender_type = BOT`。

### 3.7 异步并发限制

第一版可以暂不做复杂限流，但需要在 TODO 中标注。

后续可加：

```text
每个 conversation 同时最多 1 个 Bot 任务；
每个用户每分钟最多 N 次 @AIM；
全局 Bot worker pool，避免无限 goroutine。
```

第一版如果直接 `go handleBotAsync`，需要接受以下风险：

```text
大量用户同时 @AIM 时可能创建大量 goroutine；
LLM API 可能被限流；
费用可能失控。
```

如果实现成本不高，可以增加一个简单 semaphore：

```go
botSemaphore := make(chan struct{}, 10)

func (s *ChatBiz) handleBotAsync(req bot.HandleMentionRequest) {
    select {
    case s.botSemaphore <- struct{}{}:
        defer func() { <-s.botSemaphore }()
    default:
        log.Printf("bot async skipped: too many running tasks")
        return
    }

    // continue...
}
```

第一版可以不做，但要留 TODO。

---

## 4. Bot 回复实时广播问题

### 4.1 当前架构的问题

现有用户消息广播通常是：

```text
gateway 收到 WebSocket SEND_MESSAGE
→ gateway 调 chat-service CreateMessage
→ chat-service 返回 MessageInfo
→ gateway 广播 NEW_MESSAGE
```

但异步 Bot 回复是在：

```text
chat-service 后台 goroutine 内生成
```

因此：

```text
chat-service 创建了 Bot 回复；
gateway 不一定知道；
gateway 可能无法立即广播 Bot 回复。
```

### 4.2 可选方案

#### 方案 A：第一版不实时广播 Bot 回复

Bot 回复只写入数据库。

优点：

```text
不改 gateway；
不改 WebSocket 协议；
实现最快。
```

缺点：

```text
前端需要刷新历史消息才能看到 Bot 回复；
演示效果一般。
```

适合作为第一步验证。

#### 方案 B：前端短轮询历史消息

用户发送 `@AIM` 后，前端显示：

```text
AIM 正在回复...
```

然后每 1-2 秒拉取一次历史消息。

优点：

```text
不需要 chat-service 主动通知 gateway；
不用消息队列。
```

缺点：

```text
不够优雅；
增加查询；
需要前端改动。
```

#### 方案 C：Redis Pub/Sub 通知 gateway 广播

推荐作为后续正式方案。

流程：

```text
chat-service 生成 Bot 回复
→ publish BotReplyCreated 事件到 Redis channel
→ gateway subscribe Redis channel
→ gateway 收到事件后广播 NEW_MESSAGE
```

优点：

```text
实时体验好；
改动小于引入 Kafka/RabbitMQ；
项目已有 Redis，接入成本较低。
```

缺点：

```text
需要修改 gateway；
需要定义 Redis 事件结构；
需要处理 gateway 重启期间消息丢失问题。
```

### 4.3 第一版推荐路线

第一版先做：

```text
异步生成 Bot 回复并落库
ai_call_logs 记录成功/失败
```

然后单独开任务评估广播：

```text
Inspect whether async Bot replies can be broadcast through existing WebSocket NEW_MESSAGE path.
Do not edit files.
Output smallest-change plan.
```

不要在同一个 Codex 任务里同时做：

```text
Bot 表
LLM Client
异步触发
AI 日志
Redis Pub/Sub
gateway 订阅
前端展示
```

这样会非常烧额度，也容易引发联动 bug。

---

## 5. 新增目录建议

```text
chat-service/internal/bot/
├── service.go
├── trigger.go
└── prompt.go

chat-service/internal/llm/
├── client.go
└── openai_compatible.go
```

可选：

```text
chat-service/internal/repository/bot.go
chat-service/internal/dal/model/bot.go
```

如果现有项目习惯把模型都放在 `chat.go`，也可以先把 Bot 模型追加到已有 model 文件中，但需要保持清晰注释。

---

## 6. 新增数据表

### 6.1 bots

用于保存 Bot 基础配置。

```go
type BotStatus string

const (
    BotStatusEnabled  BotStatus = "ENABLED"
    BotStatusDisabled BotStatus = "DISABLED"
)

type Bot struct {
    ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
    Name         string    `gorm:"type:varchar(64);not null" json:"name"`
    Avatar       string    `gorm:"type:varchar(512)" json:"avatar"`
    Description  string    `gorm:"type:varchar(512)" json:"description"`
    ModelName    string    `gorm:"type:varchar(128);not null" json:"modelName"`
    SystemPrompt string    `gorm:"type:text" json:"systemPrompt"`
    CreatedBy    uint64    `gorm:"not null;index" json:"createdBy"`
    Status       BotStatus `gorm:"type:varchar(32);not null;default:'ENABLED'" json:"status"`
    CreatedAt    time.Time `json:"createdAt"`
    UpdatedAt    time.Time `json:"updatedAt"`
}
```

第一版可以通过初始化逻辑创建一个默认 Bot：

```text
Name = AIM
ModelName = 环境变量 LLM_MODEL
SystemPrompt = 你是 AIM 群聊中的 AI 助手。请基于上下文简洁、准确地回答。
CreatedBy = 0
Status = ENABLED
```

如果不想做默认数据初始化，也可以在 Bot 服务中使用内置默认配置，但长期建议落表。

### 6.2 conversation_bots

用于控制某个会话是否启用某个 Bot。

```go
type ConversationBot struct {
    ID              uint64             `gorm:"primaryKey;autoIncrement" json:"id"`
    ConversationID  uint64             `gorm:"not null;index:idx_conversation_bot,unique" json:"conversationId"`
    BotID           uint64             `gorm:"not null;index:idx_conversation_bot,unique" json:"botId"`
    Enabled         bool               `gorm:"not null;default:true" json:"enabled"`
    PermissionScope BotPermissionScope `gorm:"type:varchar(64);not null;default:'CONVERSATION_ONLY'" json:"permissionScope"`
    CreatedAt       time.Time          `json:"createdAt"`
    UpdatedAt       time.Time          `json:"updatedAt"`
}
```

注意：

```text
ConversationID 存内部自增主键，不存对外 c_xxx 字符串。
```

权限范围枚举：

```go
type BotPermissionScope string

const (
    BotScopeConversationOnly  BotPermissionScope = "CONVERSATION_ONLY"
    BotScopeKnowledgeBaseOnly BotPermissionScope = "KNOWLEDGE_BASE_ONLY"
    BotScopeConversationAndKB BotPermissionScope = "CONVERSATION_AND_KB"
)
```

### 6.3 ai_call_logs

用于记录 LLM 调用情况。

```go
type AICallStatus string

const (
    AICallStatusSuccess AICallStatus = "SUCCESS"
    AICallStatusFailed  AICallStatus = "FAILED"
)

type AICallLog struct {
    ID                uint64       `gorm:"primaryKey;autoIncrement" json:"id"`
    UserID            uint64       `gorm:"not null;index" json:"userId"`
    BotID             uint64       `gorm:"not null;index" json:"botId"`
    ConversationID    uint64       `gorm:"not null;index" json:"conversationId"`
    RequestMessageID   *uint64      `gorm:"index" json:"requestMessageId"`
    ResponseMessageID  *uint64      `gorm:"index" json:"responseMessageId"`
    ModelName          string       `gorm:"type:varchar(128);not null" json:"modelName"`
    PromptTokens       int          `gorm:"not null;default:0" json:"promptTokens"`
    CompletionTokens   int          `gorm:"not null;default:0" json:"completionTokens"`
    TotalTokens        int          `gorm:"not null;default:0" json:"totalTokens"`
    LatencyMS          int64        `gorm:"not null;default:0" json:"latencyMs"`
    Status             AICallStatus `gorm:"type:varchar(32);not null" json:"status"`
    ErrorMessage        string       `gorm:"type:text" json:"errorMessage"`
    CreatedAt          time.Time    `json:"createdAt"`
}
```

---

## 7. AutoMigrate

在 chat-service MySQL 初始化中增加：

```go
Bot{}
ConversationBot{}
AICallLog{}
```

不要破坏已有 `Conversation`、`GroupInfo`、`ConversationMember`、`Message` 的迁移逻辑。

---

## 8. LLM Client

### 8.1 环境变量

第一版使用 OpenAI-compatible Chat Completions 风格接口。

需要支持以下环境变量：

```text
LLM_BASE_URL
LLM_API_KEY
LLM_MODEL
LLM_TIMEOUT_SECONDS
```

示例：

```env
LLM_BASE_URL=https://api.deepseek.com
LLM_API_KEY=sk-xxxx
LLM_MODEL=deepseek-chat
LLM_TIMEOUT_SECONDS=30
```

不要把 API Key 写进代码、README、前端或日志。

### 8.2 Client 接口

```go
type ChatMessage struct {
    Role    string
    Content string
}

type GenerateRequest struct {
    Model    string
    Messages []ChatMessage
}

type GenerateResponse struct {
    Content          string
    PromptTokens    int
    CompletionTokens int
    TotalTokens      int
}

type Client interface {
    Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error)
}
```

### 8.3 OpenAI-compatible 实现

请求：

```http
POST {LLM_BASE_URL}/chat/completions
Authorization: Bearer {LLM_API_KEY}
Content-Type: application/json
```

请求体：

```json
{
  "model": "deepseek-chat",
  "messages": [
    {"role": "system", "content": "..."},
    {"role": "user", "content": "..."}
  ]
}
```

需要解析：

```text
choices[0].message.content
usage.prompt_tokens
usage.completion_tokens
usage.total_tokens
```

### 8.4 错误处理

必须处理：

```text
缺少环境变量
HTTP 请求失败
非 2xx 状态码
响应 JSON 解析失败
choices 为空
超时
```

错误信息不要包含完整 API Key。

---


---

## 8.5 DeepSeek OpenAI 格式接入配置

当前项目第一版使用 **OpenAI-compatible 格式** 接入 DeepSeek，不使用 Anthropic 格式。

### 8.5.1 为什么不用 Anthropic 格式

DeepSeek 文档中同时提供：

```text
OpenAI 格式 BASE URL:
https://api.deepseek.com

Anthropic 格式 BASE URL:
https://api.deepseek.com/anthropic
```

AIM 后端自己实现 Go HTTP Client，并且本 Spec 中的 `llm.Client` 设计基于 OpenAI Chat Completions 风格：

```text
POST /chat/completions
messages: [{role, content}]
stream: false
```

因此应使用：

```text
LLM_BASE_URL=https://api.deepseek.com
```

不要使用：

```text
https://api.deepseek.com/anthropic
```

Anthropic 格式主要用于兼容 Claude/Anthropic API 风格工具，不适合作为当前第一版实现目标。

### 8.5.2 .env 示例

Codex 可以创建或更新 `.env` / `.env.example`，但不要把真实 API Key 写入仓库。

推荐 `.env.example`：

```env
# LLM provider: DeepSeek OpenAI-compatible API
LLM_BASE_URL=https://api.deepseek.com
LLM_API_KEY=replace-with-your-deepseek-api-key
LLM_MODEL=deepseek-v4-flash
LLM_TIMEOUT_SECONDS=30
```

本地真实 `.env`：

```env
LLM_BASE_URL=https://api.deepseek.com
LLM_API_KEY=在这里粘贴真实 API Key
LLM_MODEL=deepseek-v4-flash
LLM_TIMEOUT_SECONDS=30
```

开发测试阶段推荐：

```env
LLM_MODEL=deepseek-v4-flash
```

如果需要更强模型，可手动改为：

```env
LLM_MODEL=deepseek-v4-pro
```

### 8.5.3 请求地址

代码中最终请求地址应拼接为：

```text
{LLM_BASE_URL}/chat/completions
```

当：

```text
LLM_BASE_URL=https://api.deepseek.com
```

最终请求地址是：

```text
https://api.deepseek.com/chat/completions
```

注意不要拼成：

```text
https://api.deepseek.com/v1/chat/completions
```

除非平台文档明确要求 `/v1`。DeepSeek 当前示例使用的是：

```text
https://api.deepseek.com/chat/completions
```

### 8.5.4 请求头

```http
Content-Type: application/json
Authorization: Bearer ${LLM_API_KEY}
```

不要在日志中打印完整 `Authorization` header。

### 8.5.5 最小非流式请求体

第一版只使用最小参数：

```json
{
  "model": "deepseek-v4-flash",
  "messages": [
    {
      "role": "system",
      "content": "你是 AIM 群聊中的 AI 助手。请基于上下文简洁、准确地回答。"
    },
    {
      "role": "user",
      "content": "你好"
    }
  ],
  "stream": false
}
```

第一版不要默认加入：

```json
{
  "thinking": {"type": "enabled"},
  "reasoning_effort": "high"
}
```

原因：

```text
1. 先用最小参数验证 API 能跑通；
2. 避免部分模型或兼容平台不支持这些扩展参数导致 400/422；
3. Bot 普通群聊回复不需要高推理模式；
4. 后续如果要支持思考模式，可通过配置开关扩展。
```

### 8.5.6 Go Client 解析目标

OpenAI-compatible 返回结果中至少解析：

```text
choices[0].message.content
usage.prompt_tokens
usage.completion_tokens
usage.total_tokens
```

如果 `usage` 缺失：

```text
PromptTokens = 0
CompletionTokens = 0
TotalTokens = 0
```

不要因为 usage 缺失导致 Bot 回复失败。

### 8.5.7 常见错误处理

需要对以下状态码给出可读错误：

```text
400: 请求体格式错误
401: API Key 错误或认证失败
402: 余额不足
422: 参数错误
429: 请求速率达到上限
500: 服务端内部故障
503: 服务繁忙
```

错误处理要求：

```text
1. 记录 status code；
2. 记录响应 body 的安全摘要；
3. 不打印 API Key；
4. LLM 调用失败时写 ai_call_logs FAILED；
5. 不影响用户原始消息发送成功。
```

### 8.5.8 Codex 写 .env 的要求

允许 Codex 创建：

```text
.env.example
```

不建议 Codex 创建带真实 Key 的 `.env`。

如果为了本地运行必须创建 `.env`，要求：

```text
1. LLM_API_KEY 使用占位符；
2. .env 必须加入 .gitignore；
3. output.md 中只能写“已创建 .env 模板”，不要记录真实 API Key；
4. 真实 API Key 由人工手动粘贴。
```

`.gitignore` 中应包含：

```gitignore
.env
.env.local
*.env
```

如果项目已有 `.gitignore`，只补缺失项，不要重写整个文件。


## 9. Bot 触发规则

第一版只支持简单前缀触发。

触发条件：

```text
sender_type == USER
message_type == TEXT
content trim 后以 "@AIM" 或 "@aim" 或 "@bot" 开头
```

不要让 Bot 回复再次触发 Bot。

伪代码：

```go
func ShouldTriggerBot(msg Message) bool {
    if msg.SenderType != SenderTypeUser {
        return false
    }
    if msg.MessageType != MessageTypeText {
        return false
    }

    content := strings.TrimSpace(msg.Content)
    lower := strings.ToLower(content)

    return strings.HasPrefix(lower, "@aim") || strings.HasPrefix(lower, "@bot")
}
```

用户问题需要去掉前缀：

```text
@AIM 总结一下刚才讨论了什么
→ 总结一下刚才讨论了什么
```

如果去掉前缀后为空，可以使用：

```text
请问你需要我帮你做什么？
```

---

## 10. Prompt 构造

第一版上下文只取当前会话最近 N 条消息。

建议：

```text
N = 20
```

不要把所有历史消息都塞给模型。

System Prompt：

```text
你是 AIM 群聊中的 AI 助手。请基于群聊上下文回答用户问题。
要求：
1. 回答简洁、准确。
2. 如果上下文不足，请直接说明不确定。
3. 不要编造群聊中没有的信息。
```

上下文格式：

```text
以下是最近的群聊消息：
[用户A]: xxx
[用户B]: xxx
[AIM]: xxx

当前用户问题：
xxx
```

LLM messages 可以这样构造：

```text
system: Bot system prompt
user: 最近群聊上下文 + 当前用户问题
```

第一版不需要把每条历史消息都映射成独立 role。

---

## 11. Bot 回复写入 Message

Bot 生成成功后，写入一条新消息。

要求：

```text
ConversationID = 当前会话内部 ID
SenderType = BOT
MessageType = BOT_REPLY
Content = LLM 返回文本
Status = NORMAL
```

SenderID 可以先使用：

```text
Bot.ID
```

由于现有 Message 的 SenderID 对 USER/BOT/SYSTEM 是多态含义，这符合当前设计。

写入成功后：

```text
更新 conversation.last_message_id
更新 conversation.last_message_at
写 ai_call_logs SUCCESS
```

如果 LLM 失败：

```text
写 ai_call_logs FAILED
不要影响用户原始消息
可选：写入一条 Bot 错误消息
```

---

## 12. AI 调用日志

调用前后需要记录日志。

推荐流程：

```text
开始处理 Bot 请求
→ 创建或准备 ai_call_log
→ 记录 start time
→ 调用 LLM
→ 成功：创建 Bot 回复消息，更新 log SUCCESS、token、latency、response_message_id
→ 失败：更新 log FAILED、latency、error_message
```

如果日志创建和 Bot 回复写入不能放在同一事务里，至少保证失败时有日志可查。

---

## 13. 第一阶段验收标准

完成后应该可以做到：

```text
1. 用户在群聊中发送：@AIM 你好
2. 用户消息立即正常发送、落库、显示
3. 后端异步调用 LLM
4. LLM 返回完整回复
5. 后端创建一条 Bot 消息
6. Bot 消息可以在历史消息中查到
7. ai_call_logs 有一条对应记录
8. LLM 失败时用户原始消息仍然成功
9. API Key 不出现在代码和日志中
```

如果广播已接通，还应做到：

```text
10. 在线成员可以通过 NEW_MESSAGE 实时收到 Bot 回复
```

---

## 14. 测试要求

至少补充以下测试：

```text
ShouldTriggerBot:
- 普通文本不触发
- @AIM 开头触发
- @aim 开头触发
- @bot 开头触发
- Bot 自己发的消息不触发

Prompt:
- 能去掉 @AIM 前缀
- 最近消息为空时也能构造 prompt
- 最近消息超过 N 时只取 N 条

LLM Client:
- 缺少环境变量时报错
- 非 2xx 状态码时报错
- choices 为空时报错

Async Bot:
- 用户消息创建成功后触发 Bot goroutine
- Bot 失败不影响 CreateMessage 返回
- Bot 回复不再次触发 Bot
```

如果业务层容易测，再补：

```text
Bot HandleMention:
- LLM 成功时创建 Bot 回复
- LLM 失败时写 FAILED 日志
```

不要求第一版做真实 API 集成测试，可以用 fake LLM client。

---

## 15. 不做的内容

第一阶段明确不做：

```text
流式回复
RAG
embedding
向量数据库
多 Bot 管理界面
Bot 配置 HTTP API
用户自带 API Key
复杂 mention 表
消息队列
独立 bot-service
修改 WebSocket 协议
修改 IDL
前端大改
```

---

## 16. 推荐 Codex 小任务拆分

### Task 1：模型和迁移

```text
Read only:
- AGENTS.md
- docs/specs/bot-spec.md
- chat-service/internal/dal/model/chat.go
- chat-service/internal/dal/mysql/init.go

Task:
Add Bot, ConversationBot, AICallLog GORM models and AutoMigrate only.

Do not modify:
- IDL
- gateway
- frontend
- docker-compose

Run:
- gofmt on changed files
- go test ./... in chat-service if cheap
```

### Task 2：LLM Client

```text
Read only:
- AGENTS.md
- docs/specs/bot-spec.md

Task:
Add chat-service/internal/llm OpenAI-compatible non-streaming client.

Do not connect it to chat business logic yet.
Do not modify IDL/gateway/frontend.

Add unit tests with httptest if reasonable.
```

### Task 3：Bot trigger + prompt

```text
Read only:
- AGENTS.md
- docs/specs/bot-spec.md
- chat-service/internal/dal/model/chat.go
- chat-service/internal/biz/chat.go

Task:
Add internal/bot trigger and prompt builder.
Do not call real LLM yet.
Add tests for trigger and prompt.
```

### Task 4：异步触发 Bot

```text
Read only:
- AGENTS.md
- docs/specs/bot-spec.md
- chat-service/internal/biz/chat.go
- chat-service/internal/dal/model/chat.go

Task:
After CreateMessage successfully creates a USER TEXT message, trigger Bot asynchronously when @AIM/@bot prefix matches.

Requirements:
- Do not block CreateMessage response.
- Do not reuse request ctx directly.
- Use context.WithTimeout(context.Background(), 30*time.Second).
- Recover panic inside goroutine.
- Bot failure must not make user message fail.
- Do not modify gateway, frontend, or IDL.
```

### Task 5：Bot service writes reply

```text
Read only:
- AGENTS.md
- docs/specs/bot-spec.md
- chat-service/internal/repository/chat.go
- chat-service/internal/biz/chat.go
- chat-service/internal/dal/model/chat.go

Task:
Implement Bot service HandleMention with fake/injected LLM client and create BOT_REPLY message.
Do not modify gateway or frontend.
```

### Task 6：AI call log

```text
Read only:
- AGENTS.md
- docs/specs/bot-spec.md
- chat-service/internal/repository/chat.go
- chat-service/internal/dal/model/chat.go

Task:
Record ai_call_logs for Bot requests.
Success logs token usage and response_message_id.
Failure logs error_message and latency.
```

### Task 7：广播问题单独评估

```text
Read only:
- AGENTS.md
- docs/specs/bot-spec.md
- gateway/internal/websocket/client.go
- gateway/internal/websocket/hub.go
- gateway/internal/websocket/event.go
- chat-service/internal/biz/chat.go

Task:
Inspect whether async Bot replies can be broadcast through the existing WebSocket NEW_MESSAGE path.
Do not edit files.
Output a short plan with the smallest possible change.
```

---

## 17. 后续 RAG 扩展预留

本 Spec 不实现 RAG，但 Bot 模块需要为 RAG 预留上下文来源。

后续 Prompt 上下文会从：

```text
最近 N 条群聊消息
```

扩展为：

```text
最近 N 条群聊消息
+
知识库检索片段
```

因此 Bot 服务内部建议设计为：

```go
type ContextProvider interface {
    BuildContext(ctx context.Context, req BuildContextRequest) (string, error)
}
```

第一版可以只有 `ConversationContextProvider`，后面再加 `RAGContextProvider`。
