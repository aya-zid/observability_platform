# Backend API

Go API server for the Medianet Observability platform.

**Tech stack:** Go 1.25 · Echo v4 · GORM · PostgreSQL 16 · Redis 7 · JWT (HS256)

## Quick Start

```bash
cd platform
docker compose up -d --build
```

The API runs on port `8080`.

## Environment Variables

Copy `.env.example` to `.env` and fill in real values before deploying.

### Required Variables

| Variable | Description | Default |
|---|---|---|
| `JWT_SECRET` | Secret key for JWT signing. Min 32 characters. | — (must set) |
| `POSTGRES_USER` | Database username | `postgres` |
| `POSTGRES_PASSWORD` | Database password | — (must set) |
| `POSTGRES_DB` | Database name | `observability` |
| `REDIS_HOST` | Redis hostname | `redis` |
| `REDIS_PORT` | Redis port | `6379` |
| `APP_PORT` | API listen port | `8080` |
| `FRONTEND_ORIGIN` | Allowed CORS origin | `http://localhost:5173` |

### API Keys

The platform uses external APIs for AI analysis and CVE correlation. Customers must obtain their own keys.

| Variable | Purpose | How to Get |
|---|---|---|
| `GEMINI_API_KEY` | AI analysis (Gemini Pro) | Get a free key from [Google AI Studio](https://aistudio.google.com). |
| `GROQ_API_KEY` | AI analysis fallback (Llama) | Get a free key from [Groq Console](https://console.groq.com). |
| `NVD_API_KEY` | CVE vulnerability data from NIST | Request a key from [NIST NVD](https://nvd.nist.gov/developers/request-an-api-key) (free, rate-limited without one). |

### Email (SMTP)

| Variable | Description | Dev Default |
|---|---|---|
| `SMTP_HOST` | SMTP server hostname | `send.mailtrap.cloud` |
| `SMTP_PORT` | SMTP server port | `2525` |
| `SMTP_USERNAME` | SMTP username | `api` |
| `SMTP_PASSWORD` | SMTP password | — (must set) |
| `SMTP_FROM` | Sender email address | `noreply@medianet.tn` |

Dev values use Mailtrap sandbox. Replace with real SMTP credentials for production.

### Security

| Variable | Description | Dev Default |
|---|---|---|
| `JWT_SECRET` | HS256 signing key. Min 32 chars. | `super-secret-change-me-in-prod-32-chars-min` |
| `POSTGRES_PASSWORD` | Database password | `pass1234` |

**⚠️ Change these before deploying to any real environment.**

## Architecture

```
                    ┌─────────────┐
                    │   Frontend  │
                    │  :5173      │
                    └──────┬──────┘
                           │ HTTP
                    ┌──────▼──────┐
                    │  Backend    │
                    │  :8080      │
                    └──┬───┬──┬──┘
                       │   │  │
          ┌────────────┘   │  └────────────┐
          ▼                ▼               ▼
    ┌──────────┐    ┌──────────┐    ┌──────────┐
    │PostgreSQL│    │  Redis   │    │VictoriaM.│
    │  :5432   │    │  :6379   │    │  :8428   │
    └──────────┘    └──────────┘    └──────────┘
```

## Project Structure

```
cmd/server/main.go              # Entry point
internal/
├── config/config.go            # Env loading and validation
├── domain/                     # GORM models (18 entities)
├── dto/                        # Request/response DTOs
├── handler/                    # Echo route handlers
│   └── routes.go               # All route registrations
├── infrastructure/             # JWT, password hashing, UUID generation
├── middleware/                  # Auth, CSRF, session middleware
├── repository/                 # Data access layer (GORM)
├── service/                    # Business logic (37+ files)
└── validator/                  # Input validation helpers
migrations/                     # Database migrations
```

## API Endpoints

### Auth

| Method | Path | Description |
|---|---|---|
| `POST` | `/auth/register` | Register new account |
| `POST` | `/auth/login` | Login, sets session cookies |
| `POST` | `/auth/logout` | Clear session |
| `GET` | `/auth/me` | Current user info |

### Integrations

| Method | Path | Description |
|---|---|---|
| `GET` | `/integrations` | List customer integrations |
| `POST` | `/integrations` | Create new integration |
| `PUT` | `/integrations/:id` | Update integration |
| `DELETE` | `/integrations/:id` | Remove integration |
| `POST` | `/integrations/:id/revoke` | Revoke API key |
| `GET` | `/integrations/:id/install-script` | Download install script |
| `POST` | `/integrations/:id/reinstall` | Regenerate install package |

### Metrics & Logs

| Method | Path | Description |
|---|---|---|
| `GET` | `/metrics/summary` | Account-wide metrics overview |
| `GET` | `/metrics/hosts` | Per-host metrics |
| `GET` | `/metrics/services` | Per-service metrics |
| `GET` | `/logs` | Query logs (with filters) |
| `GET` | `/logs/live` | Live log stream (SSE) |
| `GET` | `/logs/export` | Export logs as CSV |
| `GET` | `/logs/stats` | Log statistics |
| `GET` | `/logs/analyze` | AI log analysis |

### Services (APM)

| Method | Path | Description |
|---|---|---|
| `GET` | `/services` | List services |
| `GET` | `/services/:id` | Service detail |
| `GET` | `/services/:id/traces` | Traces for service |

### Dashboards

| Method | Path | Description |
|---|---|---|
| `GET` | `/dashboards` | List dashboards |
| `POST` | `/dashboards` | Create dashboard |
| `GET` | `/dashboards/:id` | Dashboard detail |
| `PUT` | `/dashboards/:id` | Update dashboard |
| `DELETE` | `/dashboards/:id` | Delete dashboard |

### Reports & Analytics

| Method | Path | Description |
|---|---|---|
| `GET` | `/reports` | List reports |
| `POST` | `/reports/generate` | Generate new report (AI) |
| `GET` | `/reports/:id` | Report detail |
| `GET` | `/analytics/trends` | Trend data |
| `GET` | `/analytics/correlations` | Correlation analysis |

### Alerts

| Method | Path | Description |
|---|---|---|
| `GET` | `/alerts` | List alert rules |
| `POST` | `/alerts` | Create alert rule |
| `PUT` | `/alerts/:id` | Update rule |
| `DELETE` | `/alerts/:id` | Delete rule |
| `GET` | `/alerts/history` | Alert history |
| `POST` | `/alerts/:id/acknowledge` | Acknowledge alert |

### Notifications

| Method | Path | Description |
|---|---|---|
| `GET` | `/notifications` | List notifications |
| `PUT` | `/notifications/:id/read` | Mark as read |
| `PUT` | `/notifications/read-all` | Mark all as read |
| `GET` | `/notifications/preferences` | Get notification preferences |
| `PUT` | `/notifications/preferences` | Update preferences |

### Users & Organizations

| Method | Path | Description |
|---|---|---|
| `GET` | `/users` | List users |
| `PUT` | `/users/:id` | Update user |
| `GET` | `/organizations/:id` | Organization detail |
| `PUT` | `/organizations/:id` | Update organization |

### System

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | Health check |
| `GET` | `/csrf-token` | Get CSRF token |

## Authentication

The backend uses **cookie-based sessions** with JWT tokens:

- **Access token**: Short-lived, stored in HttpOnly cookie
- **Refresh token**: Longer-lived, stored in separate HttpOnly cookie
- **CSRF protection**: Double-submit cookie pattern

### Auth Flow

```
Login → Server generates JWT pair → Sets HttpOnly cookies
Request → Middleware reads cookie → Validates JWT → Sets user context
```

### Admin vs User

Admin and regular user tokens use different claim types. Admin routes are prefixed with `/admin/`.

## Services Layer

The service layer contains 37+ service files:

| Service | Purpose |
|---|---|
| `auth_service.go` | Registration, login, JWT management |
| `integration_service.go` | Customer onboarding, install scripts |
| `report_service.go` | AI report generation (Gemini/Groq) |
| `cve_service.go` | NVD CVE correlation |
| `email_sender.go` | SMTP email notifications |
| `log_service.go` | Log aggregation queries |
| `metric_service.go` | Metrics queries (VictoriaMetrics) |
| `alert_service.go` | Alert rule evaluation |
| `dashboard_service.go` | Dashboard CRUD |
| `notification_service.go` | Notification management |
| `user_service.go` | User management |
| `organization_service.go` | Organization management |
| `analytics_service.go` | Trend and correlation analysis |
| `trace_service.go` | Distributed tracing (future) |

## Known Issues

### Integration Scripts Use Embedded Python

The current `integration_service.go` generates install scripts with embedded Python heredocs for YAML patching. This requires Python 3 to be installed on customer hosts.

**Fix needed:** Rewrite the YAML patching logic in pure bash (e.g., using `sed` or `yq`).

## Development

```bash
# Run locally (from platform/)
docker compose up -d --build

# Run tests
cd backend && go test ./...

# Lint
cd backend && golangci-lint run
```

## Production Notes

1. **Change default credentials** — `JWT_SECRET` and `POSTGRES_PASSWORD` must be changed
2. **Set real API keys** — Gemini, Groq, NVD keys are required for AI features
3. **Configure SMTP** — Replace Mailtrap with production email provider
4. **CORS** — Set `FRONTEND_ORIGIN` to your production domain
5. **TLS** — Run behind a reverse proxy (nginx/traefik) for HTTPS
6. **Database** — Use a managed PostgreSQL in production, not the Docker container
7. **Redis** — Use a managed Redis for session storage
8. **VictoriaMetrics** — Use a persistent volume for metrics storage

## Troubleshooting

| Symptom | Fix |
|---|---|
| `JWT_SECRET` error | Ensure it's at least 32 characters |
| DB connection refused | Check PostgreSQL container is running: `docker ps` |
| CORS errors | Verify `FRONTEND_ORIGIN` matches your frontend URL |
| Email not sending | Check SMTP credentials, check spam folder |
| AI analysis fails | Verify `GEMINI_API_KEY` and `GROQ_API_KEY` are set |
| CVE data missing | Verify `NVD_API_KEY` is set (optional but rate-limited without it) |
