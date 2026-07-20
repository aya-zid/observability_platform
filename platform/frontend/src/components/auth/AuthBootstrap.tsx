import { type ReactNode, useEffect, useRef, useState } from "react"

import * as authApi from "@/api/auth"
import { useAuthStore } from "@/store/authStore"

export default function AuthBootstrap({
  children,
}: {
  children: ReactNode
}) {
  const ran = useRef(false)
  const setStatus = useAuthStore((s) => s.setStatus)
  const setRole = useAuthStore((s) => s.setRole)
  const [ready, setReady] = useState(false)

  useEffect(() => {
    if (ran.current) return
    ran.current = true

    // On page load, try to refresh using HttpOnly refresh_token cookie.
    // The /refresh endpoint returns the role directly in the response body.
    authApi
      .refresh()
      .then((res) => {
        setStatus("authenticated")
        if (res.role) setRole(res.role)
      })
      .catch(() => setStatus("unauthenticated"))
      .finally(() => setReady(true))
  }, [setStatus, setRole])

  if (!ready) {
    return (
      <div className="min-h-dvh grid place-items-center">
        <div className="text-sm text-muted-foreground">Loading session…</div>
      </div>
    )
  }

  return <>{children}</>
}
