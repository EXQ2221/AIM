# AIM

AI 原生多人协作聊天平台，面向高并发实时通信与 AI 能力扩展。

## 简介

AIM 是一个以后端微服务为核心的多人聊天系统，当前已完成从鉴权、好友关系、会话与群聊、消息持久化到 WebSocket 实时通信的主链路，并在此基础上推进 AI Bot 能力。

项目目标是先保证聊天基础链路稳定可用，再逐步扩展到 Bot 与 RAG 等 AI 场景，支持后续私聊 Bot、知识库问答、异步任务解耦等演进方向。

适用场景：

- 企业或团队内部实时协作沟通
- 需要 AI 助手参与群聊协作的业务系统
- 需要可拆分、可扩展微服务架构的聊天平台

## 功能特性

- 用户注册登录、JWT 鉴权、会话管理与多端登出
- 好友关系与好友分组管理
- 单聊/群聊会话管理、成员管理与权限校验
- 消息持久化、历史消息查询、消息撤回、消息回复
- WebSocket 实时消息推送与群聊广播
- 已读回执（单聊 `readByPeer`、群聊 `readCount`）
- 群管理能力：群主转让、管理员设置、成员禁言/移除、全员禁言、群公告
- AI Bot 成员化接入、`@mention` 触发、非流式 LLM 回复、并发控制、调用日志能力

## 技术栈

- Go（微服务主体）
- Gin（gateway HTTP / WebSocket 接入）
- Kitex + Thrift IDL（服务间 RPC）
- GORM + MySQL（数据存储）
- Redis（在线状态、会话态、Pub/Sub）
- React + TypeScript + Vite（前端工作台）
- Docker / Docker Compose（部署与联调）

## 项目结构

```bash
.
├── gateway/              # HTTP 网关、JWT 中间件、WebSocket 接入
├── auth-service/         # 注册登录、Token/Session 鉴权
├── user-service/         # 用户资料、好友关系、好友分组
├── chat-service/         # 会话、群聊、消息、Bot 相关业务
├── idl/                  # Thrift 接口定义
├── shared/               # 公共组件（配置、错误码、工具）
├── frontend/             # Vite + React + TypeScript 前端
├── docs/                 # 规格文档与任务拆分文档
├── scripts/              # 代码生成与辅助脚本
├── docker-compose.yml
└── README.md
```
