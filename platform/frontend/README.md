# Medianet Observability — Tenant Frontend

This is the main customer-facing web application of the Medianet Observability platform. It is what your customers see after they sign up — their dashboard for monitoring hosts, services, containers, alerts, logs, and metrics.

---

## Table of Contents

- [What This Is](#what-this-is)
- [Tech Stack](#tech-stack)
- [Project Structure](#project-structure)
- [Pages and Routes](#pages-and-routes)
- [Authentication Flow](#authentication-flow)
- [How Data Flows](#how-data-flows)
- [Internationalization (i18n)](#internationalization-i18n)
- [Configuration](#configuration)
- [Development Setup](#development-setup)
- [Docker Build](#docker-build)

---

## What This Is

When a company signs up for Medianet Observability, they install an agent on their servers. That agent sends metrics, logs, and system data to the backend. **This frontend is where they view all of that data** — through dashboards, log explorers, alert configurations, and AI-generated reports.

Think of it as the "customer portal" — similar to what you'd see in Datadog, Grafana Cloud, or New Relic.

---

## Tech Stack

| Category | Technology |
|---|---|
| Framework | React 19 + TypeScript |
| Build Tool | Vite 7 |
| Routing | React Router DOM 7 |
| State Management | Zustand (auth, theme) |
| Server State | TanStack React Query (caching, auto-refetch) |
| Styling | Tailwind CSS 4 + shadcn/ui components |
| Charts | Recharts (line, area, bar, pie) |
| Forms | React Hook Form + Zod validation |
| HTTP Client | Axios (cookie-based auth) |
| Icons | Lucide React |
| Markdown | React Markdown (for AI reports) |

---

## Project Structure

```
src/
  main.tsx                    # Entry point
  App.tsx                     # Root: providers + router
  index.css                   # Global styles, CSS variables, dark/light theme

  api/                        # API layer — one file per domain
    client.ts                 # Axios instance, 401 refresh interceptor
    auth.ts                   # Login, signup, logout, password reset
    overview.ts               # Fleet overview, incident dismiss
    hosts.ts                  # Host metrics, CPU, memory, disk, network, logs
    alerts.ts                 # Alert rules, templates, events, CVE alerts
    container.ts              # Container discovery, dashboard, logs
    reports.ts                # AI report generation and listing
    team.ts                   # Team members, invitations, roles
    tenant.ts                 # API keys, tenant info
    billing.ts                # Plans, usage, billing
    discovery.ts              # Service discovery, host inventory
    nginx.ts                  # Nginx dashboard, logs, slow requests, domains
    apache.ts                 # Apache dashboard, logs, slow requests, domains
    postgres.ts               # Postgres dashboard, config, logs
    phpfpm.ts                 # PHP-FPM dashboard, logs
    redis.ts                  # Redis dashboard, logs
    mariadb.ts                # MariaDB dashboard, logs, config, slow queries

  store/                      # Zustand stores
    authStore.ts              # Auth status, API key, role
    themeStore.ts             # Dark/light theme (persisted to localStorage)

  router/
    index.tsx                 # All routes defined
    guards.tsx                # ProtectedRoute + PublicRoute

  components/
    auth/
      AuthBootstrap.tsx       # Restores session from cookie on page load
      AuthShell.tsx           # Split-screen auth layout
    layout/
      AppLayout.tsx           # Sidebar + TopBar + main content
      Sidebar.tsx             # Navigation sidebar with alert badge
      TopBar.tsx              # Header with logout
    shared/
      DashboardComponents.tsx # StatCard, LinePanel, AreaPanel, BarPanel, DonutCard
      charts.tsx              # Recharts wrappers
      DataTable.tsx           # Generic data table
      TimeRangeSelector.tsx   # Time range picker (15m/1h/6h/24h + custom)
      ThemeToggle.tsx         # Dark/light toggle
      LogPagination.tsx       # Log pagination controls

  pages/
    welcome/                  # Marketing landing page
    auth/                     # Login, signup, password reset, invite acceptance
    overview/                 # Fleet health dashboard
    hosts/                    # Host inventory + per-host metrics/logs
    services/                 # Service dashboards (Nginx, Apache, Postgres, etc.)
    containers/               # Container inventory + per-container metrics/logs
    alerts/                   # Alert rules and events
    reports/                  # AI-generated reports
    settings/                 # Org settings, team, API keys, billing

  i18n/
    translations.ts           # English/French translations
    LanguageContext.tsx        # Language provider
```

---

## Pages and Routes

### Public Pages

| Route | Page | What It Does |
|---|---|---|
| `/` | WelcomePage | Marketing landing page with features, pricing, and signup CTA |
| `/login` | LoginPage | Email/password login form |
| `/signup` | SignupPage | New workspace registration (company name, email, password) |
| `/forgot-password` | ForgotPasswordPage | Request a password reset email |
| `/reset-password/:token` | ResetPasswordPage | Set new password via email link |
| `/invite/:token` | AcceptInvitePage | Accept team invitation, set name and password |
| `/inactive` | InactivePage | Account deactivated notice |
| `/subscription-expired` | SubscriptionExpiredPage | Subscription expired with plan selection |
| `/account-suspended` | AccountSuspendedPage | Account suspended notice |

### Protected Pages (require login)

| Route | Page | What It Does |
|---|---|---|
| `/overview` | OverviewPage | **Main dashboard** — fleet health, incident correlation, host snapshots, activity timeline |
| `/hosts` | HostsPage | List of all monitored hosts with status, OS, services |
| `/hosts/:id` | HostDetailPage | Per-host metrics: CPU, memory, disk, network, load, processes |
| `/hosts/:id/logs` | HostLogsPage | Journald log viewer with service/priority filters |
| `/containers` | ContainersPage | Docker container inventory with inline detail drawer |
| `/containers/:id/dashboard` | ContainerDashboardPage | Per-container CPU, memory, network metrics |
| `/containers/:id/logs` | ContainerLogsPage | Per-container log viewer |
| `/services` | ServicesPage | Service inventory grouped by host |
| `/services/:id` | ServiceConfigPage | Service configuration and integration guide |
| `/services/nginx-dashboard/:id` | NginxDashboardPage | Nginx: request rate, bandwidth, status codes, response times |
| `/services/nginx-logs/:id` | NginxLogsPage | Nginx access/error log explorer with filters |
| `/services/nginx-dashboard/:id/slow-requests` | NginxSlowRequestsPage | Slow request analysis |
| `/services/apache-dashboard/:id` | ApacheDashboardPage | Apache: request rate, bandwidth, status codes |
| `/services/apache-logs/:id` | ApacheLogsPage | Apache access/error log explorer |
| `/services/postgres-dashboard/:id` | PostgresDashboardPage | PostgreSQL: connections, queries, cache hit ratio, tuples |
| `/services/postgres-config/:id` | PostgresConfigPage | PostgreSQL configuration viewer |
| `/services/postgres-logs/:id` | PostgresLogsPage | PostgreSQL log event viewer |
| `/services/phpfpm-dashboard/:id` | PHPFPMDashboardPage | PHP-FPM: active processes, request duration, pool stats |
| `/services/phpfpm-logs/:id` | PHPFPMLogsPage | PHP-FPM log viewer |
| `/services/redis-dashboard/:id` | RedisDashboardPage | Redis: hit rate, memory, connected clients, commands/sec |
| `/services/redis-logs/:id` | RedisLogsPage | Redis log viewer |
| `/services/mariadb-dashboard/:id` | MariaDBDashboardPage | MariaDB: connections, queries, slow queries, InnoDB stats |
| `/services/mariadb-logs/:id` | MariaDBLogsPage | MariaDB log and error log viewer |
| `/alerts` | AlertsPage | Alert rules, firing events, vulnerability alerts |
| `/reports` | ReportsListPage | AI report listing and generation |
| `/reports/:id` | ReportViewPage | Full AI report with markdown analysis |
| `/settings/general` | OrgSettingsPage | Organization name and timezone |
| `/settings/team` | TeamPage | Team members, invitations, roles (admin only) |
| `/settings/api-keys` | ApiKeysPage | API key management and agent install command (admin only) |
| `/settings/notifications` | NotificationsPage | Email notification channels (admin only) |
| `/settings/billing` | BillingPage | Current plan, usage, billing history (admin only) |
| `/profile` | ProfilePage | User profile and password change |

---

## Authentication Flow

1. **Session restore**: On page load, `AuthBootstrap` calls `POST /refresh` using the HttpOnly refresh token cookie. If valid, the user is authenticated. If not, they stay unauthenticated.

2. **Login**: User submits email + password → `POST /login` → session cookies are set → redirect to `/overview`.

3. **Token refresh**: When an API call returns 401, the Axios interceptor automatically calls `POST /refresh` to get new cookies, then retries the original request. Concurrent requests are queued during refresh.

4. **Special statuses**: The backend can return 403 with specific codes:
   - `USER_DEACTIVATED` → shows InactivePage
   - `SUBSCRIPTION_EXPIRED` → shows SubscriptionExpiredPage
   - `ACCOUNT_SUSPENDED` → shows AccountSuspendedPage

5. **Logout**: Calls `POST /logout`, clears the session, redirects to `/login`.

6. **No tokens in localStorage** — authentication is entirely cookie-based (HttpOnly cookies for refresh, session for access).

---

## How Data Flows

```
Customer's Server
  |
  | (agent collects metrics + logs)
  v
Medianet Backend (Go API on :8080)
  |
  +---> VictoriaMetrics (metrics storage)
  +---> Loki (log storage)
  +---> Tempo (traces - not yet implemented)
  |
  v
This Frontend (React on :5173)
  |
  | API calls to backend (/api/overview, /api/hosts, etc.)
  | Backend proxies queries to VictoriaMetrics/Loki
  v
Dashboard displays (charts, tables, logs, alerts)
```

The frontend never talks directly to VictoriaMetrics or Loki. All queries go through the Go backend, which handles tenant isolation (injecting the correct tenant ID into every query).

---

## Internationalization (i18n)

The app supports English and French. Translation strings are in `src/i18n/translations.ts` and cover:

- Service dashboard labels (Postgres, MariaDB, Redis, PHP-FPM)
- Log viewer UI text
- Alert and report labels

Users can toggle language from the sidebar. The preference is stored in `localStorage`.

---

## Configuration

### Environment Variables

| Variable | Default | Description |
|---|---|---|
| `VITE_API_URL` | `http://localhost:8080` | Backend API base URL |

### Build Output

- Development: `npm run dev` → Vite dev server on port **5173**
- Production: `npm run build` → `dist/` folder served by nginx

### nginx.conf (Production)

- Serves the SPA with `try_files $uri $uri/ /index.html` for client-side routing
- Reverse-proxies `/api/` to `http://localhost:8080/api/`

---

## Development Setup

```bash
# Install dependencies
npm install

# Start dev server
npm run dev

# Open in browser
open http://localhost:5173
```

The dev server proxies API calls to the backend at `localhost:8080`.

### Available Scripts

| Command | Description |
|---|---|
| `npm run dev` | Start Vite dev server (port 5173) |
| `npm run build` | TypeScript compile + Vite build |
| `npm run preview` | Preview production build |
| `npm run lint` | Run ESLint |

---

## Docker Build

The Dockerfile supports multi-stage builds:

```bash
# Development (with hot reload)
docker build --target dev -t obs-frontend-dev .
docker run -p 5173:5173 obs-frontend-dev

# Production (nginx serving built assets)
docker build --target prod -t obs-frontend .
docker run -p 80:80 obs-frontend
```

In the platform's `docker-compose.yml`, the frontend runs in dev mode on port 5173.
