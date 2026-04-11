import client from "@/api/client"

export type CatalogService = {
  name: string
  display_name: string
  category: string
  default_log_paths: string[]
  default_logging_mode: "file" | "journald" | "syslog" | "socket"
  metrics_pattern: "builtin" | "exporter" | "none"
  default_metrics_endpoint: string
  exporter_port?: number
  metrics_guide: string
  logs_guide: string
  journaled_log_guide: string
  detection_hints: string[]
}

export type DiscoveryHost = {
  machine_id: string
  hostname: string
  os_name: string
  arch: string
  agent_version: string
  last_seen_at: string
}

export type DiscoveryService = {
  id: number
  machine_id: string
  service_name: string
  port?: number
  version?: string
  category: string
  is_supported: boolean
  confirmed_log_path?: string
  log_path_source: string
  metrics_mode: string
  log_collection_on: boolean
  metric_collection_on: boolean
  integration_status: string
  logging_mode: string
  vuln_scan_supported: boolean
  vuln_scan_status: string
  last_vuln_scan_at?: string
}

export type DiscoveryServiceDetail = {
  id: number
  machine_id: string
  hostname: string
  service_name: string
  port?: number
  version?: string
  category: string
  is_supported: boolean
  logging_mode: string
  uses_journald: boolean
  uses_syslog: boolean
  access_log_path: string
  error_log_path: string
  confirmed_log_path?: string
  log_path_source: string
  effective_log_paths: string[]
  suggested_log_path?: string
  user_metrics_endpoint?: string
  suggested_metrics_endpoint?: string
  metrics_mode: string
  is_java_app: boolean
  jmx_exporter_port?: number
  detected_log_paths: string[]
  log_collection_on: boolean
  metric_collection_on: boolean
  integration_status: string
  vuln_scan_supported: boolean
  vuln_scan_status: string
  last_vuln_scan_at?: string
  vuln_status_message: string
  vulnerabilities: Array<{
    cve_id: string
    cvss_score: number
    severity: string
    alert_level: string
    description: string
    fixed_in_version: string
  }>
  catalog?: CatalogService
}

export async function getCatalog(): Promise<CatalogService[]> {
  const { data } = await client.get<CatalogService[]>("/api/catalog")
  return data
}

export async function getDiscoveryHosts(): Promise<DiscoveryHost[]> {
  const { data } = await client.get<DiscoveryHost[]>("/api/discovery/hosts")
  return data
}

export async function getDiscoveryServices(machineId?: string): Promise<DiscoveryService[]> {
  const { data } = await client.get<DiscoveryService[]>("/api/discovery/services", {
    params: machineId ? { machine_id: machineId } : undefined,
  })
  return data
}

export async function getDiscoveryService(id: string): Promise<DiscoveryServiceDetail> {
  const { data } = await client.get<DiscoveryServiceDetail>(`/api/discovery/services/${id}`)
  return data
}

export async function updateDiscoveryServiceCollection(
  id: string,
  payload: { log_collection_on: boolean; metric_collection_on: boolean }
): Promise<{ ok: boolean }> {
  const { data } = await client.patch<{ ok: boolean }>(
    `/api/discovery/services/${id}/collection`,
    payload
  )
  return data
}

export async function updateDiscoveryServiceConfig(
  id: string,
  payload: {
    confirmed_log_path: string
    user_metrics_endpoint: string
    metrics_mode: string
    is_java_app: boolean
    jmx_exporter_port?: number
  }
): Promise<{ ok: boolean }> {
  const { data } = await client.patch<{ ok: boolean }>(
    `/api/discovery/services/${id}/config`,
    payload
  )
  return data
}

export type IntegrationInstallResponse = {
  service_name: string
  script_url: string
  install_command: string
}

export async function createIntegrationInstallCommand(
  id: string
): Promise<IntegrationInstallResponse> {
  const { data } = await client.post<IntegrationInstallResponse>(
    `/api/discovery/services/${id}/integration`
  )
  return data
}
