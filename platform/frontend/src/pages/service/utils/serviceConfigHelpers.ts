import type { LucideIcon } from "lucide-react"
import type { VirtualHostEntry } from "@/pages/service/components/WebServerVirtualHostsSection"

export function adaptGuideToPort(guide: string, defaultEndpoint: string | undefined, detectedPort?: number) {
  if (!guide || !defaultEndpoint || !detectedPort) return guide
  try {
    const parsed = new URL(defaultEndpoint)
    if (!parsed.port) return guide
    const currentPort = Number(parsed.port)
    if (!currentPort || currentPort === detectedPort) return guide
    const updated = new URL(defaultEndpoint)
    updated.port = String(detectedPort)
    return guide.replaceAll(defaultEndpoint, updated.toString())
  } catch {
    return guide
  }
}

export function formatScanTime(value?: string) {
  if (!value) return "-"
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return value
  return parsed.toLocaleString()
}

export function isPostgresService(name: string) {
  return name.toLowerCase().includes("postgres")
}

export function groupWebServerVirtualHostsFromAPI(raw: any[]): VirtualHostEntry[] {
  if (!raw?.length) return []
  const grouped: Record<string, VirtualHostEntry> = {}
  const order: string[] = []

  for (const vh of raw) {
    const key = `${vh.domain}`
    if (!grouped[key]) {
      grouped[key] = {
        id: vh.id,
        domain: vh.domain,
        access_log: vh.access_log ?? "",
        error_log: vh.error_log ?? "",
        enabled: vh.enabled ?? true,
      }
      order.push(key)
    } else {
      if (!grouped[key].access_log && vh.access_log) grouped[key].access_log = vh.access_log
      if (!grouped[key].error_log && vh.error_log) grouped[key].error_log = vh.error_log
    }
  }

  return order.map((k) => grouped[k])
}
