# AIM

AIM 是一个基于 Go 微服务实现的 AI 原生协作聊天平台，当前重点是稳定的聊天主链路与可扩展的鉴权/权限模型。

## 技术栈与服务

- 微服务：`gateway`、`auth-service`、`user-service`、`chat-service`、`rag-service`
- 数据存储：PostgreSQL + Redis
- 通信：HTTP + WebSocket + Kitex RPC
- 部署：Docker Compose

## 快速启动

### 1）准备环境变量

复制 `.env.example` 为 `.env` 并填写配置：

```powershell
Copy-Item .env.example .env
```

至少需要：

- `POSTGRES_USER`
- `POSTGRES_PASSWORD`
- `POSTGRES_DATABASE`
- `JWT_SECRET`

如启用 RAG，还需配置：

- `EMBEDDING_BASE_URL`
- `EMBEDDING_API_KEY`
- `EMBEDDING_MODEL`
- `EMBEDDING_DIMENSION`
- `EMBEDDING_TIMEOUT_SECONDS`

### 2）构建并启动

```powershell
docker compose up -d --build
```

### 3）检查服务

```powershell
docker compose ps
curl http://127.0.0.1:8080/healthz
```

## 认证与鉴权（重点）

本项目采用“Gateway 统一接入鉴权 + 业务服务二次权限校验”的双层模型。

### 1）认证边界

- `auth-service` 负责：注册、登录、Token 刷新、会话管理、登出、登出全部设备
- `gateway` 负责：对外请求接入、JWT 中间件校验、身份透传
- `chat-service` / `user-service` 负责：在业务动作层再次校验操作权限

原则：前端传入参数不能被直接信任，关键权限必须在业务服务内校验。

### 2）JWT 机制

Token 中至少包含：

- `user_id`
- `aim_id`
- `role`
- `token_version`
- `expire_time`

Gateway 每次请求会校验：

- Token 是否过期
- Token 结构是否合法
- 身份上下文是否可解析

业务服务继续校验：

- 用户状态是否合法（如禁用用户不可操作）
- 会话成员关系是否满足
- 具体动作权限是否满足（如群管理动作）

### 3）TokenVersion 失效模型

项目通过 `token_version` 实现“主动令牌失效”：

- 修改密码后旧 Token 失效
- 全设备登出后旧 Token 失效
- 账号封禁后旧 Token 失效
- 管理员强制下线后旧 Token 失效

这可以避免仅依赖 JWT 过期时间带来的安全窗口。

### 4）会话与消息权限

`chat-service` 在消息创建时会校验：

- 会话是否存在
- 操作者是否在会话内
- 是否被禁言 / 是否触发全员禁言限制
- 消息类型与内容是否合法

消息撤回采用状态变更（`RECALLED`），不是物理删除。

### 5）WebSocket 鉴权

- 连接建立时必须携带 JWT
- Gateway 解析并绑定 `user_id -> 连接`
- 在线状态写入 Redis
- 断开连接后清理映射

WebSocket 仅负责连接与推送，不直接承载复杂业务权限逻辑。

### 6）Bot 与知识库权限

- Bot 是会话中的执行者，不是知识库资源本体
- 知识库是用户资源，可绑定到会话
- Bot 是否可用知识库由 `conversation_bots.permission_scope` 决定
- 规则：当前会话中已绑定且启用的知识库，仅对权限允许的 Bot 可见

## 历史消息搜索

已支持“会话 + 时间范围 + 关键词”的历史消息搜索。

### API

- `GET /api/v1/conversations/history/search`

查询参数：

- `startAt`（必填）：Unix 秒级时间戳
- `endAt`（必填）：Unix 秒级时间戳
- `conversationId`（可选）：限制为单个会话
- `keyword`（可选）：关键词匹配

### 前端能力

- 聊天头部 `Search` 按钮打开搜索弹窗
- 支持筛选：开始时间、结束时间、关键词、仅当前会话
- 结果显示发送者
- 点击结果可跳转并高亮对应消息

## PostgreSQL 说明

当前使用单实例多数据库隔离：

- `aim_auth`
- `aim_user`
- `aim_chat`

消息内容字段为 `jsonb`（`messages.content`），并建立消息搜索相关索引（FTS + trigram）。

## RAG 最小流程

1. 创建知识库：`POST /api/v1/knowledge-bases`
2. 导入文档：`POST /api/v1/knowledge-bases/{knowledgeBaseId}/documents/text`
3. 查看状态：`GET /api/v1/knowledge-bases/{knowledgeBaseId}/documents`
4. 分片检索：`POST /api/v1/knowledge-bases/{knowledgeBaseId}/search`
5. 绑定会话：`POST /api/v1/conversations/{conversationId}/knowledge-bases`

