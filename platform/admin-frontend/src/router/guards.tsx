import { Navigate, Outlet } from "react-router-dom"
import { useAdminAuthStore } from "@/store/adminAuthStore"

export function PublicRoute() {
  const status = useAdminAuthStore((s) => s.status)

  if (status === "unknown") {
    return (
      <div className="flex h-screen items-center justify-center">
        <div className="text-sm text-muted-foreground">Loading...</div>
      </div>
    )
  }

  return status === "authenticated" ? (
    <Navigate to="/dashboard" replace />
  ) : (
    <Outlet />
  )
}

export function ProtectedRoute() {
  const status = useAdminAuthStore((s) => s.status)

  if (status === "unknown") {
    return (
      <div className="flex h-screen items-center justify-center">
        <div className="text-sm text-muted-foreground">Loading...</div>
      </div>
    )
  }

  return status === "authenticated" ? (
    <Outlet />
  ) : (
    <Navigate to="/login" replace />
  )
}
