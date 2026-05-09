# Task 08：Bot 管理接口评估

## 目标

只评估新增 HTTP/RPC 接口的最小改动，不改代码。

## 禁止

```text
不得修改文件。
不得生成代码。
不得重新生成 Kitex。
不得运行大范围重构。
```

## 只读文件

```text
docs/specs/tasks/p3-task-08-bot-api-review.md
idl/chat.thrift
gateway/internal/router/router.go
gateway/internal/handler/chat.go
chat-service/internal/handler/chat_service.go
chat-service/internal/biz/chat.go
chat-service/internal/repository/bot.go
chat-service/internal/repository/chat.go
```

## 必须评估的接口

```http
GET /api/v1/bots
GET /api/v1/conversations/{conversationId}/bots
POST /api/v1/conversations/{conversationId}/bots
DELETE /api/v1/conversations/{conversationId}/bots/{botId}
```

## 输出必须包含

```text
1. 需要新增哪些 RPC。
2. 需要新增哪些 HTTP 接口。
3. 需要新增哪些 DTO。
4. 权限校验放在哪一层。
5. 前端需要哪些字段。
6. 最小修改计划。
7. 是否需要重新生成 Kitex。
8. 预计修改文件列表。
9. 风险点。
```

## 权限边界

必须检查：

```text
GET /bots：必须登录。
GET /conversations/{id}/bots：当前用户必须是 conversation 的 USER 成员。
POST /conversations/{id}/bots：OWNER / ADMIN。
DELETE /conversations/{id}/bots/{botId}：OWNER / ADMIN。
```

## 输出格式

```text
Bot API Review Result

1. Required RPC Changes
2. Required Gateway Changes
3. Required Chat-Service Changes
4. Required DTOs
5. Permission Model
6. Files To Modify
7. Risks
8. Implementation Readiness: READY / NOT READY
```
