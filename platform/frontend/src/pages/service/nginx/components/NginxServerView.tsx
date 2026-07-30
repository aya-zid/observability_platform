import {
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'

import { ChartBox } from '@/components/shared/ChartBox'
import { StatCard } from '@/components/shared/StatCard'
import { formatNumber } from '@/lib/queryHelpers'

const chartStyle = {
  grid: 'hsl(var(--border))',
  tick: { fill: '#94a3b8', fontSize: 10 },
  tooltip: {
    background: 'hsl(var(--card))',
    border: '1px solid hsl(var(--border))',
    borderRadius: '6px',
    padding: '8px',
    fontSize: '12px',
    color: 'hsl(var(--foreground))',
  },
}

interface NginxServerViewProps {
  activeConnectionsLatest: number | null
  totalConnectionsLatest: number | null
  readingConnectionsLatest: number | null
  writingConnectionsLatest: number | null
  waitingConnectionsLatest: number | null
  acceptedLatest: number | null
  handledLatest: number | null
  requestsLatest: number | null
  connData: Record<string, number | string>[]
  totalConnData: Record<string, number | string>[]
  requestsData: Record<string, number | string>[]
  isLoading: boolean
}

export function NginxServerView({
  activeConnectionsLatest,
  totalConnectionsLatest,
  readingConnectionsLatest,
  writingConnectionsLatest,
  waitingConnectionsLatest,
  acceptedLatest,
  handledLatest,
  requestsLatest,
  connData,
  totalConnData,
  requestsData,
  isLoading,
}: NginxServerViewProps) {
  return (
    <>
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <StatCard
          title="Active Connections"
          value={activeConnectionsLatest !== null ? formatNumber(activeConnectionsLatest) : '—'}
          loading={isLoading}
        />
        <StatCard
          title="Total Connections"
          value={totalConnectionsLatest !== null ? formatNumber(totalConnectionsLatest) : '—'}
          loading={isLoading}
        />
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <StatCard
          title="Reading"
          value={readingConnectionsLatest !== null ? formatNumber(readingConnectionsLatest) : '—'}
          loading={isLoading}
        />
        <StatCard
          title="Writing"
          value={writingConnectionsLatest !== null ? formatNumber(writingConnectionsLatest) : '—'}
          loading={isLoading}
        />
        <StatCard
          title="Waiting"
          value={waitingConnectionsLatest !== null ? formatNumber(waitingConnectionsLatest) : '—'}
          loading={isLoading}
        />
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <StatCard
          title="Accepted"
          value={acceptedLatest !== null ? formatNumber(acceptedLatest) : '—'}
          loading={isLoading}
        />
        <StatCard
          title="Handled"
          value={handledLatest !== null ? formatNumber(handledLatest) : '—'}
          loading={isLoading}
        />
        <StatCard
          title="Total Requests"
          value={requestsLatest !== null ? formatNumber(requestsLatest) : '—'}
          loading={isLoading}
        />
      </div>

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <ChartBox
          title="Active Connections"
          loading={isLoading}
          empty={connData.length === 0}
          hint="Requires the configured nginx status endpoint to be enabled and scraped"
        >
          <div className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={connData}>
                <CartesianGrid stroke={chartStyle.grid} strokeDasharray="3 3" />
                <XAxis dataKey="time" tick={chartStyle.tick} tickLine={false} axisLine={false} />
                <YAxis tick={chartStyle.tick} tickLine={false} axisLine={false} width={40} />
                <Tooltip contentStyle={chartStyle.tooltip} />
                <Legend wrapperStyle={{ fontSize: '11px' }} />
                <Line dataKey="active" dot={false} isAnimationActive={false} name="Active" stroke="#2563eb" strokeWidth={2} type="monotone" />
                <Line dataKey="waiting" dot={false} isAnimationActive={false} name="Waiting" stroke="#94a3b8" strokeWidth={1.5} type="monotone" />
                <Line dataKey="reading" dot={false} isAnimationActive={false} name="Reading" stroke="#22c55e" strokeWidth={1.5} type="monotone" />
                <Line dataKey="writing" dot={false} isAnimationActive={false} name="Writing" stroke="#f59e0b" strokeWidth={1.5} type="monotone" />
              </LineChart>
            </ResponsiveContainer>
          </div>
        </ChartBox>

        <ChartBox
          title="Total Connections Over Time"
          loading={isLoading}
          empty={totalConnData.length === 0}
          hint="Sum of all nginx connection states"
        >
          <div className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={totalConnData}>
                <CartesianGrid stroke={chartStyle.grid} strokeDasharray="3 3" />
                <XAxis dataKey="time" tick={chartStyle.tick} tickLine={false} axisLine={false} />
                <YAxis tick={chartStyle.tick} tickLine={false} axisLine={false} width={40} />
                <Tooltip contentStyle={chartStyle.tooltip} />
                <Line dataKey="total" dot={false} isAnimationActive={false} name="Total" stroke="#2563eb" strokeWidth={2} type="monotone" />
              </LineChart>
            </ResponsiveContainer>
          </div>
        </ChartBox>
      </div>

      <ChartBox
        title="Total Requests Over Time"
        loading={isLoading}
        empty={requestsData.length === 0}
        hint="Cumulative requests served by nginx"
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
    </>
  )
}
