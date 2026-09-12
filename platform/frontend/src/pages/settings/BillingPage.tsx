import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { getBilling, changePlan, getUsageHistory } from "@/api/billing"
import { SectionCard } from "@/components/shared/Page"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from "recharts"

function formatBytes(bytes: number): string {
  if (bytes >= 1073741824) return (bytes / 1073741824).toFixed(1) + " GB"
  if (bytes >= 1048576) return (bytes / 1048576).toFixed(1) + " MB"
  if (bytes >= 1024) return (bytes / 1024).toFixed(1) + " KB"
  return bytes + " B"
}

export default function BillingPage() {
  const queryClient = useQueryClient()

  const billingQuery = useQuery({
    queryKey: ["billing"],
    queryFn: getBilling,
  })

  const usageHistoryQuery = useQuery({
    queryKey: ["billing", "usage"],
    queryFn: () => getUsageHistory(7),
  })

  const changePlanMutation = useMutation({
    mutationFn: (planId: number) => changePlan(planId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["billing"] }),
  })

  const billing = billingQuery.data

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-bold tracking-tight text-foreground">Billing</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Manage your subscription plan, usage, and invoices.
        </p>
      </div>

      {billingQuery.isLoading ? (
        <div className="text-sm text-muted-foreground py-8 text-center">Loading billing info...</div>
      ) : billing ? (
        <>
          {billing.pending_request && (
            <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 p-5 text-amber-900 dark:text-amber-200">
              <div className="flex items-start gap-3">
                <div className="mt-0.5 rounded-full bg-amber-500/20 p-1 text-amber-600 dark:text-amber-400">
                  <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                </div>
                <div className="space-y-1">
                  <h3 className="font-semibold text-sm">Subscription Request Pending Admin Approval</h3>
                  <p className="text-xs text-muted-foreground leading-relaxed">
                    You submitted a request to switch your subscription to <strong className="text-foreground">{billing.pending_request.plan_name}</strong> (${billing.pending_request.plan_price.toFixed(2)}/mo). An administrator will review and process your request shortly.
                  </p>
                </div>
              </div>
            </div>
          )}

          <SectionCard title="Current Plan" description="Your active subscription plan.">
            <div className="grid gap-2">
              <div className="text-2xl font-semibold">{billing.plan.name}</div>
              <div className="text-muted-foreground">
                ${billing.plan.price_per_month.toFixed(2)} / month
              </div>
              <div className="grid grid-cols-2 gap-4 mt-4 text-sm">
                <div>
                  <span className="text-muted-foreground">Max hosts:</span> {billing.plan.max_hosts}
                </div>
                <div>
                  <span className="text-muted-foreground">Log retention:</span> {billing.plan.log_retention_days} days
                </div>
                <div>
                  <span className="text-muted-foreground">Metrics retention:</span> {billing.plan.metrics_retention_days} days
                </div>
              </div>
            </div>
          </SectionCard>

          <SectionCard title="Usage" description="Current billing period usage.">
            <div className="grid gap-4">
              <div>
                <div className="flex justify-between text-sm mb-1">
                  <span>Hosts</span>
                  <span>{billing.usage.hosts_used} / {billing.usage.hosts_limit}</span>
                </div>
                <div className="h-2 bg-muted rounded-full overflow-hidden">
                  <div
                    className={cn(
                      "h-full rounded-full transition-all",
                      billing.usage.hosts_used >= billing.usage.hosts_limit ? "bg-destructive" : "bg-primary"
                    )}
                    style={{ width: `${Math.min((billing.usage.hosts_used / billing.usage.hosts_limit) * 100, 100)}%` }}
                  />
                </div>
              </div>
            </div>

            {usageHistoryQuery.data && usageHistoryQuery.data.length > 0 && (
              <div className="mt-6 h-48">
                <ResponsiveContainer width="100%" height="100%">
                  <BarChart data={usageHistoryQuery.data}>
                    <CartesianGrid strokeDasharray="3 3" className="stroke-border" />
                    <XAxis dataKey="date" tick={{ fontSize: 11 }} />
                    <YAxis tickFormatter={(v) => formatBytes(v)} tick={{ fontSize: 11 }} />
                    <Tooltip formatter={(v) => formatBytes(Number(v))} />
                    <Bar dataKey="log_bytes" fill="hsl(var(--primary))" radius={[4, 4, 0, 0]} />
                  </BarChart>
                </ResponsiveContainer>
              </div>
            )}
          </SectionCard>

          <SectionCard title="Available Plans" description="Compare and request plan changes.">
            <div className="grid gap-4 md:grid-cols-3">
              {billing.available_plans.map((plan) => {
                const isCurrent = plan.id === billing.plan.id
                const isRequested = billing.pending_request?.plan_id === plan.id
                return (
                  <div
                    key={plan.id}
                    className={cn(
                      "rounded-xl border p-4 flex flex-col justify-between",
                      isCurrent ? "border-primary" : isRequested ? "border-amber-500/50 bg-amber-500/5" : "border-border"
                    )}
                  >
                    <div>
                      <div className="flex items-center justify-between">
                        <div className="font-semibold text-lg">{plan.name}</div>
                        {isRequested && (
                          <span className="text-[10px] font-medium uppercase px-2 py-0.5 rounded-full bg-amber-500/20 text-amber-600 dark:text-amber-400">
                            Requested
                          </span>
                        )}
                      </div>
                      <div className="text-2xl font-bold mt-2">${plan.price_per_month.toFixed(2)}</div>
                      <div className="text-sm text-muted-foreground">per month</div>
                      <ul className="mt-4 grid gap-2 text-sm">
                        <li>Up to {plan.max_hosts} hosts</li>
                        <li>{plan.log_retention_days} day log retention</li>
                      </ul>
                    </div>
                    <div className="mt-6">
                      {isCurrent ? (
                        <Button variant="outline" disabled className="w-full">Current Plan</Button>
                      ) : isRequested ? (
                        <Button variant="outline" disabled className="w-full text-amber-600 dark:text-amber-400 border-amber-500/30">
                          Request Pending
                        </Button>
                      ) : (
                        <Button
                          className="w-full"
                          onClick={() => changePlanMutation.mutate(plan.id)}
                          disabled={changePlanMutation.isPending}
                        >
                          {changePlanMutation.isPending ? "Submitting..." : "Request Plan"}
                        </Button>
                      )}
                    </div>
                  </div>
                )
              })}
            </div>
          </SectionCard>
        </>
      ) : null}
    </div>
  )
}
