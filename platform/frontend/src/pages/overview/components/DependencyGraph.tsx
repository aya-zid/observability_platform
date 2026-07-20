import type { DepGraphData, DepNode, DepEdge } from "@/api/overview"
import { cn } from "@/lib/utils"

const NODE_RADIUS = 22
const COLORS: Record<string, string> = {
  host: "#6b7280",
  web: "#3b82f6",
  app: "#8b5cf6",
  db: "#06b6d4",
  cache: "#f59e0b",
}

const STATUS_COLORS: Record<string, string> = {
  healthy: "#22c55e",
  degraded: "#f59e0b",
  down: "#ef4444",
  absent: "#9ca3af",
}

const STATUS_BG: Record<string, string> = {
  healthy: "border-green-400 dark:border-green-600",
  degraded: "border-amber-400 dark:border-amber-600",
  down: "border-red-400 dark:border-red-600",
  absent: "border-gray-300 dark:border-gray-600",
}

const TYPE_LABELS: Record<string, string> = {
  host: "Host",
  web: "Web",
  app: "App",
  db: "Database",
  cache: "Cache",
}

function buildLayout(nodes: DepNode[]): { nodes: Array<DepNode & { x: number; y: number }>; edges: DepEdge[] } {
  const byType: Record<string, DepNode[]> = {}
  for (const n of nodes) {
    if (!byType[n.type]) byType[n.type] = []
    byType[n.type].push(n)
  }

  const typeOrder = ["host", "web", "app", "db", "cache"]
  const positioned: Array<DepNode & { x: number; y: number }> = []
  const spacingX = 140
  const spacingY = 60
  const startX = 60

  for (let ti = 0; ti < typeOrder.length; ti++) {
    const t = typeOrder[ti]
    const group = byType[t]
    if (!group) continue
    const totalH = group.length * spacingY
    const startY = Math.max(60, 140 - totalH / 2)
    for (let gi = 0; gi < group.length; gi++) {
      positioned.push({
        ...group[gi],
        x: startX + ti * spacingX,
        y: startY + gi * spacingY,
      })
    }
  }

  return { nodes: positioned, edges: [] }
}

// eslint-disable-next-line react-refresh/only-export-components
export function DependencyGraph({ data }: { data: DepGraphData | null | undefined }) {
  if (!data || !data.nodes || data.nodes.length === 0) {
    return (
      <div className="rounded-lg border border-border bg-card/50 p-8 text-center text-sm text-muted-foreground">
        No hosts or services to display
      </div>
    )
  }

  const layout = buildLayout(data.nodes)
  const nodeMap = new Map(layout.nodes.map((n) => [n.id, n]))

  return (
    <div className="w-full overflow-x-auto">
      <svg
        viewBox="0 0 680 280"
        className="w-full max-w-[680px] h-auto"
        xmlns="http://www.w3.org/2000/svg"
      >
        {/* Edges */}
        {(data.edges ?? []).map((edge, i) => {
          const src = nodeMap.get(edge.source)
          const tgt = nodeMap.get(edge.target)
          if (!src || !tgt) return null
          const edgeColor = STATUS_COLORS[edge.status] ?? "#9ca3af"
          return (
            <line
              key={`edge-${i}`}
              x1={src.x}
              y1={src.y}
              x2={tgt.x}
              y2={tgt.y}
              stroke={edgeColor}
              strokeWidth={2}
              strokeOpacity={0.6}
              strokeDasharray={edge.status === "absent" ? "4 4" : undefined}
            />
          )
        })}

        {/* Nodes */}
        {layout.nodes.map((node) => {
          const fill = COLORS[node.type] ?? "#6b7280"
          const stroke = STATUS_COLORS[node.status] ?? "#22c55e"

          return (
            <g key={node.id}>
              {/* Outer ring (status indicator) */}
              <circle
                cx={node.x}
                cy={node.y}
                r={NODE_RADIUS + 3}
                fill="none"
                stroke={stroke}
                strokeWidth={2.5}
                strokeOpacity={0.7}
              />
              {/* Inner circle (type color) */}
              <circle
                cx={node.x}
                cy={node.y}
                r={NODE_RADIUS}
                fill={fill}
                fillOpacity={0.15}
                stroke={fill}
                strokeWidth={1.5}
              />
              {/* Type icon letter */}
              <text
                x={node.x}
                y={node.y + 1}
                textAnchor="middle"
                dominantBaseline="central"
                fill={fill}
                fontSize={14}
                fontWeight={600}
              >
                {TYPE_LABELS[node.type]?.charAt(0) ?? "?"}
              </text>
              {/* Label below */}
              <text
                x={node.x}
                y={node.y + NODE_RADIUS + 16}
                textAnchor="middle"
                fill="currentColor"
                className="fill-current text-[10px] text-muted-foreground"
              >
                {node.label.length > 8 ? node.label.slice(0, 8) + "…" : node.label}
              </text>
            </g>
          )
        })}

        {/* Column type labels */}
        {["host", "web", "app", "db", "cache"].map((type, idx) => {
          const hasType = layout.nodes.some((n) => n.type === type)
          if (!hasType) return null
          return (
            <text
              key={`label-${type}`}
              x={60 + idx * 140}
              y={20}
              textAnchor="middle"
              className="fill-current text-[10px] text-muted-foreground"
            >
              {TYPE_LABELS[type]}
            </text>
          )
        })}
      </svg>
    </div>
  )
}
