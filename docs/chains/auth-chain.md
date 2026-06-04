# AIM Auth 链路说明

最后更新：2026-06-04

这是精简版，只保留 auth 链路的主干，方便快速查阅。

## 角色

| 组件 | 作用 |
| --- | --- |
| `gateway` | 对外入口，读取 Cookie/Header，转发认证请求，写回登录态 |
| `auth-service` | 认证核心，负责注册、登录、校验、刷新、退出、会话管理 |
| `user-service` | 提供创建用户、校验密码、查询用户状态、提升 `token_version` |
| `postgres` | 保存 session、refresh token、审计数据 |
| `redis` | 保存 session 缓存、token 黑名单、登录失败计数等临时状态 |

## 主流程

### 注册

```text
客户端 -> gateway /auth/register -> auth-service -> user-service.CreateUser
```

返回用户身份信息，不直接建立登录态。

### 登录

```text
客户端 -> gateway /auth/login -> auth-service.Login -> user-service.VerifyCredential
```

登录成功后，`auth-service` 会：

- 生成 `session_id`
- 签发 `access_token` 和 `refresh_token`
- 写入 session 和 refresh token
- 更新最近登录状态
- 由 `gateway` 写入 Cookie

### 校验

```text
客户端请求 -> gateway middleware.Auth -> auth-service.ValidateToken
```

`ValidateToken` 会检查：

- JWT 签名
- access token 黑名单
- session 是否存在且 active
- token 中的 `user_id` / `session_id` / `token_id`
- 用户状态是否正常
- `token_version` 是否一致

### 刷新

```text
客户端 -> /auth/refresh -> auth-service.RefreshToken
```

刷新时会旋转 refresh token。旧 token 如果再次被使用，会触发复用检测，并让相关 session 失效。

### 退出

- `POST /api/v1/auth/logout`：退出当前会话
- `POST /api/v1/auth/logout-all`：退出全部会话，并提升 `token_version`
- `POST /api/v1/auth/sessions/revoke`：撤销指定会话

## 关键规则

- Cookie 名称：`access_token`、`refresh_token`、`device_id`
- JWT 至少包含：`user_id`、`aim_id`、`role`、`token_version`、`session_id`、`token_id`
- 修改密码、退出全部设备、检测到 refresh token 复用、封禁用户，都会让旧 token 失效
- `gateway` 负责入口鉴权，但最终是否可用由 `auth-service` 和 `user-service` 共同决定

## 常用接口

- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/refresh`
- `POST /api/v1/auth/logout`
- `POST /api/v1/auth/logout-all`
- `GET /api/v1/auth/sessions`
- `POST /api/v1/auth/sessions/revoke`
- `GET /ws/chat?token=<access_token>`

## 排障顺序

1. 检查 `gateway` 和 `auth-service` 是否健康
2. 检查 `JWT_SECRET` 是否一致
3. 检查 `redis` 是否可用
4. 检查 `user-service` 是否能返回正常用户状态
5. 检查浏览器是否带上了 Cookie 或 `Authorization` Header
