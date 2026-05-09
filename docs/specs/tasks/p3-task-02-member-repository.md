# Task 02：成员 repository 方法补齐

## 目标

补齐 USER/BOT 成员查询方法，不改业务逻辑。

## Preflight Check

必须确认 Task 01 已完成：

```text
ConversationMember 已支持 member_type/member_id。
ConversationMember 已移除 user_id。
```

如果未完成，必须停止实现。

## 只读文件

```text
docs/specs/tasks/p3-task-02-member-repository.md
chat-service/internal/repository/chat.go
chat-service/internal/dal/model/chat.go
```

## 允许修改

```text
chat-service/internal/repository/chat.go
chat-service/internal/repository/*member*.go
```

## 禁止修改

```text
IDL
gateway/**
frontend/**
业务逻辑
```

## 必须新增方法

### USER 成员

```go
GetUserMember(ctx, conversationID, userID uint64) (*ConversationMember, error)
IsUserMember(ctx, conversationID, userID uint64) (bool, error)
ListUserMembers(ctx, conversationID uint64) ([]ConversationMember, error)
ListUserMemberIDs(ctx, conversationID uint64) ([]uint64, error)
```

这些方法必须过滤：

```text
member_type = USER
status = NORMAL
```

### BOT 成员

```go
GetBotMember(ctx, conversationID, botID uint64) (*ConversationMember, error)
IsBotMember(ctx, conversationID, botID uint64) (bool, error)
ListBotMembers(ctx, conversationID uint64) ([]ConversationMember, error)
```

这些方法必须过滤：

```text
member_type = BOT
status = NORMAL
```

## 兼容要求

旧的按 user_id 语义查询方法不得继续作为业务入口使用。

如为了临时编译保留旧方法名，内部也必须改为 `member_type=USER AND member_id=?`。

不得在本任务迁移业务逻辑。

## 验收标准

```text
1. USER 成员查询和 BOT 成员查询方法分开。
2. WebSocket 收件人可通过 ListUserMemberIDs 获取。
3. Bot 触发可通过 GetBotMember / ListBotMembers 判断。
4. 不存在依赖 conversation_members.user_id 的 repository 查询。
```

## 验证

```text
gofmt
go test ./... in chat-service
go build ./... in chat-service
```
