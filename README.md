# AIM

AIM 是一个 AI 原生多人协作聊天平台，采用 Go 微服务架构。  
当前仓库已经具备稳定的聊天主链路，并集成了 Bot、知识库（RAG）、历史搜索、群聊总结、在线状态与可观测能力。

## 当前实现状态

- 已完成：注册登录、JWT 鉴权、好友关系、单聊/群聊、消息持久化、WebSocket 实时推送。
- 已完成：群成员角色与禁言管理、消息撤回（5 分钟窗口）、历史消息检索（会话/时间/关键词）。
- 已完成：内置 Bot + 用户自建 Bot（创建、列表、更新、删除、绑定会话、权限范围控制）。
- 已完成：知识库创建、文档导入（文本/文件）、切分、向量化、检索、会话绑定。
- 已完成：群聊总结接口（异步友好调用）、用户长期记忆写入/查询/修改。
- 已接入：Prometheus + Grafana 监控。

## 架构总览

### 服务划分

- `gateway`：HTTP API、JWT 中间件、WebSocket 接入、统一响应与聚合。
- `auth-service`：注册/登录、token 刷新、会话管理、登出、登出所有设备。
- `user-service`：用户资料、好友关系、分组、在线状态设置（含隐身）。
- `chat-service`：会话、群组、成员角色、消息、Bot 触发与 AI 调用编排。
- `rag-service`：知识库与检索能力（向量/全文混合检索）。
- `parser-service`：文档解析、图文提取、可选 LLM chunking。
- `shared`：配置、错误码、通用响应、日志、基础组件。
- `idl`：Thrift 接口定义（Kitex RPC）。

### 基础设施

- PostgreSQL（含 `pgvector`）
- Redis
- Docker Compose
- Prometheus / Grafana

## 鉴权与权限模型（重点）

### 1) 双层鉴权

- 第一层：`gateway` 校验 JWT（身份、过期、格式）。
- 第二层：业务服务二次校验权限（会话成员关系、群角色、禁言状态、资源归属）。

结论：前端参数永远不被直接信任，关键权限由服务端业务层最终裁决。

### 2) Token 与会话失效

- 使用 `token_version` 机制支持主动失效。
- 典型触发：修改密码、登出所有设备、管理员强制下线、账号状态变化。

### 3) 群聊权限

- 角色：`OWNER` / `ADMIN` / `MEMBER` / `BOT`。
- 群管理动作（转让群主、管理员设置、全员禁言、成员禁言/移除）均在 `chat-service` 做角色校验。
- 消息撤回默认限制在 5 分钟内，采用状态变更而不是物理删除。

### 4) Bot 与知识库权限边界

- 知识库是用户资源，不是 Bot 资源。
- 知识库绑定到 `conversation` 后，是否可被 Bot 使用由 `conversation_bots.permission_scope` 控制。
- 仅当前会话中已绑定且启用的知识库，对权限允许的 Bot 可见。

## 主要能力与接口

以下为核心 API 分组（统一前缀 `/api/v1`）：

- 认证：`/auth/register` `/auth/login` `/auth/refresh` `/auth/logout` `/auth/logout-all` `/auth/sessions`
- 用户：`/users/me` `/users/me/avatar` `/users/memory`（GET/POST/PUT）
- 好友：`/friends`、`/friends/requests`、`/friends/groups`、`/friends/presence/settings`
- 会话与群组：`/conversations`（列表、单聊定位、建群、成员管理、管理员、禁言、公告、已读、撤回）
- 消息：`/conversations/:id/messages`、`/conversations/history/search`
- Bot：`/bots`、`/bots/custom`、`/bots/:botId`（PUT/DELETE）、`/conversations/:id/bots`
- 知识库：`/knowledge-bases`、文档导入（text/file）、搜索、会话绑定
- 总结：`/conversations/:id/summary`
- 通知：`/notifications`

## 快速启动

### 1) 准备环境变量

```bash
cp .env.example .env
```

最少请配置：

- `POSTGRES_PASSWORD`
- `JWT_SECRET`
- `LLM_BASE_URL`
- `LLM_API_KEY`
- `LLM_MODEL`

如需知识库与检索，请同时配置：

- `EMBEDDING_BASE_URL`
- `EMBEDDING_API_KEY`
- `EMBEDDING_MODEL`
- `EMBEDDING_DIMENSION`

如需重排（推荐）：

- `RERANK_ENABLED=true`
- `RERANK_BASE_URL`
- `RERANK_API_KEY`
- `RERANK_MODEL`

### 2) 启动

```bash
docker compose up -d --build
```

### 3) 健康检查

```bash
docker compose ps
curl http://127.0.0.1:8080/healthz
```

## 关键环境变量说明

### LLM / Bot

- `LLM_BASE_URL` / `LLM_API_KEY` / `LLM_MODEL`
- `LLM_TIMEOUT_SECONDS`
- `BOT_CONTEXT_MESSAGES`
- `BOT_TASK_TIMEOUT_SECONDS`

### Summary（群聊总结）

- `SUMMARY_LLM_BASE_URL`
- `SUMMARY_LLM_API_KEY`
- `SUMMARY_LLM_MODEL`

> 默认可复用主 LLM 配置。

### Embedding / RAG

- `EMBEDDING_BASE_URL` / `EMBEDDING_API_KEY` / `EMBEDDING_MODEL`
- `EMBEDDING_TIMEOUT_SECONDS` / `EMBEDDING_MAX_RETRIES`
- `RAG_SEARCH_TIMEOUT_SECONDS`

### Rerank

- `RERANK_ENABLED`
- `RERANK_BASE_URL`
- `RERANK_API_KEY`
- `RERANK_MODEL`
- `RERANK_SCORE_THRESHOLD`

### Chunker（parser-service）

- `CHUNKER_BASE_URL` / `CHUNKER_API_KEY` / `CHUNKER_MODEL`
- `CHUNKER_TIMEOUT_SECONDS`
- `PARSER_ENABLE_LLM_CHUNKING`

## 前端与联调

- 前端目录：`frontend/`
- 默认通过 `gateway` API 与 `ws/chat` 进行联调。
- 已支持：聊天主界面、历史搜索、Bot 管理、知识库操作、群聊总结入口、在线状态设置。

## 可观测性

- Prometheus：`http://127.0.0.1:9090`
- Grafana：`http://127.0.0.1:3000`
- Gateway 暴露：`/metrics`

## 开发建议

- 修改接口优先改 `idl/*.thrift`，再生成 Kitex 代码。
- 业务逻辑尽量放 `service/biz`，`handler` 只做参数与编排。
- 统一使用 `shared/errno` 与统一响应结构返回错误。

