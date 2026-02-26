# Observability SaaS Backend

## How to run locally
cd ../../platform
docker compose up -d --build

## Structure
- cmd/server/main.go → entry point
- internal/domain → models
- internal/usecase → business logic
- internal/handler → HTTP routes
- internal/repository → database
