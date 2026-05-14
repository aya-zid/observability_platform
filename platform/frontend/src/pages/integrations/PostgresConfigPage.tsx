import { useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { Link, useParams } from "react-router-dom"

import { getDiscoveryService } from "@/api/discovery"
import { getPostgresConfigSnapshot, type PostgresConfigSetting } from "@/api/postgres"
import { Page, PageHeader, SectionCard } from "@/components/shared/Page"
import { formatBytes, formatDuration, formatNumber } from "@/lib/queryHelpers"
import { cn } from "@/lib/utils"

function formatSettingValue(setting: PostgresConfigSetting) {
  if (setting.value == null) return "Unavailable"
  if (setting.unit === "bytes") return formatBytes(setting.value)
  if (setting.unit === "seconds") return setting.value === 0 ? "0s" : formatDuration(setting.value)
  if (setting.unit === "boolean") return setting.value === 1 ? "On" : "Off"
  return formatNumber(setting.value, Number.isInteger(setting.value) ? 0 : 3)
}

function SettingCard({ setting }: { setting: PostgresConfigSetting }) {
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
          <div className="truncate font-mono text-xs text-muted-foreground" title={setting.name}>
            {setting.name}
          </div>
          <div className="mt-1 text-lg font-semibold">{formatSettingValue(setting)}</div>
        </div>
        {setting.warn ? (
          <span className="rounded border border-amber-300/60 px-2 py-0.5 text-xs text-amber-700 dark:text-amber-300">
            Review
          </span>
        ) : null}
      </div>
      <p className="mt-2 text-xs leading-relaxed text-muted-foreground">{setting.description}</p>
    </div>
  )
}

export function PostgresConfigPage() {
  const { id = "" } = useParams<{ id?: string }>()
  const [openGroups, setOpenGroups] = useState<Record<string, boolean>>({})

  const serviceQuery = useQuery({
    queryKey: ["discovery-service", id],
    queryFn: () => getDiscoveryService(id),
    enabled: !!id,
  })

  const configQuery = useQuery({
    queryKey: ["postgres-config", id],
    queryFn: () => getPostgresConfigSnapshot(id),
    enabled: !!id && !!serviceQuery.data,
    staleTime: 60_000,
    refetchOnWindowFocus: false,
  })

  const service = serviceQuery.data
  const snapshot = configQuery.data

  if (!id) return <div className="p-6 text-sm text-red-600">Missing service id.</div>
  if (serviceQuery.isLoading) return <div className="p-6 text-sm text-muted-foreground">Loading...</div>
  if (!service) return <div className="p-6 text-sm text-red-600">Service not found for id {id}.</div>

  return (
    <Page className="max-w-none">
      <PageHeader
        title="PostgreSQL Configuration"
        description={`Point-in-time PostgreSQL settings for ${service.service_name} on ${service.hostname || service.machine_id}.`}
        actions={
          <div className="flex flex-wrap items-center gap-3">
            <Link to={`/integrations/postgres-dashboard/${id}`} className="rounded-md border border-border px-3 py-2 text-sm font-medium hover:bg-muted">
              Dashboard
            </Link>
            <Link to={`/integrations/postgres-logs/${id}`} className="rounded-md border border-border px-3 py-2 text-sm font-medium hover:bg-muted">
              Logs
            </Link>
            <Link to={`/services/${id}`} className="rounded-md border border-border px-3 py-2 text-sm font-medium hover:bg-muted">
              Configure
            </Link>
          </div>
        }
      />

      {configQuery.isError ? (
        <SectionCard title="Configuration unavailable">
          <div className="text-sm text-red-600">Failed to fetch PostgreSQL configuration metrics.</div>
        </SectionCard>
      ) : null}

      <div className="grid gap-4">
        {(snapshot?.groups ?? []).map((group, index) => {
          const isOpen = openGroups[group.key] ?? index < 3
          const available = group.settings.filter((setting) => setting.value != null).length
          return (
            <SectionCard key={group.key} title={group.title} description={`${available}/${group.settings.length} metrics available`}>
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
            </SectionCard>
          )
        })}
      </div>
    </Page>
  )
}
