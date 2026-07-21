import { useEffect, useMemo } from "react"
import { useQuery } from "@tanstack/react-query"
import { Link, useSearchParams } from "react-router-dom"

import { getDiscoveryHosts } from "@/api/discovery"
import { getContainerServices, type ContainerService } from "@/api/container"
import { Page, PageHeader, SectionCard } from "@/components/shared/Page"
import { cn } from "@/lib/utils"

export default function ContainersPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const selectedHost = searchParams.get("host") ?? ""

  const hostsQuery = useQuery({
    queryKey: ["discovery-hosts"],
    queryFn: getDiscoveryHosts,
  })

  const containersQuery = useQuery({
    queryKey: ["discovery-containers", selectedHost],
    queryFn: () => getContainerServices(selectedHost || undefined),
    enabled: !!selectedHost,
  })

  const hosts = useMemo(() => hostsQuery.data ?? [], [hostsQuery.data])
  const containers = useMemo(() => containersQuery.data ?? [], [containersQuery.data])

  useEffect(() => {
    if (!selectedHost && hosts.length > 0) {
      setSearchParams({ host: hosts[0].machine_id })
    }
  }, [hosts, selectedHost, setSearchParams])

  return (
    <Page className="max-w-none">
      <PageHeader
        title="Containers"
        description="Select a host to view discovered containers, metrics, and logs."
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
          title="Discovered Containers"
          description={
            selectedHost
              ? "Containers running on this host."
              : "Select a host to view containers."
          }
        >
          {!selectedHost ? (
            <div className="flex flex-col items-center justify-center py-16 text-center">
              <div className="mb-3 text-4xl text-muted-foreground/30">
                <svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5"><path d="M22 12.6c0 2.6-2.8 4.4-7 4.4-4.2 0-7-1.8-7-4.4 0-1.3.7-2.5 2-3.4"/><path d="M22 12.6c0 2.6-2.8 4.4-7 4.4-4.2 0-7-1.8-7-4.4 0-1.3.7-2.5 2-3.4"/><path d="M8 9.2C8 6.6 10.8 4 15 4c4.2 0 7 2.6 7 5.2"/><path d="M2 12.6c0 2.6 2.8 4.4 7 4.4"/><circle cx="15" cy="9.2" r="1"/></svg>
              </div>
              <h3 className="text-lg font-semibold text-muted-foreground">No host selected</h3>
              <p className="mt-1 max-w-sm text-sm text-muted-foreground/70">
                Choose a host from the left panel to see its running containers.
              </p>
            </div>
          ) : null}

          {selectedHost && containersQuery.isLoading ? (
            <div className="flex items-center justify-center py-16 text-sm text-muted-foreground">
              Loading containers...
            </div>
          ) : null}

          {selectedHost && !containersQuery.isLoading && !containers.length ? (
            <div className="flex flex-col items-center justify-center py-16 text-center">
              <div className="mb-3 text-4xl text-muted-foreground/30">
                <svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5"><path d="M22 12.6c0 2.6-2.8 4.4-7 4.4-4.2 0-7-1.8-7-4.4 0-1.3.7-2.5 2-3.4"/><path d="M22 12.6c0 2.6-2.8 4.4-7 4.4-4.2 0-7-1.8-7-4.4 0-1.3.7-2.5 2-3.4"/><path d="M8 9.2C8 6.6 10.8 4 15 4c4.2 0 7 2.6 7 5.2"/><path d="M2 12.6c0 2.6 2.8 4.4 7 4.4"/><circle cx="15" cy="9.2" r="1"/></svg>
              </div>
              <h3 className="text-lg font-semibold text-muted-foreground">No containers found</h3>
              <p className="mt-1 max-w-sm text-sm text-muted-foreground/70">
                This host does not have any Docker containers running, or the Docker agent is not yet installed.
              </p>
            </div>
          ) : null}

          <div className="grid gap-3">
            {containers.map((container) => (
              <ContainerRow key={container.id} container={container} />
            ))}
          </div>
        </SectionCard>
      </div>
    </Page>
  )
}

function ContainerRow({ container }: { container: ContainerService }) {
  return (
    <div className="rounded-xl border border-border bg-card/50 p-3">
      <div className="flex items-center justify-between gap-3">
        <div className="font-medium">{container.container_name}</div>
        {container.hostname ? (
          <div className="text-xs text-muted-foreground">{container.hostname}</div>
        ) : null}
      </div>
      {container.container_image ? (
        <div className="mt-1 text-xs text-muted-foreground">
          Image: {container.container_image}
        </div>
      ) : null}
      <div className="mt-3 flex flex-wrap gap-2">
        <Link
          to={`/containers/${container.id}/dashboard`}
          className="inline-flex items-center rounded-md bg-emerald-600 px-2 py-1.5 text-xs font-semibold text-white hover:bg-emerald-500"
        >
          Open Dashboard
        </Link>
        <Link
          to={`/containers/${container.id}/logs`}
          className="inline-flex items-center rounded-md border border-border px-2 py-1.5 text-xs font-semibold hover:bg-muted"
        >
          Logs
        </Link>
      </div>
    </div>
  )
}
