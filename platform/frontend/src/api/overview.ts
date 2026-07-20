import client from "@/api/client"

export type GlobalHealth = {
  hosts_online: number
  hosts_total: number
  services_healthy: number
  services_degraded: number
  services_down: number
  active_signals: number
  open_findings: number
}

export type ServiceChip = {
  name: string
  status: string
  port?: number
}

export type HostOverview = {
  machine_id: string
  hostname: string
  os: string
  status: string
  last_seen: string
  cpu_pct?: number
  mem_pct?: number
  disk_pct?: number
  summary?: string
  services: ServiceChip[]
}

export type TopIssue = {
  machine_id: string
  hostname: string
  service: string
  issue: string
  value: string
  severity: string
  age: string
}

export type EvidenceItem = {
  metric: string
  value: string
  status: string
}

export type Finding = {
  id: number
  rule_id: string
  severity: string
  title: string
  message: string
  evidence: EvidenceItem[]
  fired_at: string
  hostname: string
}

export type TimelineEvent = {
  timestamp: string
  hostname: string
  service: string
  event_type: string
  message: string
}

export type TimelineData = {
  events: TimelineEvent[]
}

export type DepNode = {
  id: string
  label: string
  status: string
  type: string
}

export type DepEdge = {
  source: string
  target: string
  status: string
}

export type DepGraphData = {
  nodes: DepNode[]
  edges: DepEdge[]
}

export type OverviewResponse = {
  global: GlobalHealth
  hosts: HostOverview[]
  top_issues: TopIssue[]
  findings: Finding[]
  timeline: TimelineData
  dep_graph: DepGraphData
}

export async function getOverview(): Promise<OverviewResponse> {
  const { data } = await client.get<OverviewResponse>("/api/overview")
  return data
}
