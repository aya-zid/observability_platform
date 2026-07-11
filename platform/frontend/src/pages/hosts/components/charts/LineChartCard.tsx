import {
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts"
import { ChartBox } from "@/components/shared/ChartBox"

interface LineConfig {
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

export function LineChartCard({
  title,
  data,
  lines,
  loading,
  empty,
  hint,
  yAxisLabel,
  referenceValue,
}: {
  title: string
  data: Array<Record<string, string | number>>
  lines: LineConfig[]
  loading?: boolean
  empty?: boolean
  hint?: string
  yAxisLabel?: string
  referenceValue?: number
}) {
  return (
    <ChartBox title={title} loading={loading} empty={empty || data.length === 0} hint={hint}>
      <div className="h-64">
        <ResponsiveContainer width="100%" height="100%">
          <LineChart data={data}>
            <CartesianGrid stroke={chartStyle.grid} strokeDasharray="3 3" />
            <XAxis dataKey="time" tick={chartStyle.tick} tickLine={false} axisLine={false} />
            <YAxis
              tick={chartStyle.tick}
              tickLine={false}
              axisLine={false}
              width={50}
              label={yAxisLabel ? { value: yAxisLabel, angle: -90, position: "insideLeft", style: { fill: "#94a3b8", fontSize: 10 } } : undefined}
            />
            <Tooltip contentStyle={chartStyle.tooltip} />
            <Legend wrapperStyle={{ fontSize: "11px" }} />
            {referenceValue != null && (
              <ReferenceLine
                y={referenceValue}
                stroke="#ef4444"
                strokeDasharray="4 4"
                strokeWidth={1}
                label={{ value: `ref: ${referenceValue}`, position: "right", fill: "#ef4444", fontSize: 10 }}
              />
            )}
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
    </ChartBox>
  )
}
