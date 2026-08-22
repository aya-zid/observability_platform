import { useNavigate } from "react-router-dom"
import { ShieldAlert, LogOut } from "lucide-react"

import * as authApi from "@/api/auth"
import AuthShell from "@/components/auth/AuthShell"
import { Button } from "@/components/ui/button"
import { useAuthStore } from "@/store/authStore"

export default function InactivePage() {
  const navigate = useNavigate()
  const clearSession = useAuthStore((s) => s.clearSession)

  async function onSignOut() {
    try {
      await authApi.logout()
    } catch {
      // session cookies are cleared server-side; a failure here is non-fatal
    }
    clearSession()
    navigate("/login", { replace: true })
  }

  return (
    <AuthShell>
      <div className="rounded-2xl border border-border bg-card p-8">
        <div className="w-12 h-12 rounded-xl bg-destructive/10 flex items-center justify-center mb-5 ring-1 ring-destructive/20">
          <ShieldAlert className="w-6 h-6 text-destructive" />
        </div>
        <h1 className="text-2xl font-bold text-foreground">Account deactivated</h1>
        <p className="text-sm text-muted-foreground mt-3 leading-relaxed">
          Your account has been deactivated by your company administrator.
          You can no longer access the platform or its features until your
          status is restored.
        </p>
        <p className="text-sm text-muted-foreground mt-4 leading-relaxed">
          Please contact your admin to reactivate your account.
        </p>
        <Button
          onClick={onSignOut}
          variant="outline"
          className="w-full mt-6 rounded-md inline-flex items-center justify-center gap-2 h-10"
        >
          <LogOut className="w-4 h-4" />
          Sign out
        </Button>
      </div>
    </AuthShell>
  )
}