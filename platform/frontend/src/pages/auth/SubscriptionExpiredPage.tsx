import { useEffect, useState } from "react"
import { useNavigate } from "react-router-dom"
import { CreditCard, LogOut, Check } from "lucide-react"

import * as authApi from "@/api/auth"
import AuthShell from "@/components/auth/AuthShell"
import { Button } from "@/components/ui/button"
import { useAuthStore } from "@/store/authStore"

type Plan = {
  id: number
  name: string
  description: string
  price_per_month: number
  max_hosts: number
  max_services_per_host: number
  log_retention_days: number
  metrics_retention_days: number
  ai_reports_enabled: boolean
  duration_months: number
}

export default function SubscriptionExpiredPage() {
  const navigate = useNavigate()
  const clearSession = useAuthStore((s) => s.clearSession)
  const [plans, setPlans] = useState<Plan[]>([])
  const [loading, setLoading] = useState(true)
  const [requested, setRequested] = useState(false)

  async function onSelectPlan(planId: number) {
    setSelecting(planId)
    try {
      await fetch("/api/settings/billing/change-plan", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ plan_id: planId }),
      })
      setRequested(true)
      setSelecting(null)
    } catch {
      setSelecting(null)
    }
  }

  return (
    <AuthShell>
      <div className="rounded-2xl border border-border bg-card p-8">
        <div className="w-12 h-12 rounded-xl bg-amber-500/10 flex items-center justify-center mb-5 ring-1 ring-amber-500/20">
          <CreditCard className="w-6 h-6 text-amber-500" />
        </div>
        <h1 className="text-2xl font-bold text-foreground">Subscription expired</h1>
        <p className="text-sm text-muted-foreground mt-3 leading-relaxed">
          Your subscription has expired. Choose a plan below to restore full access
          to the platform.
        </p>

        {requested && (
          <div className="mt-4 rounded-xl border border-amber-500/30 bg-amber-500/10 p-4 text-xs text-amber-900 dark:text-amber-200">
            Subscription request submitted! An administrator will review your request shortly to activate your plan.
          </div>
        )}

        {loading ? (
          <div className="text-sm text-muted-foreground mt-6">Loading plans…</div>
        ) : (
          <div className="mt-6 space-y-3">
            {plans.map((plan) => (
              <button
                key={plan.id}
                onClick={() => onSelectPlan(plan.id)}
                disabled={selecting !== null}
                className="w-full text-left rounded-xl border border-border bg-secondary/50 p-4 hover:border-primary/40 transition-colors disabled:opacity-60"
              >
                <div className="flex items-center justify-between">
                  <div>
                    <div className="text-sm font-semibold text-foreground">{plan.name}</div>
                    <div className="text-xs text-muted-foreground mt-1">
                      {plan.description || `${plan.max_hosts} hosts · ${plan.log_retention_days} days logs · $${plan.price_per_month}/mo`}
                    </div>
                  </div>
                  {selecting === plan.id ? (
                    <div className="text-xs text-muted-foreground">Activating…</div>
                  ) : (
                    <div className="w-5 h-5 rounded-full border border-border flex items-center justify-center">
                      <Check className="w-3 h-3 text-primary opacity-0" />
                    </div>
                  )}
                </div>
              </button>
            ))}
          </div>
        )}

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
