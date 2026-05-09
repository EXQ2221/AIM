# Task 00：实现前规格审查

## 目标

在开始任何代码修改前，Codex 必须先审查当前任务所依赖的规格说明。

本任务只读，不得修改文件，不得生成代码，不得运行代码生成。

## 只读文件

```text
docs/specs/tasks/p3-task-00-spec-review.md
```

如需全局上下文，最多额外只读：

```text
docs/specs/tasks/p3-ai-bot-overview.md
docs/specs/gorm-model-spec.md
docs/specs/bot-spec.md
docs/specs/ws-notification-spec.md
chat-service/internal/dal/model/chat.go
chat-service/internal/dal/model/bot.go
chat-service/internal/repository/chat.go
chat-service/internal/repository/bot.go
chat-service/internal/bot/service.go
```

不得进行全仓库扫描。

## 审查内容

Codex 必须检查：

```text
1. 当前 task spec 是否存在前后矛盾。
2. 数据模型是否存在字段语义冲突。
3. member_type/member_id 清库重建方案是否明确，不得保留旧 user_id。
4. conversation_members 与 conversation_bots 是否存在一致性缺口。
5. Bot 触发规则是否存在多 Bot 解析歧义。
6. aliases 的数据库存储与 API 表达是否统一。
7. WebSocket 广播是否错误包含 BOT 成员。
8. USER/BOT 成员查询是否存在混用风险。
9. 普通 SINGLE 单聊是否被 Bot 成员化污染。
10. 权限边界是否明确。
11. 事务一致性是否明确。
12. 并发控制、超时、超限行为是否明确。
13. Task 拆分是否过大或跨模块过多。
14. 是否存在“推荐”“建议”“可以”“最好”等不确定措辞影响实现。
15. 模型变更是否同步记录到 docs/specs/gorm-model-spec.md。
```

## 输出格式

```text
Spec Review Result

1. Blocking Issues
- 必须修复后才能实现的问题

2. Non-blocking Issues
- 可以实现，但建议修正的问题

3. Ambiguities
- 语义不明确，需要人工决定的问题

4. Risk Points
- 实现时容易出错的地方

5. Suggested Spec Changes
- 建议修改的 spec 条目

6. Implementation Readiness
- READY / NOT READY
```

## 继续条件

只有当 Codex 输出：

```text
Implementation Readiness: READY
```

或者人工明确确认忽略相关问题后，才能进入实现任务。
