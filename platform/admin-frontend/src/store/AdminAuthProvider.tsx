import { useEffect } from "react"
import { useAdminAuthStore } from "@/store/adminAuthStore"
import client from "@/api/client"

export function AdminAuthProvider({ children }: { children: React.ReactNode }) {
  const status = useAdminAuthStore((s) => s.status)
  const setStatus = useAdminAuthStore((s) => s.setStatus)
  const setRole = useAdminAuthStore((s) => s.setRole)

  useEffect(() => {
    if (status !== "unknown") return
    client.get("/auth/me")
      .then((res) => {
        setStatus("authenticated")
        setRole(res.data.role)
      })
      .catch(() => {
        setStatus("unauthenticated")
      })
  }, [status, setStatus, setRole])

  return <>{children}</>
}
