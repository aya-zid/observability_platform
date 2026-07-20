# SaaS Platform Features — Implementation Plan

Full plan covering RBAC, Team Management, Settings, Profile, Subscription/Billing, and Template-Based Alerts.

> [!IMPORTANT]
> **Build order matters.** Each phase depends on the previous one. Do not skip ahead.

---

## Current Codebase Snapshot

Before planning changes, here's what exists today and what needs to change:

| Layer | What exists | Key files |
|---|---|---|
| **Domain models** | `User` (has `Role` field, defaults `admin`), `Tenant` (has `APIKey` as plaintext column, `SubscriptionStatus` enum) | [user.go](file:///home/aya/observability-platform/platform/backend/internal/domain/user.go), [tenant.go](file:///home/aya/observability-platform/platform/backend/internal/domain/tenant.go) |
| **Auth** | JWT access token (cookie) + refresh token (Redis), signup creates tenant+admin user, login with lockout | [auth_service.go](file:///home/aya/observability-platform/platform/backend/internal/service/auth_service.go), [jwt.go](file:///home/aya/observability-platform/platform/backend/internal/infrastructure/jwt.go) |
| **Middleware** | `AuthMiddleware` (JWT from cookie/bearer → sets `claims`), `IngestMiddleware` (API key basic auth → sets `ingest_tenant`), `RateLimitByIP` | [auth_middleware.go](file:///home/aya/observability-platform/platform/backend/internal/middleware/auth_middleware.go), [ingest_middleware.go](file:///home/aya/observability-platform/platform/backend/internal/middleware/ingest_middleware.go) |
| **API key** | Single `api_key` column on `tenants` table (plaintext), rotate = overwrite column + invalidate Redis cache | [tenant_handler.go](file:///home/aya/observability-platform/platform/backend/internal/handler/tenant_handler.go), [api_key.go](file:///home/aya/observability-platform/platform/backend/internal/infrastructure/api_key.go) |
| **Alerts** | Only vulnerability alerts (CVE scanning from NVD). No metric/log-based alert rules yet | [alerts_handler.go](file:///home/aya/observability-platform/platform/backend/internal/handler/alerts_handler.go), [alerts_repository.go](file:///home/aya/observability-platform/platform/backend/internal/repository/alerts_repository.go) |
| **Frontend** | Vite + React + TailwindCSS v4 + shadcn + Zustand + React Query. Pages: overview, alerts (vuln only), hosts, services, profile (read-only), settings/api-keys (simple rotate) | [router/index.tsx](file:///home/aya/observability-platform/platform/frontend/src/router/index.tsx), [Sidebar.tsx](file:///home/aya/observability-platform/platform/frontend/src/components/layout/Sidebar.tsx) |
| **Infra** | Postgres 16 + Redis 7 + Loki + VictoriaMetrics (cluster: vminsert/vmselect) + Tempo | [docker-compose.yml](file:///home/aya/observability-platform/platform/docker-compose.yml) |

---

## Phase 1: RBAC Middleware (Foundation)

**Why first:** Every feature below (team invite, settings, billing) needs role-based access checks. This is a pure backend change — no DB migration needed since `User.Role` already exists with values `admin`/`member`.

### Backend Changes

#### [MODIFY] [auth_middleware.go](file:///home/aya/observability-platform/platform/backend/internal/middleware/auth_middleware.go)
- Add a new middleware function `RequireRole(allowedRoles ...string) echo.MiddlewareFunc`
- This middleware extracts `claims` from context (already set by `AuthMiddleware`), reads `claims.Role`, and if the role is NOT in `allowedRoles`, returns `403 Forbidden` using `apierror.ErrForbidden`
- It must be applied **after** `AuthMiddleware` in the middleware chain (it depends on `claims` being present in context)

#### [MODIFY] [errors.go](file:///home/aya/observability-platform/platform/backend/internal/apierror/errors.go)
- `ErrForbidden` already exists. No change needed.

#### [MODIFY] [routes.go](file:///home/aya/observability-platform/platform/backend/internal/handler/routes.go)
- For now, no routes are gated yet — that happens in subsequent phases when the admin-only endpoints are added. But the middleware must be available.

### No Frontend Changes

### No DB Migration

---

## Phase 2: Team / Members Management

### DB Schema

#### [NEW] `domain/invitation.go`
Create a new domain model file at `internal/domain/invitation.go`:

```go
type Invitation struct {
    ID         uint      `gorm:"primaryKey"`
    TenantID   uint      `gorm:"not null;index"`
    Tenant     Tenant    `gorm:"foreignKey:TenantID"`
    Email      string    `gorm:"type:varchar(255);not null"`
    Role       string    `gorm:"type:varchar(50);not null;default:'member'"`
    Token      string    `gorm:"type:varchar(255);uniqueIndex;not null"`
    InvitedBy  uint      `gorm:"not null"`
    Status     string    `gorm:"type:varchar(20);not null;default:'pending'"` // pending | accepted | expired | revoked
    ExpiresAt  time.Time `gorm:"not null"`
    CreatedAt  time.Time `gorm:"autoCreateTime"`
}
```

#### [MODIFY] [main.go](file:///home/aya/observability-platform/platform/backend/cmd/server/main.go)
- Add `&domain.Invitation{}` to the `AutoMigrate` call (line 28-41)

### Backend Files

#### [NEW] `repository/invitation_repository.go`
Methods needed:
- `Create(invitation *domain.Invitation) error`
- `FindByTokenCtx(ctx, token string) (*domain.Invitation, error)`
- `ListByTenantCtx(ctx, tenantID uint) ([]domain.Invitation, error)` — only pending ones
- `UpdateStatusCtx(ctx, id uint, status string) error`
- `FindPendingByEmailAndTenantCtx(ctx, email string, tenantID uint) (*domain.Invitation, error)` — for dedup check

#### [MODIFY] `repository/user_repository.go`
Add methods:
- `ListByTenantCtx(ctx, tenantID uint) ([]domain.User, error)` — all users of a tenant
- `DeleteByIDCtx(ctx, id uint) error` — hard delete a user row
- `UpdateRoleCtx(ctx, id uint, role string) error`
- `UpdatePasswordCtx(ctx, id uint, passwordHash string) error` — needed in Phase 4 (Profile) too
- `CountByTenantAndRoleCtx(ctx, tenantID uint, role string) (int64, error)` — to enforce "at least one admin"

#### [NEW] `service/team_service.go`
Business logic:
- `InviteTeammate(ctx, tenantID uint, inviterUserID uint, email string, role string) (*domain.Invitation, error)`
  - Validate role is `admin` or `member`
  - Check email not already a user in this tenant (`userRepo.FindByEmailCtx`)
  - Check no pending invite for same email+tenant (`invitationRepo.FindPendingByEmailAndTenantCtx`)
  - Generate a secure random token (use `crypto/rand`, 32 bytes → base64url), set `ExpiresAt = now + 72h`
  - Create invitation row
  - Return the invitation (handler will construct the invite URL)
  - **No email sending for PFE scope** — just return the link. Mention in report: "Email delivery (SMTP/SendGrid integration) is out of scope; the invite link is returned directly to the admin for manual sharing."
- `AcceptInvitation(ctx, token string, fullName string, password string) error`
  - Find invitation by token, check status=pending, check not expired
  - Create user row with invitation's `tenant_id` and `role`, bcrypt the password
  - Update invitation status to `accepted`
  - All in a DB transaction
- `ListTeam(ctx, tenantID uint) (members []dto.TeamMember, pendingInvites []dto.PendingInvite, error)`
  - Combine users + pending invitations into response DTOs
- `RemoveMember(ctx, tenantID uint, targetUserID uint, callerUserID uint) error`
  - Cannot remove yourself
  - If target is last admin, reject with error
  - Hard-delete user, invalidate all their refresh tokens (delete Redis keys with pattern `refresh:*` for that user — or simpler: since refresh sessions store `user_id`, the next refresh attempt will fail when the user doesn't exist)
- `ChangeRole(ctx, tenantID uint, targetUserID uint, newRole string) error`
  - If demoting from admin → member, check at least one admin remains
  - Update user role
- `ResendInvitation(ctx, invitationID uint, tenantID uint) (*domain.Invitation, error)`
  - Find invitation, verify it belongs to tenant, status=pending
  - Generate new token, reset ExpiresAt
  - Return updated invitation
- `RevokeInvitation(ctx, invitationID uint, tenantID uint) error`
  - Set status=revoked

#### [NEW] `dto/team.go`
```go
type InviteRequest struct {
    Email string `json:"email" validate:"required,email"`
    Role  string `json:"role" validate:"required,oneof=admin member"`
}

type AcceptInviteRequest struct {
    FullName string `json:"full_name" validate:"required,min=2,max=100"`
    Password string `json:"password" validate:"required,min=8"`
}

type ChangeRoleRequest struct {
    Role string `json:"role" validate:"required,oneof=admin member"`
}

type TeamMember struct {
    ID        uint   `json:"id"`
    FullName  string `json:"full_name"`
    Email     string `json:"email"`
    Role      string `json:"role"`
    CreatedAt string `json:"created_at"`
}

type PendingInvite struct {
    ID        uint   `json:"id"`
    Email     string `json:"email"`
    Role      string `json:"role"`
    InviteURL string `json:"invite_url"`
    ExpiresAt string `json:"expires_at"`
    CreatedAt string `json:"created_at"`
}

type TeamResponse struct {
    Members []TeamMember    `json:"members"`
    Pending []PendingInvite `json:"pending"`
}
```

#### [NEW] `handler/team_handler.go`
Handlers:
- `ListTeam` → `GET /api/settings/team` (admin only)
- `InviteTeammate` → `POST /api/settings/team/invite` (admin only)
- `AcceptInvitation` → `POST /invite/:token/accept` (**public route**, not under `/api`)
- `ValidateInvitation` → `GET /invite/:token` (**public route** — frontend uses this to show the accept form)
- `RemoveMember` → `DELETE /api/settings/team/:userId` (admin only)
- `ChangeRole` → `PATCH /api/settings/team/:userId/role` (admin only)
- `ResendInvitation` → `POST /api/settings/team/invitations/:id/resend` (admin only)
- `RevokeInvitation` → `DELETE /api/settings/team/invitations/:id` (admin only)

#### [MODIFY] [routes.go](file:///home/aya/observability-platform/platform/backend/internal/handler/routes.go)
Add the new routes:
- Public (outside `api` group): `GET /invite/:token` and `POST /invite/:token/accept`
- Under `api` group, add a settings subgroup with `RequireRole("admin")`:
  ```
  settings := api.Group("/settings", custommiddleware.RequireRole("admin"))
  settings.GET("/team", teamHandler.ListTeam)
  settings.POST("/team/invite", teamHandler.InviteTeammate)
  settings.DELETE("/team/:userId", teamHandler.RemoveMember)
  settings.PATCH("/team/:userId/role", teamHandler.ChangeRole)
  settings.POST("/team/invitations/:id/resend", teamHandler.ResendInvitation)
  settings.DELETE("/team/invitations/:id", teamHandler.RevokeInvitation)
  ```

### Frontend Changes

#### [NEW] `api/team.ts`
API functions: `getTeam()`, `inviteTeammate(email, role)`, `removeMember(userId)`, `changeRole(userId, role)`, `resendInvitation(id)`, `revokeInvitation(id)`, `validateInvitation(token)`, `acceptInvitation(token, fullName, password)`

#### [NEW] `pages/settings/TeamPage.tsx`
- Table showing members: Name | Email | Role | Joined | Actions (change role dropdown, remove button)
- Section showing pending invitations: Email | Role | Expires | Actions (resend, revoke)
- "Invite teammate" button → modal with email input + role dropdown
- Guard: admin-only (hide this nav item for non-admin users, and the backend rejects anyway)

#### [NEW] `pages/auth/AcceptInvitePage.tsx`
- Public page at `/invite/:token`
- On mount, calls `validateInvitation(token)` to check token validity
- If valid, shows a form: Full Name + Password + Confirm Password
- On submit, calls `acceptInvitation(token, fullName, password)`
- On success, redirect to `/login`

#### [MODIFY] [router/index.tsx](file:///home/aya/observability-platform/platform/frontend/src/router/index.tsx)
- Add `/invite/:token` as a public route
- Add `/settings/team` as a protected route
- Move existing `/settings/api-keys` to stay under settings group

#### [MODIFY] [Sidebar.tsx](file:///home/aya/observability-platform/platform/frontend/src/components/layout/Sidebar.tsx)
- Replace flat "API Keys" link with a collapsible "Settings" section containing: Team, API Keys, Notification Channels, Billing (added in later phases)
- The "Settings" section should only show admin-only items (Team, Billing) to admin users. Use the `role` from the `/api/me` response (already available from `useQuery(["me"])`).

#### [MODIFY] [authStore.ts](file:///home/aya/observability-platform/platform/frontend/src/store/authStore.ts)
- Add `role: string | null` and `setRole(role)` to the store so components can check admin status without re-fetching `/api/me`

---

## Phase 3: Settings Page (API Key Rotation Upgrade + Org Settings + Notification Channels)

### 3A: API Key Rotation with Grace Period

#### DB Schema — [NEW] `domain/api_key.go`
```go
type APIKeyRecord struct {
    ID         uint       `gorm:"primaryKey"`
    TenantID   uint       `gorm:"not null;index"`
    KeyHash    string     `gorm:"type:varchar(255);not null"`
    KeyPrefix  string     `gorm:"type:varchar(20);not null"` // e.g. "obs_live_ab12"
    Status     string     `gorm:"type:varchar(20);not null;default:'active'"` // active | revoked
    CreatedAt  time.Time  `gorm:"autoCreateTime"`
    RevokedAt  *time.Time
    LastUsedAt *time.Time
}
```

#### [MODIFY] [main.go](file:///home/aya/observability-platform/platform/backend/cmd/server/main.go)
- Add `&domain.APIKeyRecord{}` to `AutoMigrate`

#### Migration Strategy
This replaces the `api_key` column on `tenants` table. To avoid breaking existing agents during migration:
1. On first startup after deploy, a one-time migration function reads each tenant's plaintext `api_key`, hashes it (SHA-256), and inserts an `APIKeyRecord` row with `status=active`
2. Then remove the `api_key` column from the `Tenant` domain model (but don't drop the DB column immediately — GORM will just ignore it)
3. Update `IngestMiddleware` to query `api_key_records` instead of `tenants.api_key`

> [!WARNING]
> This is a breaking migration. Write a Go migration function in `main.go` (run once via a flag or auto-detect if `api_key_records` table is empty). Alternatively, keep the existing flow and add the new `api_key_records` table alongside it without removing the old column — simpler for PFE scope.

**Recommended for PFE scope:** Keep the current simple approach (single `api_key` column on `tenants`). The current rotation already works and is simpler to explain. Instead, **add `last_used_at` tracking** to the existing column, and mention the hashed multi-key approach as a "production improvement" in your report. This saves significant time.

If keeping simple approach, the only changes are:

#### [MODIFY] [tenant.go](file:///home/aya/observability-platform/platform/backend/internal/domain/tenant.go)
- Add field: `APIKeyLastUsedAt *time.Time gorm:"column:api_key_last_used_at"`

#### [MODIFY] [ingest_middleware.go](file:///home/aya/observability-platform/platform/backend/internal/middleware/ingest_middleware.go)
- After successful auth, update `api_key_last_used_at` on the tenant (do this async/best-effort with a goroutine or debounce in Redis to avoid a DB write on every single ingest request — e.g., only update if last update was >1 min ago)

#### [MODIFY] `pages/settings/ApiKeysPage.tsx`
- Show `last_used_at` ("Last used: 2 hours ago")
- Add a confirmation modal before rotation: "This will immediately invalidate the old key. All agents using the current key will stop sending data until reconfigured."
- Show the key masked: `obs_live_ab12••••••••` with a copy button
- After rotation, show the new key in a "Copy now — you won't see this again" modal

### 3B: Organization Settings

#### [MODIFY] [tenant.go](file:///home/aya/observability-platform/platform/backend/internal/domain/tenant.go)
- Add field: `Timezone string gorm:"type:varchar(50);default:'UTC'"`

#### [NEW] `dto/settings.go`
```go
type UpdateOrgRequest struct {
    CompanyName string `json:"company_name" validate:"omitempty,min=2,max=100"`
    Timezone    string `json:"timezone" validate:"omitempty,max=50"`
}

type OrgSettingsResponse struct {
    CompanyName  string `json:"company_name"`
    ContactEmail string `json:"contact_email"`
    Timezone     string `json:"timezone"`
    CreatedAt    string `json:"created_at"`
}
```

#### [NEW] `handler/settings_handler.go`
- `GetOrgSettings` → `GET /api/settings/org` (admin only)
- `UpdateOrgSettings` → `PATCH /api/settings/org` (admin only)

#### [MODIFY] `repository/tenant_repository.go`
- Add `UpdateOrgSettingsCtx(ctx, tenantID uint, updates map[string]interface{}) error`

#### [MODIFY] [routes.go](file:///home/aya/observability-platform/platform/backend/internal/handler/routes.go)
- Add to the `settings` group: `settings.GET("/org", settingsHandler.GetOrgSettings)` and `settings.PATCH("/org", settingsHandler.UpdateOrgSettings)`

#### [NEW] `pages/settings/OrgSettingsPage.tsx`
- Form with: Company Name (editable), Contact Email (read-only), Timezone dropdown
- "Save" button
- "Danger zone" section at bottom: "Delete organization" button (can be a stub/disabled for now — mention as future work)

### 3C: Notification Channels

#### [NEW] `domain/notification_channel.go`
```go
type NotificationChannel struct {
    ID         uint      `gorm:"primaryKey"`
    TenantID   uint      `gorm:"not null;index"`
    Type       string    `gorm:"type:varchar(20);not null"` // "email" for now
    Target     string    `gorm:"type:varchar(255);not null"` // email address
    IsVerified bool      `gorm:"type:boolean;default:false"`
    CreatedAt  time.Time `gorm:"autoCreateTime"`
}
```

#### [MODIFY] [main.go](file:///home/aya/observability-platform/platform/backend/cmd/server/main.go)
- Add `&domain.NotificationChannel{}` to `AutoMigrate`

#### [NEW] `repository/notification_channel_repository.go`
- `Create`, `ListByTenantCtx`, `DeleteByIDAndTenantCtx`, `MarkVerifiedCtx`

#### [NEW] `handler/notification_handler.go`
- `ListChannels` → `GET /api/settings/notifications` (admin only)
- `AddChannel` → `POST /api/settings/notifications` (admin only) — body: `{type: "email", target: "user@example.com"}`
- `DeleteChannel` → `DELETE /api/settings/notifications/:id` (admin only)
- Skip actual email verification for PFE scope — auto-mark as verified on creation. Mention in report: "Email verification via SMTP is out of scope; channels are auto-verified in the current implementation."

#### [MODIFY] [routes.go](file:///home/aya/observability-platform/platform/backend/internal/handler/routes.go)
- Add to settings group: `/notifications` endpoints

#### [NEW] `pages/settings/NotificationsPage.tsx`
- List of notification channels (email addresses) with delete button
- "Add channel" button → modal with type dropdown (email only for now) and target input

### Frontend: Settings Layout

#### [NEW] `pages/settings/SettingsLayout.tsx`
- A layout component with a horizontal tab bar or vertical sub-nav: General | Team | API Keys | Notifications | Billing
- Each tab renders its respective page component
- Use `react-router-dom` nested routes or a simple tab-state approach

#### [MODIFY] [router/index.tsx](file:///home/aya/observability-platform/platform/frontend/src/router/index.tsx)
Restructure settings routes:
```
/settings → redirect to /settings/general
/settings/general → OrgSettingsPage
/settings/team → TeamPage (from Phase 2)
/settings/api-keys → ApiKeysPage (existing, enhanced)
/settings/notifications → NotificationsPage
/settings/billing → BillingPage (Phase 5)
```

---

## Phase 4: Profile Page (Enhanced)

### Backend Changes

#### [NEW] `service/profile_service.go`
- `UpdateProfile(ctx, userID uint, fullName string) error` — update user's name
- `ChangePassword(ctx, userID uint, currentPassword string, newPassword string) error`
  - Fetch user, verify `currentPassword` with bcrypt, hash `newPassword`, update
  - **Invalidate all refresh tokens** for this user: scan Redis for `refresh:*` keys, deserialize, delete those where `user_id` matches. Or simpler: add a `password_changed_at` field to `User` and check it in `ParseToken`/refresh flow.

#### [MODIFY] [user.go](file:///home/aya/observability-platform/platform/backend/internal/domain/user.go)
- Add field: `PasswordChangedAt *time.Time gorm:"column:password_changed_at"`

#### [NEW] `dto/profile.go`
```go
type UpdateProfileRequest struct {
    FullName string `json:"full_name" validate:"required,min=2,max=100"`
}

type ChangePasswordRequest struct {
    CurrentPassword string `json:"current_password" validate:"required"`
    NewPassword     string `json:"new_password" validate:"required,min=8"`
}
```

#### [NEW] `handler/profile_handler.go`
- `GetProfile` → `GET /api/profile` (any authenticated user) — returns same as `/api/me` but can be extended
- `UpdateProfile` → `PATCH /api/profile` (any authenticated user)
- `ChangePassword` → `POST /api/profile/change-password` (any authenticated user)

#### [MODIFY] [routes.go](file:///home/aya/observability-platform/platform/backend/internal/handler/routes.go)
- Add under `api` group (no admin restriction): `api.GET("/profile", ...)`, `api.PATCH("/profile", ...)`, `api.POST("/profile/change-password", ...)`

### Frontend Changes

#### [MODIFY] `pages/profile/ProfilePage.tsx`
Completely rewrite to include:
- **Personal Info section**: Full name (editable) + Email (read-only) + Role (read-only) + Company (read-only) + Save button
- **Change Password section**: Current password + New password + Confirm new password + Change button
- **Active Sessions section** (optional stretch — mention as future work if skipped): list of sessions with "Revoke" button

---

## Phase 5: Subscription / Billing

### DB Schema

#### [NEW] `domain/plan.go`
```go
type Plan struct {
    ID                   uint    `gorm:"primaryKey"`
    Name                 string  `gorm:"type:varchar(50);not null;uniqueIndex"`
    MaxHosts             int     `gorm:"not null"`
    MaxServicesPerHost   int     `gorm:"not null;default:20"`
    LogRetentionDays     int     `gorm:"not null"`
    MetricsRetentionDays int     `gorm:"not null"`
    MaxLogBytesPerDay    int64   `gorm:"not null"` // bytes, 0 = unlimited
    PricePerMonth        float64 `gorm:"type:numeric(10,2)"`
    IsActive             bool    `gorm:"type:boolean;default:true"`
}

type TenantSubscription struct {
    ID               uint       `gorm:"primaryKey"`
    TenantID         uint       `gorm:"not null;uniqueIndex"` // one active sub per tenant
    Tenant           Tenant     `gorm:"foreignKey:TenantID"`
    PlanID           uint       `gorm:"not null"`
    Plan             Plan       `gorm:"foreignKey:PlanID"`
    Status           string     `gorm:"type:varchar(20);not null;default:'active'"` // active | canceled
    StartedAt        time.Time  `gorm:"autoCreateTime"`
    CurrentPeriodEnd *time.Time
}
```

#### [NEW] `domain/usage.go`
```go
type TenantUsageDaily struct {
    TenantID              uint   `gorm:"primaryKey;autoIncrement:false"`
    Date                  string `gorm:"primaryKey;type:date"` // "2026-07-13"
    LogBytesIngested      int64  `gorm:"default:0"`
    MetricSamplesIngested int64  `gorm:"default:0"`
}
```

#### [MODIFY] [main.go](file:///home/aya/observability-platform/platform/backend/cmd/server/main.go)
- Add `&domain.Plan{}`, `&domain.TenantSubscription{}`, `&domain.TenantUsageDaily{}` to `AutoMigrate`
- After migration, **seed default plans** if `plans` table is empty:

| Name | MaxHosts | LogRetentionDays | MetricsRetentionDays | MaxLogBytesPerDay | Price |
|---|---|---|---|---|---|
| Free | 2 | 3 | 7 | 500 MB (524288000) | 0 |
| Pro | 10 | 14 | 30 | 5 GB (5368709120) | 49 |
| Enterprise | 50 | 30 | 90 | 50 GB (53687091200) | 199 |

- Also: when a new tenant signs up, auto-create a `TenantSubscription` row with `plan_id` = Free plan ID. Modify `auth_service.go` `Signup()` method to do this.

### Backend Files

#### [NEW] `repository/plan_repository.go`
- `ListActivePlansCtx(ctx) ([]domain.Plan, error)`
- `FindByIDCtx(ctx, id uint) (*domain.Plan, error)`

#### [NEW] `repository/subscription_repository.go`
- `FindByTenantCtx(ctx, tenantID uint) (*domain.TenantSubscription, error)` — preload Plan
- `UpdatePlanCtx(ctx, tenantID uint, planID uint) error`

#### [NEW] `repository/usage_repository.go`
- `GetTodayUsageCtx(ctx, tenantID uint) (*domain.TenantUsageDaily, error)`
- `GetUsageRangeCtx(ctx, tenantID uint, fromDate string, toDate string) ([]domain.TenantUsageDaily, error)` — for sparkline
- `UpsertDailyUsageCtx(ctx, tenantID uint, date string, logBytes int64, metricSamples int64) error` — ON CONFLICT UPDATE with increment

#### [NEW] `service/billing_service.go`
- `GetBillingOverview(ctx, tenantID uint) (*dto.BillingOverview, error)`
  - Gets current subscription (with plan), current usage, host count, service count
  - Computes usage percentages
- `ChangePlan(ctx, tenantID uint, newPlanID uint) error`
  - Validate new plan exists and is active
  - Check current usage doesn't exceed new plan's limits (can't downgrade if already over)
  - Update subscription's plan_id
- `GetUsageHistory(ctx, tenantID uint, days int) ([]dto.DailyUsage, error)` — last N days for sparkline

#### [NEW] `service/usage_metering_service.go`
- `RecordIngestion(ctx, tenantID uint, logBytes int64, metricSamples int64)`
  - Increment Redis counter: `INCRBY usage:{tenantID}:{date}:log_bytes {logBytes}` and `INCRBY usage:{tenantID}:{date}:metric_samples {metricSamples}`
  - This is called from the ingest middleware/proxy handlers
- `CheckQuota(ctx, tenantID uint) (allowed bool, err error)`
  - Read Redis counter for today, compare against plan limits
  - If over quota, return false (but don't block — soft enforcement)
- `FlushToPostgres(ctx)` — called by a background ticker every 5 minutes
  - Scan Redis for all `usage:*:{date}:*` keys
  - For each tenant+date, read values, upsert into `tenant_usage_daily`, reset Redis counters

#### [MODIFY] [ingest_middleware.go](file:///home/aya/observability-platform/platform/backend/internal/middleware/ingest_middleware.go)
- After successful auth, call `usageMeteringService.RecordIngestion()` with the request body size (`c.Request().ContentLength` or `len(body)`)
- Call `usageMeteringService.CheckQuota()` — if over quota, add a response header `X-Quota-Exceeded: true` (don't reject — soft enforcement). The frontend overview can show a banner when it detects this.

#### [MODIFY] [main.go](file:///home/aya/observability-platform/platform/backend/cmd/server/main.go)
- Start a background goroutine for `usageMeteringService.FlushToPostgres()` on a 5-minute ticker
- Start plan seeding

#### [NEW] `dto/billing.go`
```go
type BillingOverview struct {
    Plan           PlanInfo      `json:"plan"`
    Usage          UsageInfo     `json:"usage"`
    AvailablePlans []PlanInfo    `json:"available_plans"`
}

type PlanInfo struct {
    ID                   uint    `json:"id"`
    Name                 string  `json:"name"`
    MaxHosts             int     `json:"max_hosts"`
    MaxServicesPerHost   int     `json:"max_services_per_host"`
    LogRetentionDays     int     `json:"log_retention_days"`
    MetricsRetentionDays int     `json:"metrics_retention_days"`
    MaxLogBytesPerDay    int64   `json:"max_log_bytes_per_day"`
    PricePerMonth        float64 `json:"price_per_month"`
}

type UsageInfo struct {
    HostsUsed        int   `json:"hosts_used"`
    HostsLimit       int   `json:"hosts_limit"`
    LogBytesToday    int64 `json:"log_bytes_today"`
    LogBytesLimit    int64 `json:"log_bytes_limit"`
    LogRetentionDays int   `json:"log_retention_days"`
}

type ChangePlanRequest struct {
    PlanID uint `json:"plan_id" validate:"required"`
}

type DailyUsage struct {
    Date     string `json:"date"`
    LogBytes int64  `json:"log_bytes"`
    Metrics  int64  `json:"metric_samples"`
}
```

#### [NEW] `handler/billing_handler.go`
- `GetBilling` → `GET /api/settings/billing` (admin only)
- `ChangePlan` → `POST /api/settings/billing/change-plan` (admin only)
- `GetUsageHistory` → `GET /api/settings/billing/usage?days=7` (admin only)

#### [MODIFY] [routes.go](file:///home/aya/observability-platform/platform/backend/internal/handler/routes.go)
- Add to settings group: `/billing` endpoints

#### [MODIFY] `service/auth_service.go`
- In `Signup()`, after creating tenant, create a `TenantSubscription` row with the Free plan

### Quota Enforcement at Host Registration

#### [MODIFY] `handler/host_handler.go`
- In `Register`, before creating a new host, count existing hosts for this tenant and compare with `plan.MaxHosts`. If at limit, return `HTTP 402` with error message "Host limit reached. Upgrade your plan."

### Frontend Changes

#### [NEW] `api/billing.ts`
API functions: `getBilling()`, `changePlan(planId)`, `getUsageHistory(days)`

#### [NEW] `pages/settings/BillingPage.tsx`
- Current plan card: plan name, price, limits
- Usage bars: "Hosts: 3/5", "Logs today: 340 MB / 500 MB" with progress bars
- Sparkline chart of last 7 days log usage (use Recharts — already in dependencies)
- Plan comparison cards (Free / Pro / Enterprise) with "Current" badge or "Upgrade" button
- "Upgrade" button calls `changePlan` (simulated — no payment)

---

## Phase 6: Template-Based Alert Rules

### DB Schema

#### [NEW] `domain/alert.go`
```go
type AlertRuleTemplate struct {
    ID                   uint    `gorm:"primaryKey"`
    ServiceType          string  `gorm:"type:varchar(30);not null;index"` // nginx, apache, postgres, mariadb, phpfpm, redis, host
    Name                 string  `gorm:"type:varchar(255);not null"`
    Description          string  `gorm:"type:text"`
    MetricQueryTemplate  string  `gorm:"type:text;not null"` // PromQL with %s placeholders
    DefaultThreshold     float64
    Unit                 string  `gorm:"type:varchar(20)"` // ms, %, count, seconds
    ComparisonOp         string  `gorm:"type:varchar(5);default:'>'"`
    DefaultSeverity      string  `gorm:"type:varchar(20);default:'warning'"`
    DefaultForDuration   string  `gorm:"type:varchar(10);default:'5m'"`
}

type AlertRule struct {
    ID               uint       `gorm:"primaryKey"`
    TenantID         uint       `gorm:"not null;index"`
    TemplateID       *uint      `gorm:"index"` // nullable for future custom rules
    Template         *AlertRuleTemplate `gorm:"foreignKey:TemplateID"`
    ScopeType        string     `gorm:"type:varchar(30);not null"` // host | service | all_hosts | all_services_of_type
    ScopeID          *uint      // specific host_id or discovered_service_id
    Threshold        float64    `gorm:"not null"`
    ForDuration      string     `gorm:"type:varchar(10);not null"`
    Severity         string     `gorm:"type:varchar(20);not null"`
    NotifyChannelIDs string     `gorm:"type:text"` // comma-separated IDs (simple) or use a join table
    IsEnabled        bool       `gorm:"type:boolean;default:true"`
    CreatedAt        time.Time  `gorm:"autoCreateTime"`
    UpdatedAt        time.Time  `gorm:"autoUpdateTime"`
}

type AlertEvent struct {
    ID             uint       `gorm:"primaryKey"`
    RuleID         uint       `gorm:"not null;index"`
    Rule           AlertRule  `gorm:"foreignKey:RuleID"`
    TenantID       uint       `gorm:"not null;index"`
    HostMachineID  string     `gorm:"type:varchar(255)"`
    ServiceID      *uint
    Status         string     `gorm:"type:varchar(20);not null"` // firing | resolved | acknowledged
    TriggeredAt    time.Time  `gorm:"not null"`
    ResolvedAt     *time.Time
    AcknowledgedBy *uint
    CreatedAt      time.Time  `gorm:"autoCreateTime"`
}
```

#### [MODIFY] [main.go](file:///home/aya/observability-platform/platform/backend/cmd/server/main.go)
- Add `&domain.AlertRuleTemplate{}`, `&domain.AlertRule{}`, `&domain.AlertEvent{}` to `AutoMigrate`
- After migration, **seed alert rule templates** if table is empty. Use the example templates from the Claude conversation:

| ServiceType | Name | DefaultThreshold | Unit | Description |
|---|---|---|---|---|
| host | High CPU usage | 90 | % | Fires when average CPU usage exceeds threshold |
| host | High memory usage | 85 | % | Fires when memory usage exceeds threshold |
| host | Disk space critical | 90 | % | Fires when any disk partition usage exceeds threshold |
| nginx | High 5xx rate | 5 | % | Fires when 5xx error rate exceeds threshold |
| nginx | High P99 latency | 5 | seconds | Fires when 99th percentile response time exceeds threshold |
| apache | High 5xx rate | 5 | % | Fires when 5xx error rate exceeds threshold |
| apache | High P99 latency | 5 | seconds | Fires when 99th percentile response time exceeds threshold |
| postgres | Connection saturation | 80 | % | Fires when active connections exceed threshold percentage of max |
| postgres | High deadlock rate | 1 | count/min | Fires when deadlock rate exceeds threshold |
| mariadb | Slow query spike | 10 | count/min | Fires when slow query rate exceeds threshold |
| mariadb | Connection saturation | 80 | % | Fires when active connections exceed threshold percentage of max |
| redis | High memory usage | 80 | % | Fires when Redis memory usage exceeds threshold |
| redis | High eviction rate | 100 | count/min | Fires when key eviction rate exceeds threshold |
| phpfpm | High active process usage | 80 | % | Fires when active PHP-FPM processes exceed threshold percentage |

### Backend Files

#### [NEW] `repository/alert_rule_repository.go`
- `ListTemplatesByServiceTypeCtx(ctx, serviceType string) ([]domain.AlertRuleTemplate, error)`
- `ListAllTemplatesCtx(ctx) ([]domain.AlertRuleTemplate, error)`
- `CreateRuleCtx(ctx, rule *domain.AlertRule) error`
- `ListRulesByTenantCtx(ctx, tenantID uint) ([]domain.AlertRule, error)` — preload Template
- `UpdateRuleCtx(ctx, ruleID uint, tenantID uint, updates map[string]interface{}) error`
- `DeleteRuleCtx(ctx, ruleID uint, tenantID uint) error`
- `ListEnabledRulesCtx(ctx) ([]domain.AlertRule, error)` — for the evaluator, preload Template
- `CreateAlertEventCtx(ctx, event *domain.AlertEvent) error`
- `ListAlertEventsByTenantCtx(ctx, tenantID uint, limit int) ([]domain.AlertEvent, error)` — preload Rule
- `FindFiringEventCtx(ctx, ruleID uint, hostMachineID string, serviceID *uint) (*domain.AlertEvent, error)` — for dedup
- `ResolveEventCtx(ctx, eventID uint) error` — set status=resolved, set ResolvedAt
- `AcknowledgeEventCtx(ctx, eventID uint, userID uint, tenantID uint) error`

#### [NEW] `service/alert_engine_service.go`
This is the background evaluator:
- `StartEvaluator(ctx context.Context)` — spawns a goroutine with a ticker (every 60 seconds)
- On each tick:
  1. `ListEnabledRulesCtx()` — get all enabled rules across all tenants
  2. Group by tenant (to batch PromQL/LogQL queries per tenant)
  3. For each rule, expand the template's `MetricQueryTemplate` by substituting tenant UUID, host machine ID, and threshold
  4. Execute the expanded PromQL query against VictoriaMetrics (`/select/{tenantID}/prometheus/api/v1/query`)
  5. If result exceeds threshold for the specified `ForDuration`:
     - Check if there's already a firing event for this rule+host+service (dedup)
     - If not, create a new `AlertEvent` with status `firing`
     - Send notification to the rule's notification channels (for PFE scope: just log it + insert event; actual email delivery is out of scope)
  6. For previously firing events, check if the condition has cleared → resolve them

> [!IMPORTANT]
> For PFE scope, the evaluator can be simplified: run queries every 60s, compare instant value against threshold (ignore `ForDuration` sustained-for logic initially — mention sustained evaluation as a "production improvement" in report). This avoids the complexity of maintaining firing-state windows.

#### [NEW] `handler/alert_rules_handler.go`
- `ListTemplates` → `GET /api/alerts/templates` — returns all templates grouped by service type
- `ListTemplatesByService` → `GET /api/alerts/templates/:serviceType` — returns templates for a specific service
- `CreateRule` → `POST /api/alerts/rules` (any authenticated user)
- `ListRules` → `GET /api/alerts/rules` (any authenticated user)
- `UpdateRule` → `PATCH /api/alerts/rules/:id` (any authenticated user) — update threshold, severity, enabled
- `DeleteRule` → `DELETE /api/alerts/rules/:id` (any authenticated user)
- `ListAlertEvents` → `GET /api/alerts/events` (any authenticated user) — alert history
- `AcknowledgeEvent` → `POST /api/alerts/events/:id/acknowledge` (any authenticated user)

#### [MODIFY] [routes.go](file:///home/aya/observability-platform/platform/backend/internal/handler/routes.go)
Add under `api` group (not admin-only — all members should see alerts):
```
api.GET("/alerts/templates", ...)
api.GET("/alerts/templates/:serviceType", ...)
api.POST("/alerts/rules", ...)
api.GET("/alerts/rules", ...)
api.PATCH("/alerts/rules/:id", ...)
api.DELETE("/alerts/rules/:id", ...)
api.GET("/alerts/events", ...)
api.POST("/alerts/events/:id/acknowledge", ...)
```

### Frontend Changes

#### [MODIFY] `api/alerts.ts`
Add functions: `getAlertTemplates()`, `getAlertTemplatesByService(serviceType)`, `createAlertRule(data)`, `getAlertRules()`, `updateAlertRule(id, data)`, `deleteAlertRule(id)`, `getAlertEvents()`, `acknowledgeAlertEvent(id)`

#### [MODIFY] `pages/alerts/AlertsPage.tsx`
Completely restructure into a tabbed layout:
- **Tab 1: Alert Events** (the active/history view)
  - List of alert events: rule name | host | service | severity | status (firing/resolved/acknowledged) | triggered at | actions (acknowledge)
  - Filter by: severity, status, service type
  - Keep existing vulnerability alerts section below or as a separate tab
- **Tab 2: Alert Rules** (manage rules)
  - List of configured rules: name | scope | threshold | severity | enabled toggle | actions (edit, delete)
  - "Add alert rule" button → opens the template picker flow (see below)
- **Tab 3: Vulnerability Alerts** (existing CVE alerts — keep as-is but move here)

#### [NEW] `pages/alerts/CreateAlertRuleModal.tsx` (or page)
The template picker flow:
1. Step 1: Pick service type (cards: Host, Nginx, Apache, PostgreSQL, MariaDB, PHP-FPM, Redis)
2. Step 2: See available templates for that service type (e.g., "High CPU usage", "High 5xx rate") as cards with description
3. Step 3: Click a template → form appears with:
   - Threshold input (pre-filled with default, editable)
   - Scope selector: "All hosts" / "Specific host" (dropdown of tenant's hosts) or "All [service_type] services" / "Specific service" (dropdown)
   - Severity dropdown: info / warning / critical
   - Notification channels: multi-select from configured channels
   - "Create" button

---

## Summary of All New Files

### Backend (`platform/backend/internal/`)

| Path | Phase |
|---|---|
| `domain/invitation.go` | 2 |
| `domain/api_key.go` | 3A (optional) |
| `domain/notification_channel.go` | 3C |
| `domain/plan.go` | 5 |
| `domain/usage.go` | 5 |
| `domain/alert.go` | 6 |
| `repository/invitation_repository.go` | 2 |
| `repository/notification_channel_repository.go` | 3C |
| `repository/plan_repository.go` | 5 |
| `repository/subscription_repository.go` | 5 |
| `repository/usage_repository.go` | 5 |
| `repository/alert_rule_repository.go` | 6 |
| `service/team_service.go` | 2 |
| `service/profile_service.go` | 4 |
| `service/billing_service.go` | 5 |
| `service/usage_metering_service.go` | 5 |
| `service/alert_engine_service.go` | 6 |
| `handler/team_handler.go` | 2 |
| `handler/settings_handler.go` | 3B |
| `handler/notification_handler.go` | 3C |
| `handler/profile_handler.go` | 4 |
| `handler/billing_handler.go` | 5 |
| `handler/alert_rules_handler.go` | 6 |
| `dto/team.go` | 2 |
| `dto/settings.go` | 3B |
| `dto/billing.go` | 5 |
| `dto/profile.go` | 4 |

### Frontend (`platform/frontend/src/`)

| Path | Phase |
|---|---|
| `api/team.ts` | 2 |
| `api/billing.ts` | 5 |
| `pages/settings/TeamPage.tsx` | 2 |
| `pages/settings/SettingsLayout.tsx` | 3 |
| `pages/settings/OrgSettingsPage.tsx` | 3B |
| `pages/settings/NotificationsPage.tsx` | 3C |
| `pages/settings/BillingPage.tsx` | 5 |
| `pages/auth/AcceptInvitePage.tsx` | 2 |
| `pages/alerts/CreateAlertRuleModal.tsx` | 6 |

### Modified Files

| File | Phases |
|---|---|
| `middleware/auth_middleware.go` | 1 |
| `domain/user.go` | 4 |
| `domain/tenant.go` | 3A, 3B |
| `repository/user_repository.go` | 2, 4 |
| `repository/tenant_repository.go` | 3B |
| `service/auth_service.go` | 5 |
| `handler/routes.go` | 1, 2, 3, 4, 5, 6 |
| `middleware/ingest_middleware.go` | 3A, 5 |
| `cmd/server/main.go` | 2, 3, 5, 6 |
| `frontend/src/router/index.tsx` | 2, 3 |
| `frontend/src/components/layout/Sidebar.tsx` | 2, 3 |
| `frontend/src/store/authStore.ts` | 2 |
| `frontend/src/pages/profile/ProfilePage.tsx` | 4 |
| `frontend/src/pages/settings/ApiKeysPage.tsx` | 3A |
| `frontend/src/pages/alerts/AlertsPage.tsx` | 6 |
| `frontend/src/api/alerts.ts` | 6 |

---

## Verification Plan

### Per Phase
After each phase, verify:
1. Backend compiles: `go build ./...` from `platform/backend/`
2. AutoMigrate runs without errors (check server startup logs)
3. API endpoints return expected responses (use `curl` or the frontend)

### Integration Tests
- **Phase 1**: Verify `RequireRole` rejects non-admin users with 403
- **Phase 2**: Full invite flow: admin invites → GET token validates → POST accept creates user → new user can login
- **Phase 5**: Register hosts beyond plan limit → verify 402 rejection
- **Phase 6**: Create a rule → verify it appears in list → wait for evaluator tick → check if events are created

### Manual Verification
- Frontend navigation: Settings tabs render correctly, admin-only items hidden for members
- Billing page: usage bars update after agent sends data
- Alert rule creation flow: template picker → form → rule appears in list

---

## Open Questions

> [!IMPORTANT]
> **1. Email sending**: Do you want to actually send invite emails / alert notification emails, or is returning the link / logging the notification sufficient for your PFE demo? Implementing real SMTP adds complexity but is impressive.

> [!IMPORTANT]
> **2. API key migration**: Do you want the multi-key hashed approach (more secure, more complex) or keep the current simple single-key approach and document the hashed approach as "production improvement"? I recommend keeping simple for PFE.

> [!IMPORTANT]
> **3. Alert evaluator PromQL queries**: Do you have the actual PromQL queries that work with your VictoriaMetrics label structure? The templates need to match exactly how your agent labels metrics (e.g., `{tenant_id="...", host_id="..."}` vs `{job="...", instance="..."}`). I'll need to see your actual metric labels to write correct templates.

> [!IMPORTANT]
> **4. Frontend styling**: Your frontend uses Tailwind v4 + shadcn. Should the new pages follow the exact same `Page`/`PageHeader`/`SectionCard` component pattern used in existing pages, or do you want to evolve the design?
