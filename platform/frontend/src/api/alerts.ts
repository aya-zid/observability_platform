import client from "@/api/client"

export type VulnerabilityAlert = {
  id: number
  host_id: number
  hostname: string
  machine_id: string
  service_name: string
  service_version: string
  cve_id: string
  cvss_score: number
  severity: string
  alert_level: string
  description: string
  fixed_in_version: string
}

export type AlertTemplate = {
  id: string
  service_type: string
  name: string
  description: string
  explanation: string
  unit: string
  default_threshold: number
  comparison_op: string
  default_severity: string
  default_for_duration: string
}

export type AlertRuleScope = {
  id: number
  rule_id: number
  scope_type: string
  scope_id: number | null
}

export type AlertThreshold = {
  id: number
  rule_id: number
  label: string
  threshold: number
  severity: string
  for_duration: string
}

export type AlertRule = {
  id: number
  tenant_id: number
  template_id: string
  service_type: string
  template_name: string
  description: string
  explanation: string
  unit: string
  comparison_op: string
  default_severity: string
  for_duration: string
  is_enabled: boolean
  domain: string
  scopes: AlertRuleScope[]
  thresholds: AlertThreshold[]
  created_at: string
  updated_at: string
}

export type AlertEvent = {
  id: number
  rule_id: number
  rule: AlertRule | null
  threshold_id: number | null
  threshold: AlertThreshold | null
  tenant_id: number
  host_machine_id: string
  service_id: number | null
  status: string
  is_notified: boolean
  domain: string
  container_name: string
  triggered_at: string
  resolved_at: string | null
  acknowledged_by: number | null
  actual_value: number | null
  metric_string: string
  created_at: string
}

export type ScopeInput = {
  scope_type: string
  scope_id?: number | null
}

export type ThresholdInput = {
  label: string
  threshold: number
  severity?: string
  for_duration?: string
}

export async function getVulnerabilityAlerts(): Promise<VulnerabilityAlert[]> {
  const { data } = await client.get<VulnerabilityAlert[]>("/api/alerts/vulnerabilities")
  return data
}

export async function getAlertTemplates(): Promise<AlertTemplate[]> {
  const { data } = await client.get<AlertTemplate[]>("/api/alerts/templates")
  return data
}

export async function getAlertTemplatesByService(serviceType: string): Promise<AlertTemplate[]> {
  const { data } = await client.get<AlertTemplate[]>(`/api/alerts/templates/${serviceType}`)
  return data
}

export async function createAlertRule(rule: {
  template_id: string
  domain?: string
  scopes: ScopeInput[]
  thresholds: ThresholdInput[]
  for_duration?: string
}): Promise<AlertRule> {
  const { data } = await client.post<AlertRule>("/api/alerts/rules", rule)
  return data
}

export async function getAlertRules(): Promise<AlertRule[]> {
  const { data } = await client.get<AlertRule[]>("/api/alerts/rules")
  return data
}

export async function updateAlertRule(id: number, updates: {
  is_enabled?: boolean
  for_duration?: string
  scopes?: ScopeInput[]
  thresholds?: ThresholdInput[]
}): Promise<void> {
  await client.patch(`/api/alerts/rules/${id}`, updates)
}

export async function deleteAlertRule(id: number): Promise<void> {
  await client.delete(`/api/alerts/rules/${id}`)
}

export async function getAlertEvents(limit?: number): Promise<AlertEvent[]> {
  const params = limit ? `?limit=${limit}` : ""
  const { data } = await client.get<AlertEvent[]>(`/api/alerts/events${params}`)
  return data
}

export async function getAlertFiringEvents(): Promise<AlertEvent[]> {
  const { data } = await client.get<AlertEvent[]>(`/api/alerts/events/firing`)
  return data
}

export async function acknowledgeAlertEvent(id: number): Promise<void> {
  await client.post(`/api/alerts/events/${id}/acknowledge`)
}
