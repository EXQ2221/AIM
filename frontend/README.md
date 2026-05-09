# AIM Frontend

P0/P1 阶段前端工作台，覆盖：

- 注册、登录、退出、会话管理
- 当前用户资料与登录会话
- 创建群聊、加入群聊、退出群聊
- 会话列表、群成员列表、历史消息查询
- 文本消息发送与实时接收

本地开发：

```bash
npm install --prefix frontend
npm run dev --prefix frontend
```

Vite 默认把 `/api`、`/healthz`、`/ws` 代理到 `http://127.0.0.1:8080`。
如果 gateway 使用其他地址：

```bash
VITE_GATEWAY_TARGET=http://127.0.0.1:8081 npm run dev --prefix frontend
```
