# AIM P3 前置能力收口规格说明


## 0. 背景

当前 AIM 已具备基础聊天链路，但在 P3 AI Bot 完整上线前，IM 基础能力仍需收口：

```text
1. 消息类型目前只支持文本，需要补齐图片、文件、语音。
2. 消息已读回执尚未实现，需要支持单聊已读和群聊 N 人已读。
3. 群聊系统级提示尚未完整实现。
4. 群主、管理员、禁言、全员禁言等权限规则尚未完整实现。
5. 群公告尚未在前端展示，也缺少群主/管理员修改入口。
6. 消息回复尚未完整接入，但 message.reply_to_id 已具备基础字段。
```

本规格目标是将上述能力合并为较完整、可验收的功能任务，减少 Codex 反复切换上下文的消耗。

---

## 1. 总体原则

### 1.1 不推翻现有聊天主链路

实现必须基于现有：

```text
conversation
conversation_members
messages
gateway
chat-service
WebSocket NEW_MESSAGE
```

不得重写聊天主链路。

### 1.2 message 表是唯一可靠消息来源

以下内容都必须落库为 message 或 message 相关扩展数据：

```text
TEXT
IMAGE
FILE
VOICE
SYSTEM
BOT_REPLY
回复消息
```

WebSocket 只负责实时同步，不作为可靠存储。

### 1.3 WebSocket 复用 NEW_MESSAGE

以下类型必须继续复用：

```text
NEW_MESSAGE
```

```text
TEXT
IMAGE
FILE
VOICE
SYSTEM
BOT_REPLY
```

不得为了系统提示、图片、文件、语音、回复消息新增独立 WebSocket 连接。
允许扩展现有 `SEND_MESSAGE` / `NEW_MESSAGE` payload 字段，以承载 `messageType`、结构化 `content`、`replyToId` 等信息；但不得新增独立 WebSocket 事件类型或第二条消息通道。

### 1.4 SYSTEM message 不是通知提醒

`SYSTEM` message 是会话消息流的一部分，不是通知中心提醒。

服务端可以通过 `NEW_MESSAGE` 将 `SYSTEM` message 同步给当前在线的会话成员，使正在查看该群聊的用户实时看到系统提示行。

前端收到 `SYSTEM` message 后必须：

```text
1. 在聊天消息流中以居中灰色系统提示行展示。
2. 不显示头像。
3. 不显示普通消息气泡。
4. 不触发浏览器 Notification。
5. 不播放声音。
6. 不作为普通消息强提醒。
7. 不进入通知中心。
```

如果用户离线或不在当前会话，后续拉取历史消息时仍能看到该 `SYSTEM` message。

### 1.5 管理提醒与 SYSTEM message 分离

需要管理员或群主处理的事项，不得通过群内 `SYSTEM` message 提醒所有群成员。

例如：

```text
入群申请
举报
审核类事项
成员风控提醒
```

这类事项后续必须使用独立机制：

```text
ADMIN_NOTICE
notifications 表
```

并且只能投递给：

```text
OWNER
ADMIN
```

本规格不实现管理通知中心，只明确边界。

### 1.6 WebSocket 收件人只包含 USER 成员

所有 conversation 级消息同步收件人必须只来自：

```text
member_type = USER
status = NORMAL
```

不得把 BOT 成员作为 WebSocket 收件人。

### 1.7 不影响 P3 Bot 成员化

本规格不得破坏 P3 Bot 规格中的结论：

```text
Bot 可以作为 conversation_members 的 BOT 成员。
WebSocket 广播仍只发给 USER 成员。
Bot 回复不走普通用户发消息权限。
```

### 1.8 任务执行规则

Codex 每次只执行一个 Task，除非人工明确要求合并。

每个 Task 执行前必须先阅读本 Task 的目标、范围、允许修改文件和禁止事项。

---

## 2. 数据模型要求

### 2.1 MessageType

必须支持：

```go
type MessageType string

const (
    MessageTypeText     MessageType = "TEXT"
    MessageTypeImage    MessageType = "IMAGE"
    MessageTypeFile     MessageType = "FILE"
    MessageTypeVoice    MessageType = "VOICE"
    MessageTypeSystem   MessageType = "SYSTEM"
    MessageTypeBotReply MessageType = "BOT_REPLY"
)
```

如果当前枚举已有部分值，不得破坏已有值。

### 2.2 message.content JSON 结构

新消息的 `content` 必须是 JSON 文本。

#### TEXT

```json
{"text":"hello"}
```

兼容期必须支持旧 TEXT 纯文本展示。

#### IMAGE

```json
{
  "url": "https://...",
  "name": "image.png",
  "size": 123456,
  "mimeType": "image/png",
  "width": 800,
  "height": 600
}
```

#### FILE

```json
{
  "url": "https://...",
  "name": "report.pdf",
  "size": 1234567,
  "mimeType": "application/pdf"
}
```

#### VOICE

```json
{
  "url": "https://...",
  "name": "voice.m4a",
  "size": 123456,
  "mimeType": "audio/mp4",
  "durationMs": 5600
}
```

#### SYSTEM

```json
{
  "eventType": "MEMBER_JOINED",
  "actorUserId": 1001,
  "targetUserIds": [1002],
  "text": "Alice 邀请 Bob 加入群聊"
}
```

### 2.3 reply_to_id

`messages.reply_to_id` 表示当前消息回复的原消息 ID。

规则：

```text
1. reply_to_id 可以为空。
2. reply_to_id 不为空时，原消息必须存在。
3. 原消息必须属于同一 conversation。
4. 不允许回复其他 conversation 的消息。
5. 消息列表必须返回被回复消息摘要。
```

### 2.4 已读回执

已读回执必须基于：

```text
conversation_members.last_read_message_id
```

单聊：

```text
对方 last_read_message_id >= message.id
```

群聊：

```text
read_count = 当前 conversation 中 USER + NORMAL 成员里 last_read_message_id >= message.id 的人数
```

BOT 成员不得计入已读人数。

成员禁言的 canonical 数据来源必须是：
```text
conversation_members.mute_until
```

被禁言成员在未被移出群聊前，`status` 仍保持 `NORMAL`。
禁言只影响“发送普通消息”的权限，不影响：
```text
1. 接收 NEW_MESSAGE
2. 计入群聊已读统计
3. 出现在正常成员列表中
```

### 2.5 群设置字段

群聊必须支持以下配置。若已有等价字段，必须复用，不得重复建语义相同字段：

```text
announcement
announcement_updated_by
announcement_updated_at
mute_all
mute_all_updated_by
mute_all_updated_at
```

上述字段统一落在：
```text
group_infos
```

---

## 3. SYSTEM message 规则

### 3.1 SYSTEM message 必须落库

所有群系统事件必须生成：

```text
message_type = SYSTEM
sender_type = SYSTEM
```

或项目中已有的等价系统发送者。

不得只通过 WebSocket 临时推送。

### 3.2 SYSTEM message 事件类型

`content.eventType` 必须使用固定枚举：

```text
MEMBER_JOINED
MEMBER_LEFT
MEMBER_INVITED
MEMBER_REMOVED
MEMBER_MUTED
MEMBER_UNMUTED
GROUP_MUTED
GROUP_UNMUTED
ADMIN_ADDED
ADMIN_REMOVED
OWNER_TRANSFERRED
ANNOUNCEMENT_UPDATED
BOT_JOINED
BOT_REMOVED
```

### 3.3 SYSTEM message 同步

`SYSTEM` message 落库后，可以通过：

```text
NEW_MESSAGE
```

同步给在线 USER + NORMAL 成员。

该同步的目的仅是更新当前会话消息流，不是通知提醒。

### 3.4 前端展示

前端必须将 `SYSTEM` message 渲染为：

```text
聊天记录中间的居中灰色提示行
```

不得将 `SYSTEM` message 渲染为：

```text
普通消息气泡
带头像消息
通知中心提醒
浏览器 Notification
声音提醒
强提醒未读
```

### 3.5 未读数

`SYSTEM` message 默认不得计入普通消息强提醒未读。
这里的“普通消息强提醒未读”仅指前端通知/提醒语义，不强制改变后端会话未读聚合口径；若后端后续单独区分未读类型，再由专门任务补充。

如现有未读系统暂时无法区分，必须在实现输出中标记 TODO，不得声称已完成通知语义隔离。

---

## 4. 群权限规则

### 4.1 OWNER 权限

OWNER 必须拥有：

```text
转让群主
设置管理员
取消管理员
禁言任意 USER 成员
解除任意 USER 成员禁言
全员禁言
解除全员禁言
修改群公告
添加/移除 Bot
踢出普通成员和管理员
```

OWNER 不得被 ADMIN 禁言或踢出。

### 4.2 ADMIN 权限

ADMIN 必须拥有：

```text
禁言 MEMBER
解除 MEMBER 禁言
修改群公告
添加/移除 Bot
踢出 MEMBER
```

ADMIN 不得：

```text
禁言 OWNER
禁言 ADMIN
踢出 OWNER
踢出 ADMIN
设置或取消管理员
转让群主
全员禁言
```

### 4.3 MEMBER 权限

MEMBER 不得执行任何群管理操作。

### 4.4 全员禁言

全员禁言时：

```text
MEMBER 不得发送普通消息。
OWNER / ADMIN 仍可发送消息。
SYSTEM / BOT_REPLY 不受全员禁言影响。
```

---

# 5. Task 0：Spec Review

## 目标

审查本规格是否可以进入实现。

本任务只读，不得修改文件，不得生成代码。

## 只读文件

```text
docs/specs/pre-p3-closure-spec.md
chat-service/internal/dal/model/chat.go
chat-service/internal/biz/chat.go
chat-service/internal/repository/chat.go
gateway/internal/websocket/*
frontend/src/App.tsx
frontend/src/api.ts
frontend/src/types.ts
```

## 审查内容

必须检查：

```text
1. 消息 content JSON 格式是否清楚。
2. 多消息类型是否影响旧 TEXT 消息。
3. 已读回执是否有明确数据来源。
4. SYSTEM message 是否明确为聊天流提示而非通知提醒。
5. 管理提醒是否已与 SYSTEM message 分离。
6. 群权限是否有冲突。
7. 全员禁言是否排除 OWNER/ADMIN。
8. WebSocket 是否复用 NEW_MESSAGE。
9. WebSocket 收件人是否只取 USER + NORMAL。
10. Task 是否拆得足够合理。
11. 是否存在“推荐”“建议”“可以”“最好”等模糊措辞。
```

## 输出格式

```text
Spec Review Result

1. Blocking Issues
2. Non-blocking Issues
3. Ambiguities
4. Risk Points
5. Suggested Spec Changes
6. Implementation Readiness: READY / NOT READY
```

---

# 6. Task 1：多消息类型

## 目标

一次性补齐多消息类型后端模型、发送接口和前端展示。

本任务不实现真实文件上传，只处理“已经有 URL 的媒体消息”。

## 范围

本任务包含：

```text
1. MessageType 扩展。
2. message.content JSON 格式统一。
3. TEXT 旧纯文本兼容。
4. IMAGE / FILE / VOICE 发送接口。
5. WebSocket NEW_MESSAGE 兼容多消息类型。
6. 前端展示 TEXT / IMAGE / FILE / VOICE。
```

## 只读文件

```text
docs/specs/pre-p3-closure-spec.md
idl/chat.thrift
chat-service/internal/dal/model/chat.go
chat-service/internal/handler/chat_service.go
chat-service/internal/biz/chat.go
chat-service/internal/repository/chat.go
gateway/internal/handler/chat.go
frontend/src/App.tsx
frontend/src/api.ts
frontend/src/types.ts
frontend/src/styles.css
```

## 后端要求

```text
1. MessageType 必须支持 TEXT / IMAGE / FILE / VOICE / SYSTEM / BOT_REPLY。
2. 新消息 content 必须存 JSON 文本。
3. 旧 TEXT 纯文本消息必须兼容读取。
4. 发送消息接口必须支持 message_type=IMAGE/FILE/VOICE。
5. IMAGE 必须校验 url/name/mimeType。
6. FILE 必须校验 url/name/size/mimeType。
7. VOICE 必须校验 url/name/mimeType/durationMs。
8. 消息落库后继续复用 NEW_MESSAGE。
9. `BOT_REPLY` 不在本任务内改造成 JSON content，继续保持现状兼容。
```

## 前端要求

```text
1. TEXT 显示文本。
2. IMAGE 显示图片预览。
3. FILE 显示文件名、大小、下载链接。
4. VOICE 显示语音播放控件和时长。
5. 旧纯文本 TEXT 必须兼容展示。
6. 不新增独立 WebSocket 协议或独立事件类型；允许扩展现有 `SEND_MESSAGE` / `NEW_MESSAGE` payload 字段。
```

## 禁止

```text
不得引入对象存储。
不得实现文件上传。
不得实现图片压缩。
不得实现语音转文字。
不得新增独立 WebSocket 连接。
```

## 验收标准

```text
1. 文本消息仍可发送和展示。
2. 旧文本消息仍可展示。
3. 图片消息可发送和展示。
4. 文件消息可发送和展示。
5. 语音消息可发送和展示。
6. NEW_MESSAGE payload 能承载不同 message_type。
```

## 验证

```text
必要时重新生成 Kitex
gofmt
go test ./... in chat-service
go test ./... in gateway
npm run build --prefix frontend
```

---

# 7. Task 2：已读回执

## 目标

一次性实现已读回执后端和前端展示。

## 范围

本任务包含：

```text
1. 标记会话已读接口。
2. 单聊 readByPeer。
3. 群聊 readCount。
4. 群聊已读成员列表，若实现成本过高可只返回 readCount。
5. 前端已读展示。
```

## 只读文件

```text
docs/specs/pre-p3-closure-spec.md
idl/chat.thrift
chat-service/internal/dal/model/chat.go
chat-service/internal/repository/chat.go
chat-service/internal/biz/chat.go
gateway/internal/handler/chat.go
frontend/src/App.tsx
frontend/src/api.ts
frontend/src/types.ts
frontend/src/styles.css
```

## 数据来源

已读回执必须基于：

```text
conversation_members.last_read_message_id
```

## 后端要求

```text
1. 提供标记会话已读接口。
2. 入参必须包含 conversationId 和 lastReadMessageId。
3. lastReadMessageId 必须属于当前 conversation。
4. 只能更新当前 USER 成员的 last_read_message_id。
5. 不得更新 BOT 成员。
6. last_read_message_id 只能前进，不能回退。
7. 单聊消息列表返回 readByPeer。
8. 群聊消息列表返回 readCount。
9. readCount 只统计 USER + NORMAL 成员。
10. BOT 成员不得计入已读。
```

## 单聊规则

```text
readByPeer = 对方 USER 成员 last_read_message_id >= message.id
```

只对普通 SINGLE 会话生效。

BOT 不参与普通 SINGLE。

## 群聊规则

```text
readCount = USER + NORMAL 成员中 last_read_message_id >= message.id 的人数
```

## 前端要求

```text
1. 自己发送的单聊消息显示已读/未读。
2. 对方消息不显示自己的已读状态。
3. 群聊中自己发送的消息显示 N 人已读。
4. 如果有已读成员列表接口，点击 N 人已读可查看列表。
```

## 禁止

```text
不得把 BOT 成员计入已读。
不得让 last_read_message_id 回退。
不得为已读回执单独开 WebSocket。
```

## 验收标准

```text
1. 进入会话后可标记已读。
2. 单聊可显示对方是否已读。
3. 群聊可显示 N 人已读。
4. BOT 不影响已读统计。
```

## 验证

```text
必要时重新生成 Kitex
gofmt
go test ./... in chat-service
go test ./... in gateway
npm run build --prefix frontend
```

---

# 8. Task 3：SYSTEM message

## 目标

实现 SYSTEM message 基础能力，并接入群成员变更提示。

## 范围

本任务包含：

```text
1. SYSTEM message 基础能力。
2. SYSTEM content JSON 格式。
3. SYSTEM message NEW_MESSAGE 同步。
4. 入群、退群、邀请、踢人系统提示。
5. 前端系统提示行展示。
6. 为后续群权限、公告、Bot 加入提示预留统一创建方法。
```

## 只读文件

```text
docs/specs/pre-p3-closure-spec.md
chat-service/internal/dal/model/chat.go
chat-service/internal/repository/chat.go
chat-service/internal/biz/chat.go
gateway/internal/websocket/*
frontend/src/App.tsx
frontend/src/types.ts
frontend/src/styles.css
```

## 后端要求

系统事件必须创建：

```text
message_type = SYSTEM
sender_type = SYSTEM
```

或项目中已有等价系统发送者。

SYSTEM content 必须是 JSON，包含：

```text
eventType
actorUserId
targetUserIds
text
```

以下操作必须生成 SYSTEM message：

```text
成员入群：MEMBER_JOINED
成员主动退群：MEMBER_LEFT
某人邀请某人入群：MEMBER_INVITED
某人被移出群聊：MEMBER_REMOVED
```

SYSTEM message 必须：

```text
1. 落库。
2. 复用 NEW_MESSAGE 同步给在线 USER + NORMAL 成员。
3. 后续拉历史消息可见。
```

## 前端要求

前端收到或拉取到 SYSTEM message 后必须：

```text
1. 在消息列表中以居中灰色系统提示行展示。
2. 不显示头像。
3. 不显示普通消息气泡。
4. 不触发浏览器 Notification。
5. 不播放声音。
6. 不作为普通消息强提醒。
7. 不进入通知中心。
```

## 禁止

```text
不得只发 WebSocket 不落库。
不得新增独立系统通知 WebSocket。
不得把 BOT 成员作为推送目标。
不得把 SYSTEM message 当普通消息弹通知。
不得把入群申请、举报、审核类管理提醒发给所有成员。
```

## 管理提醒边界

本 Task 不实现：

```text
ADMIN_NOTICE
notifications 表
管理通知中心
```

如果后续实现管理提醒，必须只投递给 OWNER / ADMIN。

## 验收标准

```text
1. SYSTEM message 可以落库。
2. 入群生成 SYSTEM message。
3. 退群生成 SYSTEM message。
4. 邀请入群生成 SYSTEM message。
5. 踢人生成 SYSTEM message。
6. 前端以居中灰色提示行展示 SYSTEM message。
7. SYSTEM message 不触发浏览器通知或声音提醒。
```

## 验证

```text
gofmt
go test ./... in chat-service
go build ./... in chat-service
npm run build --prefix frontend
```

---

# 9. Task 4：群权限、角色管理与禁言

## 目标

一次性补齐群权限、群主转让、管理员管理、成员禁言和全员禁言。

这些能力强相关，必须在同一个 Task 中保持权限规则一致。

## 范围

本任务包含：

```text
1. OWNER / ADMIN / MEMBER 权限判断。
2. 群主转让。
3. 设置管理员。
4. 取消管理员。
5. 成员禁言。
6. 解除成员禁言。
7. 全员禁言。
8. 解除全员禁言。
9. 对应 SYSTEM message。
10. 前端基础管理入口。
```

## 只读文件

```text
docs/specs/pre-p3-closure-spec.md
idl/chat.thrift
chat-service/internal/dal/model/chat.go
chat-service/internal/repository/chat.go
chat-service/internal/biz/chat.go
gateway/internal/handler/chat.go
frontend/src/App.tsx
frontend/src/api.ts
frontend/src/types.ts
frontend/src/styles.css
```

## 权限判断方法

必须提供或补齐等价能力：

```text
CanManageMember
CanMuteMember
CanSetAdmin
CanTransferOwner
CanMuteAll
```

所有权限判断只针对 USER 成员。

## 群主转让

```text
1. OWNER 可以转让群主给 MEMBER 或 ADMIN。
2. 转让后原 OWNER 变为 MEMBER 或 ADMIN，具体策略必须固定并写入实现说明。
3. ADMIN 不得转让群主。
4. MEMBER 不得转让群主。
5. 必须生成 OWNER_TRANSFERRED SYSTEM message。
```

## 管理员管理

```text
1. OWNER 可以设置 MEMBER 为 ADMIN。
2. OWNER 可以取消 ADMIN。
3. ADMIN 不得设置或取消管理员。
4. 必须生成 ADMIN_ADDED / ADMIN_REMOVED SYSTEM message。
```

## 成员禁言

```text
1. OWNER 可以禁言任意 USER 成员。
2. ADMIN 只能禁言 MEMBER。
3. ADMIN 不得禁言 OWNER / ADMIN。
4. MEMBER 不得禁言任何人。
5. 支持禁言到指定时间。
6. 支持解除成员禁言。
7. 必须生成 MEMBER_MUTED / MEMBER_UNMUTED SYSTEM message。
```

## 全员禁言

```text
1. OWNER 可以全员禁言。
2. ADMIN 不得全员禁言。
3. 全员禁言不影响 OWNER / ADMIN。
4. SYSTEM / BOT_REPLY 不受全员禁言影响。
5. 必须生成 GROUP_MUTED / GROUP_UNMUTED SYSTEM message。
```

## SYSTEM message 展示语义

本 Task 生成的所有 SYSTEM message 都是聊天流中的居中系统提示行。

不得触发：

```text
浏览器 Notification
声音提醒
通知中心提醒
普通消息强提醒
```

## 发送消息校验

发送普通 USER 消息时，必须校验：

```text
1. 当前 USER 成员是否被单独禁言。
2. 当前群是否开启全员禁言。
3. 如果全员禁言开启，OWNER / ADMIN 仍可发送。
4. SYSTEM / BOT_REPLY 不受禁言影响。
```

## 前端要求

```text
1. 群成员管理中 OWNER 显示转让群主、设置管理员、取消管理员、禁言入口。
2. ADMIN 只显示对 MEMBER 的禁言/解除禁言入口。
3. MEMBER 不显示管理操作。
4. 群主可看到全员禁言入口。
5. 权限变更和禁言事件在聊天流中显示为居中灰色 SYSTEM message。
```

## 禁止

```text
不得让 ADMIN 禁言 ADMIN。
不得让 ADMIN 禁言 OWNER。
不得让 ADMIN 全员禁言。
不得让 MEMBER 管理群。
不得把 BOT 成员作为禁言目标。
```

## 验收标准

```text
1. 群主转让可用。
2. 设置管理员可用。
3. 取消管理员可用。
4. OWNER 可禁言 MEMBER/ADMIN。
5. ADMIN 只能禁言 MEMBER。
6. 全员禁言只允许 OWNER。
7. 全员禁言不影响 OWNER/ADMIN。
8. 所有管理操作生成 SYSTEM message。
9. SYSTEM message 只作为聊天流提示展示。
```

## 验证

```text
必要时重新生成 Kitex
gofmt
go test ./... in chat-service
go test ./... in gateway
npm run build --prefix frontend
```

---

# 10. Task 5：群公告

## 目标

实现群公告查询、修改、前端展示和 SYSTEM message。

## 范围

本任务包含：

```text
1. 群公告数据字段。
2. 查询群公告接口。
3. 修改群公告接口。
4. 权限校验。
5. ANNOUNCEMENT_UPDATED SYSTEM message。
6. 前端展示和编辑。
```

## 只读文件

```text
docs/specs/pre-p3-closure-spec.md
idl/chat.thrift
chat-service/internal/dal/model/chat.go
chat-service/internal/biz/chat.go
gateway/internal/handler/chat.go
frontend/src/App.tsx
frontend/src/api.ts
frontend/src/types.ts
frontend/src/styles.css
```

## 字段要求

必须支持：

```text
announcement
announcement_updated_by
announcement_updated_at
```

如果已有等价字段，必须复用。

## 后端要求

```text
1. 群成员可以查询公告。
2. OWNER / ADMIN 可以修改公告。
3. MEMBER 不得修改公告。
4. 修改公告必须生成 ANNOUNCEMENT_UPDATED SYSTEM message。
```

## 前端要求

```text
1. 群聊详情显示群公告。
2. 群公告摘要固定展示在群聊详情区，不要求在聊天主消息区额外重复展示。
3. OWNER / ADMIN 显示编辑入口。
4. MEMBER 只读。
5. 修改成功后刷新公告。
6. 公告更新在聊天流中显示为居中灰色 SYSTEM message。
7. 公告更新 SYSTEM message 不触发浏览器通知、声音提醒或通知中心提醒。
```

## 验收标准

```text
1. 群公告可查询。
2. 群公告可由 OWNER/ADMIN 修改。
3. MEMBER 不能修改。
4. 前端能展示公告。
5. 修改公告生成 SYSTEM message。
6. ANNOUNCEMENT_UPDATED 只作为聊天流提示展示。
```

## 验证

```text
必要时重新生成 Kitex
gofmt
go test ./... in chat-service
go test ./... in gateway
npm run build --prefix frontend
```

---

# 11. Task 6：消息回复

## 目标

实现消息回复后端和前端能力。

## 范围

本任务包含：

```text
1. 发送消息支持 replyToId。
2. replyToId 校验。
3. 消息列表返回 replyTo 摘要。
4. 前端回复交互。
5. 前端展示被回复消息摘要。
```

## 只读文件

```text
docs/specs/pre-p3-closure-spec.md
idl/chat.thrift
chat-service/internal/dal/model/chat.go
chat-service/internal/biz/chat.go
chat-service/internal/repository/chat.go
gateway/internal/handler/chat.go
frontend/src/App.tsx
frontend/src/api.ts
frontend/src/types.ts
frontend/src/styles.css
```

## 后端要求

```text
1. 发送消息接口支持 replyToId。
2. replyToId 可以为空。
3. replyToId 不为空时，原消息必须存在。
4. 原消息必须属于同一 conversation。
5. 不允许回复其他 conversation 的消息。
6. 消息列表返回 replyTo 摘要。
```

replyTo 摘要必须包含：

```text
messageId
senderId
senderType
messageType
contentPreview
```

`contentPreview` 生成规则必须统一：
```text
TEXT: 使用原文本摘要（可截断）
IMAGE: 固定为“[图片]”
FILE: 优先使用文件名；若无文件名则显示“[文件]”
VOICE: 固定为“[语音]”
SYSTEM: 使用 text
BOT_REPLY: 使用文本摘要（可截断）
```

## 前端要求

```text
1. 消息操作区提供“回复”入口。
2. 输入框上方展示正在回复的消息摘要。
3. 可取消回复。
4. 发送消息时携带 replyToId。
5. 消息列表展示被回复消息摘要。
6. 被回复消息不存在或已不可见时显示“原消息不可用”。
```

## 禁止

```text
不得允许跨 conversation 回复。
不得因为原消息缺失导致消息列表崩溃。
```

## 验收标准

```text
1. 可以回复一条消息。
2. 发送时携带 replyToId。
3. 后端校验 replyToId 属于同一 conversation。
4. 消息列表展示回复摘要。
5. 前端可取消回复。
```

## 验证

```text
必要时重新生成 Kitex
gofmt
go test ./... in chat-service
go test ./... in gateway
npm run build --prefix frontend
```

---

# 12. Task 7：文档对齐

## 目标

把 P3 前置能力收口状态写入项目文档。

## 禁止

```text
不得修改代码。
```

## 只读文件

```text
README.md
docs/specs/pre-p3-closure-spec.md
output.md
```

## 要求

文档必须明确：

```text
1. 多消息类型支持范围。
2. 已读回执规则。
3. SYSTEM message 是聊天流提示，不是通知提醒。
4. 管理提醒后续使用 ADMIN_NOTICE / notifications，仅投递 OWNER / ADMIN。
5. 群权限规则。
6. 群公告接口和前端展示。
7. 消息回复能力。
8. WebSocket 仍复用 NEW_MESSAGE。
9. Bot/P3 不受本规格破坏。
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

---

## 13. 推荐执行顺序

必须按以下顺序执行：

```text
Task 0：Spec Review
Task 1：多消息类型
Task 2：已读回执
Task 3：SYSTEM message
Task 4：群权限、角色管理与禁言
Task 5：群公告
Task 6：消息回复
Task 7：文档对齐
```

如果人工判断前端实现暂时延后，可以先完成对应 Task 的后端部分，但必须在输出中明确未完成的前端 TODO。

---

## 14. 总体验收标准

完成后必须满足：

```text
1. 文本、图片、文件、语音均可发送并展示。
2. 旧文本消息仍能展示。
3. 单聊能看到对方已读状态。
4. 群聊能看到 N 人已读。
5. 群系统事件落库为 SYSTEM message。
6. SYSTEM message 在聊天流中以居中灰色提示行展示。
7. SYSTEM message 不触发浏览器 Notification、声音提醒或通知中心提醒。
8. 群主转让可用。
9. 管理员设置和取消可用。
10. 管理员和群主禁言权限符合规则。
11. 全员禁言不影响 OWNER / ADMIN。
12. 群公告可展示和修改。
13. 消息回复可发送和展示。
14. 所有 SYSTEM message 和普通消息均复用 NEW_MESSAGE 进行实时同步。
15. WebSocket 收件人只包含 USER + NORMAL 成员。
```
