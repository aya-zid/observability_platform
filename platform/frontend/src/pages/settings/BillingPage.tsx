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
                <div>
                  <span className="text-muted-foreground">Daily log limit:</span> {formatBytes(billing.plan.max_log_bytes_per_day)}
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
              <div>
                <div className="flex justify-between text-sm mb-1">
                  <span>Logs Today</span>
                  <span>{formatBytes(billing.usage.log_bytes_today)} / {formatBytes(billing.usage.log_bytes_limit)}</span>
                </div>
                <div className="h-2 bg-muted rounded-full overflow-hidden">
                  <div
                    className={cn(
                      "h-full rounded-full transition-all",
                      billing.usage.log_bytes_today >= billing.usage.log_bytes_limit ? "bg-destructive" : "bg-primary"
                    )}
                    style={{ width: `${Math.min((billing.usage.log_bytes_today / billing.usage.log_bytes_limit) * 100, 100)}%` }}
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

          <SectionCard title="Available Plans" description="Compare and upgrade your plan.">
            <div className="grid gap-4 md:grid-cols-3">
              {billing.available_plans.map((plan) => {
                const isCurrent = plan.id === billing.plan.id
                return (
                  <div
                    key={plan.id}
                    className={cn(
                      "rounded-xl border p-4",
                      isCurrent ? "border-primary" : "border-border"
                    )}
                  >
                    <div className="font-semibold text-lg">{plan.name}</div>
                    <div className="text-2xl font-bold mt-2">${plan.price_per_month.toFixed(2)}</div>
                    <div className="text-sm text-muted-foreground">per month</div>
                    <ul className="mt-4 grid gap-2 text-sm">
                      <li>Up to {plan.max_hosts} hosts</li>
                      <li>{plan.log_retention_days} day log retention</li>
                      <li>{formatBytes(plan.max_log_bytes_per_day)} / day log limit</li>
                    </ul>
                    <div className="mt-4">
                      {isCurrent ? (
                        <Button variant="outline" disabled className="w-full">Current</Button>
                      ) : (
                        <Button
                          className="w-full"
                          onClick={() => changePlanMutation.mutate(plan.id)}
                          disabled={changePlanMutation.isPending}
                        >
                          {plan.price_per_month === 0 ? "Downgrade" : "Upgrade"}
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
