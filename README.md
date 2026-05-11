# AIM

AIM is an AI-native collaborative chat platform built with Go microservices.

## Local Development (PostgreSQL Baseline)

This repository now uses **PostgreSQL (single instance)** + Redis in Docker Compose.

### 1) Prepare environment

Copy `.env.example` to `.env` and fill in secrets:

```powershell
Copy-Item .env.example .env
```

Key variables:

- `POSTGRES_USER`
- `POSTGRES_PASSWORD`
- `POSTGRES_DATABASE`
- `JWT_SECRET`

### 2) Start infrastructure and services

```powershell
docker compose up -d --build
```

### 3) Check status

```powershell
docker compose ps
```

## PostgreSQL Layout

Current migration plan targets **one PostgreSQL container instance** with multiple databases for service isolation:

- `aim_auth`
- `aim_user`
- `aim_chat`

Database creation is initialized by SQL files mounted to:

- `./deploy/postgres/init` -> `/docker-entrypoint-initdb.d`

## Notes

- Runtime has been migrated to PostgreSQL for `auth-service` / `user-service` / `chat-service`.
- Message storage uses `jsonb` in `messages.content` (PostgreSQL).
