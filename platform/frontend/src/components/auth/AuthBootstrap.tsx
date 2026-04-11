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
  const [ready, setReady] = useState(false)

  useEffect(() => {
    if (ran.current) return
    ran.current = true

    // On page load, try to refresh using HttpOnly refresh_token cookie.
    authApi
      .refresh()
      .then(() => setStatus("authenticated"))
      .catch(() => setStatus("unauthenticated"))
      .finally(() => setReady(true))
  }, [setStatus])

  if (!ready) {
    return (
      <div className="min-h-dvh grid place-items-center">
        <div className="text-sm text-muted-foreground">Loading session…</div>
      </div>
    )
  }

  return <>{children}</>
}
