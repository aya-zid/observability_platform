import { useNavigate } from "react-router-dom"

import * as authApi from "@/api/auth"
import ThemeToggle from "@/components/shared/ThemeToggle"
import { Button } from "@/components/ui/button"
import { useAuthStore } from "@/store/authStore"

export default function TopBar() {
  const navigate = useNavigate()
  const clearSession = useAuthStore((s) => s.clearSession)

  async function onLogout() {
    try {
      await authApi.logout()
    } finally {
      clearSession()
      navigate("/login", { replace: true })
    }
  }

  return (
    <header className="sticky top-0 z-10 border-b border-border bg-background/70 backdrop-blur">
      <div className="h-14 px-4 md:px-6 flex items-center justify-end gap-2">
        <ThemeToggle />
        <Button variant="outline" onClick={onLogout}>
          Logout
        </Button>
      </div>
    </header>
  )
}
