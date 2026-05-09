# AGENTS.md

## 1. 项目简介

AIM 是一个 AI 原生多人协作聊天平台。

本仓库当前阶段优先实现后端微服务系统，不实现前端页面。项目重点是后端服务拆分、认证鉴权、用户关系、会话管理、消息存储、实时通信，以及后续 AI Bot 和 RAG 知识库能力的扩展。

### 1.1 基本需求

基本要求：
- 基于 TCP/WebSocket 的实时消息收发，支持单聊、群聊、广播消息；消息类型支持文本、图片、文件、语音。
- 消息已读回执、输入状态提示、在线状态管理；消息本地存储与云端漫游，支持按关键词、时间范围搜索历史消息。
- 好友关系管理：添加、删除、分组、备注。
- 群组管理：创建、邀请、踢出、禁言、转让群主、群公告。
- 内置聊天 Bot，接入多家厂商模型接口，用户可直接 @Bot 对话；支持用户通过 OpenAPI 自行部署机器人，也可使用 AIM 平台内置 Bot，这里有两个选择，一个是自己提供 API Key，也可以使用平台的模型，要求需要做好计费管理。
- 一键总结群聊/单聊历史消息，生成要点摘要与待办提取；根据上下文生成回复候选，用户可一键选用。
- 分布式架构，需要合理划分模块，要求至少需要使用 docker 打包部署。

实现基本要求之后的需求：

项目分阶段开发：

### P0：基础微服务骨架与鉴权

- 复用已有微服务鉴权框架
- 保留 gateway、auth-service、user-service、shared、idl 等基础结构
- 将原有空模板 biz-service 改造为 chat-service
- 跑通注册、登录、JWT 校验、服务间 RPC 调用

### P1：基础聊天能力

- 用户信息
- 好友关系
- 好友分组
- 会话管理
- 群聊管理
- 消息发送
- 消息持久化
- 历史消息查询
- P1 阶段要求消息具备可复用的创建、持久化和查询能力，底层发送逻辑应通过 RPC 或等价的服务接口完成。
- 对外发送入口在当前实现中可以直接使用 WebSocket，只要消息创建逻辑仍然独立于 WebSocket 连接管理即可。
- HTTP 发消息接口在 P1 阶段不是强制要求，可作为后续联调、开放接口或运维排障时的增强能力按需补充。

### P2：WebSocket 实时通信

- WebSocket 长连接
- 在线状态
- 实时消息推送
- 群聊消息广播

### P3：AI Bot

- Bot 表
- 会话绑定 Bot
- @Bot 触发
- 最近消息上下文
- 调用大模型
- Bot 回复写入消息表
- AI 调用日志

### P4：RAG 知识库

- 知识库管理
- 文档上传
- 文档解析
- 文本切分
- embedding
- 向量检索
- Bot 基于知识库回答问题

开发时必须优先保证基础聊天链路稳定，不要过早引入复杂 AI 功能。

---

## 2. 技术栈约定

后端：

- Go
- Gin：作为 gateway 的 HTTP API 和 WebSocket 接入层
- Kitex：作为服务间 RPC 框架
- GORM：作为 ORM
- MySQL：作为主数据库
- Redis：用于 Token、在线状态、WebSocket 连接映射、限流等场景

服务通信：

- 服务间调用使用 Kitex RPC
- IDL 使用 Thrift
- 服务接口优先通过 IDL 明确定义

部署：

- Docker
- Docker Compose

当前阶段不实现前端。

接口测试可以使用：

- curl
- Apipost
- WebSocket 客户端工具
- Go 测试代码

后续可按需求引入：

- Kafka / Redis Stream：用于异步消息、Bot 请求、消息投递解耦
- Prometheus / Grafana：用于监控
- OpenTelemetry / Jaeger：用于链路追踪
- Etcd / Nacos：用于服务注册与发现，是否使用取决于项目后续需要

AI 相关：

- 使用 OpenAI-compatible API,可接入多个厂家的API
- Bot 逻辑第二阶段之后实现
- RAG 逻辑第三阶段之后实现

---

## 3. 微服务划分原则

项目采用微服务架构，但第一阶段不要过度拆分。

当前推荐服务：

- gateway
- auth-service
- user-service
- chat-service
- shared
- idl

说明：

- gateway：对外 HTTP API、JWT 中间件、WebSocket 接入
- auth-service：注册、登录、Token、鉴权
- user-service：用户信息、好友关系、好友分组
- chat-service：会话、群聊、成员、消息、历史记录
- shared：公共配置、数据库、Redis、日志、错误码、工具方法
- idl：Kitex Thrift 接口定义

原有 `biz-service` 是空模板，可以改名或改造为 `chat-service`。

第一阶段暂时不单独拆分：

- relation-service
- ws-service
- bot-service
- rag-service

好友关系先放在 `user-service`。

WebSocket 先作为 `gateway` 的一个模块实现。

Bot 后续再拆成 `bot-service`。

RAG 后续再拆成 `rag-service`。

---

## 4. 推荐项目结构

推荐目录结构：

```text
aim/
├── gateway/
├── auth-service/
├── user-service/
├── chat-service/
├── idl/
├── shared/
├── scripts/
├── deploy/
├── docs/
│   ├── specs/
│   └── ai-coding/
├── docker-compose.yml
├── go.work
├── go.work.sum
├── README.md
└── AGENTS.md
```

## 5. 服务职责边界

### 5.1 gateway

gateway 负责：
- 对外 HTTP API
- Gin 路由
- JWT 中间件
- 请求参数基础校验
- 调用后端 Kitex 服务
- 统一响应格式
- WebSocket 握手和连接管理

gateway 不负责：
- 直接操作业务数据库
- 写复杂业务逻辑
- 直接处理群聊权限细节
- 直接调用大模型
- 直接处理 RAG 检索

### 5.2 auth-service

auth-service 负责：
- 用户注册
- 用户登录
- 密码校验
- JWT 生成
- JWT 刷新
- TokenVersion 校验
- session 校验
- 退出登录
- 账号锁定相关逻辑
- 以及其他原有框架的功能

auth-service 不负责：
- 好友关系
- 群聊管理
- 消息存储
- WebSocket 推送
- Bot 调用

### 5.3 user-service

user-service 负责：
- 用户资料查询
- 用户资料修改
- AIM ID 查询
- 昵称、头像、邮箱等基础信息维护
- 好友申请
- 好友关系
- 好友备注
- 好友分组

第一阶段不单独拆分 relation-service，好友关系直接放在 user-service。

user-service 不负责：
- 群聊消息存储
- 会话成员权限
- WebSocket 推送
- Bot 调用
- RAG 检索

### 5.4 chat-service

chat-service 由原来的 biz-service 空模板改造而来。

chat-service 负责：
- 会话 conversation
- 会话成员 conversation_member
- 群聊信息 group_info
- 消息 message
- 创建群聊
- 加入群聊
- 查询会话列表
- 发送消息
- 消息持久化
- 历史消息查询
- 基础群成员权限校验

chat-service 不负责：
- 用户登录
- 密码校验
- JWT 生成
- WebSocket 连接维护
- 直接调用大模型
- RAG 文档检索

发送消息时，chat-service 必须校验：
- conversation 是否存在
- 用户是否属于该 conversation
- 用户是否有权限发送消息
- 消息内容是否合法

### 5.5 shared

shared 存放公共代码，例如：
- 配置加载
- 数据库初始化
- Redis 初始化
- 日志
- 错误码
- 统一响应结构
- JWT 工具
- 密码加密工具
- 通用常量
- 通用 middleware
- 通用 DTO 或公共类型

shared 中不要放具体业务逻辑。

### 5.6 idl

idl 负责存放 Thrift 接口定义。
推荐文件：

```text
idl/
├── auth.thrift
├── user.thrift
├── chat.thrift
├── common.thrift
├── bot.thrift
└── rag.thrift
```
第一阶段只需要：

```text
idl/
├── auth.thrift
├── user.thrift
├── chat.thrift
├── common.thrift
```
生成代码不要手动修改。
IDL 中不要暴露数据库实现细节。

## 6. 数据库设计规则

### 6.1 主键规则

所有核心表使用 uint64 自增主键。

```go
ID uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
```
数据库内部关联必须使用 id，不要使用 aim_id、昵称、邮箱等字段做外键。

### 6.2 用户标识规则

用户表中应区分：
- id：系统内部主键
- aim_id：用户自己设置的唯一 ID，类似微信号
- nickname：展示昵称，可重复，可修改

示例：

```text
id = 10001
aim_id = xqe_0422
nickname = 小青
```
使用规则：
- 表关联使用 id
- 搜索用户可以使用 aim_id
- 聊天展示优先使用 nickname
- aim_id 注册后默认不允许修改，后续可以增加修改规则

### 6.3 用户表字段建议

用户表至少包含：

```text
id
aim_id
nickname
avatar
email
password_hash
status
role
token_version
last_login_at
last_login_ip
login_fail_count
locked_until
created_at
updated_at
deleted_at
```

### 6.4 好友关系设计规则

好友关系第一阶段放在 user-service 中。
建议表：

```text
friend_group
friend_relation
```
friend_group 用于好友分组。
friend_relation 用于好友关系、备注、状态。
好友关系推荐使用双向记录：

```text
A -> B
B -> A
```
这样每个用户都可以独立设置备注和分组。

### 6.5 会话设计规则

使用 conversation 统一表示聊天窗口。
conversation.type 可以是：
- SINGLE
- GROUP
- BOT
- SYSTEM

不要为单聊、群聊、Bot 聊天分别设计完全独立的消息系统。
所有聊天消息都应进入统一的 message 表。

### 6.6 群聊设计规则

conversation.type = GROUP 时，应存在对应的 group_info 记录。
conversation 只存通用会话信息，例如：
- 会话 ID
- 会话类型
- 创建者
- 最近一条消息
- 最近活跃时间

group_info 存群聊专属信息，例如：
- 群名称
- 群头像
- 群公告
- 群主
- 入群策略
- 是否全员禁言
- 最大成员数

不要把大量群聊专属字段塞进 conversation 表。

### 6.7 消息设计规则

message 表存储所有会话中的消息，包括：
- 用户文本消息
- Bot 回复
- 系统消息
- 文件消息
- 图片消息
- 语音消息

消息发送者通过以下字段区分：
- SenderID   uint64
- SenderType string

sender_type 可选：
- USER
- BOT
- SYSTEM

消息类型通过 message_type 区分：
- TEXT
- IMAGE
- FILE
- VOICE
- BOT_REPLY
- SYSTEM

第一阶段只需要实现 TEXT，但表结构应保留扩展空间。

### 6.8 消息状态规则

消息状态建议包括：
- NORMAL
- RECALLED
- DELETED
- FAILED

说明：
- NORMAL：正常消息
- RECALLED：已撤回，所有人可见“消息已撤回”
- DELETED：删除或隐藏
- FAILED：发送失败

消息撤回时，不要物理删除数据库记录，而是修改状态。

### 6.9 软删除规则

对于用户、会话、消息等核心数据，可以使用 GORM 软删除。
但消息默认不物理删除。
历史消息查询时，应根据业务需求决定是否过滤：
- 已撤回消息可以返回，但内容可隐藏
- 已删除消息可以不返回
- 发送失败消息通常只对发送者可见

### 6.10 第一阶段核心表

P0/P1 阶段需要实现：

- user
- friend_group
- friend_relation
- conversation
- conversation_member
- group_info
- message

P2/P3 阶段扩展：

- bot
- conversation_bot
- ai_call_log
- file_resource
- message_attachment
- notification

P4 阶段扩展：

- knowledge_base
- knowledge_document
- document_chunk
- conversation_knowledge_base

### 6.11 事务规则

以下操作必须使用数据库事务：

- 创建群聊：conversation、group_info、conversation_member 必须同时成功或失败
- 发送消息：message 创建与 conversation 最近消息更新必须同时成功或失败
- 接受好友申请：双方 friend_relation 记录必须同时成功或失败
- 以及其他会导致数据不一致的操作

## 7. 认证与权限规则

### 7.1 登录认证

使用 JWT 进行认证。
JWT 中至少包含：
- user_id
- aim_id
- role
- token_version
- expire_time

服务端校验 JWT 时，需要检查：
- Token 是否过期
- 用户是否存在
- 用户状态是否正常
- token_version 是否匹配

### 7.2 TokenVersion 规则

用户表中保留 token_version 字段。
以下情况应增加 token_version：
- 用户修改密码
- 用户退出所有设备
- 用户被封禁
- 管理员强制下线用户

这样可以让旧 Token 失效。

### 7.3 账号状态规则

用户状态包括：
- NORMAL
- BANNED
- DELETED

被封禁或删除的用户不能：
- 登录
- 建立 WebSocket 连接
- 发送消息
- 创建群聊
- 调用 Bot

### 7.4 服务间鉴权规则

外部请求统一由 gateway 校验用户 JWT。
服务间调用时，gateway 可以将用户身份信息传给后端服务，例如：
- user_id
- aim_id
- role

后端服务不能完全信任客户端传入的业务参数。
关键权限仍应由业务服务校验，例如：
- chat-service 校验用户是否属于会话
- chat-service 校验用户是否可以发送消息
- user-service 校验用户是否可以修改好友备注
- auth-service 校验账号状态和 token_version

第一阶段可以先不做复杂服务间鉴权，但服务边界要保留。
后续如果需要更严格的服务间安全，可以引入：
- 内网服务 Token
- mTLS
- 服务白名单
- RPC metadata 签名
- 其他未提到的按原有模板来

### 7.5 群聊权限规则

群聊成员角色包括：
- OWNER
- ADMIN
- MEMBER
- BOT

权限规则：
- 群主可以管理群信息
- 群主可以添加或移除管理员
- 管理员可以管理普通成员
- 普通成员不能修改群公告
- 非群成员不能发送群消息
- 被禁言用户不能发送消息
- 全员禁言时，只有群主和管理员可以发送消息

第一阶段可以只实现：
- 群主
- 普通成员
- 非群成员不能发消息

后续再扩展管理员、禁言和更复杂权限。

## 8. WebSocket 规则

### 8.1 WebSocket 说明

WebSocket 是一种长连接通信方式，用于让服务端主动向客户端推送消息。
普通 HTTP 是请求-响应模式：

```text
客户端请求
↓
服务端响应
↓
连接结束
```
WebSocket 是长连接模式：

```text
客户端连接
↓
服务端接受连接
↓
连接持续保持
↓
客户端可以随时发消息
↓
服务端可以随时推送消息
```
AIM 中 WebSocket 用于：
- 实时聊天消息
- 在线状态
- 群聊消息广播
- Bot 回复推送
- 后续输入中提示、已读回执等功能

### 8.2 WebSocket 加入时机

第一阶段可以先不实现 WebSocket；如果项目已经提前完成 WebSocket 发送链路，也可以直接以 WebSocket 作为对外消息发送入口。
推荐顺序：

```text
注册登录
↓
创建群聊
↓
RPC 跑通消息创建、持久化和查询
↓
对外入口可选择 HTTP 或 WebSocket
↓
补齐实时推送、在线状态等能力
```
也就是说，应先保证消息创建和查询链路独立成立，再决定是否通过 HTTP 或 WebSocket 暴露给客户端。
WebSocket 只是对外通信入口之一，不应改变底层消息创建逻辑，也不应承载 message 表写入等核心业务实现。

### 8.3 WebSocket 位置

第一版 WebSocket 先放在 gateway 中实现，不单独拆 ws-service。
gateway 中可以有：

```text
gateway/
├── http/
├── middleware/
├── websocket/
└── rpcclient/
```
后续如果连接量变大，再拆分独立 ws-service。

### 8.4 WebSocket 连接认证

用户连接 WebSocket 时必须携带 JWT。
可以支持：

```http
Authorization: Bearer <jwt>
```
服务端需要：
- 校验 JWT
- 获取 user_id
- 绑定 user_id 与连接
- 将在线状态写入 Redis
- 连接断开后清理 Redis 状态

### 8.5 WebSocket 职责边界

WebSocket 模块负责：
- 连接建立
- 连接关闭
- 读取客户端消息
- 调用 chat-service 创建消息
- 向在线成员推送消息

WebSocket 模块不负责：
- 直接操作 message 表
- 复杂群成员权限判断
- 直接调用 AI 模型
- 直接处理 RAG
- 直接管理群聊成员

### 8.6 群聊消息流程

第一版可以采用同步调用链路：

```text
客户端发送消息
↓
gateway WebSocket 模块接收消息
↓
gateway 调用 chat-service
↓
chat-service 校验权限
↓
chat-service 保存 message
↓
chat-service 更新 conversation.last_message_id / last_message_at
↓
gateway 获取在线连接
↓
gateway 推送消息
```
后续引入 Kafka 后，可以演进为：

```text
客户端发送消息
↓
gateway 接收消息
↓
chat-service 保存消息
↓
发布 MessageCreated 事件
↓
gateway / ws-service 消费事件并推送
↓
bot-service 消费事件判断是否触发 Bot
```
第一阶段不要强行引入 Kafka，除非 spec 明确要求。

## 9. Kitex / IDL 规则

所有服务间接口优先使用 Thrift IDL 定义。
IDL 文件建议放在：

```text
idl/
├── common.thrift
├── auth.thrift
├── user.thrift
└── chat.thrift
```
后续再增加：

```text
bot.thrift
rag.thrift
```
接口命名应清晰表达业务含义，例如：

```text
Register
Login
ValidateToken
GetUserByID
GetUserByAimID
UpdateUserProfile
CreateFriendRequest
AcceptFriendRequest
CreateConversation
CreateGroup
JoinGroup
CreateMessage
ListMessages
```
IDL 中不要泄露数据库实现细节。
生成代码不要手动修改。
如果需要修改接口，应先修改 IDL，再重新生成代码。

## 10. 分层规则

每个服务内部推荐分层：

```text
cmd/
internal/
├── handler/
├── service/
├── repository/
├── model/
├── config/
└── client/
```

### 10.1 handler 层

handler 负责：
- 接收 RPC 或 HTTP 请求
- 参数基础校验
- 调用 service
- 返回响应

handler 不负责：
- 复杂业务逻辑
- 直接操作数据库
- 直接操作 Redis
- 直接调用外部模型

### 10.2 service 层

service 负责：
- 业务逻辑
- 权限判断
- 事务控制
- 调用 repository
- 调用其他 RPC 服务
- 组织返回数据

核心业务逻辑必须放在 service 层。

### 10.3 repository 层

repository 负责：
- 数据库查询
- 数据库写入
- 数据库更新
- 数据库删除

repository 不负责：
- JWT 生成
- 权限判断
- WebSocket 推送
- AI 调用
- RAG 检索

## 11. 异步消息规则

第一阶段可以不引入 Kafka。
基础版本允许同步链路：

```text
gateway
↓
chat-service
↓
MySQL
↓
gateway 推送
```
后续需要解耦时，可以引入 Kafka / Redis Stream。
推荐异步事件：
- MessageCreated
- UserOnline
- UserOffline
- BotRequestCreated
- BotReplyCreated

引入消息队列后，事件结构必须稳定，不要直接传 GORM Model。
事件应使用明确 DTO，例如：

```text
MessageCreatedEvent
- message_id
- conversation_id
- sender_id
- sender_type
- message_type
- created_at
```

## 12. AI Bot 规则

### 12.1 Bot 加入时机

第一阶段不要实现 Bot。
必须先完成：
- 用户登录
- 创建群聊
- 加入群聊
- 发送消息
- 消息持久化
- 历史消息查询

基础聊天稳定后，再实现 Bot。

### 12.2 bot-service 职责

后续单独拆分 bot-service。

bot-service 负责：
- 识别 @Bot 请求
- 构建上下文
- 调用大模型
- 生成回复
- 记录 AI 调用日志

bot-service 不负责：
- 维护 WebSocket 连接
- 直接广播消息
- 管理群成员
- 直接处理前端连接

Bot 回复必须通过 chat-service 写入 message 表，再由 gateway / ws-service 推送。

### 12.3 Bot 消息存储规则

Bot 回复也必须进入 message 表。
示例：

```text
sender_type = BOT
message_type = BOT_REPLY
```
不要为 Bot 回复单独创建一套消息表。

### 12.4 Bot 权限规则

Bot 只能访问被授权的上下文。
普通 Bot 第一版可以只读取：
- 当前 conversation 最近 N 条消息

接入 RAG 后，Bot 可以读取：
- 当前 conversation 绑定的知识库
- 当前用户有权限访问的文档片段

Bot 不允许跨群读取未授权消息。

## 13. RAG 规则

### 13.1 RAG 加入时机

RAG 是后期功能。
不要在基础聊天还没完成时实现 RAG。
推荐顺序：

```text
基础聊天
↓
普通 AI Bot
↓
RAG 知识库
```

### 13.2 rag-service 职责

后续单独拆分 rag-service。

rag-service 负责：
- 文档上传后的解析
- 文本切分
- embedding 生成
- 向量存储
- 相似度检索
- 返回相关文档片段

rag-service 不负责：
- WebSocket 推送
- 用户聊天消息存储
- 直接管理群聊权限
- 直接生成最终回答

最终回答由 bot-service 调用 LLM 生成。

### 13.3 RAG 与 Bot 的关系

RAG 只为 Bot 提供额外上下文。
未接入 RAG 时：

```text
用户问题
+ 最近聊天记录
→ LLM
→ Bot 回复
```
接入 RAG 后：

```text
用户问题
+ 最近聊天记录
+ 知识库检索片段
→ LLM
→ Bot 回复
```
不要让 RAG 影响基础 message 

## 14. 当前阶段非目标

P0/P1 阶段暂不实现：

- 前端页面
- Bot 调用
- RAG 知识库
- Kafka / Redis Stream
- Prometheus / Grafana
- OpenTelemetry / Jaeger
- 多端同步
- 语音转码
- 图片审核
- 复杂已读回执
- 复杂管理员权限

## 15. 环境变量规范

敏感信息必须通过环境变量注入，不允许提交到 Git。

示例：

- MYSQL_DSN
- REDIS_ADDR
- JWT_SECRET
- DEEPSEEK_API_KEY
- LLM_BASE_URL
- LLM_MODEL
