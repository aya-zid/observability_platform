import { useState } from "react"
import { useQuery } from "@tanstack/react-query"

import { getDiscoveryService } from "@/api/discovery"
import { getMariaDBConfigSnapshot, type MariaDBConfigSetting } from "@/api/mariadb"
import { formatBytes, formatDuration, formatNumber } from "@/lib/queryHelpers"
import { cn } from "@/lib/utils"

function formatSettingValue(setting: MariaDBConfigSetting) {
  if (setting.value == null) return "N/A"
  if (setting.unit === "bytes") return formatBytes(setting.value)
  if (setting.unit === "seconds") return setting.value === 0 ? "0s" : formatDuration(setting.value)
  if (setting.unit === "milliseconds") return `${formatNumber(setting.value, 0)} ms`
  if (setting.unit === "microseconds") return `${formatNumber(setting.value, 0)} μs`
  if (setting.unit === "boolean") return setting.value === 1 ? "ON" : "OFF"
  return formatNumber(setting.value, Number.isInteger(setting.value) ? 0 : 3)
}

function SettingCard({ setting }: { setting: MariaDBConfigSetting }) {
  return (
    <div
      className={cn(
        "rounded-lg border p-3",
        setting.warn
          ? "border-amber-300/60 bg-amber-500/10"
          : "border-border bg-card",
      )}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="truncate font-mono text-sm text-muted-foreground" title={setting.name}>
            {setting.name}
          </div>
          <div className="mt-1 text-lg font-semibold tabular-nums">{formatSettingValue(setting)}</div>
        </div>
        {setting.warn ? (
          <span className="rounded border border-amber-300/60 px-2 py-0.5 text-sm text-amber-700 dark:text-amber-300">
            Review
          </span>
        ) : null}
      </div>
      <p className="mt-2 text-sm leading-relaxed text-muted-foreground">{setting.description}</p>
    </div>
  )
}

export function MariaDBConfigTab({ id }: { id: string }) {
  const [openGroups, setOpenGroups] = useState<Record<string, boolean>>({})

  const serviceQuery = useQuery({
    queryKey: ["discovery-service", id],
    queryFn: () => getDiscoveryService(id),
    enabled: !!id,
  })

  const configQuery = useQuery({
    queryKey: ["mariadb-config", id],
    queryFn: () => getMariaDBConfigSnapshot(id),
    enabled: !!id && !!serviceQuery.data,
    staleTime: 60_000,
    refetchOnWindowFocus: false,
  })

  const snapshot = configQuery.data
  const service = serviceQuery.data

  if (!id) return <div className="p-6 text-sm text-red-600">Missing service ID</div>
  if (serviceQuery.isLoading) return <div className="p-6 text-sm text-muted-foreground">Loading service…</div>
  if (!service) return <div className="p-6 text-sm text-red-600">Service not found</div>
  if (configQuery.isError) return <div className="text-sm text-red-600">Failed to fetch MariaDB configuration from VictoriaMetrics.</div>

  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">
        Current settings for {service.service_name} ({service.hostname || service.machine_id})
      </p>
      {(snapshot?.groups ?? []).map((group, index) => {
        const isOpen = openGroups[group.key] ?? index < 3
        const available = group.settings.filter((s) => s.value != null).length
        return (
          <section key={group.key} className="rounded-2xl border border-border bg-card/70 p-4 md:p-6">
            <div className="mb-4">
              <div className="font-medium">{group.title}</div>
              <div className="mt-1 text-sm text-muted-foreground">{available}/{group.settings.length} metrics available</div>
            </div>
            <button
              type="button"
              className="mb-3 rounded-md border border-border px-3 py-1.5 text-sm hover:bg-muted"
              onClick={() => setOpenGroups((prev) => ({ ...prev, [group.key]: !isOpen }))}
            >
              {isOpen ? "Collapse" : "Expand"}
            </button>
            {isOpen ? (
              <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
                {group.settings.map((setting) => (
                  <SettingCard key={setting.name} setting={setting} />
                ))}
              </div>
            ) : null}
          </section>
        )
      })}
    </div>
  )
}
