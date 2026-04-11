import { Navigate, Outlet } from "react-router-dom"

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

  return status === "authenticated" ? (
    <Navigate to="/overview" replace />
  ) : (
    <Outlet />
  )
}

