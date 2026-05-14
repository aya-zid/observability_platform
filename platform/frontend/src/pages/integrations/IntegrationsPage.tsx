import { useEffect, useMemo } from "react"
import { useQuery } from "@tanstack/react-query"
import { Link, useSearchParams } from "react-router-dom"

import {
  getCatalog,
  getDiscoveryHosts,
  getDiscoveryServices,
  type DiscoveryService,
} from "@/api/discovery"
import { Page, PageHeader, SectionCard } from "@/components/shared/Page"
import { cn } from "@/lib/utils"

const STATUS_COLOR: Record<string, string> = {
  active: "text-emerald-500",
  pending: "text-amber-500",
  error: "text-red-500",
  not_installed: "text-muted-foreground",
}

function isPostgresService(name: string) {
  return name.toLowerCase().includes("postgres")
}

function ServiceRow({ service }: { service: DiscoveryService }) {
  return (
    <div className="rounded-xl border border-border bg-card/50 p-3">
      <Link
        to={`/services/${service.id}`}
        className="block hover:border-primary/40 hover:bg-card transition-colors"
      >
        <div className="flex items-center justify-between gap-3">
          <div className="font-medium">{service.service_name}</div>
          <div
            className={cn(
              "text-xs uppercase tracking-wide",
              STATUS_COLOR[service.integration_status] ?? STATUS_COLOR.not_installed
            )}
          >
            {service.integration_status.replaceAll("_", " ")}
          </div>
        </div>
        <div className="mt-2 flex items-center gap-4 text-xs">
          <span className={cn(service.log_collection_on ? "text-emerald-500" : "text-red-500")}>
            Logs {service.log_collection_on ? "on" : "off"}
          </span>
          <span className={cn(service.metric_collection_on ? "text-emerald-500" : "text-red-500")}>
            Metrics {service.metric_collection_on ? "on" : "off"}
          </span>
        </div>
        {service.port || service.version ? (
          <div className="mt-2 flex flex-wrap items-center gap-4 text-xs text-muted-foreground">
            {service.port ? <span>Port {service.port}</span> : null}
            {service.version ? <span>Version {service.version}</span> : null}
          </div>
        ) : null}
      </Link>
      {service.service_name === "nginx" && service.log_collection_on && service.metric_collection_on ? (
        <div className="mt-3">
          <Link
            to={`/integrations/nginx-dashboard/${service.id}`}
            className="inline-flex items-center rounded-md bg-emerald-600 px-2 py-1.5 text-xs font-semibold text-white hover:bg-emerald-500"
          >
            Open Nginx Dashboard
          </Link>
        </div>
      ) : null}
      {isPostgresService(service.service_name) && service.log_collection_on && service.metric_collection_on ? (
        <div className="mt-3 flex flex-wrap gap-2">
          <Link
            to={`/integrations/postgres-dashboard/${service.id}`}
            className="inline-flex items-center rounded-md bg-sky-600 px-2 py-1.5 text-xs font-semibold text-white hover:bg-sky-500"
          >
            Open PostgreSQL Dashboard
          </Link>
          <Link
            to={`/integrations/postgres-logs/${service.id}`}
            className="inline-flex items-center rounded-md border border-border px-2 py-1.5 text-xs font-semibold hover:bg-muted"
          >
            Logs
          </Link>
        </div>
      ) : null}
    </div>
  )
}

export default function IntegrationsPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const selectedHost = searchParams.get("host") ?? ""

  const hostsQuery = useQuery({
    queryKey: ["discovery-hosts"],
    queryFn: getDiscoveryHosts,
  })

  const catalogQuery = useQuery({
    queryKey: ["catalog"],
    queryFn: getCatalog,
  })

  const servicesQuery = useQuery({
    queryKey: ["discovery-services", selectedHost],
    queryFn: () => getDiscoveryServices(selectedHost || undefined),
    enabled: !!selectedHost,
  })

  const hosts = useMemo(() => hostsQuery.data ?? [], [hostsQuery.data])
  const catalog = useMemo(() => catalogQuery.data ?? [], [catalogQuery.data])
  const services = useMemo(() => servicesQuery.data ?? [], [servicesQuery.data])

  const categoryLabelByKey: Record<string, string> = {}
  for (const item of catalog) categoryLabelByKey[item.category] = item.category

  const servicesByCategory: Record<string, DiscoveryService[]> = {}
  for (const service of services) {
    if (!servicesByCategory[service.category]) servicesByCategory[service.category] = []
    servicesByCategory[service.category].push(service)
  }

  useEffect(() => {
    if (!selectedHost && hosts.length > 0) {
      setSearchParams({ host: hosts[0].machine_id })
    }
  }, [hosts, selectedHost, setSearchParams])

  return (
    <Page className="max-w-none">
      <PageHeader
        title="Integrations"
        description="Select a host, review discovered services, and enable log/metrics collection per service."
      />

      <div className="grid gap-4 lg:grid-cols-[280px_1fr]">
        <SectionCard title="Hosts" description="Machines discovered for your tenant.">
          <div className="grid gap-2">
            {hosts.map((host) => (
              <button
                key={host.machine_id}
                type="button"
                onClick={() => setSearchParams({ host: host.machine_id })}
                className={cn(
                  "text-left rounded-xl border px-3 py-3 transition-colors",
                  selectedHost === host.machine_id
                    ? "border-primary bg-primary/10"
                    : "border-border bg-card/40 hover:border-primary/40"
                )}
              >
                <div className="font-medium">{host.hostname}</div>
                <div className="mt-1 text-xs text-muted-foreground">
                  {host.os_name} • {host.arch}
                </div>
                <div className="mt-1 text-[11px] text-muted-foreground/80">
                  {host.machine_id.slice(0, 14)}...
                </div>
              </button>
            ))}
            {!hosts.length ? (
              <div className="text-sm text-muted-foreground">No hosts discovered yet.</div>
            ) : null}
          </div>
        </SectionCard>

        <SectionCard
          title="Discovered Services"
          description={
            selectedHost
              ? "Supported services use catalog categories. Unknown services stay visible so you can configure logs and metrics manually."
              : "Select a host to view services."
          }
        >
          {!selectedHost ? (
            <div className="text-sm text-muted-foreground">Pick a host from the left panel.</div>
          ) : null}

          {selectedHost && servicesQuery.isLoading ? (
            <div className="text-sm text-muted-foreground">Loading services...</div>
          ) : null}

          {selectedHost && !servicesQuery.isLoading && !services.length ? (
            <div className="text-sm text-muted-foreground">
              No discovered services found on this host yet.
            </div>
          ) : null}

          <div className="grid gap-5">
            {Object.entries(servicesByCategory).map(([category, rows]) => (
              <div key={category} className="grid gap-2">
                <div className="text-xs uppercase tracking-[0.14em] text-muted-foreground">
                  {categoryLabelByKey[category] ?? category}
                </div>
                <div className="grid gap-2">
                  {rows.map((service) => (
                    <ServiceRow key={service.id} service={service} />
                  ))}
                </div>
              </div>
            ))}
          </div>
        </SectionCard>
      </div>
    </Page>
  )
}
