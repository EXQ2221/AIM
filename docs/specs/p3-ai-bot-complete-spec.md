# AIM P3 AI Bot 完整补完 Spec

## 0. P3 总目标

P3 的目标不是只让 AI 能回复，而是把 AIM 的 AI Bot 做成一个完整、可维护、可扩展的群聊能力。

最终效果：

```text
群主/管理员可以把 Bot 拉进群聊
Bot 作为会话成员出现在群聊中
只有已加入群聊并启用的 Bot 才能被 @ 触发
Bot 使用 bots 表中的模型和 system prompt 配置
Bot 回复以 BOT_REPLY message 形式落库
Bot 回复更新会话最近消息
Bot 调用写入 ai_call_logs
Bot 回复通过 Redis Pub/Sub 通知 gateway
gateway 复用 NEW_MESSAGE 广播给在线用户
前端支持断线补偿、后台通知和 AI 助手管理入口
```

当前已有能力：

```text
内置 AIM Bot 可通过 @AIM / @aim / @bot 触发
异步调用 LLM，不阻塞用户消息
获取最近 20 条消息作为上下文
非流式生成回复
写入 BOT_REPLY message
写入 ai_call_logs
Redis Pub/Sub 通知 gateway
gateway 复用 NEW_MESSAGE 广播
前端断线补偿和后台通知
```

本 Spec 要补完的内容：

```text
1. Bot 成员化：Bot 加入群聊时也写入 conversation_members。
2. 会话级 Bot 绑定：conversation_bots 继续保存 Bot 在群里的配置。
3. Bot 触发校验：必须同时校验 BOT 成员、conversation_bots.enabled、bots.status。
4. Bot 配置读取：LLM 使用 bots.model_name 和 bots.system_prompt。
5. Bot 管理接口：查询 Bot、添加 Bot、移除 Bot。
6. 前端 AI 助手面板：在群聊详情中管理 Bot。
7. 事务一致性：Bot 回复落库和 conversation.last_message 更新保持一致。
8. 基础并发限制：避免无限 goroutine 和 API 费用失控。
9. 文档对齐：README/spec/output 记录准确状态。
```

---

## 1. 设计原则

### 1.1 三表职责分离

P3 最终保留三张表：

```text
bots
conversation_members
conversation_bots
```

职责：

```text
bots：
- Bot 自身配置
- 名称、头像、描述、模型、system prompt、状态、创建者

conversation_members：
- 谁在会话里
- USER 和 BOT 都可以是成员
- 用于成员列表、加入/移除、成员状态、WebSocket 收件人过滤
- 不保留旧 user_id，统一使用 member_type + member_id

conversation_bots：
- Bot 在某个会话内的 AI 配置
- enabled、permission_scope
- 会话内 display_name_override、mention_name_override、aliases_override
- 后续可扩展 model_override、system_prompt_override、knowledge_base_id
```

### 1.2 Bot 必须加入群聊后才能使用

Bot 默认不属于任何群聊。

只有执行“添加 Bot 到群聊”后，才会同时写入：

```text
conversation_members:
- member_type = BOT
- member_id = bot.id
- role = BOT
- status = NORMAL

conversation_bots:
- conversation_id = 当前会话内部 ID
- bot_id = bot.id
- enabled = true
- permission_scope = CONVERSATION_ONLY
```

之后群聊里发送：

```text
@AIM ...
@bot ...
```

才允许触发内置 AIM Bot。
后续多个 Bot 共存时，触发名必须来自 bots.mention_name / bots.aliases 或 conversation_bots 的 override 字段。

### 1.3 第一版不做 Bot 私聊

当前 P3 只支持群聊 Bot。

单聊逻辑只处理真实用户成员：

```text
member_type = USER
```

Bot 不参与好友关系，不参与用户单聊。

### 1.4 WebSocket 只发给 USER 成员

即使 Bot 也在 `conversation_members` 中，WebSocket 广播收件人只能是：

```text
member_type = USER
status = NORMAL
```

不能把 Bot 的 `member_id` 当成 user_id 推送。

### 1.5 Bot 回复不走普通用户发送权限

普通用户发消息需要校验：

```text
member_type = USER
member_id = sender_user_id
```

Bot 回复由 BotService 创建：

```text
sender_type = BOT
sender_id = bot.id
message_type = BOT_REPLY
```

不走用户鉴权，不走好友关系校验。

---

## 2. 数据模型设计

## 2.1 bots

```go
type BotStatus string

const (
    BotStatusEnabled  BotStatus = "ENABLED"
    BotStatusDisabled BotStatus = "DISABLED"
)

type Bot struct {
    ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
    Name         string    `gorm:"type:varchar(64);not null" json:"name"`
    MentionName  string    `gorm:"type:varchar(64);not null;uniqueIndex" json:"mentionName"`
    Aliases      string    `gorm:"type:text" json:"aliases"` // JSON array text
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

说明：

```text
bots 表是全局配置表。
一个 Bot 可以加入多个群聊。
mention_name 是不带 @ 的全局默认触发名，例如 AIM。
aliases 是全局别名列表，数据库存 JSON 文本，API/DTO 使用 []string，例如 ["aim", "bot"]。
触发匹配时需要大小写不敏感，并按 @mention_name / @alias 的完整 token 匹配。
不要在触发逻辑里长期硬编码 @AIM/@bot。
```

---

## 2.2 conversation_members

### 2.2.1 推荐最终模型

```go
type MemberType string

const (
    MemberTypeUser MemberType = "USER"
    MemberTypeBot  MemberType = "BOT"
)

type ConversationMemberRole string

const (
    MemberRoleOwner  ConversationMemberRole = "OWNER"
    MemberRoleAdmin  ConversationMemberRole = "ADMIN"
    MemberRoleMember ConversationMemberRole = "MEMBER"
    MemberRoleBot    ConversationMemberRole = "BOT"
)

type ConversationMemberStatus string

const (
    MemberStatusNormal  ConversationMemberStatus = "NORMAL"
    MemberStatusMuted   ConversationMemberStatus = "MUTED"
    MemberStatusRemoved ConversationMemberStatus = "REMOVED"
)

type ConversationMember struct {
    ID             uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
    ConversationID uint64 `gorm:"not null;index:idx_conversation_member_identity,unique" json:"conversationId"`

    MemberType MemberType `gorm:"type:varchar(32);not null;default:'USER';index:idx_conversation_member_identity,unique" json:"memberType"`
    MemberID   uint64     `gorm:"not null;index:idx_conversation_member_identity,unique" json:"memberId"`

    Role   ConversationMemberRole   `gorm:"type:varchar(32);not null;default:'MEMBER'" json:"role"`
    Status ConversationMemberStatus `gorm:"type:varchar(32);not null;default:'NORMAL'" json:"status"`

    NicknameInGroup   string     `gorm:"type:varchar(64)" json:"nicknameInGroup"`
    MuteUntil         *time.Time `json:"muteUntil"`
    IsPinned          bool       `gorm:"not null;default:false" json:"isPinned"`
    IsMuted           bool       `gorm:"not null;default:false" json:"isMuted"`
    LastReadMessageID *uint64    `gorm:"index" json:"lastReadMessageId"`

    JoinedAt  time.Time `gorm:"not null" json:"joinedAt"`
    CreatedAt time.Time `json:"createdAt"`
    UpdatedAt time.Time `json:"updatedAt"`
}
```

唯一索引：

```text
conversation_id + member_type + member_id
```

这样可以避免：

```text
用户 ID 和 Bot ID 数字相同导致冲突
同一个 Bot 重复加入同一个群聊
同一个用户重复加入同一个会话
```

### 2.2.2 开发阶段重建约定

当前处于开发阶段，数据库没有重要历史数据。

```text
允许清库重建。
不保留旧 conversation_members.user_id。
不做旧 user_id 回填。
不使用 user_id=0 或 user_id=null 兼容 BOT 成员。
```

新建 USER 成员时写：

```text
member_type = USER
member_id = userID
```

新建 BOT 成员时写：

```text
member_type = BOT
member_id = botID
```

---

## 2.3 conversation_bots

```go
type BotPermissionScope string

const (
    BotScopeConversationOnly  BotPermissionScope = "CONVERSATION_ONLY"
    BotScopeKnowledgeBaseOnly BotPermissionScope = "KNOWLEDGE_BASE_ONLY"
    BotScopeConversationAndKB BotPermissionScope = "CONVERSATION_AND_KB"
)

type ConversationBot struct {
    ID                  uint64             `gorm:"primaryKey;autoIncrement" json:"id"`
    ConversationID      uint64             `gorm:"not null;index:idx_conversation_bot,unique" json:"conversationId"`
    BotID               uint64             `gorm:"not null;index:idx_conversation_bot,unique" json:"botId"`
    Enabled             bool               `gorm:"not null;default:true" json:"enabled"`
    PermissionScope     BotPermissionScope `gorm:"type:varchar(64);not null;default:'CONVERSATION_ONLY'" json:"permissionScope"`
    DisplayNameOverride string             `gorm:"type:varchar(64)" json:"displayNameOverride"`
    MentionNameOverride string             `gorm:"type:varchar(64)" json:"mentionNameOverride"`
    AliasesOverride     string             `gorm:"type:text" json:"aliasesOverride"` // JSON array text
    CreatedAt           time.Time          `json:"createdAt"`
    UpdatedAt           time.Time          `json:"updatedAt"`
}
```

说明：

```text
conversation_bots 是会话级 Bot 配置表，不是成员表。
即使 Bot 已经在 conversation_members 中，也不能删除 conversation_bots。
display_name_override 只影响当前会话的展示名称。
mention_name_override / aliases_override 只影响当前会话的触发名。
override 为空时使用 bots 表中的全局 name / mention_name / aliases。
```

---

## 2.4 ai_call_logs

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
    RequestMessageID  *uint64      `gorm:"index" json:"requestMessageId"`
    ResponseMessageID *uint64      `gorm:"index" json:"responseMessageId"`
    ModelName         string       `gorm:"type:varchar(128);not null" json:"modelName"`
    PromptTokens      int          `gorm:"not null;default:0" json:"promptTokens"`
    CompletionTokens  int          `gorm:"not null;default:0" json:"completionTokens"`
    TotalTokens       int          `gorm:"not null;default:0" json:"totalTokens"`
    LatencyMS         int64        `gorm:"not null;default:0" json:"latencyMs"`
    Status            AICallStatus `gorm:"type:varchar(32);not null" json:"status"`
    ErrorMessage      string       `gorm:"type:text" json:"errorMessage"`
    CreatedAt         time.Time    `json:"createdAt"`
}
```

当前阶段不加 cost，只记录 token。

---

## 3. P3 完整业务流程

## 3.1 添加 Bot 到群聊

触发方式：

```text
群聊详情 → AI 助手 → 添加 Bot → 选择 AIM → 确认加入
```

后端行为：

```text
1. 根据对外 conversationId 查询内部 conversation。
2. 校验 conversation.type == GROUP。
3. 校验操作者是 OWNER 或 ADMIN。
4. 校验 bot 存在且 bots.status = ENABLED。
5. upsert conversation_members：
   - member_type = BOT
   - member_id = bot.id
   - role = BOT
   - status = NORMAL
6. upsert conversation_bots：
   - bot_id = bot.id
   - enabled = true
   - permission_scope = CONVERSATION_ONLY
7. 可选写 SYSTEM message：AIM 助手已加入群聊。
8. 如果写系统消息，复用 NEW_MESSAGE 广播。
```

## 3.2 移除 Bot

触发方式：

```text
群聊详情 → AI 助手 → 移除
```

后端行为：

```text
1. 根据对外 conversationId 查询内部 conversation。
2. 校验 conversation.type == GROUP。
3. 校验操作者是 OWNER 或 ADMIN。
4. 更新 conversation_members：
   - member_type = BOT
   - member_id = bot.id
   - status = REMOVED
5. 更新 conversation_bots：
   - enabled = false
6. 可选写 SYSTEM message：AIM 助手已移出群聊。
```

## 3.3 触发 Bot

用户发送：

```text
@AIM 总结一下
```

如果同一个会话里存在多个 Bot，触发名来自：

```text
1. conversation_bots.mention_name_override
2. bots.mention_name
3. conversation_bots.aliases_override
4. bots.aliases
```

其中 override 只在当前会话生效。

后端流程：

```text
1. 用户消息成功落库。
2. CreateMessage 返回成功，不等待 LLM。
3. 异步 Bot 任务启动。
4. 判断消息 sender_type=USER、message_type=TEXT。
5. 查询当前 conversation 中 NORMAL 的 BOT 成员。
6. 查询 conversation_bots.enabled=true。
7. 查询 bots.status=ENABLED。
8. 基于 mention_name / aliases 解析本次 @ 的目标 Bot。
9. 校验 permission_scope。
10. 查询最近 20 条消息。
11. 使用 bots.model_name 和 bots.system_prompt 构造 LLM 请求。
12. 非流式调用 LLM。
13. 创建 BOT_REPLY message。
14. 更新 conversation.last_message_id / last_message_at。
15. 写 ai_call_logs。
16. Redis Pub/Sub 发布 BotReplyCreated。
17. gateway 订阅后复用 NEW_MESSAGE 推送给 USER 成员。
```

---

## 4. Bot 触发校验规则

只有同时满足以下条件才触发：

```text
1. sender_type = USER
2. message_type = TEXT
3. content 命中当前会话已启用 Bot 的 mention_name 或 aliases
4. 当前会话存在 BOT 成员：
   - member_type = BOT
   - status = NORMAL
5. 当前会话存在 conversation_bots：
   - enabled = true
6. bots.status = ENABLED
7. permission_scope = CONVERSATION_ONLY
```

多 Bot 注意事项：

```text
1. @ 匹配大小写不敏感。
2. @ 匹配必须按完整 token，避免 @aim 命中 @aimer。
3. 如果 @bot 这类通用别名命中多个 Bot，不能随机选择，必须记录日志并跳过本次 Bot 触发。
4. 第一版可以让内置 AIM Bot 默认 mention_name=AIM、aliases=aim,bot。
```

当前阶段不支持：

```text
KNOWLEDGE_BASE_ONLY
CONVERSATION_AND_KB
```

如果遇到上述 scope：

```text
不触发 LLM
记录日志
不影响用户消息
```

---

## 5. Bot 配置来源

LLM 调用使用：

```text
bots.model_name
bots.system_prompt
```

环境变量只负责供应商配置：

```text
LLM_BASE_URL
LLM_API_KEY
LLM_TIMEOUT_SECONDS
```

可保留：

```text
LLM_MODEL
```

作为模型兜底。

优先级：

```text
1. bots.model_name
2. LLM_MODEL
3. 报错并记录 ai_call_logs FAILED
```

system prompt 优先级：

```text
1. bots.system_prompt
2. 默认 AIM 群聊助手 prompt
```

展示名和触发名优先级：

```text
展示名：
1. conversation_bots.display_name_override
2. bots.name

主触发名：
1. conversation_bots.mention_name_override
2. bots.mention_name

触发别名：
1. conversation_bots.aliases_override
2. bots.aliases
```

默认 prompt：

```text
你是 AIM 群聊中的 AI 助手。请基于群聊上下文回答用户问题。
要求：
1. 回答简洁、准确。
2. 如果上下文不足，请直接说明不确定。
3. 不要编造群聊中没有的信息。
```

---

## 6. 必须注意的业务边界

### 6.1 用户发送消息权限只看 USER

```text
member_type = USER
member_id = sender_user_id
status = NORMAL
```

不要用模糊的 `member_id` 查成员。

### 6.2 WebSocket 广播只发给 USER

```text
member_type = USER
status = NORMAL
```

Bot 不接收 WebSocket。

### 6.3 单聊只处理 USER

单聊创建、单聊发消息、好友关系校验，都只看：

```text
member_type = USER
```

不要让 Bot 进入单聊好友关系逻辑。

### 6.4 成员资料查询要分流

```text
USER → user-service 查询 nickname/avatar
BOT → bots 表查询 name/avatar/description
```

不要拿 bot_id 去 user-service 查。

### 6.5 Bot 回复不走用户发消息权限

Bot 回复走专用逻辑：

```text
sender_type=BOT
sender_id=bot.id
message_type=BOT_REPLY
```

---

## 7. 后端接口设计

## 7.1 查询可用 Bot

```http
GET /api/v1/bots
```

返回：

```json
[
  {
    "botId": 1,
    "name": "AIM",
    "mentionName": "AIM",
    "aliases": ["aim", "bot"],
    "avatar": "",
    "description": "群聊 AI 助手",
    "modelName": "deepseek-v4-flash",
    "status": "ENABLED"
  }
]
```

## 7.2 查询当前会话 Bot

```http
GET /api/v1/conversations/{conversationId}/bots
```

返回：

```json
[
  {
    "botId": 1,
    "name": "AIM",
    "displayName": "AIM",
    "mentionName": "AIM",
    "aliases": ["aim", "bot"],
    "displayNameOverride": "",
    "mentionNameOverride": "",
    "aliasesOverride": [],
    "avatar": "",
    "description": "群聊 AI 助手",
    "enabled": true,
    "permissionScope": "CONVERSATION_ONLY",
    "memberStatus": "NORMAL"
  }
]
```

## 7.3 添加 Bot 到群聊

```http
POST /api/v1/conversations/{conversationId}/bots
```

请求：

```json
{
  "botId": 1,
  "permissionScope": "CONVERSATION_ONLY",
  "displayNameOverride": "",
  "mentionNameOverride": "",
  "aliasesOverride": []
}
```

权限：

```text
OWNER / ADMIN
```

## 7.4 移除 Bot

```http
DELETE /api/v1/conversations/{conversationId}/bots/{botId}
```

权限：

```text
OWNER / ADMIN
```

---

## 8. 前端 AI 助手面板

群聊详情页增加：

```text
AI 助手
```

包含：

```text
当前已加入 Bot
可添加 Bot
移除 Bot
权限提示
```

普通成员：

```text
只读展示
不能添加/移除
```

OWNER / ADMIN：

```text
可添加 Bot
可移除 Bot
```

前端展示建议：

```text
群成员
- 用户 A
- 用户 B

AI 助手
- AIM Bot [AI助手]
```

或合并成员列表展示：

```text
AIM Bot [AI助手]
用户 A
用户 B
```

第一版推荐分开展示，避免前端成员逻辑复杂化。

---

## 9. 事务一致性

Bot 回复生成成功后，至少保证同一事务内：

```text
1. 创建 BOT_REPLY message
2. 更新 conversation.last_message_id / last_message_at
```

推荐同时更新：

```text
3. ai_call_logs SUCCESS
```

如果日志模块暂时不适合放入事务，至少保证 message 和 conversation 最近消息一致。

失败时：

```text
不留下半更新状态
记录 FAILED 日志
不影响用户原始消息
```

---

## 10. 基础并发限制

增加全局 Bot 任务并发限制：

```text
BOT_MAX_CONCURRENCY=10
```

默认：

```text
10
```

超限时：

```text
不调用 LLM
不创建 BOT_REPLY
不影响用户消息
记录日志
可选写 ai_call_logs FAILED
```

第一版不做：

```text
单用户限流
单会话限流
队列化
Redis Stream
```

---

## 11. 数据重建策略

当前处于开发阶段，数据库没有重要历史数据。

```text
1. 允许清库重建。
2. conversation_members 直接移除 user_id。
3. conversation_members 直接使用 member_type + member_id。
4. 唯一索引直接使用 conversation_id + member_type + member_id。
5. 不做旧 user_id 回填。
6. 不保留旧 conversation_id + user_id 唯一索引。
7. 不使用 user_id=0 或 user_id=null 兼容 BOT 成员。
```

模型相关任务必须同步更新：

```text
docs/specs/gorm-model-spec.md
output.md
```

`docs/specs/gorm-model-spec.md` 只作为模型演进记录和参考基线，不覆盖当前 task spec。

---

## 12. 推荐 repository 方法

### 12.1 用户成员

```go
GetUserMember(ctx, conversationID, userID uint64) (*ConversationMember, error)
IsUserMember(ctx, conversationID, userID uint64) (bool, error)
ListUserMembers(ctx, conversationID uint64) ([]ConversationMember, error)
ListUserMemberIDs(ctx, conversationID uint64) ([]uint64, error)
```

### 12.2 Bot 成员

```go
GetBotMember(ctx, conversationID, botID uint64) (*ConversationMember, error)
IsBotMember(ctx, conversationID, botID uint64) (bool, error)
ListBotMembers(ctx, conversationID uint64) ([]ConversationMember, error)
```

### 12.3 Bot 配置

```go
ListEnabledConversationBots(ctx, conversationID uint64) ([]ConversationBot, error)
GetEnabledConversationBot(ctx, conversationID, botID uint64) (*ConversationBot, error)
EnableConversationBot(ctx, conversationID, botID uint64, scope BotPermissionScope) error
DisableConversationBot(ctx, conversationID, botID uint64) error
GetBotByID(ctx, botID uint64) (*Bot, error)
ListEnabledBots(ctx) ([]Bot, error)
```

---

# 13. 细分任务

## Task 1：模型重建与 GORM 对齐

目标：

```text
让 conversation_members 直接使用 member_type/member_id，不保留旧 user_id。
```

只读：

```text
docs/specs/p3-ai-bot-complete-spec.md
docs/specs/gorm-model-spec.md
chat-service/internal/dal/model/chat.go
chat-service/internal/dal/mysql/init.go
```

允许修改：

```text
chat-service/internal/dal/model/chat.go
chat-service/internal/dal/mysql/init.go
docs/specs/gorm-model-spec.md
output.md
```

要求：

```text
1. 新增 MemberType 枚举 USER/BOT。
2. ConversationMember 新增 member_type/member_id。
3. ConversationMember 移除 user_id。
4. 唯一索引改为 conversation_id + member_type + member_id。
5. 不做旧数据回填，开发环境允许清库重建。
6. Bot 新增 mention_name / aliases。
7. ConversationBot 新增 display_name_override / mention_name_override / aliases_override。
8. docs/specs/gorm-model-spec.md 同步记录本次模型调整。
9. 不修改业务逻辑。
10. 不修改 IDL/gateway/frontend。
```

验证：

```text
gofmt
go test ./... in chat-service
go build ./... in chat-service
```

---

## Task 2：成员 repository 方法补齐

目标：

```text
补齐 USER/BOT 成员查询方法，暂不改业务逻辑。
```

只读：

```text
docs/specs/p3-ai-bot-complete-spec.md
chat-service/internal/repository/chat.go
chat-service/internal/dal/model/chat.go
```

允许修改：

```text
chat-service/internal/repository/chat.go
chat-service/internal/repository/*member*.go
```

要求：

```text
1. 新增 GetUserMember / IsUserMember。
2. 新增 ListUserMembers / ListUserMemberIDs。
3. 新增 GetBotMember / IsBotMember / ListBotMembers。
4. 如保留旧方法名，只能作为 USER 查询包装，内部必须使用 member_type=USER + member_id。
5. 不修改业务逻辑。
```

验证：

```text
gofmt
go test ./... in chat-service
go build ./... in chat-service
```

---

## Task 3：用户成员逻辑迁移

目标：

```text
所有真实用户相关逻辑明确使用 member_type=USER。
```

只读：

```text
docs/specs/p3-ai-bot-complete-spec.md
chat-service/internal/biz/chat.go
chat-service/internal/repository/chat.go
chat-service/internal/rpc/user_client.go
```

要求：

```text
1. 用户发送消息权限校验使用 GetUserMember。
2. WebSocket recipient 查询只返回 USER 成员 ID。
3. 单聊对端查找只看 USER 成员。
4. 好友关系校验只看 USER 成员。
5. 禁言逻辑只对 USER 成员生效。
6. 不再读取 conversation_members.user_id。
7. 不修改 IDL/gateway/frontend。
```

验证：

```text
gofmt
go test ./... in chat-service
go build ./... in chat-service
```

---

## Task 4：Bot 加入/移除底层能力

目标：

```text
实现 Bot 成员化加入和移除的 service/repository 能力，不接 HTTP。
```

只读：

```text
docs/specs/p3-ai-bot-complete-spec.md
chat-service/internal/dal/model/bot.go
chat-service/internal/dal/model/chat.go
chat-service/internal/repository/bot.go
chat-service/internal/repository/chat.go
chat-service/internal/bot/service.go
```

要求：

```text
1. 添加 Bot 时写入或恢复 conversation_members BOT 成员。
2. 添加 Bot 时写入或启用 conversation_bots。
3. 移除 Bot 时 conversation_members.status=REMOVED。
4. 移除 Bot 时 conversation_bots.enabled=false。
5. 当前不接 HTTP，不改 IDL/gateway/frontend。
```

验证：

```text
gofmt
go test ./... in chat-service
go build ./... in chat-service
```

---

## Task 5：Bot 触发双校验

目标：

```text
Bot 触发时必须校验 BOT 成员 + conversation_bots + bots.status。
```

只读：

```text
docs/specs/p3-ai-bot-complete-spec.md
chat-service/internal/bot/service.go
chat-service/internal/bot/trigger.go
chat-service/internal/repository/bot.go
chat-service/internal/repository/chat.go
chat-service/internal/dal/model/bot.go
```

要求：

```text
1. 查询 conversation_members：
   - member_type=BOT
   - status=NORMAL
2. 查询 conversation_bots：
   - enabled=true
3. 查询 bots.status=ENABLED。
4. 使用 mention_name / aliases 解析被 @ 的目标 Bot。
5. conversation_bots 的 mention_name_override / aliases_override 优先于 bots 默认值。
6. 如果一个 @ 别名命中多个 Bot，记录日志并跳过，不随机选择。
7. 使用 bots.model_name。
8. 使用 bots.system_prompt，若为空则默认 prompt。
9. 当前只支持 permission_scope=CONVERSATION_ONLY。
10. 未加入或已移除 Bot 时不触发 LLM。
11. 不修改 IDL/gateway/frontend。
```

验证：

```text
gofmt
go test ./... in chat-service
go build ./... in chat-service
```

---

## Task 6：Bot 回复事务一致性

目标：

```text
Bot 回复 message 创建和 conversation.last_message 更新事务化。
```

只读：

```text
docs/specs/p3-ai-bot-complete-spec.md
chat-service/internal/bot/service.go
chat-service/internal/repository/chat.go
chat-service/internal/repository/tx.go
chat-service/internal/dal/model/chat.go
chat-service/internal/dal/model/bot.go
```

要求：

```text
1. LLM 成功后，事务内创建 BOT_REPLY message。
2. 同一事务内更新 conversation.last_message_id 和 last_message_at。
3. 如果事务失败，不留下 last_message 半更新状态。
4. 尽量把 ai_call_logs SUCCESS 更新也放入事务；如果不方便，至少保证 message 和 conversation 一致。
5. 不修改 IDL/gateway/frontend。
```

验证：

```text
gofmt
go test ./... in chat-service
go build ./... in chat-service
```

---

## Task 7：基础并发限制

目标：

```text
限制 Bot 异步任务并发。
```

只读：

```text
docs/specs/p3-ai-bot-complete-spec.md
chat-service/internal/biz/chat.go
chat-service/internal/bot/service.go
chat-service/cmd/server/main.go
```

要求：

```text
1. 支持 BOT_MAX_CONCURRENCY 环境变量。
2. 默认最大并发为 10。
3. 超限时不调用 LLM。
4. 超限时不影响用户消息。
5. 超限时记录日志。
6. 不新增队列，不做 Redis Stream。
7. 不修改 IDL/gateway/frontend。
```

验证：

```text
gofmt
go test ./... in chat-service
go build ./... in chat-service
```

---

## Task 8：Bot 管理接口评估

目标：

```text
评估新增 HTTP/RPC 接口的最小改动，不改代码。
```

只读：

```text
docs/specs/p3-ai-bot-complete-spec.md
idl/chat.thrift
gateway/internal/router/router.go
gateway/internal/handler/chat.go
chat-service/internal/handler/chat_service.go
chat-service/internal/biz/chat.go
chat-service/internal/repository/bot.go
chat-service/internal/repository/chat.go
```

要求输出：

```text
1. 需要新增哪些 RPC。
2. 需要新增哪些 HTTP 接口。
3. 需要新增哪些 DTO。
4. 权限校验放在哪一层。
5. 前端需要哪些字段。
6. 最小修改计划。
7. 是否需要重新生成 Kitex。
```

禁止：

```text
不要修改文件
不要生成代码
```

---

## Task 9：实现后端 Bot 管理接口

目标：

```text
实现查询/添加/移除 Bot 的后端接口。
```

只读：

```text
docs/specs/p3-ai-bot-complete-spec.md
idl/chat.thrift
gateway/internal/router/router.go
gateway/internal/handler/chat.go
chat-service/internal/handler/chat_service.go
chat-service/internal/biz/chat.go
chat-service/internal/repository/bot.go
chat-service/internal/repository/chat.go
```

要求：

```text
1. GET /api/v1/bots
2. GET /api/v1/conversations/{conversationId}/bots
3. POST /api/v1/conversations/{conversationId}/bots
4. DELETE /api/v1/conversations/{conversationId}/bots/{botId}
5. OWNER/ADMIN 才能添加和移除。
6. 添加 Bot 同时维护 conversation_members 和 conversation_bots。
7. 移除 Bot 同时维护 conversation_members 和 conversation_bots。
8. 普通成员只能查询，不能管理。
9. Bot 列表返回 mentionName / aliases。
10. 会话 Bot 列表返回 displayNameOverride / mentionNameOverride / aliasesOverride。
11. 添加 Bot 时允许传入 displayNameOverride / mentionNameOverride / aliasesOverride，可为空。
12. 不做前端。
```

验证：

```text
必要时重新生成 Kitex
gofmt
go test ./... in chat-service
go test ./... in gateway
go build ./... in chat-service
go build ./... in gateway
```

---

## Task 10：前端 AI 助手面板

目标：

```text
在群聊详情中提供 Bot 管理 UI。
```

只读：

```text
docs/specs/p3-ai-bot-complete-spec.md
frontend/src/App.tsx
frontend/src/api.ts
frontend/src/types.ts
frontend/src/styles.css
```

要求：

```text
1. 查询可用 Bot。
2. 查询当前群已加入 Bot。
3. OWNER/ADMIN 可以添加 Bot。
4. OWNER/ADMIN 可以移除 Bot。
5. 普通成员只读展示。
6. 添加后刷新 Bot 列表。
7. 移除后刷新 Bot 列表。
8. 展示 conversation_bots.display_name_override，空值时展示 bots.name。
9. 展示 Bot 当前触发名和别名，方便用户知道应该 @ 谁。
10. 不改 WebSocket 协议。
```

验证：

```text
npm run build --prefix frontend
```

---

## Task 11：成员列表展示 Bot

目标：

```text
让群成员列表能展示 USER 和 BOT。
```

只读：

```text
docs/specs/p3-ai-bot-complete-spec.md
idl/chat.thrift
chat-service/internal/biz/chat.go
chat-service/internal/handler/chat_service.go
gateway/internal/handler/chat.go
frontend/src/App.tsx
frontend/src/types.ts
```

要求：

```text
1. 成员返回结构支持 memberType/memberId。
2. USER 成员补 user-service 昵称头像。
3. BOT 成员补 bots 表名称头像。
4. 前端展示 Bot 为 AI 助手。
5. 不影响 USER 成员显示。
6. 不影响 WebSocket 广播收件人过滤。
```

验证：

```text
必要时重新生成 Kitex
go test ./... in chat-service
go test ./... in gateway
npm run build --prefix frontend
```

---

## Task 12：文档对齐

目标：

```text
把 P3 当前完成度和设计边界写清楚。
```

只读：

```text
README.md
docs/specs/bot-spec.md
docs/specs/gorm-model-spec.md
docs/specs/ws-notification-spec.md
docs/specs/p3-ai-bot-complete-spec.md
output.md
```

要求：

```text
1. 明确 P3 AI Bot 已完成闭环。
2. 明确 Bot 成员化方案。
3. 明确 bots/conversation_members/conversation_bots 三表职责。
4. 明确 WebSocket 只发给 USER 成员。
5. 明确当前不做 RAG。
6. 明确当前不做 Bot 私聊。
7. 明确当前不做用户自带 API Key。
```

禁止：

```text
不要修改代码
```

---

## 14. 推荐执行顺序

最稳顺序：

```text
Task 1：模型重建与 GORM 对齐
Task 2：成员 repository 方法补齐
Task 3：用户成员逻辑迁移
Task 4：Bot 加入/移除底层能力
Task 5：Bot 触发双校验
Task 6：Bot 回复事务一致性
Task 7：基础并发限制
Task 8：Bot 管理接口评估
Task 9：实现后端 Bot 管理接口
Task 10：前端 AI 助手面板
Task 11：成员列表展示 Bot
Task 12：文档对齐
```

省额度建议：

```text
每次只做 1 个 Task。
Task 8 只读评估，可以和 Task 9 分开。
涉及 IDL / gateway / frontend 的任务必须单独做。
```

---

## 15. 验收标准

### 15.1 数据层

```text
1. conversation_members 支持 USER/BOT。
2. conversation_members 不包含 user_id。
3. Bot 加入群聊后有 BOT 成员记录。
4. Bot 加入群聊后有 conversation_bots enabled=true。
5. Bot 移除后 BOT 成员 status=REMOVED。
6. Bot 移除后 conversation_bots enabled=false。
```

### 15.2 触发层

```text
1. 未加入 Bot 的群聊中 @AIM 不触发 LLM。
2. 已加入 Bot 的群聊中 @AIM 触发 LLM。
3. 移除 Bot 后 @AIM 不再触发。
4. bots.status=DISABLED 不触发。
5. conversation_bots.enabled=false 不触发。
6. permission_scope 非 CONVERSATION_ONLY 当前不触发。
```

### 15.3 消息层

```text
1. 用户消息正常发送不被 Bot 阻塞。
2. Bot 回复写入 message 表。
3. Bot 回复 sender_type=BOT。
4. Bot 回复 message_type=BOT_REPLY。
5. conversation.last_message 正确更新。
6. ai_call_logs 成功/失败记录正确。
```

### 15.4 实时层

```text
1. Bot 回复 Redis Pub/Sub 发布成功。
2. gateway 复用 NEW_MESSAGE 推送。
3. WebSocket 收件人只包含 USER 成员。
4. 断线后能通过历史消息补齐。
```

### 15.5 前端层

```text
1. 群聊详情显示 AI 助手面板。
2. OWNER/ADMIN 可以添加 Bot。
3. OWNER/ADMIN 可以移除 Bot。
4. 普通成员只读。
5. 成员列表能展示 Bot 或 AI 助手区域。
```

---

## 16. 不做的内容

P3 不做：

```text
RAG
embedding
知识库检索
Bot 私聊
Bot 好友关系
用户自定义 Bot
用户自带 API Key
多 Bot 精确名称解析
Bot 商店
复杂计费
单用户限流
单会话限流
队列化 Bot 任务
Redis Stream
PWA Push
原生 App 推送
```
