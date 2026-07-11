import { useQuery } from "@tanstack/react-query"
import { getHostSummary, getHostCPU, getHostMemory, getHostDisk, getHostNetwork, getHostProcesses, getHostLogsHealthSummary } from "@/api/hosts"

export function useHostSummary(machineId: string) {
  return useQuery({
    queryKey: ["host-summary", machineId],
    queryFn: () => getHostSummary(machineId),
    refetchInterval: 15_000,
    staleTime: 10_000,
  })
}

export function useHostCPU(machineId: string, params: { start: string; end: string; step: string }) {
  return useQuery({
    queryKey: ["host-cpu", machineId, params],
    queryFn: () => getHostCPU(machineId, params),
    refetchInterval: 30_000,
  })
}

export function useHostMemory(machineId: string, params: { start: string; end: string; step: string }) {
  return useQuery({
    queryKey: ["host-memory", machineId, params],
    queryFn: () => getHostMemory(machineId, params),
    refetchInterval: 30_000,
  })
}

export function useHostDisk(machineId: string, params: { start: string; end: string; step: string }) {
  return useQuery({
    queryKey: ["host-disk", machineId, params],
    queryFn: () => getHostDisk(machineId, params),
    refetchInterval: 60_000,
  })
}

export function useHostNetwork(machineId: string, params: { start: string; end: string; step: string }) {
  return useQuery({
    queryKey: ["host-network", machineId, params],
    queryFn: () => getHostNetwork(machineId, params),
    refetchInterval: 30_000,
  })
}

export function useHostProcesses(machineId: string, params: { start: string; end: string; step: string }) {
  return useQuery({
    queryKey: ["host-processes", machineId, params],
    queryFn: () => getHostProcesses(machineId, params),
    refetchInterval: 30_000,
  })
}

export function useHostLogsHealth(machineId: string, params: { start: string; end: string; step: string }) {
  return useQuery({
    queryKey: ["host-logs-health", machineId, params],
    queryFn: () => getHostLogsHealthSummary(machineId, params),
    refetchInterval: 30_000,
  })
}
