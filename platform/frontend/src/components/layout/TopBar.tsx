import { useNavigate } from "react-router-dom"
import { LogOut } from "lucide-react"

import * as authApi from "@/api/auth"
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
        <button
          onClick={onLogout}
          className="inline-flex items-center gap-1.5 h-8 px-3 rounded-md text-sm font-medium text-muted-foreground hover:text-foreground hover:bg-muted transition-colors border border-border"
        >
          <LogOut className="w-3.5 h-3.5" />
          Logout
        </button>
      </div>
    </header>
  )
}
