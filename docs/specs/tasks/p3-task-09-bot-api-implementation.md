# Task 09：实现后端 Bot 管理接口

## 目标

实现 Bot 查询、添加、移除后端接口。

## Preflight Check

必须确认：

```text
Task 08 已完成且 Implementation Readiness 为 READY。
Task 04 的 Bot 加入/移除底层能力已完成。
```

## 只读文件

```text
docs/specs/tasks/p3-task-09-bot-api-implementation.md
idl/chat.thrift
gateway/internal/router/router.go
gateway/internal/handler/chat.go
chat-service/internal/handler/chat_service.go
chat-service/internal/biz/chat.go
chat-service/internal/repository/bot.go
chat-service/internal/repository/chat.go
```

## 必须实现的接口

```http
GET /api/v1/bots
GET /api/v1/conversations/{conversationId}/bots
POST /api/v1/conversations/{conversationId}/bots
DELETE /api/v1/conversations/{conversationId}/bots/{botId}
```

## 权限

```text
GET /api/v1/bots：必须登录。
GET /api/v1/conversations/{conversationId}/bots：当前用户必须是该 conversation 的 USER 成员。
POST：OWNER / ADMIN。
DELETE：OWNER / ADMIN。
```

以下字段只能由 OWNER / ADMIN 设置或修改：

```text
display_name_override
mention_name_override
aliases_override
permission_scope
```

P3 可以只在添加 Bot 时设置 override。

如果后续增加 `PATCH /conversations/{id}/bots/{botId}`，权限也必须是 OWNER / ADMIN。

## 字段校验

```text
mentionName / mentionNameOverride：
- 长度 2~32。
- 不包含 @。
- 大小写不敏感比较。
- 不得使用保留名 all、here、everyone、system。
- P3 阶段 bots.mention_name 对平台内置 Bot 保持全局唯一。
- future: 如果支持多租户 / 工作空间 / 用户自定义 Bot，再改为 tenant_id/workspace_id + mention_name 唯一。

aliases / aliasesOverride：
- API 使用 []string。
- 数据库存 JSON 文本。
- repository / mapper 层负责 JSON string <-> []string 转换。
- 不允许一处使用逗号字符串、一处使用数组的混合表达。
- 每个 alias 使用同样校验规则。
- bots.aliases 不做全局唯一索引。
- conversation_bots.mention_name_override / aliases_override 只要求当前 conversation 内不冲突。
- 当前 conversation 内不得与其他已启用 Bot 的 mentionName / aliases / override 冲突。
```

## 要求

```text
1. 添加 Bot 必须同时维护 conversation_members 和 conversation_bots。
2. 移除 Bot 必须同时维护 conversation_members 和 conversation_bots。
3. 普通成员只能查询，不能管理。
4. 必要时重新生成 Kitex。
5. 不做前端。
```

## 验证

```text
必要时重新生成 Kitex
gofmt
go test ./... in chat-service
go test ./... in gateway
go build ./... in chat-service
go build ./... in gateway
```
