import {
  Area,
  AreaChart,
  CartesianGrid,
  Legend,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts"
import { ChartBox } from "@/components/shared/ChartBox"

interface AreaConfig {
  dataKey: string
  color: string
  name: string
}

const chartStyle = {
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

export function StackedAreaCard({
  title,
  data,
  areas,
  loading,
  empty,
  hint,
}: {
  title: string
  data: Array<Record<string, string | number>>
  areas: AreaConfig[]
  loading?: boolean
  empty?: boolean
  hint?: string
}) {
  return (
    <ChartBox title={title} loading={loading} empty={empty || data.length === 0} hint={hint}>
      <div className="h-64">
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart data={data}>
            <CartesianGrid stroke={chartStyle.grid} strokeDasharray="3 3" />
            <XAxis dataKey="time" tick={chartStyle.tick} tickLine={false} axisLine={false} />
            <YAxis tick={chartStyle.tick} tickLine={false} axisLine={false} width={50} />
            <Tooltip contentStyle={chartStyle.tooltip} />
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
                fillOpacity={0.3}
                strokeWidth={1.5}
                type="monotone"
                stackId="1"
              />
            ))}
          </AreaChart>
        </ResponsiveContainer>
      </div>
    </ChartBox>
  )
}
