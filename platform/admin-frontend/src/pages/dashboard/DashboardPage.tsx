import { useQuery } from "@tanstack/react-query"
import {
  Building2,
  Users,
  Server,
  Activity,
  DollarSign,
  TrendingUp,
} from "lucide-react"
import client from "@/api/client"

type StatsResponse = {
  total_tenants: number
  active_tenants: number
  banned_tenants: number
  total_hosts: number
  total_ingest_today: number
  revenue_estimate: number
  plan_distribution: { plan_name: string; count: number }[]
  ingestion_trend: { date: string; log_bytes: number; metrics: number }[]
}

function StatCard({
  title,
  value,
  icon: Icon,
  suffix,
  variant = "default",
}: {
  title: string
  value: number
  icon: React.ComponentType<{ className?: string }>
  suffix?: string
  variant?: "default" | "success" | "danger"
}) {
  const colorClass =
    variant === "success"
      ? "text-status-green"
      : variant === "danger"
        ? "text-status-red"
        : "text-primary"

  return (
    <div className="rounded-lg border border-border bg-card p-4 shadow-card">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">{title}</p>
        <Icon className={`h-4 w-4 ${colorClass}`} />
      </div>
      <p className="text-2xl font-bold mt-2">
        {value.toLocaleString()}
        {suffix && <span className="text-sm font-normal text-muted-foreground ml-1">{suffix}</span>}
      </p>
    </div>
  )
}

export default function DashboardPage() {
  const { data, isLoading } = useQuery<StatsResponse>({
    queryKey: ["admin", "dashboard", "stats"],
    queryFn: async () => {
      const res = await client.get("/dashboard/stats")
      return res.data
    },
  })

  if (isLoading) {
    return (
      <div className="flex h-screen items-center justify-center">
        <div className="text-sm text-muted-foreground">Loading dashboard…</div>
      </div>
    )
  }

  if (!data) {
    return (
      <div className="p-8 text-destructive">Failed to load dashboard stats.</div>
    )
  }

  return (
    <div className="p-6 md:p-8 space-y-6">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Dashboard</h1>
        <p className="text-muted-foreground mt-1">Platform overview and health</p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard title="Total Tenants" value={data.total_tenants} icon={Building2} />
        <StatCard title="Active Tenants" value={data.active_tenants} icon={Activity} variant="success" />
        <StatCard title="Banned Tenants" value={data.banned_tenants} icon={Users} variant="danger" />
        <StatCard title="Total Hosts" value={data.total_hosts} icon={Server} />
        <StatCard
          title="Ingestion Today"
          value={data.total_ingest_today}
          icon={TrendingUp}
          suffix="bytes"
        />
        <StatCard
          title="Revenue (est.)"
          value={Math.round(data.revenue_estimate)}
          icon={DollarSign}
          suffix="$"
        />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="rounded-lg border border-border bg-card p-5 shadow-card">
          <h2 className="text-lg font-semibold mb-4">Plan Distribution</h2>
          {data.plan_distribution.length === 0 ? (
            <p className="text-muted-foreground">No data yet.</p>
          ) : (
            <div className="space-y-3">
              {data.plan_distribution.map((p) => (
                <div
                  key={p.plan_name}
                  className="flex items-center justify-between rounded-md bg-secondary/50 px-3 py-2"
                >
                  <span className="text-sm font-medium">{p.plan_name}</span>
                  <span className="text-sm text-muted-foreground">{p.count} tenants</span>
                </div>
              ))}
            </div>
          )}
        </div>

        <div className="rounded-lg border border-border bg-card p-5 shadow-card">
          <h2 className="text-lg font-semibold mb-4">Ingestion Trend (7 days)</h2>
          {data.ingestion_trend.length === 0 ? (
            <p className="text-muted-foreground">No data yet.</p>
          ) : (
            <div className="space-y-3">
              {data.ingestion_trend.map((t) => (
                <div
                  key={t.date}
                  className="flex items-center justify-between rounded-md bg-secondary/50 px-3 py-2"
                >
                  <span className="text-sm">{t.date}</span>
                  <span className="text-sm font-medium">
                    {(t.log_bytes / 1024 / 1024).toFixed(1)} MB
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
