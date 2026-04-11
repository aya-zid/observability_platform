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

export async function getVulnerabilityAlerts(): Promise<VulnerabilityAlert[]> {
  const { data } = await client.get<VulnerabilityAlert[]>("/api/alerts/vulnerabilities")
  return data
}
