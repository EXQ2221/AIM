# AIM 数据库迁移说明（MySQL -> PostgreSQL）

## 1. 当前状态（截至 2026-05-11）

- Task1 已完成：基础设施切到 PostgreSQL + Redis。
- Task2 已完成：`auth-service` / `user-service` / `chat-service` 运行时驱动与 DSN 切到 PostgreSQL。
- Task3 已完成：容器联启动通过，服务健康可用。
- Task4 已完成：`messages.content` 改为 `jsonb`，并完成前后端结构统一：
  - 前端发送 `contentPayload`（对象）
  - 网关统一序列化
  - chat-service 入库到 `jsonb`
- Task5 已完成：端到端冒烟验证通过（注册、登录、建群、消息列表、成员列表、Bot 列表、AI 调用日志接口可用）。
- Task6 进行中：文档与配置统一收尾。

## 2. PostgreSQL 部署形态

采用单实例多数据库隔离：

- `aim_auth`（auth-service）
- `aim_user`（user-service）
- `aim_chat`（chat-service）

初始化 SQL：

- `deploy/postgres/init/01-create-databases.sql`

Compose 挂载：

- `./deploy/postgres/init` -> `/docker-entrypoint-initdb.d`

## 3. 关键配置约定

`.env.example` 当前约定（服务级 DSN）：

```env
AUTH_POSTGRES_DSN=host=postgres user=aim password=change-me-db-password dbname=aim_auth port=5432 sslmode=disable TimeZone=UTC
USER_POSTGRES_DSN=host=postgres user=aim password=change-me-db-password dbname=aim_user port=5432 sslmode=disable TimeZone=UTC
CHAT_POSTGRES_DSN=host=postgres user=aim password=change-me-db-password dbname=aim_chat port=5432 sslmode=disable TimeZone=UTC
```

说明：

- `TimeZone` 已统一为 `UTC`，避免 Alpine 环境下 `Asia/Shanghai` 报 `unknown time zone`。

## 4. Task4 技术落地摘要

仅改动 `messages.content` 字段类型：

- Go 模型：`string` -> `datatypes.JSON`
- GORM tag：`gorm:"type:jsonb;not null"`

保持不变的字段：

- `conversation_id`
- `sender_id`
- `sender_type`
- `message_type`
- `reply_to_id`
- `status`
- `created_at`
- `updated_at`

消息发送链路统一为：

1. 前端发送 `contentPayload`（JSON 对象）
2. 网关校验并 `JSON.stringify`
3. chat-service 做消息类型规范化
4. PostgreSQL `messages.content (jsonb)` 入库

## 5. Task5 验证结论

### 5.1 通过项

- `docker compose ps`：`postgres/redis/gateway/auth/user/chat` 全部 `healthy`
- `GET /healthz`：返回 `ok`
- 认证链路：注册、登录成功
- 会话链路：建群成功
- 查询链路：消息列表、成员列表、会话列表可用
- Bot 链路：内置 Bot 列表可用
- AI 日志链路：`ai-call-logs` 接口可用

### 5.2 当前已知问题

- `chat-service` 单测尚未收敛：
  - `messages.content` 升级为 `datatypes.JSON` 后，测试中的 string 字面量与 mock 需同步
  - `BotRepository` 接口新增方法后，部分 fake repo 未补齐

这不影响当前运行时服务可用性，但影响 CI/回归完整性。

## 6. 回滚策略（保留）

- 保留 MySQL 基线分支或 tag。
- 若迁移后异常，按以下顺序 5 分钟内回退：
  1. 切回 MySQL 基线代码
  2. `docker compose down`
  3. `docker compose up -d --build`

## 7. SQL 方言差异检查清单（保留）

- `ON DUPLICATE KEY UPDATE` -> `ON CONFLICT ... DO UPDATE`
- 时间函数差异（`NOW()` / `CURRENT_TIMESTAMP` / 时区）
- `LIMIT/OFFSET` 用法和排序稳定性
- 布尔与整型语义差异（`tinyint` vs `boolean`）
- JSON 查询表达式与索引差异

## 8. Seed 幂等验证（保留）

必须验证：

- 默认 seed 可重复执行，不产生唯一键冲突
- Bot seed 按稳定主键或唯一业务键 upsert
- 重启服务不会导致 seed 失败阻塞启动

## 9. 下一步建议

1. 修复 `chat-service` 单测（JSON 类型与 fake repo 接口补齐）。
2. 在 CI 中补一条 PostgreSQL 冒烟 job（启动 compose + 核心 API 冒烟）。
3. 为 `messages.content` 增加针对系统消息的表达式索引（按实际查询热点决定）。

