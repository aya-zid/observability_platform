import { useQuery } from "@tanstack/react-query"
import { useNavigate } from "react-router-dom"
import {
  Building2,
  Users,
  Server,
  Activity,
  DollarSign,
  TrendingUp,
  CreditCard,
  AlertTriangle,
  ArrowRight,
  ShieldAlert,
  Sparkles,
  Package,
} from "lucide-react"
import {
  AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer,
  PieChart, Pie, Cell, Legend
} from "recharts"
import client from "@/api/client"
import { Skeleton } from "@/components/ui/skeleton"
import { Button } from "@/components/ui/button"

type StatsResponse = {
  total_tenants: number
  active_tenants: number
  banned_tenants: number
  total_hosts: number
  total_ingest_today: number
  revenue_estimate: number
  pending_requests?: number
  expired_subscriptions?: number
  plan_distribution: { plan_name: string; count: number }[]
  ingestion_trend: { date: string; log_bytes: number; metrics: number }[]
}

const PIE_COLORS = ["#3b82f6", "#10b981", "#8b5cf6", "#f59e0b", "#ef4444", "#06b6d4"]

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B"
  const k = 1024
  const sizes = ["B", "KB", "MB", "GB", "TB"]
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i]
}

export default function DashboardPage() {
  const navigate = useNavigate()

  const { data, isLoading, refetch } = useQuery<StatsResponse>({
    queryKey: ["admin", "dashboard", "stats"],
    queryFn: async () => {
      const res = await client.get("/dashboard/stats")
      return res.data
    },
    refetchInterval: 30000,
  })

  if (isLoading) {
    return (
      <div className="p-6 md:p-8 space-y-6 max-w-7xl mx-auto">
        <Skeleton className="h-10 w-48" />
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          {Array.from({ length: 8 }).map((_, i) => (
            <Skeleton key={i} className="h-28 rounded-xl" />
          ))}
        </div>
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          <Skeleton className="h-80 rounded-xl" />
          <Skeleton className="h-80 rounded-xl" />
        </div>
      </div>
    )
  }

  if (!data) {
    return (
      <div className="p-8 text-center space-y-4">
        <p className="text-destructive">Failed to load dashboard statistics.</p>
        <Button onClick={() => refetch()}>Try Again</Button>
      </div>
    )
  }

  const pendingRequests = data.pending_requests || 0
  const expiredSubs = data.expired_subscriptions || 0

  const chartData = data.ingestion_trend.map((item) => ({
    date: item.date.slice(5),
    "Logs (MB)": +(item.log_bytes / (1024 * 1024)).toFixed(2),
    Metrics: item.metrics,
  }))

  return (
    <div className="p-6 md:p-8 space-y-6 max-w-7xl mx-auto">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h1 className="text-3xl font-extrabold tracking-tight text-foreground flex items-center gap-2">
            Platform Command Center
            <Sparkles className="w-6 h-6 text-amber-400 animate-pulse" />
          </h1>
          <p className="text-sm text-muted-foreground mt-1">
            Real-time overview of tenant activity, revenue estimates, and pack subscriptions.
          </p>
        </div>
        <Button
          onClick={() => refetch()}
          variant="outline"
          size="sm"
          className="self-start sm:self-auto"
        >
          Refresh Overview
        </Button>
      </div>

      {/* Pending Requests Alert Banner */}
      {pendingRequests > 0 && (
        <div className="p-4 rounded-xl bg-gradient-to-r from-amber-500/15 via-amber-500/10 to-transparent border border-amber-500/30 flex flex-col sm:flex-row sm:items-center justify-between gap-4 animate-in fade-in duration-300">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-lg bg-amber-500/20 flex items-center justify-center text-amber-400 shrink-0 ring-1 ring-amber-500/40">
              <AlertTriangle className="w-5 h-5" />
            </div>
            <div>
              <h3 className="font-semibold text-foreground text-sm">
                {pendingRequests} Enterprise Subscription Request{pendingRequests > 1 ? "s" : ""} Pending Review
              </h3>
              <p className="text-xs text-muted-foreground mt-0.5">
                Tenants are awaiting pack approval. Review demand and activate enterprise packs.
              </p>
            </div>
          </div>
          <Button
            size="sm"
            onClick={() => navigate("/subscriptions")}
            className="bg-amber-500 hover:bg-amber-600 text-black font-semibold gap-1.5 shrink-0"
          >
            Review Demands
            <ArrowRight className="w-4 h-4" />
          </Button>
        </div>
      )}

      {/* Primary Metrics Grid */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <div className="p-5 rounded-xl border border-border bg-card shadow-sm hover:border-primary/40 transition-colors">
          <div className="flex items-center justify-between text-muted-foreground">
            <span className="text-xs font-medium uppercase tracking-wider">Total Tenants</span>
            <div className="p-2 rounded-lg bg-primary/10 text-primary">
              <Building2 className="w-5 h-5" />
            </div>
          </div>
          <div className="mt-3">
            <span className="text-3xl font-extrabold text-foreground">{data.total_tenants}</span>
            <span className="text-xs text-muted-foreground ml-2">registered</span>
          </div>
        </div>

        <div className="p-5 rounded-xl border border-border bg-card shadow-sm hover:border-emerald-500/40 transition-colors">
          <div className="flex items-center justify-between text-muted-foreground">
            <span className="text-xs font-medium uppercase tracking-wider">Active Tenants</span>
            <div className="p-2 rounded-lg bg-emerald-500/10 text-emerald-400">
              <Activity className="w-5 h-5" />
            </div>
          </div>
          <div className="mt-3">
            <span className="text-3xl font-extrabold text-foreground">{data.active_tenants}</span>
            <span className="text-xs text-emerald-400 font-medium ml-2">
              {data.total_tenants > 0 ? Math.round((data.active_tenants / data.total_tenants) * 100) : 0}% active
            </span>
          </div>
        </div>

        <div className="p-5 rounded-xl border border-border bg-card shadow-sm hover:border-purple-500/40 transition-colors">
          <div className="flex items-center justify-between text-muted-foreground">
            <span className="text-xs font-medium uppercase tracking-wider">Est. Monthly Revenue</span>
            <div className="p-2 rounded-lg bg-purple-500/10 text-purple-400">
              <DollarSign className="w-5 h-5" />
            </div>
          </div>
          <div className="mt-3">
            <span className="text-3xl font-extrabold text-foreground">
              ${Math.round(data.revenue_estimate).toLocaleString()}
            </span>
            <span className="text-xs text-muted-foreground ml-2">/mo</span>
          </div>
        </div>

        <div className="p-5 rounded-xl border border-border bg-card shadow-sm hover:border-sky-500/40 transition-colors">
          <div className="flex items-center justify-between text-muted-foreground">
            <span className="text-xs font-medium uppercase tracking-wider">Total Hosts Monitored</span>
            <div className="p-2 rounded-lg bg-sky-500/10 text-sky-400">
              <Server className="w-5 h-5" />
            </div>
          </div>
          <div className="mt-3">
            <span className="text-3xl font-extrabold text-foreground">{data.total_hosts}</span>
            <span className="text-xs text-muted-foreground ml-2">agents active</span>
          </div>
        </div>

        <div className="p-5 rounded-xl border border-border bg-card shadow-sm hover:border-amber-500/40 transition-colors">
          <div className="flex items-center justify-between text-muted-foreground">
            <span className="text-xs font-medium uppercase tracking-wider">Pack Requests</span>
            <div className="p-2 rounded-lg bg-amber-500/10 text-amber-400">
              <CreditCard className="w-5 h-5" />
            </div>
          </div>
          <div className="mt-3 flex items-baseline justify-between">
            <span className="text-3xl font-extrabold text-foreground">{pendingRequests}</span>
            {pendingRequests > 0 && (
              <span className="px-2 py-0.5 text-xs rounded-full bg-amber-500/20 text-amber-400 font-semibold">
                Action required
              </span>
            )}
          </div>
        </div>

        <div className="p-5 rounded-xl border border-border bg-card shadow-sm hover:border-rose-500/40 transition-colors">
          <div className="flex items-center justify-between text-muted-foreground">
            <span className="text-xs font-medium uppercase tracking-wider">Expired Subs</span>
            <div className="p-2 rounded-lg bg-rose-500/10 text-rose-400">
              <ShieldAlert className="w-5 h-5" />
            </div>
          </div>
          <div className="mt-3">
            <span className="text-3xl font-extrabold text-foreground">{expiredSubs}</span>
            <span className="text-xs text-muted-foreground ml-2">need renewal</span>
          </div>
        </div>

        <div className="p-5 rounded-xl border border-border bg-card shadow-sm hover:border-blue-500/40 transition-colors">
          <div className="flex items-center justify-between text-muted-foreground">
            <span className="text-xs font-medium uppercase tracking-wider">Ingestion Today</span>
            <div className="p-2 rounded-lg bg-blue-500/10 text-blue-400">
              <TrendingUp className="w-5 h-5" />
            </div>
          </div>
          <div className="mt-3">
            <span className="text-3xl font-extrabold text-foreground">
              {formatBytes(data.total_ingest_today)}
            </span>
          </div>
        </div>

        <div className="p-5 rounded-xl border border-border bg-card shadow-sm hover:border-rose-500/40 transition-colors">
          <div className="flex items-center justify-between text-muted-foreground">
            <span className="text-xs font-medium uppercase tracking-wider">Banned Tenants</span>
            <div className="p-2 rounded-lg bg-rose-500/10 text-rose-400">
              <Users className="w-5 h-5" />
            </div>
          </div>
          <div className="mt-3">
            <span className="text-3xl font-extrabold text-foreground">{data.banned_tenants}</span>
            <span className="text-xs text-muted-foreground ml-2">restricted</span>
          </div>
        </div>
      </div>

      {/* Visual Charts Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Ingestion Trend Area Chart */}
        <div className="lg:col-span-2 p-5 rounded-xl border border-border bg-card shadow-sm flex flex-col justify-between">
          <div className="flex items-center justify-between mb-4">
            <div>
              <h2 className="text-base font-bold text-foreground">7-Day Ingestion Trend</h2>
              <p className="text-xs text-muted-foreground">Volume of log data processed daily across all tenants</p>
            </div>
          </div>
          {chartData.length === 0 ? (
            <div className="h-64 flex items-center justify-center text-sm text-muted-foreground">
              No ingestion data available yet.
            </div>
          ) : (
            <div className="h-64 w-full">
              <ResponsiveContainer width="100%" height="100%">
                <AreaChart data={chartData} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
                  <defs>
                    <linearGradient id="colorLogs" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.4} />
                      <stop offset="95%" stopColor="#3b82f6" stopOpacity={0.0} />
                    </linearGradient>
                  </defs>
                  <XAxis dataKey="date" stroke="#6b7280" fontSize={11} tickLine={false} />
                  <YAxis stroke="#6b7280" fontSize={11} tickLine={false} />
                  <Tooltip
                    contentStyle={{
                      backgroundColor: "#18181b",
                      borderColor: "#27272a",
                      borderRadius: "8px",
                      fontSize: "12px",
                      color: "#fff",
                    }}
                  />
                  <Area
                    type="monotone"
                    dataKey="Logs (MB)"
                    stroke="#3b82f6"
                    strokeWidth={2}
                    fillOpacity={1}
                    fill="url(#colorLogs)"
                  />
                </AreaChart>
              </ResponsiveContainer>
            </div>
          )}
        </div>

        {/* Plan Distribution Donut Chart */}
        <div className="p-5 rounded-xl border border-border bg-card shadow-sm flex flex-col justify-between">
          <div>
            <h2 className="text-base font-bold text-foreground">Pack Distribution</h2>
            <p className="text-xs text-muted-foreground">Breakdown of active tenant subscriptions</p>
          </div>
          {data.plan_distribution.length === 0 ? (
            <div className="h-64 flex items-center justify-center text-sm text-muted-foreground">
              No subscription plan data.
            </div>
          ) : (
            <div className="h-64 w-full flex items-center justify-center">
              <ResponsiveContainer width="100%" height="100%">
                <PieChart>
                  <Pie
                    data={data.plan_distribution}
                    dataKey="count"
                    nameKey="plan_name"
                    cx="50%"
                    cy="50%"
                    innerRadius={50}
                    outerRadius={80}
                    paddingAngle={4}
                  >
                    {data.plan_distribution.map((_, index) => (
                      <Cell key={`cell-${index}`} fill={PIE_COLORS[index % PIE_COLORS.length]} />
                    ))}
                  </Pie>
                  <Tooltip
                    contentStyle={{
                      backgroundColor: "#18181b",
                      borderColor: "#27272a",
                      borderRadius: "8px",
                      fontSize: "12px",
                    }}
                  />
                  <Legend verticalAlign="bottom" height={36} iconType="circle" />
                </PieChart>
              </ResponsiveContainer>
            </div>
          )}
        </div>
      </div>

      {/* Quick Action Navigation Grid */}
      <div className="p-5 rounded-xl border border-border bg-card shadow-sm">
        <h2 className="text-base font-bold text-foreground mb-4">Quick Management Actions</h2>
        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 gap-4">
          <button
            onClick={() => navigate("/subscriptions")}
            className="p-4 rounded-lg bg-accent/40 hover:bg-accent border border-border/50 text-left transition-colors group flex flex-col justify-between"
          >
            <CreditCard className="w-5 h-5 text-amber-400 mb-2 group-hover:scale-110 transition-transform" />
            <div>
              <div className="font-semibold text-foreground text-sm">Subscription Demands</div>
              <div className="text-xs text-muted-foreground mt-0.5">Review & approve pack upgrades</div>
            </div>
          </button>

          <button
            onClick={() => navigate("/tenants")}
            className="p-4 rounded-lg bg-accent/40 hover:bg-accent border border-border/50 text-left transition-colors group flex flex-col justify-between"
          >
            <Building2 className="w-5 h-5 text-emerald-400 mb-2 group-hover:scale-110 transition-transform" />
            <div>
              <div className="font-semibold text-foreground text-sm">Tenant Directory</div>
              <div className="text-xs text-muted-foreground mt-0.5">Ban/unban & direct pack assign</div>
            </div>
          </button>

          <button
            onClick={() => navigate("/packs")}
            className="p-4 rounded-lg bg-accent/40 hover:bg-accent border border-border/50 text-left transition-colors group flex flex-col justify-between"
          >
            <Package className="w-5 h-5 text-purple-400 mb-2 group-hover:scale-110 transition-transform" />
            <div>
              <div className="font-semibold text-foreground text-sm">Manage Packs</div>
              <div className="text-xs text-muted-foreground mt-0.5">Configure pricing & host limits</div>
            </div>
          </button>

          <button
            onClick={() => navigate("/settings")}
            className="p-4 rounded-lg bg-accent/40 hover:bg-accent border border-border/50 text-left transition-colors group flex flex-col justify-between"
          >
            <Activity className="w-5 h-5 text-sky-400 mb-2 group-hover:scale-110 transition-transform" />
            <div>
              <div className="font-semibold text-foreground text-sm">Admin Settings</div>
              <div className="text-xs text-muted-foreground mt-0.5">Credentials & system parameters</div>
            </div>
          </button>
        </div>
      </div>
    </div>
  )
}
