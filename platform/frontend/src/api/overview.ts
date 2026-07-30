import client from "@/api/client"

export type GlobalHealth = {
  hosts_online: number
  hosts_total: number
  services_healthy: number
  services_degraded: number
  services_down: number
  active_signals: number
  active_critical?: number
  active_warning?: number
  containers_healthy?: number
  containers_total?: number
  cve_exposure?: number
  open_findings?: number
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

export type InvestigationLink = {
  label: string
  url: string
}

export type EvidenceItem = {
  metric: string
  value: string
  status: string
}

export type DiagnosedIssue = {
  id: string
  machine_id: string
  hostname: string
  service: string
  severity: string
  diagnosis: string
  what: string
  why: string
  impact: string
  action: string
  evidence: EvidenceItem[]
  log_snippet?: string
  fired_at: string
  duration: string
}

export type Finding = {
  id: number
  rule_id: string
  severity: string
  title: string
  message: string
  evidence: EvidenceItem[]
  recommended_action?: string
  dashboard_link?: string
  service_name?: string
  investigation_links?: InvestigationLink[]
  hostname: string
  fired_at: string
  duration: string
}

export type IncidentMember = {
  rule_id: number
  template_id: string
  rule_name: string
  service_name?: string
  domain?: string
  pool?: string
  role: "root" | "symptom" | "standalone"
  severity: string
  actual_value?: number
  status: string
  since: string
  resolved_at?: string
  investigate_url: string
}

export type IncidentCard = {
  id: string
  host_machine_id: string
  hostname: string
  category: "performance" | "security"
  case_id: number
  status: string
  title: string
  narrative: string
  severity: string
  members: IncidentMember[]
  firing_since: string
  resolved_at?: string
  enrichments?: string[]
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

export type OverviewResponse = {
  global: GlobalHealth
  incidents?: IncidentCard[]
  hosts: HostOverview[]
  issues?: DiagnosedIssue[]
  findings?: Finding[]
  timeline: TimelineData
}

export async function dismissIncident(id: string): Promise<void> {
  await client.post(`/api/incidents/${encodeURIComponent(id)}/dismiss`)
}

export async function getOverview(timelineStart?: number, timelineEnd?: number): Promise<OverviewResponse> {
  const params: Record<string, string> = {}
  if (timelineStart !== undefined) params.timeline_start = String(timelineStart)
  if (timelineEnd !== undefined) params.timeline_end = String(timelineEnd)
  const { data } = await client.get<OverviewResponse>("/api/overview", { params })
  return data
}
