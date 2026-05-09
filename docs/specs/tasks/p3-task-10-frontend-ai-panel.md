# Task 10：前端 AI 助手面板

## 目标

在群聊详情中提供 Bot 管理 UI。

## Preflight Check

必须确认：

```text
Task 09 后端接口已完成。
前端已有群聊详情或可挂载 AI 助手入口的位置。
```

## 只读文件

```text
docs/specs/tasks/p3-task-10-frontend-ai-panel.md
frontend/src/App.tsx
frontend/src/api.ts
frontend/src/types.ts
frontend/src/styles.css
```

## 要求

```text
1. 查询可用 Bot。
2. 查询当前群已加入 Bot。
3. OWNER/ADMIN 可以添加 Bot。
4. OWNER/ADMIN 可以移除 Bot。
5. 普通成员只读展示。
6. 展示 Bot 当前触发名和别名。
7. 添加后刷新 Bot 列表。
8. 移除后刷新 Bot 列表。
9. 不改 WebSocket 协议。
10. display_name_override、mention_name_override、aliases_override、permission_scope 只能由 OWNER / ADMIN 设置或修改。
11. 普通成员只能查看 override 和 permissionScope。
12. P3 可以只在添加 Bot 时设置 override。
```

## 展示字段

必须展示：

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

## DTO 复用

AI 助手面板必须复用 Bot 展示 DTO。

统一 Bot 展示 DTO 至少包含：

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

aliases / aliasesOverride 在 API / DTO 中必须是 []string。

不得在前端多处重复拼装 Bot 名称、头像、触发名、别名。

不允许 AI 助手面板和成员列表分别手动拼装不同结构。

## 验证

```text
npm run build --prefix frontend
```
