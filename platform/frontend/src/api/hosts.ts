import client from "@/api/client"
import { secondsToNanos } from "@/lib/queryHelpers"

export type HostLogEvent = {
  timestamp: string
  timestamp_ns: string
  message: string
  priority?: string
  severity: string
  unit?: string
  identifier?: string
  pid?: string
  transport?: string
  command?: string
  raw?: Record<string, string>
  labels?: Record<string, string>
}

export type HostLogsSummary = {
  total: number
  by_level: Record<string, number>
  by_unit: Record<string, number>
  units: string[]
  first_seen?: string
  last_seen?: string
}

export type HostLogsPage = {
  events: HostLogEvent[]
  summary: HostLogsSummary
  total: number
  page: number
  page_size: number
  has_more: boolean
}

export type HostLogsParams = {
  start: string
  end: string
  priority?: string
  unit?: string
  identifier?: string
  search?: string
  page?: number
  page_size?: number
}

export async function getHostSummary(machineId: string): Promise<any> {
  const { data } = await client.get(`/api/hosts/${machineId}/dashboard/summary`)
  return data
}

export async function getHostCPU(
  machineId: string,
  params: { start: string; end: string; step: string },
): Promise<any> {
  const { data } = await client.get(`/api/hosts/${machineId}/dashboard/cpu`, { params })
  return data
}

export async function getHostMemory(
  machineId: string,
  params: { start: string; end: string; step: string },
): Promise<any> {
  const { data } = await client.get(`/api/hosts/${machineId}/dashboard/memory`, { params })
  return data
}

export async function getHostDisk(
  machineId: string,
  params: { start: string; end: string; step: string },
): Promise<any> {
  const { data } = await client.get(`/api/hosts/${machineId}/dashboard/disk`, { params })
  return data
}

export async function getHostNetwork(
  machineId: string,
  params: { start: string; end: string; step: string },
): Promise<any> {
  const { data } = await client.get(`/api/hosts/${machineId}/dashboard/network`, { params })
  return data
}

export async function getHostProcesses(
  machineId: string,
  params: { start: string; end: string; step: string },
): Promise<any> {
  const { data } = await client.get(`/api/hosts/${machineId}/dashboard/processes`, { params })
  return data
}

export async function getHostLogs(machineId: string, params: HostLogsParams): Promise<HostLogsPage> {
  const { data } = await client.get<HostLogsPage>(`/api/hosts/${machineId}/logs`, { params })
  return data
}

export type LogHealthTimeBucket = {
  time: number
  errors: number
  warnings: number
  total: number
}

export type LogHealthRestartEvent = {
  time: number
  unit: string
  failed: boolean
  message: string
}

export type LogHealthSSHBucket = {
  time: number
  failed: number
  ok: number
}

export type LogHealthOOMEvent = {
  time: number
  message: string
}

export type LogHealthDiskErrorBucket = {
  time: number
  count: number
}

export type LogHealthSummary = {
  error_rate: LogHealthTimeBucket[]
  restart_events: LogHealthRestartEvent[]
  ssh_attempts: LogHealthSSHBucket[]
  oom_kills: LogHealthOOMEvent[]
  disk_errors: LogHealthDiskErrorBucket[]
  total_errors: number
  total_warnings: number
  total_restarts: number
  total_oom_kills: number
}

export async function getHostLogsHealthSummary(
  machineId: string,
  params: { start: string; end: string; step: string },
): Promise<LogHealthSummary> {
  const { data } = await client.get<LogHealthSummary>(`/api/hosts/${machineId}/logs/health-summary`, {
    params: {
      start: secondsToNanos(Number(params.start)),
      end: secondsToNanos(Number(params.end)),
      step: params.step,
    },
  })
  return data
}
