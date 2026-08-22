import { Navigate, Outlet } from "react-router-dom"

import InactivePage from "@/pages/auth/InactivePage"
import SubscriptionExpiredPage from "@/pages/auth/SubscriptionExpiredPage"
import AccountSuspendedPage from "@/pages/auth/AccountSuspendedPage"
import { useAuthStore } from "@/store/authStore"

export function ProtectedRoute() {
  const status = useAuthStore((s) => s.status)

  if (status === "unknown") {
    return (
      <div className="min-h-dvh grid place-items-center">
        <div className="text-sm text-muted-foreground">Checking session…</div>
      </div>
    )
  }

  if (status === "inactive") {
    return <InactivePage />
  }

  if (status === "subscription_expired") {
    return <SubscriptionExpiredPage />
  }

  if (status === "account_suspended") {
    return <AccountSuspendedPage />
  }

  return status === "authenticated" ? (
    <Outlet />
  ) : (
    <Navigate to="/login" replace />
  )
}

export function PublicRoute() {
  const status = useAuthStore((s) => s.status)

  if (status === "unknown") {
    return (
      <div className="min-h-dvh grid place-items-center">
        <div className="text-sm text-muted-foreground">Loading…</div>
      </div>
    )
  }

  if (status === "inactive") {
    return <InactivePage />
  }

  if (status === "subscription_expired") {
    return <SubscriptionExpiredPage />
  }

  if (status === "account_suspended") {
    return <AccountSuspendedPage />
  }

  return status === "authenticated" ? (
    <Navigate to="/overview" replace />
  ) : (
    <Outlet />
  )
}

