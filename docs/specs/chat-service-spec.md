# Chat Service Spec

## 1. 背景

AIM 是一个 AI 原生多人协作聊天平台。当前阶段已完成鉴权模块迁移，接下来需要实现基础聊天能力，为后续 AI Bot 和 RAG 知识库功能提供消息底座。

本阶段重点是实现：

- 群聊会话创建
- 群成员管理
- WebSocket 实时消息
- 消息持久化
- 历史消息查询

本阶段不实现 Bot、RAG、文件消息、语音消息、复杂已读回执。

---

## 2. 目标

实现一个基础可用的 chat-service，使用户可以：

1. 登录后创建群聊
2. 加入已有群聊
3. 查询自己的会话列表
4. 建立 WebSocket 连接
5. 在群聊中发送文本消息
6. 群内在线成员实时收到消息
7. 消息落库
8. 刷新页面后可以查询历史消息

最终验收标准：

> 两个不同用户登录后，可以加入同一个群聊，并通过 WebSocket 实时收发文本消息；刷新页面后可以看到历史消息。

---

## 3. 本阶段范围

### 3.1 需要实现

- Conversation 会话模块
- GroupInfo 群聊信息模块
- ConversationMember 会话成员模块
- Message 消息模块
- WebSocket 实时通信模块
- 基础权限校验
- 基础接口测试

### 3.2 暂不实现

- AI Bot
- RAG
- 单聊
- 文件消息
- 图片消息
- 语音消息
- 消息撤回
- 消息编辑
- 消息已读详情
- 离线推送
- 消息队列
- 多端同步

---

## 4. 数据模型

### 4.1 Conversation

用于统一表示聊天会话。

```go
type ConversationType string

const (
	ConversationTypeSingle ConversationType = "SINGLE"
	ConversationTypeGroup  ConversationType = "GROUP"
	ConversationTypeBot    ConversationType = "BOT"
	ConversationTypeSystem ConversationType = "SYSTEM"
)

type Conversation struct {
	ID            uint64           `gorm:"primaryKey;autoIncrement" json:"id"`
	Type          ConversationType `gorm:"type:varchar(32);not null;index" json:"type"`
	Title         string           `gorm:"type:varchar(128)" json:"title"`
	Avatar        string           `gorm:"type:varchar(512)" json:"avatar"`
	CreatedBy     uint64           `gorm:"not null;index" json:"createdBy"`
	LastMessageID *uint64          `gorm:"index" json:"lastMessageId"`
	LastMessageAt *time.Time       `gorm:"index" json:"lastMessageAt"`
	CreatedAt     time.Time        `json:"createdAt"`
	UpdatedAt     time.Time        `json:"updatedAt"`
	DeletedAt     gorm.DeletedAt   `gorm:"index" json:"-"`
}
```

### 4.2 GroupInfo

用于存储群聊专属信息。

```go
type GroupJoinPolicy string

const (
	GroupJoinFree       GroupJoinPolicy = "FREE"
	GroupJoinApproval   GroupJoinPolicy = "APPROVAL"
	GroupJoinInviteOnly GroupJoinPolicy = "INVITE_ONLY"
)

type GroupInfo struct {
	ID             uint64          `gorm:"primaryKey;autoIncrement" json:"id"`
	ConversationID uint64          `gorm:"not null;uniqueIndex" json:"conversationId"`
	Name           string          `gorm:"type:varchar(128);not null" json:"name"`
	Avatar         string          `gorm:"type:varchar(512)" json:"avatar"`
	Announcement   string          `gorm:"type:text" json:"announcement"`
	OwnerID        uint64          `gorm:"not null;index" json:"ownerId"`
	JoinPolicy     GroupJoinPolicy `gorm:"type:varchar(32);not null;default:'INVITE_ONLY'" json:"joinPolicy"`
	MuteAll        bool            `gorm:"not null;default:false" json:"muteAll"`
	MaxMembers     int             `gorm:"not null;default:500" json:"maxMembers"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}
```

### 4.3 ConversationMember

用于记录用户和会话的成员关系。

```go
type ConversationMemberRole string

const (
	MemberRoleOwner  ConversationMemberRole = "OWNER"
	MemberRoleAdmin  ConversationMemberRole = "ADMIN"
	MemberRoleMember ConversationMemberRole = "MEMBER"
	MemberRoleBot    ConversationMemberRole = "BOT"
)

type ConversationMemberStatus string

const (
	MemberStatusNormal  ConversationMemberStatus = "NORMAL"
	MemberStatusMuted   ConversationMemberStatus = "MUTED"
	MemberStatusRemoved ConversationMemberStatus = "REMOVED"
)

type ConversationMember struct {
	ID                uint64                   `gorm:"primaryKey;autoIncrement" json:"id"`
	ConversationID    uint64                   `gorm:"not null;index:idx_conversation_user,unique" json:"conversationId"`
	UserID            uint64                   `gorm:"not null;index:idx_conversation_user,unique" json:"userId"`
	Role              ConversationMemberRole   `gorm:"type:varchar(32);not null;default:'MEMBER'" json:"role"`
	Status            ConversationMemberStatus `gorm:"type:varchar(32);not null;default:'NORMAL'" json:"status"`
	NicknameInGroup   string                   `gorm:"type:varchar(64)" json:"nicknameInGroup"`
	MuteUntil         *time.Time               `json:"muteUntil"`
	IsPinned          bool                     `gorm:"not null;default:false" json:"isPinned"`
	IsMuted           bool                     `gorm:"not null;default:false" json:"isMuted"`
	LastReadMessageID *uint64                  `gorm:"index" json:"lastReadMessageId"`
	JoinedAt          time.Time                `gorm:"not null" json:"joinedAt"`
	CreatedAt         time.Time                `json:"createdAt"`
	UpdatedAt         time.Time                `json:"updatedAt"`
}
```

### 4.4 Message

用于存储所有会话消息。

```go
type SenderType string

const (
	SenderTypeUser   SenderType = "USER"
	SenderTypeBot    SenderType = "BOT"
	SenderTypeSystem SenderType = "SYSTEM"
)

type MessageType string

const (
	MessageTypeText     MessageType = "TEXT"
	MessageTypeImage    MessageType = "IMAGE"
	MessageTypeFile     MessageType = "FILE"
	MessageTypeVoice    MessageType = "VOICE"
	MessageTypeBotReply MessageType = "BOT_REPLY"
	MessageTypeSystem   MessageType = "SYSTEM"
)

type MessageStatus string

const (
	MessageStatusNormal   MessageStatus = "NORMAL"
	MessageStatusRecalled MessageStatus = "RECALLED"
	MessageStatusDeleted  MessageStatus = "DELETED"
	MessageStatusFailed   MessageStatus = "FAILED"
)

type Message struct {
	ID             uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	ConversationID uint64         `gorm:"not null;index:idx_conversation_created" json:"conversationId"`
	SenderID       uint64         `gorm:"not null;index" json:"senderId"`
	SenderType     SenderType     `gorm:"type:varchar(32);not null" json:"senderType"`
	MessageType    MessageType    `gorm:"type:varchar(32);not null;default:'TEXT'" json:"messageType"`
	Content        string         `gorm:"type:text" json:"content"`
	ReplyToID      *uint64        `gorm:"index" json:"replyToId"`
	Status         MessageStatus  `gorm:"type:varchar(32);not null;default:'NORMAL'" json:"status"`
	CreatedAt      time.Time      `gorm:"index:idx_conversation_created" json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`

	ReplyTo *Message `gorm:"foreignKey:ReplyToID" json:"-"`
}
```

---

## 5. REST API 设计

### 5.1 创建群聊

```http
POST /api/conversations/group
```

#### 请求头

```http
Authorization: Bearer <access_token>
```

#### 请求体

```json
{
  "name": "AIM 测试群",
  "avatar": "",
  "announcement": "欢迎使用 AIM",
  "joinPolicy": "INVITE_ONLY"
}
```

#### 处理逻辑

1. 从 JWT 中解析当前用户 `userId`
2. 校验 `name` 不为空，长度不超过 128
3. 开启数据库事务
4. 创建 `conversation`
   - `type = GROUP`
   - `title = name`
   - `avatar = avatar`
   - `created_by = userId`
5. 创建 `group_info`
   - `conversation_id = conversation.ID`
   - `owner_id = userId`
   - `name = name`
6. 创建 `conversation_member`
   - `conversation_id = conversation.ID`
   - `user_id = userId`
   - `role = OWNER`
   - `status = NORMAL`
7. 提交事务
8. 返回群聊信息

#### 响应

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "conversationId": 1,
    "type": "GROUP",
    "name": "AIM 测试群",
    "ownerId": 10001
  }
}
```

---

### 5.2 查询我的会话列表

```http
GET /api/conversations
```

#### 请求头

```http
Authorization: Bearer <access_token>
```

#### 查询逻辑

1. 从 JWT 中解析当前用户 `userId`
2. 查询 `conversation_member` 中 `user_id = userId` 且 `status != REMOVED` 的记录
3. 关联查询 `conversation`
4. 按 `conversation.last_message_at DESC` 排序
5. 返回会话列表

#### 响应

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "conversationId": 1,
      "type": "GROUP",
      "title": "AIM 测试群",
      "avatar": "",
      "lastMessageId": 101,
      "lastMessageAt": "2026-04-28T10:00:00Z",
      "role": "OWNER",
      "isPinned": false,
      "isMuted": false
    }
  ]
}
```

---

### 5.3 查询群成员

```http
GET /api/conversations/{conversationId}/members
```

#### 权限规则

只有当前会话成员可以查询成员列表。

#### 响应

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "userId": 10001,
      "nickname": "小青",
      "avatar": "",
      "role": "OWNER",
      "status": "NORMAL",
      "joinedAt": "2026-04-28T10:00:00Z"
    }
  ]
}
```

---

### 5.4 加入群聊

```http
POST /api/conversations/{conversationId}/members
```

#### 本阶段规则

第一版只支持自由加入或邀请加入的简化逻辑。

处理逻辑：

1. 校验 conversation 存在
2. 校验 conversation.type = GROUP
3. 校验当前用户是否已经是成员
4. 如果不是成员，插入 `conversation_member`
5. 角色默认为 `MEMBER`

#### 暂不实现

- 入群审批
- 邀请码
- 黑名单
- 群人数复杂限制

---

### 5.5 退出群聊

```http
DELETE /api/conversations/{conversationId}/members/me
```

#### 规则

1. 普通成员可以退出群聊
2. 群主暂不允许直接退出
3. 群主退出需要先转让群主，本阶段不实现转让

---

### 5.6 查询历史消息

```http
GET /api/conversations/{conversationId}/messages?beforeId=10086&limit=30
```

#### 权限规则

只有当前会话成员可以查询历史消息。

#### 参数说明

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `beforeId` | uint64 | 否 | 查询该消息 ID 之前的消息 |
| `limit` | int | 否 | 每次查询数量，默认 30，最大 100 |

#### 查询逻辑

1. 校验当前用户是会话成员
2. 如果 `beforeId` 为空，查询最新消息
3. 如果 `beforeId` 不为空，查询 `id < beforeId` 的消息
4. 按 `id DESC` 查询
5. 返回时可以按时间正序或倒序，由前端决定

#### 响应

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 101,
      "conversationId": 1,
      "senderId": 10001,
      "senderType": "USER",
      "messageType": "TEXT",
      "content": "hello",
      "replyToId": null,
      "status": "NORMAL",
      "createdAt": "2026-04-28T10:00:00Z"
    }
  ]
}
```

---

## 6. WebSocket 设计

### 6.1 连接地址

```http
GET /ws/chat?token=<access_token>
```

或者使用请求头：

```http
Authorization: Bearer <access_token>
```

第一版可以使用 query 参数传 token，后续再改为 Header 或子协议。

---

### 6.2 连接建立逻辑

1. 客户端携带 JWT 建立连接
2. 服务端校验 JWT
3. 解析 `userId`
4. 绑定 `userId -> websocket connection`
5. 将连接加入连接管理器
6. 返回连接成功事件

服务端返回：

```json
{
  "type": "CONNECTED",
  "data": {
    "userId": 10001
  }
}
```

---

### 6.3 客户端发送消息事件

```json
{
  "type": "SEND_MESSAGE",
  "clientMsgId": "tmp-123456",
  "data": {
    "conversationId": 1,
    "content": "hello",
    "replyToId": null
  }
}
```

字段说明：

| 字段 | 说明 |
|---|---|
| `type` | 事件类型 |
| `clientMsgId` | 前端生成的临时消息 ID，用于匹配 ACK |
| `conversationId` | 会话 ID |
| `content` | 文本内容 |
| `replyToId` | 被回复消息 ID，可为空 |

---

### 6.4 服务端处理 SEND_MESSAGE

处理步骤：

1. 校验 WebSocket 连接已认证
2. 校验 `conversationId` 存在
3. 校验当前用户是该会话成员
4. 校验成员状态不是 `REMOVED`
5. 校验用户未被禁言
6. 校验群未开启全员禁言，或者当前用户是 OWNER / ADMIN
7. 校验 `content` 不为空
8. 创建 `message`
   - `sender_id = userId`
   - `sender_type = USER`
   - `message_type = TEXT`
   - `status = NORMAL`
9. 更新 `conversation.last_message_id`
10. 更新 `conversation.last_message_at`
11. 向发送者返回 ACK
12. 向该会话在线成员广播 NEW_MESSAGE

---

### 6.5 发送成功 ACK

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

---

### 6.6 发送失败 ACK

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

---

### 6.7 服务端广播新消息

```json
{
  "type": "NEW_MESSAGE",
  "data": {
    "id": 101,
    "conversationId": 1,
    "senderId": 10001,
    "senderType": "USER",
    "messageType": "TEXT",
    "content": "hello",
    "replyToId": null,
    "status": "NORMAL",
    "createdAt": "2026-04-28T10:00:00Z"
  }
}
```

---

## 7. 权限规则

### 7.1 会话访问权限

用户只有在 `conversation_member` 中存在正常成员关系时，才能访问该会话。

判断条件：

```text
conversation_member.conversation_id = conversationId
conversation_member.user_id = currentUserId
conversation_member.status != REMOVED
```

---

### 7.2 消息发送权限

用户发送消息前必须满足：

1. 用户是会话成员
2. 用户状态正常
3. 成员状态正常
4. 用户未被禁言
5. 如果群开启全员禁言，则只有 OWNER / ADMIN 可以发送消息

---

### 7.3 群管理权限

第一版只需要：

| 操作 | 权限 |
|---|---|
| 创建群聊 | 登录用户 |
| 查询群成员 | 群成员 |
| 加入群聊 | 登录用户，后续根据 joinPolicy 扩展 |
| 退出群聊 | 普通成员 |
| 发送群消息 | 正常群成员 |
| 修改群公告 | OWNER / ADMIN，第一版可暂不实现 |

---

## 8. 错误码

| 错误码 | HTTP 状态码 | 说明 |
|---|---:|---|
| `UNAUTHORIZED` | 401 | 未登录或 Token 无效 |
| `FORBIDDEN` | 403 | 无权限 |
| `NOT_FOUND` | 404 | 资源不存在 |
| `BAD_REQUEST` | 400 | 请求参数错误 |
| `CONVERSATION_NOT_FOUND` | 404 | 会话不存在 |
| `NOT_CONVERSATION_MEMBER` | 403 | 当前用户不是会话成员 |
| `MEMBER_MUTED` | 403 | 当前用户被禁言 |
| `GROUP_MUTED_ALL` | 403 | 当前群开启全员禁言 |
| `MESSAGE_EMPTY` | 400 | 消息内容为空 |
| `INTERNAL_ERROR` | 500 | 服务内部错误 |

---

## 9. 事务要求

### 9.1 创建群聊事务

创建群聊必须在一个事务中完成：

1. 创建 `conversation`
2. 创建 `group_info`
3. 创建 `conversation_member`

任一步失败都需要回滚。

---

### 9.2 发送消息事务

发送消息建议在一个事务中完成：

1. 创建 `message`
2. 更新 `conversation.last_message_id`
3. 更新 `conversation.last_message_at`

事务提交后再进行 WebSocket 广播。

原因：

> 避免消息已经广播，但数据库写入失败。

---

## 10. WebSocket 连接管理

### 10.1 连接管理器

需要维护：

```text
userId -> connections
conversationId -> online userIds
connection -> userId
```

第一版可以只维护内存 Map。

示例：

```go
type Hub struct {
	userConns map[uint64]map[*Client]bool
	mu        sync.RWMutex
}
```

如果一个用户多开浏览器，可以允许一个用户对应多个连接。

---

### 10.2 广播逻辑

发送 `NEW_MESSAGE` 时：

1. 查询该 conversation 的成员列表
2. 找出当前在线成员
3. 向在线成员的连接发送消息

第一版可以每次从数据库查成员。

后续优化可以用 Redis 缓存会话成员。

---

### 10.3 断开连接

连接断开时：

1. 从连接管理器中删除当前连接
2. 如果该用户没有其他连接，可以认为用户离线
3. 本阶段不需要持久化在线状态

---

## 11. 测试要求

### 11.1 Conversation 测试

需要覆盖：

- 登录用户可以创建群聊
- 创建群聊后生成 conversation
- 创建群聊后生成 group_info
- 创建群聊后创建者成为 OWNER
- 未登录用户不能创建群聊

---

### 11.2 Member 测试

需要覆盖：

- 用户可以加入群聊
- 重复加入群聊应该失败或幂等返回
- 群成员可以查询群成员列表
- 非群成员不能查询群成员列表
- 普通成员可以退出群聊
- 群主不能直接退出群聊

---

### 11.3 Message 测试

需要覆盖：

- 群成员可以发送消息
- 非群成员不能发送消息
- 空消息不能发送
- 被禁言成员不能发送消息
- 群全员禁言时普通成员不能发送消息
- 消息发送成功后写入 message 表
- 消息发送成功后更新 conversation.last_message_id

---

### 11.4 WebSocket 测试

需要覆盖：

- 有效 Token 可以建立连接
- 无效 Token 不能建立连接
- 两个用户在同一群时可以实时收到消息
- 非群成员通过 WebSocket 发送消息失败
- 发送成功后客户端收到 MESSAGE_ACK
- 群成员收到 NEW_MESSAGE

---

## 12. 验收标准

本阶段完成后，应满足：

1. 可以通过 REST API 创建群聊
2. 可以查询当前用户的会话列表
3. 可以加入群聊
4. 可以查询群成员列表
5. 可以通过 WebSocket 建立连接
6. 可以通过 WebSocket 发送文本消息
7. 消息成功写入数据库
8. 同一群在线成员可以实时收到消息
9. 可以查询历史消息
10. 非群成员不能发送或查看该群消息
11. README 中提供接口说明和 WebSocket 消息格式说明

---

## 13. 推荐开发顺序

1. 完成 GORM 模型和数据库迁移
2. 实现 ConversationRepository
3. 实现 GroupRepository
4. 实现 MemberRepository
5. 实现 MessageRepository
6. 实现创建群聊接口
7. 实现查询会话列表接口
8. 实现加入群聊接口
9. 实现查询历史消息接口
10. 实现 WebSocket 鉴权连接
11. 实现 WebSocket 发送消息
12. 实现群内广播
13. 补充基础测试
14. 更新 README

---

## 14. 目录结构建议

```text
chat-service/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── config/
│   ├── middleware/
│   ├── model/
│   ├── conversation/
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repository.go
│   │   └── dto.go
│   ├── group/
│   │   ├── service.go
│   │   ├── repository.go
│   │   └── dto.go
│   ├── member/
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repository.go
│   │   └── dto.go
│   ├── message/
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repository.go
│   │   └── dto.go
│   └── websocket/
│       ├── hub.go
│       ├── client.go
│       ├── handler.go
│       ├── event.go
│       └── service.go
├── pkg/
│   ├── response/
│   ├── errors/
│   └── jwt/
├── docs/
│   └── websocket.md
├── go.mod
└── README.md
```

---

## 15. 后续扩展预留

本阶段完成后，后续可以在不推翻 chat-service 的基础上扩展：

### 15.1 AI Bot

Bot 回复作为普通消息写入 `message` 表：

```text
sender_type = BOT
message_type = BOT_REPLY
```

### 15.2 RAG

RAG 只作为 Bot 的上下文增强，不直接影响 chat-service 主链路。

### 15.3 文件消息

增加：

- `file_resource`
- `message_attachment`

同时复用 `message.message_type = FILE`

### 15.4 消息撤回

更新：

```text
message.status = RECALLED
```

并广播：

```json
{
  "type": "MESSAGE_RECALLED",
  "data": {
    "messageId": 101,
    "conversationId": 1
  }
}
```

### 15.5 已读位置

使用：

```text
conversation_member.last_read_message_id
```

表示用户在某个会话中读到哪条消息。
