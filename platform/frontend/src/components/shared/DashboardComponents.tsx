import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  Pie,
  PieChart,
  Cell,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts"
import { cn } from "@/lib/utils"
import { useTranslation } from "@/i18n/LanguageContext"

export const DASHBOARD_COLORS = {
  blue: "#3b82f6",
  green: "#22c55e",
  red: "#ef4444",
  orange: "#f59e0b",
  purple: "#8b5cf6",
  pink: "#ec4899",
  cyan: "#06b6d4",
  lime: "#84cc16",
  amber: "#f97316",
  teal: "#14b8a6",
  emerald: "#10b981",
  slate: "#94a3b8",
  yellow: "#eab308",
  indigo: "#6366f1",
}

export const DASHBOARD_CHART_STYLE = {
  grid: "hsl(var(--border))",
  tick: { fill: "#94a3b8", fontSize: 10 },
  tooltip: {
    background: "hsl(var(--card))",
    border: "1px solid hsl(var(--border))",
    borderRadius: "6px",
    padding: "8px",
    fontSize: "12px",
    color: "hsl(var(--foreground))",
  },
}

export function DashboardStatCard({
  label,
  value,
  hint,
  tone = "neutral",
  icon,
}: {
  label: string
  value: string | number | React.ReactNode
  hint?: string
  tone?: "neutral" | "good" | "warn" | "bad"
  icon?: string
}) {
  const borderColor = {
    neutral: "border-border",
    good: "border-green-500/50",
    warn: "border-amber-500/50",
    bad: "border-red-500/50",
  }[tone]

  return (
    <div className={cn("rounded-lg border bg-card p-2.5", borderColor)}>
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="mt-0.5 text-sm font-semibold">{value}</p>
      {hint ? <p className="mt-0.5 text-xs text-muted-foreground">{hint}</p> : null}
      {icon ? <span className="text-base">{icon}</span> : null}
    </div>
  )
}

export function DashboardSectionHeader({
  title,
  description,
}: {
  title: string
  description?: string
}) {
  return (
    <div className="mb-3">
      <h3 className="text-lg font-semibold tracking-tight text-foreground">{title}</h3>
      {description ? <p className="mt-0.5 text-sm text-muted-foreground">{description}</p> : null}
    </div>
  )
}

export function DashboardNoData() {
  const { t } = useTranslation()
  return (
    <div className="flex h-48 items-center justify-center text-xs text-muted-foreground">
      {t("dashboard.noData")}
    </div>
  )
}

export function DashboardLinePanel({
  title,
  data,
  lines,
  formatter,
  height = 260,
  onClickTime,
}: {
  title: string
  data: Array<Record<string, string | number>>
  lines: Array<{ dataKey: string; color: string; name: string }>
  formatter?: (value: number, name: string) => string
  height?: number
  onClickTime?: (time: number) => void
}) {
  const hasData = data.length > 0
  return (
    <div className="rounded-lg border border-border bg-card p-3">
      <h3 className="mb-2 text-xs font-semibold text-foreground">{title}</h3>
      {!hasData ? (
        <div className="flex h-48 items-center justify-center text-xs text-muted-foreground">
          No data
        </div>
      ) : (
        <div style={{ height }}>
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={data} onClick={onClickTime ? (e) => e?.activeLabel && onClickTime(Number(e.activeLabel)) : undefined}>
              <CartesianGrid stroke={DASHBOARD_CHART_STYLE.grid} strokeDasharray="3 3" />
              <XAxis dataKey="time" tick={DASHBOARD_CHART_STYLE.tick} tickLine={false} axisLine={false} />
              <YAxis tick={DASHBOARD_CHART_STYLE.tick} tickLine={false} axisLine={false} width={50} />
              <Tooltip
                contentStyle={DASHBOARD_CHART_STYLE.tooltip}
                formatter={formatter ? ((val: number, name: string) => formatter(val, name)) as any : undefined}
              />
              <Legend wrapperStyle={{ fontSize: "11px" }} />
              {lines.map((l) => (
                <Line
                  key={l.dataKey}
                  dataKey={l.dataKey}
                  dot={false}
                  isAnimationActive={false}
                  name={l.name}
                  stroke={l.color}
                  strokeWidth={2}
                  type="monotone"
                />
              ))}
            </LineChart>
          </ResponsiveContainer>
        </div>
      )}
    </div>
  )
}

export function DashboardAreaPanel({
  title,
  data,
  areas,
  formatter,
  height = 260,
  stacked = true,
  onClickTime,
}: {
  title: string
  data: Array<Record<string, string | number>>
  areas: Array<{ dataKey: string; color: string; name: string }>
  formatter?: (value: number, name: string) => string
  height?: number
  stacked?: boolean
  onClickTime?: (time: number) => void
}) {
  const hasData = data.length > 0
  return (
    <div className="rounded-lg border border-border bg-card p-3">
      <h3 className="mb-2 text-xs font-semibold text-foreground">{title}</h3>
      {!hasData ? (
        <div className="flex h-48 items-center justify-center text-xs text-muted-foreground">
          No data
        </div>
      ) : (
        <div style={{ height }}>
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={data} onClick={onClickTime ? (e) => e?.activeLabel && onClickTime(Number(e.activeLabel)) : undefined}>
              <CartesianGrid stroke={DASHBOARD_CHART_STYLE.grid} strokeDasharray="3 3" />
              <XAxis dataKey="time" tick={DASHBOARD_CHART_STYLE.tick} tickLine={false} axisLine={false} />
              <YAxis tick={DASHBOARD_CHART_STYLE.tick} tickLine={false} axisLine={false} width={50} />
              <Tooltip
                contentStyle={DASHBOARD_CHART_STYLE.tooltip}
                formatter={formatter ? ((val: number, name: string) => formatter(val, name)) as any : undefined}
              />
              <Legend wrapperStyle={{ fontSize: "11px" }} />
              {areas.map((a) => (
                <Area
                  key={a.dataKey}
                  dataKey={a.dataKey}
                  dot={false}
                  isAnimationActive={false}
                  name={a.name}
                  stroke={a.color}
                  fill={a.color}
                  fillOpacity={0.2}
                  strokeWidth={1.5}
                  type="monotone"
                  stackId={stacked ? "1" : undefined}
                />
              ))}
            </AreaChart>
          </ResponsiveContainer>
        </div>
      )}
    </div>
  )
}

export function DashboardBarPanel({
  title,
  data,
  formatter,
  height = 260,
}: {
  title: string
  data: Array<{ name: string; value: number }>
  formatter?: (value: number) => string
  height?: number
}) {
  const hasData = data.length > 0
  return (
    <div className="rounded-lg border border-border bg-card p-3">
      <h3 className="mb-2 text-xs font-semibold text-foreground">{title}</h3>
      {!hasData ? (
        <div className="flex h-48 items-center justify-center text-xs text-muted-foreground">
          No data
        </div>
      ) : (
        <div style={{ height }}>
          <ResponsiveContainer width="100%" height="100%">
            <BarChart data={data} layout="vertical">
              <CartesianGrid stroke={DASHBOARD_CHART_STYLE.grid} strokeDasharray="3 3" />
              <XAxis type="number" tick={DASHBOARD_CHART_STYLE.tick} tickLine={false} axisLine={false} />
              <YAxis type="category" dataKey="name" tick={DASHBOARD_CHART_STYLE.tick} tickLine={false} axisLine={false} width={80} />
              <Tooltip
                contentStyle={DASHBOARD_CHART_STYLE.tooltip}
                formatter={formatter ? ((val: number) => formatter(val)) as any : undefined}
              />
              <Bar dataKey="value" fill={DASHBOARD_COLORS.blue} radius={[0, 4, 4, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}
    </div>
  )
}

export function DashboardDonutCard({
  title,
  value,
  label,
  color,
  hint,
}: {
  title: string
  value: number
  label: string
  color: string
  hint?: string
}) {
  const data = [{ name: label, value }, { name: "remaining", value: 100 - value }]
  return (
    <div className="rounded-lg border border-border bg-card p-2.5">
      <p className="mb-1 text-xs text-muted-foreground">{title}</p>
      <ResponsiveContainer width="100%" height={60}>
        <PieChart>
          <Pie data={data} cx="50%" cy="50%" innerRadius={16} outerRadius={28} dataKey="value" startAngle={90} endAngle={-270}>
            <Cell fill={color} />
            <Cell fill="hsl(var(--muted))" />
          </Pie>
        </PieChart>
      </ResponsiveContainer>
      <p className="text-center text-sm font-bold" style={{ color }}>{value}{hint || "%"}</p>
      <p className="text-center text-xs text-muted-foreground">{label}</p>
    </div>
  )
}

export function DashboardCustomBarPanel({
  title,
  data,
  formatter,
  height = 260,
}: {
  title: string
  data: Array<{ name: string; value: number; pct?: number }>
  formatter?: (value: number) => string
  height?: number
}) {
  const hasData = data.length > 0
  const maxVal = Math.max(...data.map((d) => d.value), 1)
  return (
    <div className="rounded-lg border border-border bg-card p-3">
      <h3 className="mb-2 text-xs font-semibold text-foreground">{title}</h3>
      {!hasData ? (
        <div className="flex items-center justify-center text-xs text-muted-foreground" style={{ height }}>
          No data
        </div>
      ) : (
        <div className="space-y-1.5" style={{ height }}>
          {data.map((d) => (
            <div key={d.name} className="flex items-center gap-1.5">
              <span className="w-20 truncate text-xs text-muted-foreground">{d.name}</span>
              <div className="flex-1 h-4 rounded bg-muted overflow-hidden">
                <div
                  className="h-full rounded bg-blue-500 transition-all"
                  style={{ width: `${(d.value / maxVal) * 100}%` }}
                />
              </div>
              <span className="w-14 text-right text-xs font-medium">
                {formatter ? formatter(d.value) : d.value}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
