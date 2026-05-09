# Task 11：成员列表展示 Bot

## 目标

群成员列表必须支持 USER 和 BOT 展示。

## Preflight Check

必须确认：

```text
Task 01 已完成。
Task 02 已完成。
Task 09 后端接口或成员接口已经能提供 BOT 资料。
```

## 只读文件

```text
docs/specs/tasks/p3-task-11-member-list-bot-display.md
idl/chat.thrift
chat-service/internal/biz/chat.go
chat-service/internal/handler/chat_service.go
gateway/internal/handler/chat.go
frontend/src/App.tsx
frontend/src/types.ts
```

## 要求

```text
1. 成员返回结构支持 memberType/memberId。
2. USER 成员补 user-service 昵称头像。
3. BOT 成员补 bots 表名称头像。
4. 前端展示 Bot 为 AI 助手。
5. 不影响 USER 成员显示。
6. 不影响 WebSocket 广播收件人过滤。
7. 所有 WebSocket 广播收件人必须来自 ListUserMemberIDs。
8. ListUserMemberIDs 必须只返回 member_type=USER 且 status=NORMAL 的成员。
9. 普通消息广播、Bot 回复 Pub/Sub 广播、系统消息广播都必须只发给 USER 成员。
10. 不得把 BOT 成员的 member_id 或旧 user_id=0 当作 userId 推送。
```

## DTO

成员展示 DTO 必须至少包含：

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

成员列表展示 Bot 时必须复用统一 Bot 展示 DTO。

不允许成员列表和 AI 助手面板各自拼装不同结构。

## 禁止

```text
不得让 user-service 查询 bot_id。
不得让 WebSocket recipient 包含 BOT。
```

## 验证

```text
必要时重新生成 Kitex
go test ./... in chat-service
go test ./... in gateway
npm run build --prefix frontend
```
