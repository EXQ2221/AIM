# AIM

AIM is an AI-native collaborative chat platform built with Go microservices.

## Runtime Baseline

- Go microservices: `gateway`, `auth-service`, `user-service`, `chat-service`
- Data stores: PostgreSQL + Redis
- Container orchestration: Docker Compose

## Quick Start

### 1) Prepare environment

Copy `.env.example` to `.env` and fill required values:

```powershell
Copy-Item .env.example .env
```

At minimum:

- `POSTGRES_USER`
- `POSTGRES_PASSWORD`
- `POSTGRES_DATABASE`
- `JWT_SECRET`

For RAG features, also configure:

- `EMBEDDING_BASE_URL`
- `EMBEDDING_API_KEY`
- `EMBEDDING_MODEL`
- `EMBEDDING_DIMENSION`
- `EMBEDDING_TIMEOUT_SECONDS`
- `RAG_CHUNK_SIZE`
- `RAG_CHUNK_OVERLAP`
- `RAG_TOP_K`

### 2) Build and start

```powershell
docker compose up -d --build
```

### 3) Verify service health

```powershell
docker compose ps
```

Gateway health check:

```powershell
curl http://127.0.0.1:8080/healthz
```

## PostgreSQL Layout

Current plan uses one PostgreSQL instance with multi-database isolation:

- `aim_auth`
- `aim_user`
- `aim_chat`

Init SQLs are mounted from:

- `./deploy/postgres/init` -> `/docker-entrypoint-initdb.d`

## RAG Prerequisites (Important)

### 1) pgvector must exist in PostgreSQL runtime

`chat-service` creates `knowledge_chunks.embedding vector(...)`. If `vector` type is missing, `chat-service` becomes unhealthy.

Check extension:

```powershell
docker exec aim-postgres psql -U aim -d aim_chat -c "SELECT extname FROM pg_extension WHERE extname='vector';"
```

Expected:

- one row: `vector`

If zero rows, enable it:

```powershell
docker exec aim-postgres psql -U aim -d aim_chat -c "CREATE EXTENSION IF NOT EXISTS vector;"
```

If extension package is missing in container (`extension "vector" is not available`), install pgvector in the PostgreSQL runtime image/environment first.

### 2) Embedding model must be compatible with OpenAI-compatible embeddings API

RAG embedding calls use `/embeddings`. If model is incompatible, knowledge document processing/search may fail with `model_not_supported`.

## Minimal RAG Smoke Flow

1. Create knowledge base:
- `POST /api/v1/knowledge-bases`

2. Import text/markdown document:
- `POST /api/v1/knowledge-bases/{knowledgeBaseId}/documents/text`

3. Verify document status:
- `GET /api/v1/knowledge-bases/{knowledgeBaseId}/documents`

4. Search chunks:
- `POST /api/v1/knowledge-bases/{knowledgeBaseId}/search`

5. Bind KB to group conversation:
- `POST /api/v1/conversations/{conversationId}/knowledge-bases`

6. Verify bindings:
- `GET /api/v1/conversations/{conversationId}/knowledge-bases`

## Notes

- Runtime has been migrated to PostgreSQL for `auth-service` / `user-service` / `chat-service`.
- Message storage uses PostgreSQL `jsonb` in `messages.content`.
