# AIM WebSocket API

当前阶段 WebSocket 只实现基础群聊文本消息：

- 连接鉴权
- `SEND_MESSAGE`
- `MESSAGE_ACK`
- `NEW_MESSAGE`

暂不实现 Bot、RAG、文件消息、图片消息、语音消息、撤回、已读回执、离线推送和消息队列。

## 连接

```http
GET /ws/chat?token=<access_token>
```

也支持通过 Cookie `access_token` 或请求头传入：

```http
Authorization: Bearer <access_token>
```

连接成功后服务端返回：

```json
{
  "type": "CONNECTED",
  "data": {
    "userId": 10001
  }
}
```

## 发送消息

客户端发送：

```json
{
  "type": "SEND_MESSAGE",
  "clientMsgId": "tmp-123456",
  "data": {
    "conversationId": "c_abc123",
    "content": "hello",
    "replyToId": null
  }
}
```

发送成功 ACK：

```json
{
  "type": "MESSAGE_ACK",
  "clientMsgId": "tmp-123456",
  "data": {
    "messageId": 101,
    "status": "SUCCESS"
  }
}
```

发送失败 ACK：

```json
{
  "type": "MESSAGE_ACK",
  "clientMsgId": "tmp-123456",
  "data": {
    "status": "FAILED",
    "errorCode": "FORBIDDEN",
    "errorMessage": "user is not a member of this conversation"
  }
}
```

## 新消息广播

同一会话的在线成员会收到：

```json
{
  "type": "NEW_MESSAGE",
  "data": {
    "id": 101,
    "conversationId": "c_abc123",
    "senderId": 10001,
    "senderType": "USER",
    "messageType": "TEXT",
    "content": "hello",
    "replyToId": null,
    "status": "NORMAL",
    "createdAt": 1777360800
  }
}
```

## 测试建议

1. 登录两个不同用户，分别拿到 `access_token`。
2. 用户 A 创建群聊，用户 B 加入群聊。
3. 两个 WebSocket 客户端分别连接：

```text
ws://localhost:8080/ws/chat?token=<access_token>
```

4. 用户 A 发送 `SEND_MESSAGE`。
5. 用户 A 应收到 `MESSAGE_ACK`，用户 A 和用户 B 都应收到 `NEW_MESSAGE`。
6. 使用 `GET /api/v1/conversations/{conversationId}/messages` 查询历史消息，确认消息已落库。
