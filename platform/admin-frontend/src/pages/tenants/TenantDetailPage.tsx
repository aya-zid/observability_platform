import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useParams, useNavigate } from "react-router-dom"
import { ArrowLeft, Ban, UserCheck, RefreshCw } from "lucide-react"
import { Button } from "@/components/ui/button"
import client from "@/api/client"

type TenantDetail = {
  id: number
  company_name: string
  contact_email: string
  contact_phone: string
  is_active: boolean
  subscription_status: string
  timezone: string
  hosts_used: number
  plan_name: string
  plan_price: number
  log_retention_days: number
  metrics_retention_days: number
  max_hosts: number
  max_log_bytes_per_day: number
  current_period_end: string | null
  started_at: string
  banned_at: string | null
  ban_reason: string
  ai_reports_enabled: boolean
  users_count: number
  created_at: string
}

export default function TenantDetailPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const { data, isLoading } = useQuery<TenantDetail>({
    queryKey: ["admin", "tenant", id],
    queryFn: async () => {
      const res = await client.get(`/dashboard/tenants/${id}`)
      return res.data
    },
    enabled: !!id,
  })

  const banMutation = useMutation({
    mutationFn: async (reason: string) => {
      await client.post(`/tenants/${id}/ban`, { reason })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["admin"] })
    },
  })

  const unbanMutation = useMutation({
    mutationFn: async () => {
      await client.post(`/tenants/${id}/unban`)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["admin"] })
    },
  })

  const renewMutation = useMutation({
    mutationFn: async (months: number) => {
      await client.post(`/tenants/${id}/renew`, { duration_months: months })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["admin"] })
    },
  })

  if (isLoading) {
    return <div className="p-8">Loading...</div>
  }

  if (!data) {
    return <div className="p-8">Tenant not found</div>
  }

  const handleBan = () => {
    const reason = prompt("Ban reason:")
    if (reason !== null) {
      banMutation.mutate(reason)
    }
  }

  const handleRenew = () => {
    const months = prompt("Renew for how many months?", "1")
    if (months !== null) {
      const m = parseInt(months, 10)
      if (m > 0) {
        renewMutation.mutate(m)
      }
    }
  }

  return (
    <div className="p-8 space-y-6">
      <div className="flex items-center gap-4">
        <button onClick={() => navigate("/tenants")} className="p-2 hover:bg-muted rounded">
          <ArrowLeft className="h-4 w-4" />
        </button>
        <h1 className="text-3xl font-bold">{data.company_name}</h1>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        <div className="rounded-lg border p-4">
          <p className="text-sm text-muted-foreground">Contact Email</p>
          <p className="font-medium">{data.contact_email}</p>
        </div>
        <div className="rounded-lg border p-4">
          <p className="text-sm text-muted-foreground">Contact Phone</p>
          <p className="font-medium">{data.contact_phone || "N/A"}</p>
        </div>
        <div className="rounded-lg border p-4">
          <p className="text-sm text-muted-foreground">Timezone</p>
          <p className="font-medium">{data.timezone}</p>
        </div>
        <div className="rounded-lg border p-4">
          <p className="text-sm text-muted-foreground">Status</p>
          <span className={`inline-flex items-center rounded-full px-2 py-1 text-xs font-medium ${
            data.is_active ? "bg-green-100 text-green-700" : "bg-red-100 text-red-700"
          }`}>
            {data.subscription_status}
          </span>
        </div>
        <div className="rounded-lg border p-4">
          <p className="text-sm text-muted-foreground">Plan</p>
          <p className="font-medium">{data.plan_name || "None"} {data.plan_price > 0 ? `($${data.plan_price}/mo)` : ""}</p>
        </div>
        <div className="rounded-lg border p-4">
          <p className="text-sm text-muted-foreground">Hosts</p>
          <p className="font-medium">{data.hosts_used} / {data.max_hosts}</p>
        </div>
        <div className="rounded-lg border p-4">
          <p className="text-sm text-muted-foreground">Log Retention</p>
          <p className="font-medium">{data.log_retention_days} days</p>
        </div>
        <div className="rounded-lg border p-4">
          <p className="text-sm text-muted-foreground">Metrics Retention</p>
          <p className="font-medium">{data.metrics_retention_days} days</p>
        </div>
        <div className="rounded-lg border p-4">
          <p className="text-sm text-muted-foreground">AI Reports</p>
          <p className="font-medium">{data.ai_reports_enabled ? "Enabled" : "Disabled"}</p>
        </div>
        <div className="rounded-lg border p-4">
          <p className="text-sm text-muted-foreground">Users</p>
          <p className="font-medium">{data.users_count}</p>
        </div>
        <div className="rounded-lg border p-4">
          <p className="text-sm text-muted-foreground">Started At</p>
          <p className="font-medium">{data.started_at}</p>
        </div>
        <div className="rounded-lg border p-4">
          <p className="text-sm text-muted-foreground">Current Period End</p>
          <p className="font-medium">{data.current_period_end || "N/A"}</p>
        </div>
      </div>

      {data.banned_at && (
        <div className="rounded-lg border border-destructive/50 bg-destructive/5 p-4">
          <p className="text-sm font-medium text-destructive">Banned</p>
          <p className="text-sm text-muted-foreground">Reason: {data.ban_reason}</p>
          <p className="text-sm text-muted-foreground">Date: {data.banned_at}</p>
        </div>
      )}

      <div className="flex gap-3">
        {data.is_active ? (
          <Button variant="destructive" onClick={handleBan} disabled={banMutation.isPending}>
            <Ban className="h-4 w-4 mr-2" /> Ban Tenant
          </Button>
        ) : (
          <Button variant="outline" onClick={() => unbanMutation.mutate()} disabled={unbanMutation.isPending}>
            <UserCheck className="h-4 w-4 mr-2" /> Unban Tenant
          </Button>
        )}
        <Button variant="outline" onClick={handleRenew} disabled={renewMutation.isPending}>
          <RefreshCw className="h-4 w-4 mr-2" /> Renew Subscription
        </Button>
      </div>
    </div>
  )
}
