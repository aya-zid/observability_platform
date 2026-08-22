# Medianet Observability — Backend API

Go HTTP server that powers the entire Medianet Observability platform: authentication, multi-tenant SaaS management, agent data ingestion, metric/log querying, alerting, AI-powered reports, CVE vulnerability scanning, billing, and team management.

---

## Table of Contents

- [What This Does](#what-this-does)
- [Tech Stack](#tech-stack)
- [Architecture](#architecture)
- [Environment Variables](#environment-variables)
- [Getting API Keys](#getting-api-keys)
- [Setup and Running](#setup-and-running)
- [Database Schema](#database-schema)
- [Authentication System](#authentication-system)
- [API Endpoints](#api-endpoints)
- [Service Catalog](#service-catalog)
- [Alert Engine](#alert-engine)
- [AI Reports](#ai-reports)
- [CVE Vulnerability Scanning](#cve-vulnerability-scanning)
- [Background Workers](#background-workers)
- [Known Issues](#known-issues)
- [Production Notes](#production-notes)
- [Troubleshooting](#troubleshooting)

---

## What This Does

The backend is the central brain of the platform. It:

- **Authenticates** users and admin accounts (JWT + HttpOnly cookies)
- **Manages tenants** — company accounts, subscriptions, billing, team invitations
- **Receives agent data** — host registration, heartbeats, service discovery, metrics, logs, traces
- **Proxies queries** to VictoriaMetrics (PromQL) and Loki (LogQL)
- **Evaluates alert rules** against live telemetry data, fires notifications
- **Generates AI reports** using Google Gemini / Groq
- **Scans for CVEs** using the NIST NVD API
- **Serves install/integration scripts** for agent deployment
- **Enforces plan limits** — retention, daily quotas, host limits

---

## Tech Stack

| Category | Technology |
|---|---|
| Language | Go 1.25 |
| HTTP Framework | Echo v4 |
| ORM | GORM (PostgreSQL driver) |
| Database | PostgreSQL 16 |
| Cache / Sessions | Redis 7 (go-redis) |
| Authentication | JWT (HS256) + HttpOnly cookies |
| Password Hashing | bcrypt (golang.org/x/crypto) |
| Validation | go-playground/validator |
| Metrics Storage | VictoriaMetrics (cluster) |
| Log Storage | Grafana Loki |
| Trace Storage | Grafana Tempo (not yet active) |
| AI Reports | Google Gemini / Groq (configurable, with fallback) |
| CVE Scanning | NIST NVD REST API |
| Email | SMTP (configurable) |
| Containerization | Docker (Alpine-based, multi-stage) |

---

## Architecture

```
                            +-----------------+
                            |  Agent (host)   |
                            |  install.sh     |
                            +--------+--------+
                                     |
                     HTTP (metrics, logs, discovery, heartbeat)
                                     |
                                     v
+-----------------------------------------------------------+
|                    Go Backend (:8080)                       |
|                                                           |
|  +-------------+  +-----------+  +-------------+          |
|  | Auth        |  | Discovery |  | Query Proxy |          |
|  | (JWT,       |  | (hosts,   |  | (PromQL,    |          |
|  |  cookies)   |  |  services)|  |  LogQL)     |          |
|  +-------------+  +-----------+  +-------------+          |
|                                                           |
|  +-------------+  +-----------+  +-------------+          |
|  | Alert       |  | Billing   |  | Reports     |          |
|  | Engine      |  | (plans,   |  | (Gemini/    |          |
|  | (eval loop) |  |  subs)    |  |  Groq AI)   |          |
|  +-------------+  +-----------+  +-------------+          |
|                                                           |
|  +-------------+  +-----------+  +-------------+          |
|  | CVE Scanner |  | Email     |  | Usage       |          |
|  | (NVD API)   |  | (SMTP)    |  | Metering    |          |
|  +-------------+  +-----------+  +-------------+          |
|                                                           |
+---------+-----------+-----------+-----------+--------+----+
          |           |           |           |        |
          v           v           v           v        v
     +--------+  +--------+  +-------+  +--------+  +-----+
     |Postgres|  | Redis  |  |VM     |  | Loki   |  | NVD |
     | (data) |  | (cache)|  |(metrics)| | (logs) |  | API |
     +--------+  +--------+  +-------+  +--------+  +-----+

+-----------------------------------------------------------+
|  Frontends                                                 |
|  Tenant UI (:5173)  <-->  Backend (:8080)  <-- Admin UI   |
|  Admin UI  (:5174)  <-->  Backend (:8080)                  |
+-----------------------------------------------------------+
```

### Code Layers

```
cmd/server/main.go              # Entry point, wires everything
internal/
├── config/                     # Env loading, validation
├── domain/                     # GORM models (24 tables)
├── dto/                        # Request/response structs
├── handler/                    # Echo route handlers (35+ files)
│   └── routes.go               # All route registrations
├── infrastructure/             # DB, Redis, JWT, API key gen
├── middleware/                  # Auth, CSRF, rate limiting
├── proxy/                      # Reverse proxy to VM/Loki/Tempo
├── repository/                 # Data access layer (16 files)
├── service/                    # Business logic (40+ files)
├── cache/                      # Redis-backed tenant cache
├── catalog/                    # Service catalog + integration guides
└── validator/                  # Input validation
```

---

## Environment Variables

Copy `.env.example` to `.env` and fill in your values.

> **Important:** The `.env.example` file contains dev defaults. Change `JWT_SECRET`, `REFRESH_TOKEN_PEPPER`, `DB_PASSWORD`, and `SMTP_PASS` before deploying to production.

### Database

| Variable | Default | Description |
|---|---|---|
| `DB_HOST` | `postgres` | PostgreSQL hostname |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USER` | `obsuser` | PostgreSQL username |
| `DB_PASSWORD` | `pass1234` | PostgreSQL password (**change in production**) |
| `DB_NAME` | `obs_saas_dev` | Database name |
| `DB_SSLMODE` | `disable` | SSL mode (`disable` for local, `require` for production) |

### Authentication

| Variable | Default | Description |
|---|---|---|
| `JWT_SECRET` | `super-secret-change-me-in-prod-32-chars-min` | HS256 signing key. **Must be 32+ chars. Change in production.** |
| `REFRESH_TOKEN_PEPPER` | `dev-refresh-token-pepper-32-chars-minimum!` | HMAC key for refresh tokens. **Must be 32+ chars. Change in production.** |
| `JWT_ISSUER` | `obs-platform` | JWT issuer claim |
| `JWT_AUDIENCE` | `obs-api` | JWT audience claim |
| `ACCESS_TOKEN_TTL_MINUTES` | `15` | Access token lifetime (1-1440 min) |
| `REFRESH_TOKEN_TTL_DAYS` | `7` | Refresh token lifetime (1-365 days) |
| `PASSWORD_RESET_TOKEN_TTL_MINUTES` | `60` | Password reset link expiry |

### Cookies

| Variable | Default | Description |
|---|---|---|
| `COOKIE_DOMAIN` | *(empty)* | Set to your domain in production |
| `COOKIE_SECURE` | `false` | Set to `true` in production (requires HTTPS) |
| `COOKIE_SAMESITE` | `Lax` | `Lax`, `Strict`, or `None` |

### CORS

| Variable | Default | Description |
|---|---|---|
| `CORS_ALLOW_ORIGINS` | `http://localhost:5173,http://localhost:5174` | Comma-separated. **Must include your frontend URLs in production.** |

### Rate Limiting

| Variable | Default | Description |
|---|---|---|
| `AUTH_RATE_LIMIT_WINDOW_SECONDS` | `60` | Rate limit window |
| `AUTH_SIGNUP_RATE_LIMIT_PER_WINDOW` | `5` | Max signups per window per IP |
| `AUTH_LOGIN_RATE_LIMIT_PER_WINDOW` | `10` | Max login attempts per window per IP |
| `AUTH_REFRESH_RATE_LIMIT_PER_WINDOW` | `20` | Max token refreshes per window per IP |

### Server & Redis

| Variable | Default | Description |
|---|---|---|
| `SERVER_PORT` | `8080` | Backend listen port |
| `REDIS_HOST` | `redis` | Redis hostname |
| `REDIS_PORT` | `6379` | Redis port |
| `REDIS_PASSWORD` | *(empty)* | Redis password (empty = no auth) |

### Infrastructure (Internal)

These connect the backend to the observability stack. **Docker-internal addresses — not exposed to agents or the internet.**

| Variable | Default | Description |
|---|---|---|
| `VMINSERT_HOST` | `vminsert` | VictoriaMetrics insert endpoint |
| `VMINSERT_PORT` | `8480` | VictoriaMetrics insert port |
| `VMSELECT_HOST` | `vmselect` | VictoriaMetrics query endpoint |
| `VMSELECT_PORT` | `8481` | VictoriaMetrics query port |
| `LOKI_HOST` | `loki` | Loki endpoint |
| `LOKI_PORT` | `3100` | Loki port |
| `TEMPO_HOST` | `tempo` | Tempo endpoint (not yet active) |
| `TEMPO_PORT` | `4318` | Tempo OTLP port (not yet active) |
| `GRAFANA_URL` | `http://grafana:3000` | Grafana URL (testing only) |
| `AGENT_INSTALL_SCRIPT_PATH` | `/opt/agent/install.sh` | Path to agent install script |

### SMTP / Email

Used for password resets, team invitations, and alert notifications.

| Variable | Default | Description |
|---|---|---|
| `SMTP_HOST` | `smtp.gmail.com` | SMTP server hostname |
| `SMTP_PORT` | `587` | SMTP port (STARTTLS) |
| `SMTP_USER` | *(required for email)* | SMTP username (your email address) |
| `SMTP_PASS` | *(required for email)* | SMTP password (app-specific password for Gmail) |
| `SMTP_FROM_ADDRESS` | *(required for email)* | "From" email address |
| `SMTP_FROM_NAME` | `Observability Medianet` | "From" display name |
| `PLATFORM_URL` | `http://localhost:5173` | Base URL for email links |

### AI Reports

| Variable | Default | Description |
|---|---|---|
| `GEMINI_API_KEY` | *(empty)* | Google Gemini API key |
| `GEMINI_MODEL` | `gemini-3.5-flash` | Gemini model to use |
| `AI_PROVIDER` | `gemini` | Primary provider: `"gemini"` or `"groq"` |
| `GROQ_API_KEY` | *(empty)* | Groq API key (used as fallback) |
| `GROQ_MODEL` | `openai/gpt-oss-120b` | Groq model to use |
| `AI_CHUNK_BYTES` | `35000` | Max payload size before chunking per region |

### NVD Vulnerability Scanning

| Variable | Default | Description |
|---|---|---|
| `NVD_API_KEY` | *(empty but recommended)* | NIST NVD API key. Without it, rate limits will block the scanner. |
| `VULN_SCAN_INTERVAL_HOURS` | `2` | How often to scan for CVEs |
| `VULNERABILITY_SEVERITY_MINIMUM` | `MEDIUM` | Minimum severity: `MEDIUM`, `HIGH`, or `CRITICAL` |

---

## Getting API Keys

### Google Gemini (for AI reports)

1. Go to [Google AI Studio](https://aistudio.google.com)
2. Sign in with your Google account
3. Click "Get API key" then "Create API key"
4. Copy the key and set it as `GEMINI_API_KEY` in `.env`
5. Free tier is sufficient for development

### Groq (for AI reports fallback)

1. Go to [Groq Console](https://console.groq.com)
2. Sign up / log in
3. Go to API Keys then Create API Key
4. Copy the key and set it as `GROQ_API_KEY` in `.env`
5. Free tier available

### NIST NVD (for CVE vulnerability scanning)

1. Go to [NIST NVD Developers](https://nvd.nist.gov/developers/request-an-api-key)
2. Click "Request API Key"
3. Fill in the form (free, takes about 1 minute)
4. You will receive the key by email (usually within seconds)
5. Set it as `NVD_API_KEY` in `.env`
6. Without this key, the NVD API rate-limits to about 5 requests per 30 seconds. With a key, it is about 50 requests per 30 seconds.

### SMTP (for emails)

**For development (Gmail app password):**

1. Go to your Google Account settings > Security
2. Enable 2-Step Verification if not already enabled
3. Go to "App passwords" (search for it in the search bar)
4. Select "Mail" and your device, click "Generate"
5. Copy the 16-character password
6. Set these in `.env`:
   - `SMTP_USER=your-email@gmail.com`
   - `SMTP_PASS=your-16-char-app-password`
   - `SMTP_FROM_ADDRESS=your-email@gmail.com`
   - `SMTP_HOST=smtp.gmail.com`
   - `SMTP_PORT=587`

**For production:**

Use your transactional email provider (SendGrid, Mailgun, AWS SES, etc.). Update the SMTP settings accordingly.

**Without SMTP configured:** The backend still starts. Emails are logged to the console but not sent. Password resets and team invitations will not work without email.

---

## Setup and Running

### Docker (recommended)

From the `platform/` directory:

```bash
docker compose up -d --build
```

This starts: PostgreSQL (port 5434), Redis, Backend (port 8080), Frontend (port 5173), Admin Frontend (port 5174), pgAdmin (port 5050).

### Local Development

```bash
cd platform/backend

# Copy and edit environment
cp .env.example .env

# Install dependencies
go mod download

# Run the server
go run cmd/server/main.go
```

Requirements: Go 1.25+, PostgreSQL 16, Redis 7, running observability infrastructure (VictoriaMetrics, Loki).

### What Happens at Startup

1. Loads config from `.env` via `godotenv`
2. Validates all config values (fatal on invalid)
3. Connects to PostgreSQL (pool: 25 open / 10 idle, 5min lifetime)
4. Connects to Redis (pool: 25 open, 5s dial timeout)
5. Runs GORM AutoMigrate for 24 domain models
6. Backfills `status` column on users table
7. Seeds 6 default subscription plans if none exist
8. Starts background goroutines:
   - CVE vulnerability scanner (configurable interval, default 2h)
   - Usage metering flush (Redis to Postgres, every 5 min)
   - Alert rule evaluator (configurable interval)
   - Notification dispatcher (background)
   - Subscription expiration checker (every 1 hour)
   - Retention cleanup worker (every 1 min)
9. Starts Echo HTTP server on port 8080

### Default Plans Seeded

| Plan | Hosts | Log Retention | Metrics Retention | Price/mo |
|------|-------|---------------|-------------------|----------|
| Free | 2 | 3 days | 7 days | $0 |
| Basic | 5 | 7 days | 14 days | $19 |
| Pro | 10 | 14 days | 30 days | $49 |
| Premium | 25 | 30 days | 60 days | $99 |
| Enterprise | 50 | 30 days | 90 days | $199 |
| Trial | 25 | 30 days | 60 days | $0 (14-day trial) |

---

## Database Schema

24 tables managed by GORM AutoMigrate at startup. No SQL migration files — schema is defined entirely through Go struct tags.

### Core Tables

| Table | Description |
|---|---|
| `tenants` | Company accounts. UUID-based, API key for agent auth, subscription status. |
| `users` | Tenant-scoped user accounts. Roles: `admin`, `member`. |
| `admins` | Platform administrator accounts (manage all tenants). |
| `hosts` | Monitored servers. Unique per (tenant, machine_id). Last seen via heartbeat. |
| `discovered_services` | Software services found on hosts (nginx, postgres, etc.). Log/metric collection config. |
| `discovered_service_log_paths` | Additional log file paths for a service. |
| `discovered_container_services` | Docker containers running services. |
| `service_vulnerabilities` | CVE findings for discovered services. |
| `nginx_virtual_hosts` | Nginx virtual host configs (domain, log path, log kind). |
| `apache_virtual_hosts` | Apache virtual host configs. |
| `invitations` | Pending team invitations (token-based, 72h expiry). |
| `password_reset_tokens` | Password reset links (token-based, 60min expiry). |
| `notification_channels` | Alert notification destinations. |
| `plans` | Subscription plan definitions (pricing, limits). |
| `tenant_subscriptions` | Links tenants to plans. One active subscription per tenant. |
| `tenant_usage_dailies` | Daily usage counters (log bytes, metric samples). |
| `alert_rules` | Alert rule definitions with template references. |
| `alert_rule_scopes` | Host/service/domain scope constraints for alert rules. |
| `alert_thresholds` | Threshold values and severity levels for alert rules. |
| `alert_events` | Alert firing/resolved events. |
| `incidents` | Correlated alert events with root cause analysis. |
| `incident_signals` | Individual signals within an incident. |
| `dismissed_incidents` | Incidents dismissed by users. |
| `reports` | AI-generated reports (JSONB payloads). |

---

## Authentication System

### Three Auth Mechanisms

**1. Browser Auth (JWT in HttpOnly cookies)**

Used by the tenant frontend and admin frontend. Two tokens:

- **Access token** (`access_token` cookie): Short-lived (15 min default). Contains user ID, tenant ID, email, role.
- **Refresh token** (`refresh_token` cookie): Longer-lived (7 days default). HMAC-SHA256 hashed, stored in Redis.

CSRF protection uses double-submit cookie pattern.

**2. Agent Ingest Auth (API key via Basic Auth)**

Used by observability agents. The API key acts as both username and password in HTTP Basic Auth. Resolved through Redis cache (5 min TTL) then Postgres fallback.

API keys are generated with 256-bit entropy, prefixed with `obs_live_`. Stored in cleartext in the database by design.

**3. Admin Auth (separate JWT)**

Platform administrators use separate JWT tokens with different issuer/audience claims (`obs-platform-admin` / `obs-api-admin`). Stored in `admin_access_token` cookie.

### RBAC (Role-Based Access Control)

| Role | Permissions |
|---|---|
| `admin` | Full access to all mutations (CRUD on all resources) |
| `member` | Read-only access to most routes. Can update own profile and dismiss incidents. Mutations blocked. |

Members can still GET on admin-only routes but cannot POST/PUT/PATCH/DELETE.

### Token Flow

```
Login -> Validate credentials -> Generate access JWT + refresh token
      -> Set HttpOnly cookies -> Return user info

Request -> Middleware reads cookie -> Validates JWT -> Checks:
          - Token not expired
          - Tenant is active
          - User is not deactivated
          - Subscription is not expired/suspended
        -> Sets user context

Refresh -> Validate refresh token from Redis -> Generate new pair
         -> Delete old refresh token -> Set new cookies
```

---

## API Endpoints

### Public Routes (no auth)

| Method | Path | Description |
|---|---|---|
| `POST` | `/signup` | Register new account (rate-limited) |
| `POST` | `/login` | Login (rate-limited) |
| `POST` | `/refresh` | Refresh tokens (rate-limited) |
| `POST` | `/logout` | Clear session |
| `POST` | `/forgot-password` | Request password reset email (rate-limited) |
| `GET` | `/reset-password/:token` | Validate reset token |
| `POST` | `/reset-password/:token` | Set new password (rate-limited) |
| `GET` | `/install.sh` | Agent install script |
| `GET` | `/uninstall.sh` | Agent uninstall script |
| `GET` | `/integrations/nginx.sh` | Nginx integration script |
| `GET` | `/integrations/generic.sh` | Generic service integration script |
| `GET` | `/invite/:token` | Validate invitation token |
| `POST` | `/invite/:token/accept` | Accept invitation |
| `GET` | `/plans` | List active subscription plans |
| `GET` | `/health` | Health check (DB + Redis status) |

### Platform API (`/api/` — JWT auth required)

**Profile & Settings (member accessible):**

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/profile` | Get own profile |
| `PATCH` | `/api/profile` | Update own profile |
| `POST` | `/api/profile/change-password` | Change own password |
| `GET` | `/api/settings/org` | Get organization settings |

**Service Configuration (member accessible):**

| Method | Path | Description |
|---|---|---|
| `PATCH` | `/api/discovery/services/:id/collection` | Enable/disable log/metric collection |
| `PATCH` | `/api/discovery/services/:id/config` | Update service config |
| `POST` | `/api/discovery/services/:id/integration` | Generate integration install command |

**Incidents (member accessible):**

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/incidents/:id/dismiss` | Dismiss an incident |

**Tenant & API Key (admin mutations only):**

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/me` | Get tenant info |
| `GET` | `/api/api-key` | Get API key |
| `POST` | `/api/api-key/rotate` | Rotate API key |

**Team Management (admin only):**

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/settings/team` | List team members |
| `POST` | `/api/settings/team/invite` | Invite teammate |
| `DELETE` | `/api/settings/team/:userId` | Remove member |
| `PATCH` | `/api/settings/team/:userId/role` | Change role |
| `PATCH` | `/api/settings/team/:userId/status` | Activate/deactivate |
| `POST` | `/api/settings/team/invitations/:id/resend` | Resend invitation |
| `DELETE` | `/api/settings/team/invitations/:id` | Revoke invitation |

**Notifications & Billing (admin only):**

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/settings/notifications` | List notification channels |
| `POST` | `/api/settings/notifications` | Add channel |
| `DELETE` | `/api/settings/notifications/:id` | Delete channel |
| `GET` | `/api/settings/billing` | Get billing overview |
| `POST` | `/api/settings/billing/change-plan` | Change subscription plan |
| `GET` | `/api/settings/billing/usage` | Get usage history |
| `PATCH` | `/api/settings/org` | Update org settings |

**Alerts (admin mutations):**

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/alerts/templates` | List all alert templates |
| `GET` | `/api/alerts/templates/:serviceType` | List templates by service type |
| `POST` | `/api/alerts/rules` | Create alert rule |
| `GET` | `/api/alerts/rules` | List alert rules |
| `PATCH` | `/api/alerts/rules/:id` | Update alert rule |
| `DELETE` | `/api/alerts/rules/:id` | Delete alert rule |
| `GET` | `/api/alerts/events` | List alert events |
| `GET` | `/api/alerts/events/firing` | List currently firing events |
| `POST` | `/api/alerts/events/:id/acknowledge` | Acknowledge event |
| `GET` | `/api/alerts/vulnerabilities` | List vulnerability alerts |

**Overview & Reports (admin mutations):**

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/overview` | Overview dashboard data |
| `POST` | `/api/reports/generate` | Generate AI report |
| `GET` | `/api/reports` | List reports |
| `GET` | `/api/reports/:id` | Get report detail |
| `POST` | `/api/reports/:id/regenerate` | Regenerate report |
| `DELETE` | `/api/reports/:id` | Delete report |

**Discovery (admin mutations):**

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/catalog` | Service catalog |
| `GET` | `/api/discovery/hosts` | List hosts |
| `GET` | `/api/discovery/services` | List discovered services |
| `GET` | `/api/discovery/services/:id` | Service detail |
| `GET` | `/api/discovery/containers` | List containers |
| `GET` | `/api/discovery/containers/:id` | Container detail |

**Query Proxy (admin mutations):**

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/query/metrics` | Query VictoriaMetrics (PromQL) |
| `GET` | `/api/query/logs` | Query Loki (LogQL) |
| `GET` | `/api/query/logs/range` | Query Loki range |
| `GET` | `/api/query/logs/labels` | List Loki labels |
| `GET` | `/api/grafana/dashboard-url` | Generate Grafana dashboard URL |

**Service Dashboards (admin mutations):**

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/nginx/:id/domains` | Nginx virtual hosts |
| `GET` | `/api/nginx/:id/dashboard` | Nginx dashboard |
| `GET` | `/api/nginx/:id/explorer` | Nginx log explorer |
| `GET` | `/api/nginx/:id/error-logs` | Nginx error logs |
| `GET` | `/api/nginx/:id/ip-investigation` | Nginx IP investigation |
| `GET` | `/api/nginx/:id/slow-requests` | Nginx slow requests |
| `GET` | `/api/nginx/:id/performance/heatmap` | Nginx performance heatmap |
| `GET` | `/api/nginx/:id/performance/scatter` | Nginx scatter plot |
| `GET` | `/api/nginx/:id/performance/dist` | Nginx distribution |
| `GET` | `/api/postgres/:id/dashboard` | PostgreSQL dashboard |
| `GET` | `/api/postgres/:id/config` | PostgreSQL config |
| `GET` | `/api/postgres/:id/logs` | PostgreSQL logs |
| `GET` | `/api/phpfpm/:id/dashboard` | PHP-FPM dashboard |
| `GET` | `/api/phpfpm/:id/logs` | PHP-FPM logs |
| `GET` | `/api/redis/:id/dashboard` | Redis dashboard |
| `GET` | `/api/redis/:id/logs` | Redis logs |
| `GET` | `/api/mariadb/:id/dashboard` | MariaDB dashboard |
| `GET` | `/api/mariadb/:id/config` | MariaDB config |
| `GET` | `/api/mariadb/:id/logs` | MariaDB logs |
| `GET` | `/api/mariadb/:id/error-logs` | MariaDB error logs |
| `GET` | `/api/mariadb/:id/slow-query-logs` | MariaDB slow queries |
| `GET` | `/api/apache/:id/dashboard` | Apache dashboard |
| `GET` | `/api/apache/:id/slow-requests` | Apache slow requests |
| `GET` | `/api/apache/:id/explorer` | Apache log explorer |
| `GET` | `/api/apache/:id/domains` | Apache virtual hosts |
| `GET` | `/api/apache/:id/error-logs` | Apache error logs |
| `GET` | `/api/apache/:id/ip-investigation` | Apache IP investigation |
| `GET` | `/api/apache/:id/performance/heatmap` | Apache heatmap |
| `GET` | `/api/apache/:id/performance/scatter` | Apache scatter |
| `GET` | `/api/apache/:id/performance/dist` | Apache distribution |

**Host Dashboard (admin mutations):**

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/hosts/:machineId/dashboard/summary` | Host summary |
| `GET` | `/api/hosts/:machineId/dashboard/cpu` | CPU metrics |
| `GET` | `/api/hosts/:machineId/dashboard/memory` | Memory metrics |
| `GET` | `/api/hosts/:machineId/dashboard/disk` | Disk metrics |
| `GET` | `/api/hosts/:machineId/dashboard/network` | Network metrics |
| `GET` | `/api/hosts/:machineId/dashboard/processes` | Process list |
| `GET` | `/api/hosts/:machineId/logs` | Host logs |
| `GET` | `/api/hosts/:machineId/logs/health-summary` | Host log health |
| `GET` | `/api/containers/:id/dashboard` | Container dashboard |
| `GET` | `/api/containers/:id/logs` | Container logs |

### Ingest API (`/ingest/` — API key auth)

| Method | Path | Description |
|---|---|---|
| `POST` | `/ingest/hosts/register` | Agent host registration |
| `POST` | `/ingest/hosts/heartbeat` | Agent liveness heartbeat |
| `POST` | `/ingest/discovery/host-services` | Host service discovery |
| `POST` | `/ingest/discovery/container-services` | Container service discovery |
| `POST` | `/ingest/discovery/report` | Combined install report |
| `POST` | `/ingest/integrations/status` | Integration status update |
| `POST` | `/ingest/metrics` | Prometheus remote_write -> VictoriaMetrics |
| `POST` | `/ingest/logs/*` | OTLP logs -> Loki |
| `POST` | `/ingest/traces/*` | OTLP traces -> Tempo |

**Tenant isolation in ingest:**
- Metrics: URL path `/insert/{tenantID}/prometheus/api/v1/write`
- Logs: `X-Scope-OrgID` header set to tenant UUID
- Traces: `X-Scope-OrgID` header set to tenant UUID

### Admin Panel API (`/admin/api/`)

| Method | Path | Description |
|---|---|---|
| `POST` | `/admin/api/auth/signup` | Admin registration |
| `POST` | `/admin/api/auth/login` | Admin login |
| `POST` | `/admin/api/auth/refresh` | Admin token refresh |
| `POST` | `/admin/api/auth/logout` | Admin logout |
| `GET` | `/admin/api/auth/me` | Current admin info |
| `POST` | `/admin/api/auth/change-password` | Admin password change |
| `GET` | `/admin/api/dashboard/stats` | Platform-wide statistics |
| `GET` | `/admin/api/dashboard/tenants` | Tenant list with details |
| `GET` | `/admin/api/dashboard/tenants/:id` | Tenant detail |
| `POST` | `/admin/api/tenants/:id/ban` | Ban tenant |
| `POST` | `/admin/api/tenants/:id/unban` | Unban tenant |
| `POST` | `/admin/api/tenants/:id/change-plan` | Change tenant plan |
| `POST` | `/admin/api/tenants/:id/renew` | Renew subscription |
| `GET` | `/admin/api/packs` | List plans/packs |
| `GET` | `/admin/api/packs/:id` | Get pack detail |
| `POST` | `/admin/api/packs` | Create pack |
| `PATCH` | `/admin/api/packs/:id` | Update pack |
| `DELETE` | `/admin/api/packs/:id` | Delete pack |
| `GET` | `/admin/api/subscriptions/expired` | List expired subscriptions |

---

## Service Catalog

6 supported services with detailed integration guides, detection hints, and NVD product mappings:

| Service | Category | Metrics | Log Paths | NVD Product |
|---|---|---|---|---|
| **Nginx** | web | stub_status (built-in) | `/var/log/nginx/access.log`, `error.log` | nginx/nginx |
| **Apache** | web | mod_status (built-in) | `/var/log/apache2/access.log`, `error.log` | apache/http_server |
| **PostgreSQL** | database | postgres_exporter | `/var/log/postgresql/` | postgresql/postgresql |
| **MariaDB** | database | mysqld_exporter | `/var/log/mysql/` | mariadb/mariadb |
| **Redis** | cache | redis_exporter | syslog/journald | redis/redis |
| **PHP-FPM** | appserver | php-fpm_exporter | `/var/log/php-fpm/` | php/php |

Each service includes: detection hints, metrics guide, logs guide, canonical name aliases, and NVD product mapping for CVE scanning.

---

## Alert Engine

Background goroutine that evaluates alert rules against live telemetry data.

### Supported Alert Types (60+ templates)

**Host alerts:** CPU, memory, disk, swap, load average, disk I/O latency, conntrack saturation, network errors, SSH brute force, OOM kills, service crashes

**Nginx alerts:** Connection saturation, traffic drops, brute force detection, 5xx rate, 4xx rate, P99 latency, worker saturation

**Apache alerts:** Similar to Nginx (connections, traffic, brute force, error rates, latency, workers)

**PostgreSQL alerts:** Connection count, deadlocks, cache hit rate, replication lag, long transactions, temp files, auth failures

**MariaDB alerts:** Similar to PostgreSQL

**Redis alerts:** Memory usage, evictions, cache hit rate, blocked clients, rejected connections

**PHP-FPM alerts:** Worker saturation, queue depth, slow requests, max children reached, OPcache

**Container alerts:** Stopped containers, CPU usage, memory usage

**Host service alerts:** Service stopped

### How It Works

1. Engine polls enabled alert rules on a configurable interval
2. For each rule, builds PromQL or LogQL query from template + parameters
3. Queries VictoriaMetrics or Loki for current values
4. Compares against thresholds using configured operators (`>`, `<`, `==`, etc.)
5. If breached: creates/updates `AlertEvent` record
6. If resolved: marks event as resolved
7. Fires email notifications via NotificationDispatcher
8. Correlates related events into incidents with root cause analysis

### Deduplication

Prevents duplicate alerts within a cooldown period. Each event has a dedup key based on rule ID, host, service, and threshold.

---

## AI Reports

Generates AI-powered operations reports analyzing infrastructure health.

### Architecture

```
ReportService -> AIProvider (interface)
                    |
                    v
              fallbackAIProvider
              (tries primary, falls back to secondary)
                    |
                    v
              chunkedAIProvider
              (splits large payloads by UTC region)
                    |
                    v
              Gemini API  /  Groq API
```

### Report Types

| Type | Description |
|---|---|
| `today` | Daily summary of infrastructure health |
| `compare` | Day-over-day comparison with deltas |

### Flow

1. User requests report via `POST /api/reports/generate`
2. Backend creates report row with status `generating`, returns immediately
3. Background goroutine:
   a. Collects numeric telemetry (CPU, memory, disk, request rates, error rates, active alerts, incidents)
   b. Builds structured payload per UTC region
   c. If payload > `AI_CHUNK_BYTES`: splits by region, calls AI per chunk, then assembly call merges
   d. Stores JSONB result (numbers + AI narrative)
   e. Marks report as `ready`
4. User polls `GET /api/reports/:id` until status is `ready`

### Provider Fallback

If the primary provider (e.g., Gemini) fails, the fallback provider (e.g., Groq) is used automatically. Both API keys must be set for fallback to work.

### Token Management

Large telemetry payloads are split by UTC region to fit within provider token limits. The chunked provider sends each region separately, then an assembly call merges partial results into one cohesive report.

---

## CVE Vulnerability Scanning

Automated scanning of discovered service versions against the NIST NVD database.

### How It Works

1. Background goroutine runs on configurable interval (default 2 hours)
2. For each discovered service with a known version:
   a. Maps service name to NVD product (via `catalog.go` NVD mappings)
   b. Queries NVD API: `https://services.nvd.nist.gov/rest/json/cves/2.0`
   c. Matches CVEs against service version using CPE matching
   d. Filters by minimum severity (configurable: MEDIUM/HIGH/CRITICAL)
3. Results stored in `service_vulnerabilities` table
4. High-risk CVEs visible in vulnerability alerts

### NVD API

- Endpoint: `https://services.nvd.nist.gov/rest/json/cves/2.0`
- Timeout: 20 seconds per request
- Rate limit: ~5 requests/30s without API key, ~50 requests/30s with key
- Supports pagination via `startIndex` and `resultsPerPage`

### Scan Status

| Status | Meaning |
|---|---|
| `not_scanned` | Service has not been scanned yet |
| `scanned` | CVE scan completed |
| `unsupported` | Service type not supported for scanning |
| `failed` | Scan failed (NVD error, timeout, etc.) |
| `no_version` | Service version unknown, cannot scan |

---

## Background Workers

| Worker | Interval | Purpose |
|---|---|---|
| CVE Scanner | Configurable (default 2h) | Scans NVD for CVEs matching discovered service versions |
| Usage Metering | 5 min | Flushes Redis usage counters (log bytes, metric samples) to Postgres daily rows |
| Alert Engine | Configurable | Evaluates alert rules against VictoriaMetrics/Loki queries |
| Notification Dispatcher | Background | Sends alert notifications via email |
| Subscription Expiration | 1 hour | Marks expired subscriptions, expires 14-day trials |
| Retention Cleanup | 1 min | Enforces per-plan log/metric retention in Loki and VictoriaMetrics |

---

## Known Issues

### Integration Scripts Use Embedded Python

The `integration_service.go` (1529 lines) generates install scripts that contain embedded Python3 heredocs for YAML patching of the otelcol config file. This means:

- **Customers must have Python 3 installed** on their hosts to run the integration scripts
- The Python code handles: log receiver config, metrics receiver config, operator pipelines, and file path insertion
- **This is not ideal** — the scripts should be rewritten in pure bash (using `sed`, `yq`, or similar) to avoid requiring Python on customer hosts

**Location:** `internal/service/integration_service.go` — the `renderGenericScript()` method starting around line 165.

### Other Notes

- The `main` binary, `obs-backend` binary, and `server` binary exist in the repo but are gitignored — they are compiled artifacts
- The `.env` file should never be committed (it is in `.gitignore` but may have been committed historically)
- Tempo integration is defined but not yet active — traces are proxied but not queried
- Grafana is for development/testing only and should be removed or secured in production

---

## Production Notes

1. **Change all dev credentials** before deploying:
   - `JWT_SECRET` — generate a random 32+ char string
   - `REFRESH_TOKEN_PEPPER` — generate a random 32+ char string
   - `DB_PASSWORD` — use a strong password
   - `SMTP_PASS` — use production SMTP credentials

2. **Set real API keys:**
   - `GEMINI_API_KEY` and/or `GROQ_API_KEY` — required for AI reports
   - `NVD_API_KEY` — required for CVE scanning (optional but rate-limited without it)

3. **Configure CORS** — set `CORS_ALLOW_ORIGINS` to your production domain(s)

4. **Cookie security** — set `COOKIE_SECURE=true` and `COOKIE_DOMAIN=your-domain.com`

5. **Database** — use a managed PostgreSQL (AWS RDS, GCP Cloud SQL, etc.), not the Docker container

6. **Redis** — use a managed Redis for session storage

7. **VictoriaMetrics** — use persistent volumes for metrics storage

8. **TLS** — run behind a reverse proxy (nginx, traefik, Caddy) for HTTPS

9. **SMTP** — configure a transactional email provider (SendGrid, Mailgun, AWS SES)

10. **Monitor the backend** — expose `/health` endpoint for uptime monitoring

---

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `JWT_SECRET` validation error | Secret is less than 32 characters | Generate a longer secret |
| `REFRESH_TOKEN_PEPPER` validation error | Pepper is less than 32 characters | Generate a longer pepper |
| DB connection refused | PostgreSQL not running or wrong credentials | Check `docker ps`, verify `DB_HOST`, `DB_PASSWORD` |
| CORS errors | Frontend URL not in allowed origins | Add your domain to `CORS_ALLOW_ORIGINS` |
| Emails not sending | SMTP not configured or wrong credentials | Check `SMTP_HOST`, `SMTP_USER`, `SMTP_PASS` |
| AI reports fail | API keys not set | Set `GEMINI_API_KEY` or `GROQ_API_KEY` |
| CVE data missing | NVD API key not set or rate limited | Set `NVD_API_KEY` in `.env` |
| Rate limit 429 errors | Too many requests from same IP | Increase rate limit values or wait for window to expire |
| Agent can't connect | API key invalid or tenant suspended | Check API key, verify tenant subscription status |
| 403 on mutations | User role is `member` | Change user role to `admin` via team settings |
| Stale data | Background workers not running | Check backend logs for errors, restart backend |
| Memory issues | Large number of alert rules or high telemetry volume | Tune `AI_CHUNK_BYTES`, adjust alert evaluation interval |
