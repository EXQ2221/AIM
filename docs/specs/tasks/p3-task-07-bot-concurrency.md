# Task 07：Bot 并发控制

## 目标

限制 Bot 异步任务并发，允许同一个 Bot 在不同会话中并发响应。

## Preflight Check

必须确认：

```text
Bot 异步触发逻辑已存在。
Bot 任务使用 context.WithTimeout 或等价超时机制。
goroutine 内已有 recover 或可在本任务补齐。
```

## 只读文件

```text
docs/specs/tasks/p3-task-07-bot-concurrency.md
chat-service/internal/biz/chat.go
chat-service/internal/bot/service.go
chat-service/cmd/server/main.go
```

## 配置项

必须支持：

```env
BOT_MAX_CONCURRENCY=10
BOT_MAX_CONVERSATION_CONCURRENCY=1
BOT_TASK_TIMEOUT_SECONDS=30
```

默认值：

```text
BOT_MAX_CONCURRENCY = 10
BOT_MAX_CONVERSATION_CONCURRENCY = 1
BOT_TASK_TIMEOUT_SECONDS = 30
```

## 规则

```text
1. 不得对 bot_id 加全局互斥锁。
2. 同一个 Bot 可以在不同 conversation 中并发响应。
3. 同一个 conversation 超过并发限制时，不调用 LLM。
4. 全局超过并发限制时，不调用 LLM。
5. 超限不得影响用户原始消息发送成功。
6. 超限必须记录日志。
7. 超限必须写 ai_call_logs FAILED。
8. Bot 任务必须有超时。
9. conversation 超限时不得创建 BOT_REPLY。
10. 全局超限时不得创建 BOT_REPLY。
11. conversation 超限时 error_message 必须标明 conversation concurrency limit reached。
12. 全局超限时 error_message 必须标明 global concurrency limit reached。
13. P3 不要求给用户发送“AI 助手繁忙”的 Bot 回复。
```

## 禁止

```text
不得新增 Redis Stream。
不得新增任务队列。
不得实现失败重试。
不得实现死信队列。
不得修改 IDL/gateway/frontend。
```

## 验收标准

```text
1. 同一个 Bot 可在不同 conversation 中同时处理请求。
2. 不存在 bot_id 级全局锁。
3. 全局超限不调用 LLM。
4. 会话超限不调用 LLM。
5. 超限不影响用户消息。
6. 超限写 ai_call_logs FAILED。
7. 超限不创建 BOT_REPLY。
8. P3 不引入 Redis Stream。
```

## 验证

```text
gofmt
go test ./... in chat-service
go build ./... in chat-service
```
