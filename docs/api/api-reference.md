# AIM API 接口文档（Gateway）

最后更新：2026-06-04  
适用服务：`gateway`  
基础路径：`/api/v1`

> 说明：`/healthz`、`/metrics`、`/ws/chat` 不在 `/api/v1` 下。

## 1. 通用约定

### 1.1 鉴权

- 受保护接口默认需要登录态。
- HTTP 接口支持两种 Access Token 读取方式：
  - Cookie：`access_token`
  - Header：`Authorization: Bearer <token>`
- WebSocket 额外支持查询参数：
  - `GET /ws/chat?token=<token>`

### 1.2 统一响应格式

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

- `code = 0` 表示成功。
- 非 0 表示失败，`message` 为可读错误信息。

### 1.3 常见错误

- `400` 参数错误
- `401` 未认证或 token 失效
- `403` 无权限
- `404` 资源不存在
- `409` 资源冲突
- `500` 服务内部错误
- 时间字段和时间范围参数默认使用 Unix 秒级时间戳（`int64`），除非接口另有说明。

## 2. 公共接口

- `GET /healthz`：健康检查
- `GET /metrics`：Prometheus 指标
- `GET /ws/chat`：WebSocket 长连接

WebSocket 鉴权可通过以下任一方式传入 token：

- `?token=<token>`
- Cookie：`access_token`
- Header：`Authorization: Bearer <token>`

主要用途：

- 实时消息推送
- Bot 回复推送
- 好友在线状态推送

## 3. Auth

前缀：`/api/v1/auth`

### 3.1 路由

- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/refresh`
- `POST /api/v1/auth/logout`
- `POST /api/v1/auth/logout-all`
- `GET /api/v1/auth/sessions`
- `POST /api/v1/auth/sessions/revoke`

### 3.2 请求示例

`POST /api/v1/auth/register`

```json
{
  "aim_id": "xqe_0422",
  "email": "user@example.com",
  "nickname": "小青",
  "password": "******"
}
```

`POST /api/v1/auth/login`

```json
{
  "email": "user@example.com",
  "password": "******",
  "device_name": "web-client"
}
```

`POST /api/v1/auth/refresh`

```json
{
  "refresh_token": "optional_when_cookie_exists"
}
```

`POST /api/v1/auth/logout-all`

```json
{
  "password": "******"
}
```

`POST /api/v1/auth/sessions/revoke`

```json
{
  "session_id": "session-id",
  "password": "******"
}
```

## 4. Users

前缀：`/api/v1/users`

### 4.1 路由

- `GET /api/v1/users/me`
- `POST /api/v1/users/me/avatar`
- `GET /api/v1/users/memory`
- `POST /api/v1/users/memory`
- `PUT /api/v1/users/memory/:memoryId`
- `GET /api/v1/users/memory/settings`
- `PUT /api/v1/users/memory/settings`

### 4.2 请求示例

`POST /api/v1/users/memory`

```json
{
  "content": "最多 512 字的用户长期记忆"
}
```

`PUT /api/v1/users/memory/:memoryId`

```json
{
  "content": "更新后的记忆内容"
}
```

`PUT /api/v1/users/memory/settings`

```json
{
  "enabled": true,
  "scope": "SELECTED_GROUPS",
  "conversationIds": ["c_1001", "c_1002"]
}
```

### 4.3 说明

- `GET /api/v1/users/memory` 支持查询参数 `limit`。
- `GET /api/v1/users/memory/settings` 返回：

```json
{
  "enabled": true,
  "scope": "ALL_GROUPS",
  "conversationIds": [],
  "updatedAt": 1710000000
}
```

- `scope` 当前支持：
  - `ALL_GROUPS`
  - `SELECTED_GROUPS`
- `conversationIds` 会被去重、裁剪空白，并过滤掉不合法值。

## 5. Uploads

前缀：`/api/v1/uploads`

### 5.1 路由

- `POST /api/v1/uploads/images`
- `POST /api/v1/uploads/files`
- `POST /api/v1/uploads/voices`

### 5.2 说明

- 请求类型：`multipart/form-data`
- 文件字段名统一为 `file`

## 6. Friends

前缀：`/api/v1/friends`

### 6.1 路由

- `GET /api/v1/friends`
- `POST /api/v1/friends`
- `GET /api/v1/friends/requests`
- `POST /api/v1/friends/requests/:requestId/respond`
- `PATCH /api/v1/friends/:friendUserId`
- `DELETE /api/v1/friends/:friendUserId`
- `GET /api/v1/friends/groups`
- `POST /api/v1/friends/groups`
- `GET /api/v1/friends/presence/settings`
- `PUT /api/v1/friends/presence/settings`

### 6.2 请求示例

`POST /api/v1/friends`

```json
{
  "target_aim_id": "target_aim_id",
  "remark": "同事",
  "group_id": 1
}
```

`POST /api/v1/friends/requests/:requestId/respond`

```json
{
  "action": "ACCEPTED"
}
```

`PATCH /api/v1/friends/:friendUserId`

```json
{
  "remark": "新的备注",
  "group_id": 2
}
```

`PUT /api/v1/friends/presence/settings`

```json
{
  "invisible": true
}
```

### 6.3 说明

- `action` 支持：
  - `ACCEPTED`
  - `REJECTED`

## 7. Conversations

前缀：`/api/v1/conversations`

### 7.1 会话与群聊

- `POST /api/v1/conversations/group`
- `GET /api/v1/conversations`
- `GET /api/v1/conversations/single?targetUserId=123`
- `GET /api/v1/conversations/:conversationId/group`
- `POST /api/v1/conversations/:conversationId/members`
- `GET /api/v1/conversations/:conversationId/join-requests`
- `POST /api/v1/conversations/:conversationId/join-requests/:requestId/review`
- `POST /api/v1/conversations/:conversationId/members/invite`
- `DELETE /api/v1/conversations/:conversationId/members/me`
- `DELETE /api/v1/conversations/:conversationId/members/:targetUserId`
- `GET /api/v1/conversations/:conversationId/members`
- `POST /api/v1/conversations/:conversationId/owner/transfer`
- `POST /api/v1/conversations/:conversationId/admins`
- `DELETE /api/v1/conversations/:conversationId/admins/:targetUserId`
- `POST /api/v1/conversations/:conversationId/mute-all`
- `DELETE /api/v1/conversations/:conversationId/mute-all`
- `POST /api/v1/conversations/:conversationId/members/:targetUserId/mute`
- `DELETE /api/v1/conversations/:conversationId/members/:targetUserId/mute`
- `PUT /api/v1/conversations/:conversationId/announcement`
- `PUT /api/v1/conversations/:conversationId/avatar`
- `DELETE /api/v1/conversations/:conversationId/group`

### 7.2 消息与历史

- `GET /api/v1/conversations/:conversationId/messages?beforeId=999&limit=30`
- `POST /api/v1/conversations/:conversationId/read`
- `POST /api/v1/conversations/:conversationId/messages/:messageId/recall`
- `GET /api/v1/conversations/history/search`
- `POST /api/v1/conversations/:conversationId/summary`

### 7.3 Bot 与知识库绑定

- `GET /api/v1/conversations/:conversationId/bots`
- `POST /api/v1/conversations/:conversationId/bots`
- `DELETE /api/v1/conversations/:conversationId/bots/:botId`
- `POST /api/v1/conversations/:conversationId/knowledge-bases`
- `GET /api/v1/conversations/:conversationId/knowledge-bases`
- `DELETE /api/v1/conversations/:conversationId/knowledge-bases/:knowledgeBaseId`
- `GET /api/v1/conversations/:conversationId/ai-call-logs`

### 7.4 请求示例

`POST /api/v1/conversations/group`

```json
{
  "name": "项目讨论群",
  "avatar": "",
  "announcement": "欢迎加入",
  "joinPolicy": "INVITE_ONLY"
}
```

`POST /api/v1/conversations/:conversationId/join-requests/:requestId/review`

```json
{
  "action": "APPROVE"
}
```

`PUT /api/v1/conversations/:conversationId/avatar`

```json
{
  "avatar": "https://cdn.example.com/group-avatar.png"
}
```

`POST /api/v1/conversations/:conversationId/read`

```json
{
  "lastReadMessageId": 123456
}
```

`POST /api/v1/conversations/:conversationId/summary`

```json
{
  "messageCount": 100
}
```

`POST /api/v1/conversations/:conversationId/bots`

```json
{
  "botId": 100001,
  "displayNameOverride": "群内显示名",
  "mentionNameOverride": "触发名",
  "aliasesOverride": ["别名1", "别名2"],
  "permissionScope": "CONVERSATION_AND_KB",
  "modelNameOverride": "deepseek-v4-flash"
}
```

`permissionScope` 常用值：

- `CONVERSATION_ONLY`
- `KB_ONLY`
- `CONVERSATION_AND_KB`

`POST /api/v1/conversations/:conversationId/knowledge-bases`

```json
{
  "knowledgeBaseId": 900001
}
```

### 7.5 说明

- `GET /api/v1/conversations/:conversationId/join-requests` 支持查询参数 `limit`，默认 `50`。
- `POST /api/v1/conversations/:conversationId/join-requests/:requestId/review` 的 `action` 支持：
  - `APPROVE`
  - `REJECT`
- `GET /api/v1/conversations/:conversationId/messages` 支持查询参数 `beforeId`、`limit`，`limit` 默认 `30`。
- `GET /api/v1/conversations/:conversationId/ai-call-logs` 支持查询参数：
  - `beforeId`
  - `limit`
  - `botId`
  - `status`
- 其中 `limit` 默认 `30`。
- 响应包含：
  - `logs`
  - `quota`
- `GET /api/v1/conversations/history/search` 支持查询参数：
  - `startAt`：必填，Unix 秒级时间戳
  - `endAt`：必填，Unix 秒级时间戳
  - `conversationId`：可选
  - `keyword`：可选
  - `conversationType`：可选，支持 `ALL`、`GROUP`、`SINGLE`
- 如果传了 `conversationId`，会优先限定在该会话内搜索。
- `POST /api/v1/conversations/:conversationId/summary` 仅支持群聊，`messageCount` 范围为 `20 ~ 500`，默认 `100`。

## 8. Bots

前缀：`/api/v1/bots`

### 8.1 路由

- `GET /api/v1/bots`
- `GET /api/v1/bots/custom`
- `POST /api/v1/bots`
- `PUT /api/v1/bots/:botId`
- `DELETE /api/v1/bots/:botId`

### 8.2 请求示例

`POST /api/v1/bots`

```json
{
  "name": "我的 Bot",
  "mentionName": "mybot",
  "aliases": ["小助手"],
  "description": "自定义 OpenAPI Bot",
  "apiBaseUrl": "https://example.com/v1",
  "apiKey": "sk-***",
  "modelName": "deepseek-v4-flash",
  "supportedModels": ["deepseek-v4-flash"],
  "systemPrompt": "你是一个简洁的助手"
}
```

### 8.3 说明

- `GET /api/v1/bots` 返回平台 bot 与当前用户可见 bot。
- `GET /api/v1/bots/custom` 返回当前用户自建 bot。

## 9. Knowledge Bases（RAG）

前缀：`/api/v1/knowledge-bases`

### 9.1 路由

- `GET /api/v1/knowledge-bases`
- `POST /api/v1/knowledge-bases`
- `POST /api/v1/knowledge-bases/:knowledgeBaseId/documents/text`
- `POST /api/v1/knowledge-bases/:knowledgeBaseId/documents/file`
- `GET /api/v1/knowledge-bases/:knowledgeBaseId/documents`
- `DELETE /api/v1/knowledge-bases/:knowledgeBaseId/documents/:documentId`
- `POST /api/v1/knowledge-bases/:knowledgeBaseId/search`
- `POST /api/v1/knowledge-bases/:knowledgeBaseId/query`

### 9.2 请求示例

`POST /api/v1/knowledge-bases`

```json
{
  "name": "产品知识库",
  "description": "客服资料"
}
```

`POST /api/v1/knowledge-bases/:knowledgeBaseId/documents/text`

```json
{
  "title": "README",
  "sourceType": "MARKDOWN",
  "content": "文档正文"
}
```

`POST /api/v1/knowledge-bases/:knowledgeBaseId/search`

```json
{
  "query": "话剧的内容是什么",
  "topK": 5
}
```

`POST /api/v1/knowledge-bases/:knowledgeBaseId/query`

```json
{
  "query": "这份文档的核心结论是什么？",
  "topK": 5,
  "conversationId": "c_1001",
  "botId": 100001
}
```

### 9.3 说明

- `POST /api/v1/knowledge-bases/:knowledgeBaseId/search` 只返回检索片段。
- `POST /api/v1/knowledge-bases/:knowledgeBaseId/query` 返回问答结果，包含：
  - `status`
  - `answer`
  - `model`
  - `plan`
  - `citations`
  - `quotes`
  - `chunks`
- `query` 的 `conversationId` 和 `botId` 会参与模型配置解析。

## 10. Notifications

前缀：`/api/v1/notifications`

### 10.1 路由

- `GET /api/v1/notifications`
- `POST /api/v1/notifications/read-all`
- `POST /api/v1/notifications/:notificationId/read`

### 10.2 说明

- `GET /api/v1/notifications` 支持查询参数：
  - `unreadOnly`
  - `limit`
- `unreadOnly` 可用 `true` / `1` 表示只看未读。
- 响应包含：
  - `notifications`
  - `unreadCount`

## 11. Query Router

前缀：`/api/v1/query-router`

### 11.1 路由

- `POST /api/v1/query-router/plan`

### 11.2 请求示例

```json
{
  "userQuery": "帮我总结这份知识库的要点",
  "conversationId": "c_1001",
  "botId": 100001,
  "selectedTargets": [
    {
      "id": "kb_900001",
      "type": "knowledge_base",
      "title": "产品知识库"
    }
  ],
  "availableSpaces": {
    "conversation": true,
    "knowledge_base": true,
    "selected_documents": true,
    "all_documents": false,
    "metadata": true,
    "mixed": true
  },
  "capabilities": {
    "can_lookup": true,
    "can_full_read_document": true,
    "can_synthesize_multi_document": true,
    "can_extract_exact_quote": true,
    "can_control_bindings": true,
    "can_use_external_web": false
  },
  "contextHints": {
    "conversation_id": "c_1001",
    "current_document_ids": ["doc_1", "doc_2"],
    "current_kb_ids": ["kb_900001"]
  }
}
```

### 11.3 说明

- 响应类型为 `QueryRoutePlanInfo`。
- 主要字段包括：
  - `plan_version`
  - `family`
  - `source_space`
  - `scope`
  - `read_depth`
  - `output_mode`
  - `evidence_mode`
  - `targets`
  - `constraints`
  - `confidence`
  - `fallback_family`
  - `reason`

## 12. 维护建议

- 网关新增或变更路由后，请同步更新本文档。
- 字段定义以 `gateway/internal/model/dto` 和 `idl/*.thrift` 为准。
- 如果后续需要 Swagger/OpenAPI，可在本文档基础上继续生成。
