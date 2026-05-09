# Task 06：Bot 回复事务一致性

## 目标

Bot 回复 message 创建和 conversation.last_message 更新必须事务化。

## Preflight Check

必须确认：

```text
Task 05 已完成。
Bot 回复已经可以创建 BOT_REPLY message。
conversation last_message 更新方法存在。
事务工具可用。
```

如果缺失，必须停止实现。

## 只读文件

```text
docs/specs/tasks/p3-task-06-bot-reply-transaction.md
chat-service/internal/bot/service.go
chat-service/internal/repository/chat.go
chat-service/internal/repository/tx.go
chat-service/internal/dal/model/chat.go
chat-service/internal/dal/model/bot.go
```

## 要求

LLM 成功后，必须在同一数据库事务内完成：

```text
1. 创建 BOT_REPLY message。
2. 更新 conversation.last_message_id。
3. 更新 conversation.last_message_at。
```

如果实现上允许，必须尽量将：

```text
ai_call_logs SUCCESS
```

也放入同一事务。

事务失败时，不得留下 conversation.last_message 与 message 表不一致的半更新状态。

## 禁止

```text
不得修改 IDL。
不得修改 gateway。
不得修改 frontend。
不得修改 WebSocket 协议。
```

## 验收标准

```text
1. BOT_REPLY 创建和 conversation.last_message 更新同事务。
2. 事务失败时不会只创建 message。
3. 事务失败时不会只更新 last_message。
4. ai_call_logs 成功记录能关联 response_message_id。
```

## 验证

```text
gofmt
go test ./... in chat-service
go build ./... in chat-service
```
