import { CartesianGrid, Legend, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { formatBytes, formatNumber } from '@/lib/queryHelpers'
import { DASHBOARD_CHART_STYLE as chartStyle } from '@/components/shared/DashboardComponents'
import { StatCard } from '@/components/shared/StatCard'
import { ChartBox } from '@/components/shared/ChartBox'

interface Props {
  isLoading: boolean
  uptimeSeconds: number | null
  cpuLoadPercent: number | null
  requestsLatest: number | null
  trafficBytesTotal: number | null
  busyWorkers: number | null
  idleWorkers: number | null
  load1: number | null
  load5: number | null
  workersData: Record<string, number | string>[]
  trafficData: Record<string, number | string>[]
  loadData: Record<string, number | string>[]
  requestsData: Record<string, number | string>[]
  formatUptime: (seconds: number | null) => string
}

export function ApacheServerView({
  isLoading,
  uptimeSeconds,
  cpuLoadPercent,
  requestsLatest,
  trafficBytesTotal,
  busyWorkers,
  idleWorkers,
  load1,
  load5,
  workersData,
  trafficData,
  loadData,
  requestsData,
  formatUptime,
}: Props) {
  return (
    <>
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-4">
        <StatCard
          title="Uptime"
          value={formatUptime(uptimeSeconds)}
          loading={isLoading}
        />
        <StatCard
          title="CPU Load"
          value={cpuLoadPercent !== null ? `${Number(cpuLoadPercent.toFixed(1))}%` : '—'}
          loading={isLoading}
        />
        <StatCard
          title="Total Requests"
          value={requestsLatest !== null ? formatNumber(requestsLatest) : '—'}
          loading={isLoading}
        />
        <StatCard
          title="Total Traffic"
          value={trafficBytesTotal !== null ? formatBytes(trafficBytesTotal) : '—'}
          loading={isLoading}
        />
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          title="Busy Workers"
          value={busyWorkers !== null ? formatNumber(busyWorkers) : '—'}
          loading={isLoading}
          status={busyWorkers !== null && idleWorkers !== null && busyWorkers > idleWorkers ? 'red' : undefined}
        />
        <StatCard
          title="Idle Workers"
          value={idleWorkers !== null ? formatNumber(idleWorkers) : '—'}
          loading={isLoading}
        />
        <StatCard
          title="Load (1m)"
          value={load1 !== null ? Number(load1.toFixed(2)).toString() : '—'}
          loading={isLoading}
        />
        <StatCard
          title="Load (5m)"
          value={load5 !== null ? Number(load5.toFixed(2)).toString() : '—'}
          loading={isLoading}
        />
      </div>

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <ChartBox
          title="Workers (Busy / Idle)"
          loading={isLoading}
          empty={workersData.length === 0}
          hint="Apache worker processes over time"
        >
          <div className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={workersData}>
                <CartesianGrid stroke={chartStyle.grid} strokeDasharray="3 3" />
                <XAxis dataKey="time" tick={chartStyle.tick} tickLine={false} axisLine={false} />
                <YAxis tick={chartStyle.tick} tickLine={false} axisLine={false} width={40} />
                <Tooltip contentStyle={chartStyle.tooltip} />
                <Legend wrapperStyle={{ fontSize: '11px' }} />
                <Line dataKey="busy" dot={false} isAnimationActive={false} name="Busy" stroke="#ef4444" strokeWidth={2} type="monotone" />
                <Line dataKey="idle" dot={false} isAnimationActive={false} name="Idle" stroke="#22c55e" strokeWidth={2} type="monotone" />
              </LineChart>
            </ResponsiveContainer>
          </div>
        </ChartBox>

        <ChartBox
          title="Traffic"
          loading={isLoading}
          empty={trafficData.length === 0}
          hint="Total bytes served by Apache"
        >
          <div className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={trafficData}>
                <CartesianGrid stroke={chartStyle.grid} strokeDasharray="3 3" />
                <XAxis dataKey="time" tick={chartStyle.tick} tickLine={false} axisLine={false} />
                <YAxis tick={chartStyle.tick} tickLine={false} axisLine={false} width={50} tickFormatter={(v) => formatBytes(v)} />
                <Tooltip contentStyle={chartStyle.tooltip} formatter={(v) => (typeof v === 'number' ? formatBytes(v) : v)} />
                <Line dataKey="bytes" dot={false} isAnimationActive={false} name="Traffic" stroke="#3b82f6" strokeWidth={2} type="monotone" />
              </LineChart>
            </ResponsiveContainer>
          </div>
        </ChartBox>
      </div>

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <ChartBox
          title="Load Averages (1m / 5m / 15m)"
          loading={isLoading}
          empty={loadData.length === 0}
          hint="System load averages from Apache"
        >
          <div className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={loadData}>
                <CartesianGrid stroke={chartStyle.grid} strokeDasharray="3 3" />
                <XAxis dataKey="time" tick={chartStyle.tick} tickLine={false} axisLine={false} />
                <YAxis tick={chartStyle.tick} tickLine={false} axisLine={false} width={40} />
                <Tooltip contentStyle={chartStyle.tooltip} />
                <Legend wrapperStyle={{ fontSize: '11px' }} />
                <Line dataKey="load1" dot={false} isAnimationActive={false} name="1m" stroke="#ef4444" strokeWidth={2} type="monotone" />
                <Line dataKey="load5" dot={false} isAnimationActive={false} name="5m" stroke="#f59e0b" strokeWidth={1.5} type="monotone" />
                <Line dataKey="load15" dot={false} isAnimationActive={false} name="15m" stroke="#22c55e" strokeWidth={1.5} type="monotone" />
              </LineChart>
            </ResponsiveContainer>
          </div>
        </ChartBox>

        <ChartBox
          title="Total Requests Over Time"
          loading={isLoading}
          empty={requestsData.length === 0}
          hint="Cumulative requests served by Apache"
        >
          <div className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={requestsData}>
                <CartesianGrid stroke={chartStyle.grid} strokeDasharray="3 3" />
                <XAxis dataKey="time" tick={chartStyle.tick} tickLine={false} axisLine={false} />
                <YAxis tick={chartStyle.tick} tickLine={false} axisLine={false} width={50} tickFormatter={(v) => formatNumber(v)} />
                <Tooltip contentStyle={chartStyle.tooltip} formatter={(v) => (typeof v === 'number' ? formatNumber(v) : v)} />
                <Line dataKey="requests" dot={false} isAnimationActive={false} name="Requests" stroke="#22c55e" strokeWidth={2} type="monotone" />
              </LineChart>
            </ResponsiveContainer>
          </div>
        </ChartBox>
      </div>
    </>
  )
}
