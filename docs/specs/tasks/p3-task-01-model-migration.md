# Task 01：模型重建与 GORM 对齐

## 目标

让 `conversation_members` 直接使用 `member_type/member_id`，不保留旧 `user_id`。

当前处于开发阶段，数据库没有重要历史数据。本任务按清库重建思路设计，不做旧数据兼容迁移和回填。

同时补齐 Bot 触发名字段：

```text
bots.mention_name
bots.aliases
conversation_bots.display_name_override
conversation_bots.mention_name_override
conversation_bots.aliases_override
```

## Preflight Check

实现前必须确认：

```text
1. 已确认当前环境允许清库重建。
2. 当前 ConversationMember 仍可能存在 user_id，但本任务必须移除。
3. 当前旧唯一索引 conversation_id + user_id 不再保留。
4. docs/specs/gorm-model-spec.md 将同步记录本次模型调整。
```

如果无法确认可以清库重建，必须停止实现并输出缺失项。

## 只读文件

```text
docs/specs/tasks/p3-task-01-model-migration.md
docs/specs/gorm-model-spec.md
chat-service/internal/dal/model/chat.go
chat-service/internal/dal/model/bot.go
chat-service/internal/dal/mysql/init.go
```

## 允许修改

```text
chat-service/internal/dal/model/chat.go
chat-service/internal/dal/model/bot.go
chat-service/internal/dal/mysql/init.go
docs/specs/gorm-model-spec.md
output.md
```

## 禁止修改

```text
IDL
gateway/**
frontend/**
auth-service/**
user-service/**
业务逻辑
```

## 数据模型要求

### MemberType

必须新增：

```go
type MemberType string

const (
    MemberTypeUser MemberType = "USER"
    MemberTypeBot  MemberType = "BOT"
)
```

### ConversationMember

必须新增：

```text
member_type
member_id
```

必须删除：

```text
user_id
```

最终唯一索引必须为：

```text
conversation_id + member_type + member_id
```

新建 USER 成员时必须写：

```text
member_type = USER
member_id = userID
```

新建 BOT 成员时必须写：

```text
member_type = BOT
member_id = botID
```

禁止用 `user_id=0` 或 `user_id=null` 兼容 BOT 成员。

### Bot

必须新增：

```go
MentionName string `gorm:"type:varchar(64);not null;uniqueIndex" json:"mentionName"`
Aliases     string `gorm:"type:text" json:"aliases"` // JSON array text
```

aliases 数据库必须是 JSON 文本，API/DTO 后续必须是 `[]string`。

repository / mapper 层必须负责 JSON string <-> []string 转换。

不允许一处使用逗号字符串、一处使用数组的混合表达。

P3 阶段 `bots.mention_name` 对平台内置 Bot 保持全局唯一。

`bots.aliases` 不做全局唯一索引。

### ConversationBot

必须新增：

```go
DisplayNameOverride string `gorm:"type:varchar(64)" json:"displayNameOverride"`
MentionNameOverride string `gorm:"type:varchar(64)" json:"mentionNameOverride"`
AliasesOverride     string `gorm:"type:text" json:"aliasesOverride"` // JSON array text
```

`conversation_bots.mention_name_override` / `aliases_override` 只要求当前 conversation 内不冲突。

## 清库重建策略

当前开发阶段不保留旧数据，允许清库重建。

必须按以下原则：

```text
1. GORM 模型直接移除 user_id。
2. GORM 模型直接新增 member_type/member_id。
3. 唯一索引直接使用 conversation_id + member_type + member_id。
4. 开发环境可删除旧表或清库后重新 AutoMigrate。
5. 不写旧 user_id 回填逻辑。
6. 不保留旧 conversation_id + user_id 唯一索引。
7. docs/specs/gorm-model-spec.md 必须同步写明本次模型调整。
```

## 验收标准

```text
1. conversation_members 支持 member_type/member_id。
2. conversation_members 不再包含 user_id。
3. bots 支持 mention_name/aliases。
4. conversation_bots 支持 display/mention/aliases override。
5. aliases 和 aliases_override 使用 JSON 文本。
6. docs/specs/gorm-model-spec.md 已同步记录模型调整。
7. output.md 已追加执行记录。
8. 不修改业务逻辑。
```

## 验证

```text
gofmt
go test ./... in chat-service
go build ./... in chat-service
```
