# Observability SaaS Backend

Backend service for the Observability Platform, built with:
- Echo (HTTP server)
- GORM + PostgreSQL (persistence)
- JWT (authentication)

## Run Locally

From the repository root:

```bash
cd platform
docker compose up -d --build
```

## Environment Variables

Copy `.env.example` to `.env` and adjust values as needed.

Notes:
- `JWT_SECRET` must be at least 32 characters.
- Current server startup binds on `:8080`.

## Project Structure

- `cmd/server/main.go`: application entry point
- `internal/config`: environment loading and validation
- `internal/domain`: GORM models
- `internal/dto`: request/response DTOs
- `internal/handler`: Echo route registration and handlers
- `internal/infrastructure`: database and JWT infrastructure
- `internal/middleware`: HTTP middleware
- `internal/repository`: data access layer
- `internal/service`: business logic layer
- `internal/validator`: input validation helpers
- `migrations`: migration assets
- `tests`: test code

