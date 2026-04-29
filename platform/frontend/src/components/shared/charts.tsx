/**
 * Reusable chart components for dashboards
 */

import {
  ResponsiveContainer,
  LineChart,
  BarChart,
  PieChart,
  Line,
  Bar,
  Pie,
  Cell,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
} from 'recharts'

export interface ChartProps {
  title: string
  loading?: boolean
  error?: string | null
  isEmpty?: boolean
  emptyMessage?: string
}

/**
 * Generic chart error state container
 */
export function ChartError({ title, message }: { title: string; message: string }) {
  return (
    <div className="rounded-lg border border-red-200 bg-red-50 dark:border-red-900 dark:bg-red-950/20 p-4 h-[270px] flex flex-col">
      <h3 className="mb-2 font-semibold text-sm">{title}</h3>
      <p className="text-xs text-red-600 dark:text-red-400">{message}</p>
    </div>
  )
}

/**
 * Generic chart loading skeleton
 */
export function ChartSkeleton({ title }: { title: string }) {
  return (
    <div className="rounded-lg border border-border bg-card p-4 h-[270px] flex flex-col">
      <h3 className="mb-4 font-semibold text-sm">{title}</h3>
      <div className="flex-1 flex items-center justify-center">
        <div className="text-sm text-muted-foreground animate-pulse">Loading...</div>
      </div>
    </div>
  )
}

/**
 * Generic empty state for charts
 */
export function ChartEmpty({ title, hint }: { title: string; hint: string }) {
  return (
    <div className="rounded-lg border border-border bg-card p-4 h-[270px] flex flex-col">
      <h3 className="mb-4 font-semibold text-sm">{title}</h3>
      <div className="flex-1 flex items-center justify-center">
        <p className="text-sm text-muted-foreground text-center">{hint}</p>
      </div>
    </div>
  )
}

/**
 * Response time distribution chart (P50/P95/P99)
 * Requires rt= field in nginx logs
 */
export interface ResponseTimeData {
  time: string
  p50: number
  p95: number
  p99: number
}

export function ResponseTimeChart({ data, loading }: { data: ResponseTimeData[], loading?: boolean }) {
  if (loading) return <ChartSkeleton title="Response Time (ms)" />
  if (!data?.length) {
    return <ChartEmpty 
      title="Response Time Distribution" 
      hint="Enable rt= field in nginx.conf to see response times: log_format platform '$remote_addr [...] rt=$request_time';"
    />
  }

  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <h3 className="mb-4 font-semibold text-sm">Response Time Distribution (P50/P95/P99)</h3>
      <ResponsiveContainer width="100%" height={250}>
        <LineChart data={data}>
          <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" opacity={0.5} />
          <XAxis dataKey="time" tick={{ fontSize: 10 }} tickLine={false} />
          <YAxis tick={{ fontSize: 10 }} tickLine={false} axisLine={false} label={{ value: 'ms', angle: -90, position: 'insideLeft' }} />
          <Tooltip
            formatter={(value) => typeof value === 'number' ? `${value.toFixed(1)} ms` : `${value} ms`}
            contentStyle={{ background: 'hsl(var(--card))', border: '1px solid hsl(var(--border))', borderRadius: 8 }}
          />
          <Legend wrapperStyle={{ fontSize: 11 }} />
          <Line type="monotone" dataKey="p50" stroke="#3b82f6" strokeWidth={2} dot={false} name="P50 (Median)" />
          <Line type="monotone" dataKey="p95" stroke="#f59e0b" strokeWidth={2} dot={false} name="P95" />
          <Line type="monotone" dataKey="p99" stroke="#ef4444" strokeWidth={2} dot={false} name="P99 (Worst)" />
        </LineChart>
      </ResponsiveContainer>
    </div>
  )
}

/**
 * Bandwidth served over time chart
 */
export interface BandwidthData {
  time: string
  mb_per_sec: number
}

export function BandwidthChart({ data, loading }: { data: BandwidthData[], loading?: boolean }) {
  if (loading) return <ChartSkeleton title="Bandwidth Served (MB/s)" />
  if (!data?.length) {
    return <ChartEmpty title="Bandwidth Served (MB/s)" hint="No bandwidth data" />
  }

  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <h3 className="mb-4 font-semibold text-sm">Bandwidth Served (MB/s)</h3>
      <ResponsiveContainer width="100%" height={250}>
        <BarChart data={data}>
          <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" opacity={0.5} />
          <XAxis dataKey="time" tick={{ fontSize: 10 }} tickLine={false} />
          <YAxis tick={{ fontSize: 10 }} tickLine={false} axisLine={false} label={{ value: 'MB/s' }} />
          <Tooltip
            formatter={(value) => typeof value === 'number' ? `${value.toFixed(2)} MB/s` : `${value} MB/s`}
            contentStyle={{ background: 'hsl(var(--card))', border: '1px solid hsl(var(--border))', borderRadius: 8 }}
          />
          <Bar dataKey="mb_per_sec" fill="hsl(var(--primary))" radius={[4, 4, 0, 0]} />
        </BarChart>
      </ResponsiveContainer>
    </div>
  )
}

/**
 * HTTP status breakdown pie chart
 */
export interface StatusBreakdownData {
  name: string
  value: number
  status: number
}

const STATUS_COLORS = {
  2: '#22c55e', // 2xx green
  3: '#eab308', // 3xx yellow
  4: '#f97316', // 4xx orange
  5: '#ef4444', // 5xx red
}

export function StatusBreakdownPie({
  data,
  loading,
}: {
  data: StatusBreakdownData[]
  loading?: boolean
}) {
  if (loading) return <ChartSkeleton title="Status Code Distribution" />
  if (!data?.length) {
    return <ChartEmpty title="Status Code Distribution" hint="No status data" />
  }

  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <h3 className="mb-4 font-semibold text-sm">Status Code Distribution</h3>
      <ResponsiveContainer width="100%" height={250}>
        <PieChart>
          <Pie
            data={data}
            cx="50%"
            cy="50%"
            labelLine={false}
            label={({ name, percent = 0 }) => `${name}: ${(percent * 100).toFixed(0)}%`}
            outerRadius={80}
            fill="#8884d8"
            dataKey="value"
          >
            {data.map((entry, index) => (
              <Cell
                key={`cell-${index}`}
                fill={STATUS_COLORS[entry.status as keyof typeof STATUS_COLORS] || '#999'}
              />
            ))}
          </Pie>
          <Tooltip formatter={(v) => `${v} requests`} />
        </PieChart>
      </ResponsiveContainer>
    </div>
  )
}

/**
 * HTTP method distribution chart
 */
export interface MethodData {
  time: string
  get?: number
  post?: number
  put?: number
  delete?: number
  patch?: number
}

export function MethodBreakdownChart({ data, loading }: { data: MethodData[], loading?: boolean }) {
  if (loading) return <ChartSkeleton title="Requests by HTTP Method" />
  if (!data?.length) {
    return <ChartEmpty title="Requests by HTTP Method" hint="No method data" />
  }

  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <h3 className="mb-4 font-semibold text-sm">Requests by HTTP Method</h3>
      <ResponsiveContainer width="100%" height={250}>
        <LineChart data={data}>
          <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" opacity={0.5} />
          <XAxis dataKey="time" tick={{ fontSize: 10 }} tickLine={false} />
          <YAxis tick={{ fontSize: 10 }} tickLine={false} axisLine={false} />
          <Tooltip
            contentStyle={{ background: 'hsl(var(--card))', border: '1px solid hsl(var(--border))', borderRadius: 8 }}
          />
          <Legend wrapperStyle={{ fontSize: 11 }} />
          <Line type="stepAfter" dataKey="get" stroke="#3b82f6" strokeWidth={2} dot={false} name="GET" />
          <Line type="stepAfter" dataKey="post" stroke="#8b5cf6" strokeWidth={2} dot={false} name="POST" />
          <Line type="stepAfter" dataKey="put" stroke="#f59e0b" strokeWidth={2} dot={false} name="PUT" />
          <Line type="stepAfter" dataKey="delete" stroke="#ef4444" strokeWidth={2} dot={false} name="DELETE" />
          <Line type="stepAfter" dataKey="patch" stroke="#06b6d4" strokeWidth={2} dot={false} name="PATCH" />
        </LineChart>
      </ResponsiveContainer>
    </div>
  )
}

/**
 * Connection state distribution (from nginx_status)
 */
export interface ConnectionData {
  time: string
  active: number
  reading: number
  writing: number
  waiting: number
}

export function ConnectionChart({ data, loading }: { data: ConnectionData[], loading?: boolean }) {
  if (loading) return <ChartSkeleton title="Active Connections" />
  if (!data?.length) {
    return <ChartEmpty title="Active Connections" hint="nginx_status metrics unavailable" />
  }

  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <h3 className="mb-4 font-semibold text-sm">Active Connections (Active/Reading/Writing/Waiting)</h3>
      <ResponsiveContainer width="100%" height={250}>
        <LineChart data={data}>
          <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" opacity={0.5} />
          <XAxis dataKey="time" tick={{ fontSize: 10 }} tickLine={false} />
          <YAxis tick={{ fontSize: 10 }} tickLine={false} axisLine={false} />
          <Tooltip
            contentStyle={{ background: 'hsl(var(--card))', border: '1px solid hsl(var(--border))', borderRadius: 8 }}
          />
          <Legend wrapperStyle={{ fontSize: 11 }} />
          <Line type="monotone" dataKey="active" stroke="#ef4444" strokeWidth={2.5} dot={false} name="Active" isAnimationActive={false} />
          <Line type="monotone" dataKey="reading" stroke="#3b82f6" strokeWidth={2} dot={false} name="Reading" isAnimationActive={false} />
          <Line type="monotone" dataKey="writing" stroke="#f59e0b" strokeWidth={2} dot={false} name="Writing" isAnimationActive={false} />
          <Line type="monotone" dataKey="waiting" stroke="#10b981" strokeWidth={2} dot={false} name="Waiting" isAnimationActive={false} />
        </LineChart>
      </ResponsiveContainer>
    </div>
  )
}
