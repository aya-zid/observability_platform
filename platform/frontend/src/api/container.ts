import client from "@/api/client"

export type ContainerService = {
  id: number
  machine_id: string
  container_name: string
  service_name: string
  container_image?: string
  log_collection_on: boolean
  metric_collection_on: boolean
  hostname?: string
}

export type ContainerServiceDetail = {
  id: number
  machine_id: string
  hostname?: string
  container_name: string
  service_name: string
  container_image?: string
  log_collection_on: boolean
  metric_collection_on: boolean
}

export type ContainerDashboardSummary = {
  up?: number
  cpu_percent?: number
  mem_usage_bytes?: number
  mem_limit_bytes?: number
  mem_percent?: number
  net_rx_bytes_total?: number
  net_tx_bytes_total?: number
  block_read_bytes_total?: number
  block_write_bytes_total?: number
  pids?: number

  cpu_timeseries: Record<string, number | null>[]
  mem_timeseries: Record<string, number | null>[]
  mem_percent_timeseries: Record<string, number | null>[]
  mem_limit_timeseries: Record<string, number | null>[]
  net_timeseries: Record<string, number | null>[]
  block_timeseries: Record<string, number | null>[]
  pids_timeseries: Record<string, number | null>[]
}

export type ContainerLogEvent = {
  timestamp: string
  line: string
}

export type ContainerLogsPage = {
  events: ContainerLogEvent[]
  total: number
  page: number
  page_size: number
  has_more: boolean
}

export async function getContainerServices(machineId?: string): Promise<ContainerService[]> {
  const { data } = await client.get<ContainerService[]>("/api/discovery/containers", {
    params: machineId ? { machine_id: machineId } : undefined,
  })
  return data
}

export async function getContainerService(id: string): Promise<ContainerServiceDetail> {
  const { data } = await client.get<ContainerServiceDetail>(`/api/discovery/containers/${id}`)
  return data
}

export async function getContainerDashboardSummary(
  id: string,
  params: { start: string; end: string; step: string },
): Promise<ContainerDashboardSummary> {
  const { data } = await client.get<ContainerDashboardSummary>(`/api/containers/${id}/dashboard`, { params })
  return data
}

export type ContainerLogsParams = {
  start: string
  end: string
  search?: string
  page?: number
  page_size?: number
}

export async function getContainerLogs(
  id: string,
  params: ContainerLogsParams,
): Promise<ContainerLogsPage> {
  const { data } = await client.get<ContainerLogsPage>(`/api/containers/${id}/logs`, {
    params: {
      start: params.start,
      end: params.end,
      search: params.search ?? "",
      page: params.page ?? 1,
      page_size: params.page_size ?? 100,
    },
  })
  return data
}
