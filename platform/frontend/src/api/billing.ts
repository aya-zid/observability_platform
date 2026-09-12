import client from "@/api/client"

export type PlanInfo = {
  id: number
  name: string
  max_hosts: number
  max_services_per_host: number
  log_retention_days: number
  metrics_retention_days: number
  price_per_month: number
}

export type UsageInfo = {
  hosts_used: number
  hosts_limit: number
  log_bytes_today: number
  log_retention_days: number
}

export type PendingRequestInfo = {
  id: number
  plan_id: number
  plan_name: string
  plan_price: number
  status: string
  notes?: string
  created_at: string
}

export type BillingOverview = {
  plan: PlanInfo
  usage: UsageInfo
  available_plans: PlanInfo[]
  pending_request?: PendingRequestInfo
}

export type DailyUsage = {
  date: string
  log_bytes: number
  metric_samples: number
}

export async function getBilling(): Promise<BillingOverview> {
  const { data } = await client.get<BillingOverview>("/api/settings/billing")
  return data
}

export async function changePlan(planId: number): Promise<void> {
  await client.post("/api/settings/billing/change-plan", { plan_id: planId })
}

export async function getUsageHistory(days: number = 7): Promise<DailyUsage[]> {
  const { data } = await client.get<DailyUsage[]>(`/api/settings/billing/usage?days=${days}`)
  return data
}
