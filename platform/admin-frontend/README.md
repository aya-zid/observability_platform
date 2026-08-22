# Medianet Observability — Admin Frontend

This is the **internal administration panel** for the Medianet Observability platform. It is used by your team (platform administrators) — not by customers. From here you manage tenants, subscriptions, service packs, and monitor platform-wide usage and revenue.

---

## Table of Contents

- [What This Is](#what-this-is)
- [How It Differs from the Tenant Frontend](#how-it-differs-from-the-tenant-frontend)
- [Tech Stack](#tech-stack)
- [Project Structure](#project-structure)
- [Pages and Routes](#pages-and-routes)
- [Authentication Flow](#authentication-flow)
- [API Integration](#api-integration)
- [Configuration](#configuration)
- [Development Setup](#development-setup)
- [Docker Build](#docker-build)

---

## What This Is

When a company signs up for Medianet Observability, they get their own tenant account. This admin panel is where **your team** manages all tenants — view who's active, ban accounts, renew subscriptions, create service packs (pricing tiers), and monitor platform-wide metrics like total revenue and log ingestion.

Think of it as the "back office" for the platform operators.

---

## How It Differs from the Tenant Frontend

| Aspect | Admin Frontend | Tenant Frontend |
|---|---|---|
| **URL** | Port 5174 (`/admin`) | Port 5173 |
| **Users** | Platform administrators | Customer companies |
| **API prefix** | `/admin/api/*` | `/api/*` |
| **Data scope** | All tenants, platform-wide | Single tenant only |
| **Features** | Tenant management, packs, revenue stats | Host/service/container dashboards, alerts, logs |
| **Auth** | Admin role | User role with tenant isolation |

The two frontends are completely separate applications that connect to the same Go backend, but hit different API endpoints.

---

## Tech Stack

| Category | Technology |
|---|---|
| Framework | React 19 + TypeScript |
| Build Tool | Vite 7 |
| Routing | React Router DOM 7 |
| State Management | Zustand (auth only) |
| Server State | TanStack React Query |
| Styling | Tailwind CSS 4 + shadcn/ui |
| Forms | React Hook Form + Zod |
| HTTP Client | Axios (cookie-based auth) |
| Icons | Lucide React |

---

## Project Structure

```
src/
  main.tsx                    # Entry point
  App.tsx                     # Root: providers + router
  index.css                   # Global styles, CSS variables, dark/light theme

  api/
    client.ts                 # Axios instance (base: /admin/api), 401 refresh

  store/
    adminAuthStore.ts         # Auth state: status + role
    AdminAuthProvider.tsx     # Bootstrap: calls GET /auth/me on mount

  router/
    index.tsx                 # All routes
    guards.tsx                # ProtectedRoute + PublicRoute

  components/
    auth/
      AuthShell.tsx           # Split-screen login/signup layout
    layout/
      AdminLayout.tsx         # Sidebar nav + logout + main content
    ui/
      button.tsx              # shadcn Button
      input.tsx               # shadcn Input

  pages/
    auth/
      LoginPage.tsx           # Admin login
      SignupPage.tsx          # Admin account creation
    dashboard/
      DashboardPage.tsx       # Platform overview stats
    tenants/
      TenantsPage.tsx         # Tenant list with ban/unban
      TenantDetailPage.tsx    # Single tenant detail + renew
    packs/
      PacksPage.tsx           # Service pack CRUD
    settings/
      SettingsPage.tsx        # Admin password change
```

---

## Pages and Routes

### Public Pages

| Route | Page | What It Does |
|---|---|---|
| `/login` | LoginPage | Admin email/password login |
| `/signup` | SignupPage | Create the first admin account (full name, email, password) |

### Protected Pages

| Route | Page | What It Does |
|---|---|---|
| `/dashboard` | DashboardPage | **Platform overview** — total tenants, active tenants, banned tenants, total hosts, today's log ingestion (bytes), estimated revenue ($) |
| `/tenants` | TenantsPage | Paginated list of all tenants with search. Shows company, email, plan, hosts used, status. Actions: Ban/Unban |
| `/tenants/:id` | TenantDetailPage | Single tenant detail: contact info, plan, hosts, retention settings, AI reports toggle, subscription status. Actions: Ban, Unban, Renew Subscription |
| `/packs` | PacksPage | Full CRUD for service packs: name, price, max hosts, duration, log/metrics retention, max log bytes/day, AI reports toggle. Create, edit, delete packs |
| `/settings` | SettingsPage | Change admin password |

---

## Authentication Flow

1. **Bootstrap**: On mount, `AdminAuthProvider` calls `GET /auth/me`. If the session cookie is valid, the admin is authenticated. If not, they see the login page.

2. **Login**: Email + password → `POST /admin/api/auth/login` → session cookies set → redirect to `/dashboard`.

3. **Token refresh**: On 401 responses, the Axios interceptor calls `POST /admin/api/auth/refresh` to get new cookies, then retries the failed request.

4. **Logout**: Calls `POST /admin/api/auth/logout`, clears session, redirects to `/login`.

5. **No tokens in localStorage** — all authentication is cookie-based (HttpOnly cookies).

---

## API Integration

All API calls go through a shared Axios instance at `src/api/client.ts` with base URL `/admin/api`.

| Method | Endpoint | Used By | Description |
|---|---|---|---|
| `GET` | `/auth/me` | AdminAuthProvider | Check session validity |
| `POST` | `/auth/login` | LoginPage | Admin login |
| `POST` | `/auth/signup` | SignupPage | Create admin account |
| `POST` | `/auth/refresh` | Axios interceptor | Refresh session cookies |
| `POST` | `/auth/logout` | AdminLayout | End session |
| `POST` | `/auth/change-password` | SettingsPage | Change admin password |
| `GET` | `/dashboard/stats` | DashboardPage | Platform-wide statistics |
| `GET` | `/dashboard/tenants` | TenantsPage | Paginated tenant list |
| `GET` | `/dashboard/tenants/:id` | TenantDetailPage | Single tenant detail |
| `POST` | `/tenants/:id/ban` | TenantsPage, TenantDetailPage | Ban a tenant |
| `POST` | `/tenants/:id/unban` | TenantsPage, TenantDetailPage | Unban a tenant |
| `POST` | `/tenants/:id/renew` | TenantDetailPage | Renew subscription |
| `GET` | `/packs` | PacksPage | List all service packs |
| `POST` | `/packs` | PacksPage | Create a new pack |
| `PATCH` | `/packs/:id` | PacksPage | Update a pack |
| `DELETE` | `/packs/:id` | PacksPage | Delete a pack |

---

## Configuration

### Environment Variables

| Variable | Default | Description |
|---|---|---|
| `VITE_API_URL` | `http://localhost:8080` | Backend API base URL |

### Build Output

- Development: `npm run dev` → Vite dev server on port **5174**
- Production: `npm run build` → `dist/` folder served by nginx

### nginx.conf (Production)

- Serves the SPA with `try_files $uri $uri/ /index.html`
- Reverse-proxies `/api/` to `http://localhost:8080/admin/api/`

---

## Development Setup

```bash
# Install dependencies
npm install

# Start dev server
npm run dev

# Open in browser
open http://localhost:5174
```

### Available Scripts

| Command | Description |
|---|---|
| `npm run dev` | Start Vite dev server (port 5174) |
| `npm run build` | TypeScript compile + Vite build |
| `npm run preview` | Preview production build |
| `npm run lint` | Run ESLint |

---

## Docker Build

```bash
# Development
docker build --target dev -t obs-admin-dev .
docker run -p 5174:5174 obs-admin-dev

# Production (nginx)
docker build --target prod -t obs-admin .
docker run -p 80:80 obs-admin
```

In the platform's `docker-compose.yml`, the admin frontend runs in dev mode on port 5174.
