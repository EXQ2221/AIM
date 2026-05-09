# AIM P3 AI Bot 总设计索引

本文档是 P3 AI Bot 的总纲索引。Codex 执行具体任务时，不应默认通读完整总设计，而应只读取对应的 task spec。

## 固定设计结论

P3 使用三张表表达 Bot 能力：

```text
bots
conversation_members
conversation_bots
```

职责：

```text
bots：
- Bot 全局配置。
- 名称、头像、mention_name、aliases、模型、system prompt、状态。

conversation_members：
- 会话成员。
- USER 和 BOT 都是成员。
- 用于成员列表、成员状态、加入/移除、WebSocket 收件人过滤。
- 不保留旧 user_id 字段，统一使用 member_type + member_id。

conversation_bots：
- Bot 在某个会话内的 AI 配置。
- enabled、permission_scope、display_name_override、mention_name_override、aliases_override。
```

## 文档优先级

P3 阶段文档优先级必须为：

```text
1. docs/specs/tasks/*.md
2. docs/specs/tasks/p3-ai-bot-overview.md
3. docs/specs/ws-notification-spec.md
4. docs/specs/gorm-model-spec.md
5. docs/specs/bot-spec.md
```

说明：

```text
- P3 阶段以 docs/specs/tasks/*.md 和 docs/specs/tasks/p3-ai-bot-overview.md 为最高优先级。
- bot-spec.md 是早期 Bot 接入 spec，只作为历史背景。
- gorm-model-spec.md 如与 P3 task spec 冲突，以 P3 task spec 为准。
- ws-notification-spec.md 只作为 WebSocket / Redis PubSub 策略参考。
- Codex 执行 P3 task 时，不得用旧 spec 覆盖 P3 task spec 的设计结论。
```

## 字段格式结论

aliases 相关字段格式必须统一：

```text
数据库：
- bots.aliases
- conversation_bots.aliases_override
- 必须存 JSON 文本

API / DTO：
- aliases
- aliasesOverride
- 必须是 []string

分层职责：
- repository / mapper 层负责 JSON string <-> []string 转换
- 不允许一处使用逗号字符串、一处使用数组的混合表达
```

## 收件人结论

所有 WebSocket 广播收件人必须来自：

```text
ListUserMemberIDs
```

约束：

```text
- ListUserMemberIDs 只返回 member_type=USER 且 status=NORMAL 的成员
- 普通消息广播、Bot 回复 Pub/Sub 广播、系统消息广播都必须只发给 USER 成员
- 不得把 BOT 成员的 member_id 当作 userId 推送
- 不得把旧 user_id=0 当作 userId 推送
```

## 单聊边界

P3 不实现 Bot 私聊，但未来路径必须明确：

```text
- 普通 SINGLE 会话只允许 USER + USER
- 未来 Bot 私聊应使用 conversation.type=BOT
- BOT 类型会话成员为 USER + BOT
- Bot 私聊不走好友关系校验
- 当前 USER 单聊逻辑不得被 Bot 成员化污染
```

## 命名冲突规则

mention / alias 冲突范围必须明确：

```text
- bots.mention_name 在 P3 阶段对平台内置 Bot 保持全局唯一
- 未来如果支持多租户 / 工作空间 / 用户自定义 Bot，再改为 tenant_id/workspace_id + mention_name 唯一
- bots.aliases 不做全局唯一索引
- conversation_bots.mention_name_override / aliases_override 只要求当前 conversation 内不冲突
- 添加 Bot 或修改 override 时，必须校验当前 conversation 内所有 enabled Bot 的 mention_name / aliases / override 不冲突
- 如果运行时一个 @token 命中多个 Bot，不调用 LLM，不随机选择，只记录日志
```

## 权限结论

以下字段只能由 OWNER / ADMIN 设置或修改：

```text
display_name_override
mention_name_override
aliases_override
permission_scope
```

普通成员：

```text
- 只能查看
- 不能设置或修改 override
```

P3 约束：

```text
- P3 可以只在添加 Bot 时设置 override
- 如果后续增加 PATCH /conversations/{id}/bots/{botId}，权限也必须是 OWNER / ADMIN
```

## 并发结论

并发超限处理必须是强约束：

```text
- 超过 BOT_MAX_CONCURRENCY 或 BOT_MAX_CONVERSATION_CONCURRENCY 时，不调用 LLM
- 不创建 BOT_REPLY
- 不影响用户原始消息
- 必须记录日志
- 必须写 ai_call_logs FAILED
- error_message 必须标明 global concurrency limit reached 或 conversation concurrency limit reached
- P3 不要求给用户发送“AI 助手繁忙”的 Bot 回复
```

## DTO 结论

AI 助手面板和成员列表展示 Bot 时必须复用统一 Bot 展示 DTO。

DTO 至少包含：

```text
botId
memberType
memberId
name
displayName
mentionName
aliases
avatar
description
enabled
permissionScope
memberStatus
```

不允许前端两个页面分别手动拼装不同结构。

## P3 范围

P3 必须完成：

```text
Bot 成员化
Bot 加入/移除群聊
Bot 触发双校验
Bot 配置从 bots 表读取
Bot 回复事务一致性
Bot 并发控制
Bot 管理接口
前端 AI 助手面板
成员列表展示 Bot
文档对齐
```

P3 不实现：

```text
RAG
embedding
知识库检索
Bot 私聊
Bot 好友关系
用户自定义 Bot
用户自带 API Key
多租户 Bot 命名空间
Bot 商店
复杂计费
单用户限流
单 Bot 限流
Redis Stream
任务队列
失败重试
死信队列
PWA Push
原生 App 推送
```

## 执行规则

每次 Codex 只执行一个 task spec。

不得默认读取完整总纲。

每个 task 开始前必须先执行该 task 内的 Preflight Check。

当前处于开发阶段，数据库没有重要历史数据。P3 模型调整允许清库重建，不做旧数据兼容迁移。

模型相关任务必须同步更新：

```text
docs/specs/gorm-model-spec.md
output.md
```

`docs/specs/gorm-model-spec.md` 只作为模型演进记录和参考基线，不覆盖当前 task spec。

## 执行建议

Task 09 涉及 IDL、chat-service、gateway，改动较大。

如果 Codex 额度紧张，可拆为：

```text
Task 09A：IDL + 生成代码
Task 09B：chat-service RPC 实现
Task 09C：gateway HTTP handler 实现
```

默认仍保留 Task 09，且必须先完成 Task 08 评估。

## 推荐执行顺序

```text
00：Spec Review ✅
01：模型重建与 GORM 对齐 ✅
02：成员 repository 方法补齐 ✅
03：USER 业务逻辑迁移 ✅
04：Bot 加入/移除底层能力 ✅
05：Bot 触发双校验与目标解析 ✅
06：Bot 回复事务一致性 ✅
07：Bot 并发控制 ✅
08：Bot 管理接口评估 ✅
09：实现后端 Bot 管理接口 ✅
10：前端 AI 助手面板 ✅
11：成员列表展示 Bot ✅
12：文档对齐 ✅ ← 当前任务
```

> **P3 全部 Task 已于 2026-05-07 完成。**
