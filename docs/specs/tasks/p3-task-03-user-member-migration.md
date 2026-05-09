# Task 03：USER 业务逻辑迁移

## 目标

所有真实用户相关逻辑必须明确使用 `member_type=USER`。

本任务不得引入 Bot 管理接口，不得修改前端和 gateway。

## Preflight Check

必须确认 Task 02 已完成：

```text
GetUserMember
IsUserMember
ListUserMemberIDs
```

如果缺失，必须停止实现。

## 只读文件

```text
docs/specs/tasks/p3-task-03-user-member-migration.md
chat-service/internal/biz/chat.go
chat-service/internal/repository/chat.go
chat-service/internal/rpc/user_client.go
```

## 要求

```text
1. 用户发送消息权限校验必须使用 GetUserMember / IsUserMember。
2. WebSocket recipient 查询必须只返回 USER 成员 ID。
3. 单聊对端查找必须只看 USER 成员。
4. 好友关系校验必须只看 USER 成员。
5. 禁言逻辑只对 USER 成员生效。
6. Bot 回复不得走普通用户发送消息权限。
7. 不得再读取 conversation_members.user_id。
8. 用户 ID 语义统一来自 member_type=USER 的 member_id。
9. 所有 WebSocket 广播收件人必须来自 ListUserMemberIDs。
10. ListUserMemberIDs 必须只返回 member_type=USER 且 status=NORMAL 的成员。
11. 当前普通 SINGLE 会话只允许 USER + USER。
12. 未来 Bot 私聊应使用 conversation.type=BOT，成员为 USER + BOT。
13. Bot 私聊不走好友关系校验。
14. 当前 USER 单聊逻辑不得被 Bot 成员化污染。
```

## 禁止

```text
不得修改 IDL。
不得修改 gateway。
不得修改 frontend。
不得接入 Bot 管理接口。
不得实现 Bot 触发逻辑。
```

## 验收标准

```text
1. USER 权限校验不会误把 BOT 当用户。
2. WebSocket recipientUserIds 不包含 BOT。
3. SINGLE 单聊逻辑不包含 BOT。
4. user-service 不会收到 bot_id 查询。
5. chat-service 不再依赖 conversation_members.user_id。
6. 所有普通消息、Bot 回复、系统消息广播收件人都来自 ListUserMemberIDs。
```

## 验证

```text
gofmt
go test ./... in chat-service
go build ./... in chat-service
```
