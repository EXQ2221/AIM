# AIM

AIM 是一个 AI 原生多人协作聊天平台（AI Native Multiplayer Chat Platform）。

当前已完成：

```text
P0  微服务骨架 + JWT 鉴权链路
P1  用户系统 + 好友关系 + 单聊/群聊 + WebSocket 实时消息
P2  前端工作台（Vite + React + TypeScript）
P3  AI Bot 成员化 + @mention 触发 + LLM 非流式回复 + Redis Pub/Sub 广播
```

## 当前结构

## 当前结构

- `gateway/`：Gin HTTP 网关，负责对外 API、JWT 中间件、统一响应、RPC 调用、WebSocket 接入和 Redis Pub/Sub 订阅。
- `auth-service/`：注册、登录、JWT 校验、refresh token rotation、会话管理、登出。
- `user-service/`：用户资料、AIM ID、密码校验、账号状态、`token_version`、好友关系。
- `chat-service/`：聊天服务，承载会话（单聊/群聊）、成员管理（USER+BOT）、消息持久化、Bot 异步触发与 LLM 调用、AI 调用日志、Bot 并发控制。
- `idl/`：Thrift IDL 定义。
- `shared/`：公共错误码、响应、工具和配置辅助。

## 鉴权链路

当前已迁移并适配的能力：

- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/refresh`
- `POST /api/v1/auth/logout`
- `POST /api/v1/auth/logout-all`
- `GET /api/v1/auth/sessions`
- `POST /api/v1/auth/sessions/revoke`
- `GET /api/v1/users/me`

JWT 中包含：

- `user_id`
- `aim_id`
- `role`
- `token_version`
- `expire_time`
- `sid`
- `jti`

服务端校验 JWT 时会检查：

- JWT 签名和过期时间
- access token `jti` 是否在 Redis 黑名单
- session 是否存在且仍为 active
- 当前 session 的最新 `jti` 是否匹配
- 用户是否存在且状态为 `NORMAL`
- 用户当前 `token_version` 是否与 JWT 一致

## 本地构建

```bash
go build ./...
```

各服务也可以分别构建：

```bash
cd user-service && go build ./...
cd auth-service && go build ./...
cd gateway && go build ./...
cd chat-service && go build ./...
```

## Docker Compose

敏感配置通过环境变量注入：

```bash
export MYSQL_ROOT_PASSWORD='change-me'
export MYSQL_PASSWORD='change-me'
export JWT_SECRET='change-me'
docker compose up --build
```

默认端口：

- `gateway`: `8080`
- `user-service`: `9001`
- `auth-service`: `9002`
- `chat-service`: `9003`
- `mysql`: `3306`
- `redis`: `6379`

## IDL 生成

安装 `kitex` 后执行：

```bash
./scripts/gen.sh
```

生成代码不要手动修改；需要调整 RPC 接口时，先改 `idl/*.thrift`，再重新生成。

## P3 AI Bot 架构

### 三表设计

```text
bots                — Bot 全局配置（名称、头像、mention_name、模型、system prompt）
conversation_members — 会话成员（USER 和 BOT 都是成员，统一用 member_type + member_id）
conversation_bots    — Bot 在某会话内的 AI 配置（enabled、permission_scope、overrides）
```

### 核心约束

- **WebSocket 只发给 USER 成员**：所有广播收件人来自 `ListUserMemberIDs`（只返回 member_type=USER 且 status=NORMAL）
- **Bot 不做私聊**：P3 只支持群聊中的 @mention 触发
- **并发控制**：goroutine + semaphore（全局上限 10，单会话上限 1）
- **不做 RAG / Redis Stream / 用户自带 Key**：这些留给 P4/P5

### P3 不做的内容

```text
RAG / embedding / 知识库检索
Bot 私聊（conversation.type=BOT）
用户自定义 Bot / 用户自带 API Key
多租户命名空间
Redis Stream / 任务队列 / 死信队列
流式回复（当前只有非流式 BOT_REPLY）
PWA Push / 原生 App 推送
```

文档优先级：

1. `docs/specs/tasks/*.md`（最高，当前 task spec）
2. `docs/specs/tasks/p3-ai-bot-overview.md`（P3 总纲）
3. `docs/specs/ws-notification-spec.md`
4. `docs/specs/gorm-model-spec.md`（模型演进记录，以代码为准）

## Chat API

Gateway exposes the chat-service endpoints:

### 会话与消息

- `POST /api/v1/conversations/group` — 创建群聊
- `POST /api/v1/conversations/single`（内部：加好友自动创建单聊）
- `GET /api/v1/conversations` — 会话列表（含最近消息摘要）
- `POST /api/v1/conversations/{conversationId}/members` — 加入会话
- `DELETE /api/v1/conversations/{conversationId}/members/me` — 退出/离开
- `GET /api/v1/conversations/{conversationId}/members` — 成员列表（含 USER + BOT）
- `GET /api/v1/conversations/{conversationId}/messages?beforeId=10086&limit=30` — 历史消息
- `GET /ws/chat?token=<access_token>` — WebSocket 实时消息

### Bot 管理

- `GET /api/v1/bots` — 可用 Bot 列表
- `GET /api/v1/conversations/{conversationId}/bots` — 会话内 Bot 列表
- `POST /api/v1/conversations/{conversationId}/bots` — 添加 Bot 到群聊
- `DELETE /api/v1/conversations/{conversationId}/bots/{botId}` — 从群聊移除 Bot

### 消息发送路径

External clients send chat messages through WebSocket at `GET /ws/chat?token=<access_token>`.

The underlying message creation logic lives in chat-service RPC and is reused by the WebSocket module.

### Bot 回复路径

```text
用户在群聊中 @BotName 发送消息
→ 用户消息正常落库并广播
→ chat-service 异步触发 Bot（goroutine + semaphore 并发控制）
→ Bot 调用 OpenAI-compatible LLM（非流式）
→ Bot 回复写入 message 表
→ chat-service 发布 Redis Pub/Sub 事件 aim:bot_reply_created
→ gateway 订阅事件，复用 NEW_MESSAGE 广播给在线 USER 成员
```

WebSocket event details are documented in `docs/websocket.md`。
