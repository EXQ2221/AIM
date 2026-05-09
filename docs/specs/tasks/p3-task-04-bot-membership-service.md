# Task 04：Bot 加入/移除底层能力

## 目标

实现 Bot 成员化加入和移除的 service/repository 能力，不接 HTTP。

## Preflight Check

必须确认：

```text
Task 01 已完成。
Task 02 已完成。
事务工具可用。
bots 表和 conversation_bots 表已存在。
```

如果缺失，必须停止实现。

## 只读文件

```text
docs/specs/tasks/p3-task-04-bot-membership-service.md
chat-service/internal/dal/model/bot.go
chat-service/internal/dal/model/chat.go
chat-service/internal/repository/bot.go
chat-service/internal/repository/chat.go
chat-service/internal/repository/tx.go
chat-service/internal/bot/service.go
```

## 要求

添加 Bot 时，必须在同一数据库事务内完成：

```text
1. 写入或恢复 conversation_members BOT 成员：
   - member_type=BOT
   - member_id=botID
   - role=BOT
   - status=NORMAL

2. 写入或启用 conversation_bots：
   - bot_id=botID
   - enabled=true
   - permission_scope=CONVERSATION_ONLY
```

移除 Bot 时，必须在同一数据库事务内完成：

```text
1. conversation_members.status=REMOVED
2. conversation_bots.enabled=false
```

事务失败时，必须回滚全部操作。

## 禁止

```text
不得修改 IDL。
不得修改 gateway。
不得修改 frontend。
不得接 HTTP。
不得生成系统消息，除非已有系统消息能力且无需跨模块修改。
```

## 验收标准

```text
1. 添加 Bot 不会只写 conversation_members。
2. 添加 Bot 不会只写 conversation_bots。
3. 移除 Bot 不会只更新单表。
4. 事务失败不会留下单表成功状态。
```

## 验证

```text
gofmt
go test ./... in chat-service
go build ./... in chat-service
```
