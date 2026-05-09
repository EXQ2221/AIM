# AIM WebSocket、后台通知与消息补偿策略 Spec

## 0. 目标

本 Spec 用于指导 AIM Web 网页端的实时消息、Bot 回复广播、后台通知、断线重连和漏消息补偿设计。

核心目标：

```text
在线时尽量实时；
断线时不丢数据；
回到前台后能补齐；
网页关闭后不承诺像原生 App 一样持续推送。
```

本阶段不做原生 App 推送，不做 PWA Web Push，只做适合 Web 版 AIM 的稳定策略。

---

## 1. 背景

AIM 当前消息链路：

```text
用户发送消息
→ gateway WebSocket 收到 SEND_MESSAGE
→ gateway 调 chat-service CreateMessage
→ chat-service 写入 message 表
→ gateway 广播 NEW_MESSAGE
```

Bot 回复链路：

```text
用户发送 @AIM
→ 用户消息正常落库
→ chat-service 后台异步生成 Bot 回复
→ Bot 回复写入 message 表
```

问题：

```text
异步 Bot 回复由 chat-service 后台 goroutine 生成；
gateway 不参与这次请求；
所以 gateway 默认不知道 Bot 回复已产生；
因此不能自动通过现有 WebSocket NEW_MESSAGE 广播。
```

推荐方案：

```text
chat-service 生成 Bot 回复后发布 Redis Pub/Sub 事件；
gateway 订阅事件；
gateway 复用现有 NEW_MESSAGE 事件广播给在线用户。
```

---

## 2. 设计原则

### 2.1 message 表是唯一可靠来源

WebSocket、Redis Pub/Sub、浏览器通知都只是实时通知手段。

真正可靠的数据来源是：

```text
messages 表
conversation.last_message_id
conversation_members.last_read_message_id
```

因此：

```text
如果 WebSocket 消息漏收，前端必须能通过历史消息接口补齐；
如果 Redis Pub/Sub 事件丢失，消息仍然在数据库里；
如果浏览器后台断开，用户回到前台后重新拉取最新数据。
```

### 2.2 WebSocket 只保证在线实时，不保证离线可靠

WebSocket 适合：

```text
页面打开时实时收消息
页面后台但连接未断时继续收消息
当前会话实时显示
```

WebSocket 不适合保证：

```text
页面关闭后继续收消息
电脑休眠后继续收消息
手机锁屏后继续收消息
网络切换期间不漏消息
```

### 2.3 网页版不承诺原生 App 级后台通知

网页关闭后，页面 JavaScript 和 WebSocket 都不存在。

因此 Web 版 AIM 当前阶段不承诺：

```text
关闭浏览器后仍像 QQ/微信一样弹通知
锁屏后持续收消息
系统级稳定后台保活
```

后续如需实现类似能力，应另开 PWA Web Push 或原生客户端推送方案。

---

## 3. WebSocket 连接策略

### 3.1 连接时机

用户登录成功后建立 WebSocket：

```text
GET /ws/chat
```

WebSocket 鉴权沿用现有方式：

```text
query token
Cookie access_token
Authorization Bearer
```

### 3.2 收到 NEW_MESSAGE

前端收到：

```json
{
  "type": "NEW_MESSAGE",
  "payload": {
    "id": 123,
    "conversationId": "c_xxx",
    "senderId": 1,
    "senderType": "USER",
    "messageType": "TEXT",
    "content": "hello",
    "createdAt": "..."
  }
}
```

处理规则：

```text
1. 根据 message.id 去重。
2. 如果属于当前会话，追加到当前消息列表。
3. 如果不属于当前会话，更新对应会话 lastMessage，并增加运行态未读数。
4. 如果页面不在前台，可尝试弹浏览器 Notification。
```

### 3.3 消息去重

前端本地必须按 `message.id` 去重。

原因：

```text
同一条消息可能通过 WebSocket 收到；
也可能在重连后通过历史消息接口再次拉到；
不能重复显示。
```

推荐逻辑：

```ts
function mergeMessages(oldMessages, incomingMessages) {
  const map = new Map()
  for (const msg of oldMessages) map.set(msg.id, msg)
  for (const msg of incomingMessages) map.set(msg.id, msg)
  return Array.from(map.values()).sort((a, b) => a.id - b.id)
}
```

---

## 4. 自动重连策略

### 4.1 断线后自动重连

WebSocket `onclose` 或 `onerror` 后，前端应自动重连。

推荐指数退避：

```text
第 1 次：1s
第 2 次：2s
第 3 次：5s
第 4 次：10s
后续最多：30s
```

不要高频无限重连。

### 4.2 重连成功后的补偿动作

WebSocket `onopen` 后必须执行：

```text
1. 标记 wsConnected = true。
2. 拉取会话列表。
3. 拉取当前会话最近消息。
4. 用 message.id 去重合并。
5. 清理当前会话运行态未读数。
```

如果后端支持 `afterId`，优先使用：

```text
GET /api/v1/conversations/:conversationId/messages?afterId=lastLocalMessageId
```

如果暂时不支持 `afterId`，第一版使用：

```text
GET /api/v1/conversations/:conversationId/messages?limit=50
```

然后前端去重合并。

### 4.3 不要只依赖 WebSocket 重连

即使 WebSocket 没断，后台页面也可能因为浏览器节流导致消息处理延迟。

因此还需要页面回前台刷新策略。

---

## 5. 页面可见性恢复策略

监听：

```js
document.addEventListener("visibilitychange", ...)
```

当页面从后台回到前台：

```text
document.visibilityState === "visible"
```

执行：

```text
1. 如果 WebSocket 已断开，立即重连。
2. 无论 WebSocket 是否断开，都刷新会话列表。
3. 刷新当前会话最近消息。
4. 用 message.id 去重合并。
5. 更新 lastMessage 和未读状态。
```

示例：

```ts
document.addEventListener("visibilitychange", () => {
  if (document.visibilityState === "visible") {
    reconnectIfNeeded()
    refreshConversations()
    refreshCurrentConversationMessages()
  }
})
```

这样即使后台期间漏掉 Bot 回复，也能在用户回来时补齐。

---

## 6. 浏览器通知策略

### 6.1 适用范围

浏览器 Notification API 只作为增强体验。

适用场景：

```text
网页仍打开；
用户已授权浏览器通知；
页面处于后台或非当前会话；
WebSocket 收到 NEW_MESSAGE。
```

不保证：

```text
网页关闭后还能通知；
手机锁屏后还能通知；
浏览器被系统杀死后还能通知。
```

### 6.2 权限申请

不要首次打开页面就强行申请通知权限。

推荐在用户进入设置页或第一次需要时提示：

```text
是否开启 AIM 浏览器通知？
```

用户点击后：

```ts
Notification.requestPermission()
```

### 6.3 通知触发条件

收到 `NEW_MESSAGE` 时：

```text
1. Notification.permission === "granted"
2. document.visibilityState !== "visible" 或消息不属于当前会话
3. 消息不是当前用户自己发送的
```

示例：

```ts
if (
  Notification.permission === "granted" &&
  document.visibilityState !== "visible" &&
  message.senderId !== currentUser.id
) {
  new Notification("AIM 新消息", {
    body: message.content || "收到一条新消息",
  })
}
```

### 6.4 通知内容安全

通知内容不要显示敏感信息过多。

第一版可显示：

```text
标题：AIM 新消息
正文：消息内容前 50 个字符
```

如果是 Bot 回复：

```text
标题：AIM Bot 回复
正文：Bot 回复内容前 50 个字符
```

---

## 7. Bot 回复广播策略

### 7.1 当前问题

用户消息广播路径：

```text
gateway 收到 SEND_MESSAGE
→ 调 chat-service CreateMessage
→ 得到 MessageInfo
→ gateway 调 broadcastNewMessage
→ Hub.SendToUsers(..., EventNewMessage)
```

Bot 回复路径：

```text
chat-service 后台 goroutine 生成 Bot 回复
→ 写 message 表
→ gateway 不知道
```

所以 Bot 回复不会自动实时广播。

### 7.2 推荐最小正式方案：Redis Pub/Sub

新增 Redis channel：

```text
aim:bot_reply_created
```

事件名：

```text
BotReplyCreated
```

事件 payload 建议：

```json
{
  "message": {
    "id": 102,
    "conversationId": "c_xxx",
    "senderId": 1,
    "senderType": "BOT",
    "messageType": "BOT_REPLY",
    "content": "AI 回复内容",
    "createdAt": "..."
  },
  "recipientUserIds": [1, 2, 3]
}
```

注意：

```text
message 使用 gateway 已有 MessageInfo JSON 字段结构；
recipientUserIds 是当前会话成员 user_id 列表。
```

**P3 收件人约束**（来自 `p3-ai-bot-overview.md`）：

```text
recipientUserIds 必须只包含 member_type=USER 且 status=NORMAL 的成员。
不得把 BOT 成员的 member_id 当作 userId 推送。
所有广播（普通消息 / Bot 回复 / 系统消息）都遵循此规则。
```

### 7.3 chat-service 发布事件

Bot 回复成功写入 message 表后：

```text
1. 查询会话成员 user_id 列表。
2. 组装 MessageInfo 结构。
3. publish 到 Redis channel aim:bot_reply_created。
```

伪代码：

```go
event := BotReplyCreatedEvent{
    Message:          messageInfo,
    RecipientUserIDs: memberIDs,
}

publisher.Publish(ctx, "aim:bot_reply_created", event)
```

发布失败处理：

```text
1. 记录日志。
2. 不回滚 message 表。
3. 不影响 Bot 回复已生成的事实。
```

原因：

```text
Redis Pub/Sub 只是实时通知；
message 表才是可靠存储。
```

### 7.4 gateway 订阅事件

gateway 启动时启动 subscriber：

```text
subscribe aim:bot_reply_created
```

收到事件后：

```text
1. 解析 message。
2. 解析 recipientUserIds。
3. 复用现有 NEW_MESSAGE 事件。
4. 调 Hub.SendToUsers(recipientUserIds, EventNewMessage, message)。
```

不要新增 WebSocket 事件类型。

前端继续只处理：

```text
NEW_MESSAGE
```

### 7.5 Pub/Sub 丢事件怎么办

Redis Pub/Sub 不保存历史事件。

如果 gateway 重启期间事件丢失：

```text
前端不会实时收到；
但 message 表已经有 Bot 回复；
用户重连或回到前台时会拉历史消息补齐。
```

这是可接受的第一版设计。

如果后续需要可靠服务间事件，可改为：

```text
Redis Stream
Kafka
RabbitMQ
```

---

## 8. 前端补消息策略

### 8.1 重连后补

WebSocket 重连成功后：

```text
refreshConversations()
refreshCurrentMessages()
```

### 8.2 回前台补

页面 `visibilitychange` 到 visible 后：

```text
refreshConversations()
refreshCurrentMessages()
```

### 8.3 手动刷新补

用户可以点击刷新按钮：

```text
刷新会话
刷新当前消息
```

### 8.4 去重合并

所有补拉消息都必须按 `message.id` 去重。

---

## 9. 未读数策略

### 9.1 当前阶段

当前阶段可以继续使用前端运行态未读数：

```text
收到非当前会话 NEW_MESSAGE
→ 未读 +1
进入会话
→ 未读清零
```

但需要在文档中说明：

```text
刷新页面后未读数不保证保留。
```

### 9.2 后续持久化方案

后续使用：

```text
conversation_members.last_read_message_id
```

实现：

```text
持久化未读
多端一致
已读回执
```

推荐接口：

```text
POST /api/v1/conversations/:conversationId/read
body: { lastReadMessageId }
```

会话列表返回：

```text
unreadCount
lastReadMessageId
```

---

## 10. 不做的内容

当前阶段不做：

```text
PWA Web Push
Service Worker Push
iOS APNs
Android 厂商推送
FCM
桌面客户端系统后台进程
关闭浏览器后的稳定通知
消息撤回后的跨端通知一致性
可靠消息队列
```

---

## 11. 验收标准

### 11.1 WebSocket 实时

```text
1. 页面在线时，用户消息能实时收到 NEW_MESSAGE。
2. 页面在线时，Bot 回复通过 Redis Pub/Sub 后能实时收到 NEW_MESSAGE。
3. 前端不需要新增 BOT_REPLY_* 事件。
```

### 11.2 断线重连

```text
1. 手动断开 WebSocket 后，前端能自动重连。
2. 重连成功后拉取会话列表。
3. 重连成功后拉取当前会话最近消息。
4. 断线期间生成的 Bot 回复能通过历史消息补齐。
```

### 11.3 后台恢复

```text
1. 切到后台后再切回前台，会刷新会话列表。
2. 切回前台后，会刷新当前会话消息。
3. 后台期间漏掉的 Bot 回复能补齐。
```

### 11.4 浏览器通知

```text
1. 用户授权通知后，页面后台收到 NEW_MESSAGE 可弹通知。
2. 自己发送的消息不弹通知。
3. 没有授权时不报错。
```

---

## 12. 推荐 Codex 任务拆分

### Task A：只评估现有 WebSocket 重连

```text
只读：
- frontend/src/App.tsx
- frontend/src/api.ts
- frontend/src/types.ts
- gateway/internal/websocket/client.go
- gateway/internal/websocket/hub.go

任务：
检查当前前端是否已有 WebSocket 自动重连、visibilitychange 刷新、message.id 去重。
不要修改文件。
输出缺口清单和最小修改计划。
```

### Task B：前端补消息策略

```text
只读：
- docs/specs/ws-notification-spec.md
- frontend/src/App.tsx
- frontend/src/api.ts
- frontend/src/types.ts

任务：
实现 WebSocket 断线重连、页面回前台刷新会话和当前消息、message.id 去重合并。
不要修改 gateway、chat-service、IDL。

验证：
- npm run build --prefix frontend
```

### Task C：浏览器通知

```text
只读：
- docs/specs/ws-notification-spec.md
- frontend/src/App.tsx
- frontend/src/styles.css

任务：
增加浏览器 Notification API 支持。
只在用户授权后、页面后台收到非本人 NEW_MESSAGE 时弹通知。
不要实现 PWA Push。
不要修改后端。

验证：
- npm run build --prefix frontend
```

### Task D：Bot 回复 Redis Pub/Sub 方案评估

```text
只读：
- docs/specs/ws-notification-spec.md
- chat-service/internal/bot/service.go
- chat-service/internal/repository/chat.go
- gateway/internal/websocket/hub.go
- gateway/internal/websocket/event.go
- gateway/internal/websocket/client.go
- docker-compose.yml

任务：
评估 chat-service 发布 BotReplyCreated 到 Redis Pub/Sub，gateway 订阅后复用 NEW_MESSAGE 广播的最小改动。
不要修改文件。
输出具体修改计划。
```

### Task E：实现 Bot 回复 Redis Pub/Sub

```text
只读：
- docs/specs/ws-notification-spec.md
- chat-service/internal/bot/service.go
- chat-service/internal/repository/chat.go
- gateway/internal/websocket/hub.go
- gateway/internal/websocket/event.go
- docker-compose.yml

任务：
实现 Bot 回复 Redis Pub/Sub 实时广播。
要求：
1. chat-service 在 Bot 回复落库后发布事件。
2. gateway 订阅事件。
3. gateway 复用 NEW_MESSAGE 广播。
4. 不新增 WebSocket 事件类型。
5. Pub/Sub 发布失败不回滚消息。
6. 不修改 frontend 协议。

验证：
- go test ./... in chat-service
- go test ./... in gateway
- go build ./... in chat-service
- go build ./... in gateway
```

---

## 13. 后续规划

如果后续需要接近 QQ/微信体验，可以考虑：

```text
PWA + Service Worker + Web Push
Electron / Tauri 桌面客户端
Android 原生 App + 厂商推送
iOS App + APNs
Redis Stream / Kafka 做可靠服务间事件
conversation_members.last_read_message_id 做多端未读一致性
```

但这些不属于当前阶段。

### 当前并发方案

P3 阶段 Bot 并发控制使用 **goroutine + semaphore（in-memory）**，不引入 Redis Stream 或外部任务队列。具体实现见 `chat-service/internal/bot/concurrency.go`。

后续如需可靠服务间事件或持久化任务队列，可从 Redis Stream / Kafka 中选择。
