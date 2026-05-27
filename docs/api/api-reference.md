# AIM API 接口文档（Gateway）

最后更新：2026-05-27  
适用服务：`gateway`  
基础路径：`/api/v1`

## 1. 通用约定

### 1.1 鉴权

- 受保护接口需要登录态。
- 网关支持两种 Access Token 读取方式：
  - Cookie：`access_token`
  - Header：`Authorization: Bearer <token>`

### 1.2 统一响应格式

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

- `code=0` 表示成功。
- 非 0 表示失败，`message` 为可读错误信息。

### 1.3 常见错误

- `400` 参数错误
- `401` 未认证或 token 失效
- `403` 无权限
- `404` 资源不存在
- `409` 资源冲突
- `500` 服务内部错误

## 2. 健康与观测

- `GET /healthz`：健康检查
- `GET /metrics`：Prometheus 指标

## 3. Auth

### 3.1 无需鉴权

- `POST /auth/register`
- `POST /auth/login`
- `POST /auth/refresh`

### 3.2 需鉴权

- `POST /auth/logout`
- `POST /auth/logout-all`
- `GET /auth/sessions`
- `POST /auth/sessions/revoke`

### 3.3 请求示例

`POST /auth/login`

```json
{
  "email": "user@example.com",
  "password": "******",
  "device_name": "web-client"
}
```

`POST /auth/refresh`

```json
{
  "refresh_token": "optional_when_cookie_exists"
}
```

## 4. Users

均需鉴权。

- `GET /users/me`
- `POST /users/me/avatar`（multipart，字段 `file`）
- `GET /users/memory`
- `POST /users/memory`
- `PUT /users/memory/:memoryId`

`POST /users/memory` / `PUT /users/memory/:memoryId`

```json
{
  "content": "最多 512 字的用户长期记忆"
}
```

## 5. Uploads

均需鉴权，`multipart/form-data`，字段名统一为 `file`。

- `POST /uploads/images`
- `POST /uploads/files`
- `POST /uploads/voices`

## 6. Friends

均需鉴权。

- `GET /friends`
- `POST /friends`
- `GET /friends/requests`
- `POST /friends/requests/:requestId/respond`
- `PATCH /friends/:friendUserId`
- `DELETE /friends/:friendUserId`
- `GET /friends/groups`
- `POST /friends/groups`
- `GET /friends/presence/settings`
- `PUT /friends/presence/settings`

请求示例：

`POST /friends`

```json
{
  "target_aim_id": "target_aim_id",
  "remark": "同事",
  "group_id": 1
}
```

`PUT /friends/presence/settings`

```json
{
  "invisible": true
}
```

## 7. Conversations（会话、群组、消息）

均需鉴权。

### 7.1 会话与群组

- `POST /conversations/group`
- `GET /conversations`
- `GET /conversations/single?targetUserId=123`
- `GET /conversations/:conversationId/group`
- `POST /conversations/:conversationId/members`
- `POST /conversations/:conversationId/members/invite`
- `DELETE /conversations/:conversationId/members/me`
- `DELETE /conversations/:conversationId/members/:targetUserId`
- `GET /conversations/:conversationId/members`
- `POST /conversations/:conversationId/owner/transfer`
- `POST /conversations/:conversationId/admins`
- `DELETE /conversations/:conversationId/admins/:targetUserId`
- `POST /conversations/:conversationId/members/:targetUserId/mute`
- `DELETE /conversations/:conversationId/members/:targetUserId/mute`
- `POST /conversations/:conversationId/mute-all`
- `DELETE /conversations/:conversationId/mute-all`
- `PUT /conversations/:conversationId/announcement`

请求示例：

`POST /conversations/group`

```json
{
  "name": "项目讨论群",
  "avatar": "",
  "announcement": "欢迎加入",
  "joinPolicy": "INVITE_ONLY"
}
```

`POST /conversations/:conversationId/members/invite`

```json
{
  "targetUserId": 10002
}
```

### 7.2 消息

- `GET /conversations/:conversationId/messages?beforeId=999&limit=30`
- `POST /conversations/:conversationId/read`
- `POST /conversations/:conversationId/messages/:messageId/recall`
- `GET /conversations/history/search?startAt=...&endAt=...&conversationId=...&keyword=...`

说明：

- `history/search` 当前只支持群聊范围检索。
- `recall` 默认受撤回时间窗口限制（服务端校验）。

请求示例：

`POST /conversations/:conversationId/read`

```json
{
  "lastReadMessageId": 123456
}
```

### 7.3 总结

- `POST /conversations/:conversationId/summary`

请求体：

```json
{
  "messageCount": 100
}
```

规则：

- 最小 20，最大 500，默认 100。
- 每用户每天限制调用次数（当前为 2 次）。
- 仅支持群聊总结。

## 8. Bots

均需鉴权。

### 8.1 Bot 列表与自建 Bot

- `GET /bots`（平台 + 可见 bot）
- `GET /bots/custom`（当前用户自建 bot）
- `POST /bots`
- `PUT /bots/:botId`
- `DELETE /bots/:botId`

`POST /bots` 请求示例：

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

### 8.2 会话绑定 Bot

- `GET /conversations/:conversationId/bots`
- `POST /conversations/:conversationId/bots`
- `DELETE /conversations/:conversationId/bots/:botId`
- `GET /conversations/:conversationId/ai-call-logs`

`POST /conversations/:conversationId/bots` 请求示例：

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

## 9. Knowledge Bases（RAG）

均需鉴权。

- `GET /knowledge-bases`
- `POST /knowledge-bases`
- `POST /knowledge-bases/:knowledgeBaseId/documents/text`
- `POST /knowledge-bases/:knowledgeBaseId/documents/file`
- `GET /knowledge-bases/:knowledgeBaseId/documents`
- `DELETE /knowledge-bases/:knowledgeBaseId/documents/:documentId`
- `POST /knowledge-bases/:knowledgeBaseId/search`

以及会话绑定：

- `POST /conversations/:conversationId/knowledge-bases`
- `GET /conversations/:conversationId/knowledge-bases`
- `DELETE /conversations/:conversationId/knowledge-bases/:knowledgeBaseId`

请求示例：

`POST /knowledge-bases`

```json
{
  "name": "产品知识库",
  "description": "客服资料"
}
```

`POST /knowledge-bases/:knowledgeBaseId/documents/text`

```json
{
  "title": "README",
  "sourceType": "MARKDOWN",
  "content": "文档正文"
}
```

`POST /knowledge-bases/:knowledgeBaseId/search`

```json
{
  "query": "话剧的内容是什么",
  "topK": 5
}
```

## 10. Notifications

均需鉴权。

- `GET /notifications`
- `POST /notifications/read-all`
- `POST /notifications/:notificationId/read`

## 11. WebSocket

- 路径：`GET /ws/chat`
- 鉴权：同样依赖 `access_token`（Cookie 或 Authorization Header）
- 主要用途：
  - 实时消息推送
  - Bot 回复推送
  - 好友在线状态推送

---

## 12. 维护建议

- 网关新增/变更路由后，同步更新本文件。
- 字段定义以 `gateway/internal/model/dto` 与 `idl/*.thrift` 为准。
- 如果需要 Swagger，可在此文档基础上生成 OpenAPI 描述。

