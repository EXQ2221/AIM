# Task 12：文档对齐

## 目标

把 P3 状态和边界写入项目文档。

## 禁止

```text
不得修改代码。
不得生成代码。
```

## 只读文件

```text
README.md
docs/specs/bot-spec.md
docs/specs/gorm-model-spec.md
docs/specs/ws-notification-spec.md
docs/specs/tasks/p3-ai-bot-overview.md
docs/specs/tasks/*.md
output.md
```

## 要求

文档必须明确：

```text
1. P3 AI Bot 已完成或计划完成的闭环。
2. Bot 成员化方案。
3. bots / conversation_members / conversation_bots 三表职责。
4. WebSocket 只发给 USER 成员。
5. conversation_members 不保留旧 user_id，开发阶段允许清库重建。
6. docs/specs/gorm-model-spec.md 是模型演进记录和参考基线，具体实现以当前 task spec 和代码为准。
7. P3 不做 RAG。
8. P3 不做 Bot 私聊。
9. P3 不做用户自带 API Key。
10. P3 不做 Redis Stream。
11. P3 使用 goroutine + semaphore / in-memory runner 控制 Bot 并发。
12. 后续 P4/RAG 可引入 Redis Stream。
```

## 输出要求

```text
Changed files
What changed
Tests run
Tests not run
Remaining TODOs
```

并追加到 output.md。
