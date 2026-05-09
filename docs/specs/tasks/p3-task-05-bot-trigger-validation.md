# Task 05：Bot 触发双校验与目标解析

## 目标

Bot 触发时必须校验 BOT 成员、conversation_bots、bots.status，并通过 mention/alias 精确解析目标 Bot。

## Preflight Check

必须确认：

```text
Task 01 已完成。
Task 02 已完成。
Task 04 已完成。
aliases JSON 转换方法已存在。
repository / mapper 已能处理 JSON string <-> []string 转换。
```

如果缺失，必须停止实现。

## 只读文件

```text
docs/specs/tasks/p3-task-05-bot-trigger-validation.md
chat-service/internal/bot/service.go
chat-service/internal/bot/trigger.go
chat-service/internal/bot/prompt.go
chat-service/internal/repository/bot.go
chat-service/internal/repository/chat.go
chat-service/internal/dal/model/bot.go
```

## 触发候选条件

候选 Bot 必须同时满足：

```text
conversation_members:
- member_type = BOT
- status = NORMAL

conversation_bots:
- enabled = true

bots:
- status = ENABLED
```

## @token 匹配规则

aliases 相关格式必须统一：

```text
- 数据库中的 aliases 和 aliases_override 必须存 JSON 文本
- API / DTO 中的 aliases 和 aliasesOverride 必须是 []string
- repository / mapper 层负责 JSON string <-> []string 转换
- 不允许一处使用逗号字符串、一处使用数组的混合表达
```

@token 必须匹配候选 Bot 的以下任一项：

```text
conversation_bots.mention_name_override
bots.mention_name
conversation_bots.aliases_override
bots.aliases
```

匹配规则：

```text
1. 不区分大小写。
2. @ 不参与存储值。
3. 必须按完整 token 匹配。
4. @aim 不得误命中 @aimer。
5. 一个 @token 命中多个 Bot 时，不得随机选择；本次不触发 LLM，并记录日志。
6. bots.mention_name 在 P3 阶段对平台内置 Bot 保持全局唯一。
7. bots.aliases 不做全局唯一索引。
8. conversation_bots.mention_name_override / aliases_override 只要求当前 conversation 内不冲突。
9. 添加 Bot 或修改 override 时，必须校验当前 conversation 内所有 enabled Bot 的 mention_name / aliases / override 不冲突。
```

## permission_scope

P3 只允许：

```text
CONVERSATION_ONLY
```

以下 scope 不得调用 LLM：

```text
KNOWLEDGE_BASE_ONLY
CONVERSATION_AND_KB
```

## LLM 配置来源

模型名优先级：

```text
1. bots.model_name
2. LLM_MODEL
3. 报错并写 ai_call_logs FAILED
```

system prompt 优先级：

```text
1. bots.system_prompt
2. 默认 AIM 群聊助手 prompt
```

## 禁止

```text
不得修改 IDL。
不得修改 gateway。
不得修改 frontend。
不得实现 RAG。
不得实现 Bot 私聊。
```

## 验收标准

```text
1. 未加入 Bot 的群聊中 @AIM 不触发 LLM。
2. 已加入 Bot 的群聊中 @AIM 触发 LLM。
3. @aim 大小写不敏感触发。
4. alias 命中可触发。
5. override 优先于 bots 全局字段。
6. alias 冲突时不触发。
7. 移除 Bot 后不触发。
8. bots.status=DISABLED 不触发。
9. conversation_bots.enabled=false 不触发。
10. permission_scope 非 CONVERSATION_ONLY 不触发。
```

## 验证

```text
gofmt
go test ./... in chat-service
go build ./... in chat-service
```
