import { useEffect, useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Link, useParams } from "react-router-dom"

import {
  createIntegrationInstallCommand,
  getDiscoveryService,
  updateDiscoveryServiceConfig,
  updateDiscoveryServiceCollection,
} from "@/api/discovery"
import { Button } from "@/components/ui/button"
import { Page, PageHeader, SectionCard } from "@/components/shared/Page"
import { cn } from "@/lib/utils"

function adaptGuideToPort(guide: string, defaultEndpoint: string | undefined, detectedPort?: number) {
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

function formatScanTime(value?: string) {
  if (!value) return "-"
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return value
  return parsed.toLocaleString()
}

function resolveInitialMetricsMode(
  metricsMode: string | undefined,
  isSupported: boolean,
  suggestedMetricsEndpoint?: string
) {
  if (metricsMode && metricsMode !== "unknown") {
    return metricsMode
  }
  if (isSupported) {
    return "catalog"
  }
  if (suggestedMetricsEndpoint) {
    return "prometheus"
  }
  return "unknown"
}

export default function IntegrationServicePage() {
  const { id = "" } = useParams()
  const queryClient = useQueryClient()

  const serviceQuery = useQuery({
    queryKey: ["discovery-service", id],
    queryFn: () => getDiscoveryService(id),
    enabled: !!id,
  })

  const [logOn, setLogOn] = useState(false)
  const [metricsOn, setMetricsOn] = useState(false)
  const [confirmedLogPath, setConfirmedLogPath] = useState("")
  const [metricsMode, setMetricsMode] = useState("unknown")
  const [metricsEndpoint, setMetricsEndpoint] = useState("")
  const [isJavaApp, setIsJavaApp] = useState(false)
  const [jmxExporterPort, setJMXExporterPort] = useState("")
  const [installCommand, setInstallCommand] = useState("")

  useEffect(() => {
    if (serviceQuery.data) {
      setLogOn(serviceQuery.data.log_collection_on)
      setMetricsOn(serviceQuery.data.metric_collection_on)
      setConfirmedLogPath(serviceQuery.data.confirmed_log_path ?? serviceQuery.data.suggested_log_path ?? "")
      setMetricsMode(
        resolveInitialMetricsMode(
          serviceQuery.data.metrics_mode,
          serviceQuery.data.is_supported,
          serviceQuery.data.suggested_metrics_endpoint
        )
      )
      setMetricsEndpoint(
        serviceQuery.data.user_metrics_endpoint ?? serviceQuery.data.suggested_metrics_endpoint ?? ""
      )
      setIsJavaApp(serviceQuery.data.is_java_app)
      setJMXExporterPort(serviceQuery.data.jmx_exporter_port ? String(serviceQuery.data.jmx_exporter_port) : "")
    }
  }, [serviceQuery.data])

  const saveMutation = useMutation({
    mutationFn: () =>
      updateDiscoveryServiceCollection(id, {
        log_collection_on: logOn,
        metric_collection_on: metricsOn,
      }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["discovery-service", id] })
      await queryClient.invalidateQueries({ queryKey: ["discovery-services"] })
    },
  })

  const configMutation = useMutation({
    mutationFn: () =>
      updateDiscoveryServiceConfig(id, {
        confirmed_log_path: confirmedLogPath,
        user_metrics_endpoint: metricsEndpoint,
        metrics_mode: metricsMode,
        is_java_app: isJavaApp,
        jmx_exporter_port: jmxExporterPort ? Number(jmxExporterPort) : undefined,
      }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["discovery-service", id] })
      await queryClient.invalidateQueries({ queryKey: ["discovery-services"] })
    },
  })

  const installMutation = useMutation({
    mutationFn: () => createIntegrationInstallCommand(id),
    onSuccess: (res) => {
      setInstallCommand(res.install_command)
    },
  })

  const data = serviceQuery.data
  const canEditMetricsEndpoint = metricsMode === "prometheus" || metricsMode === "catalog" || metricsMode === "unknown"
  const metricsGuide = data?.catalog
    ? data.catalog.metrics_pattern === "builtin"
      ? adaptGuideToPort(data.catalog.metrics_guide, data.catalog.default_metrics_endpoint, data.port)
      : data.catalog.metrics_guide
    : ""

  return (
    <Page>
      <PageHeader
        title={data ? `Integration • ${data.service_name}` : "Integration"}
        description="Review discovered details, follow the guide, then enable collection toggles."
        actions={
          <Link to={`/integrations?host=${data?.machine_id ?? ""}`} className="text-sm underline underline-offset-4">
            Back to host services
          </Link>
        }
      />

      {serviceQuery.isLoading ? (
        <SectionCard title="Loading">
          <div className="text-sm text-muted-foreground">Loading service details...</div>
        </SectionCard>
      ) : null}

      {!data && !serviceQuery.isLoading ? (
        <SectionCard title="Service not found">
          <div className="text-sm text-muted-foreground">This service does not exist or you no longer have access.</div>
        </SectionCard>
      ) : null}

      {data ? (
        <>
          <SectionCard title="Discovered Details">
            <div className="grid gap-2 text-sm">
              <div><span className="text-muted-foreground">Host:</span> {data.hostname || "-"} ({data.machine_id})</div>
              <div><span className="text-muted-foreground">Category:</span> {data.category}</div>
              <div><span className="text-muted-foreground">Port:</span> {data.port ?? "-"}</div>
              <div><span className="text-muted-foreground">Version:</span> {data.version || "-"}</div>
              <div><span className="text-muted-foreground">Logging mode:</span> {data.logging_mode}</div>
              <div><span className="text-muted-foreground">Log path source:</span> {data.log_path_source}</div>
              <div><span className="text-muted-foreground">Metrics mode:</span> {data.metrics_mode}</div>
              <div><span className="text-muted-foreground">Integration status:</span> {data.integration_status}</div>
              <div><span className="text-muted-foreground">Vulnerability scan status:</span> {data.vuln_scan_status}</div>
              <div><span className="text-muted-foreground">Last vulnerability scan:</span> {formatScanTime(data.last_vuln_scan_at)}</div>
              <div><span className="text-muted-foreground">Access log:</span> {data.access_log_path || "-"}</div>
              <div><span className="text-muted-foreground">Error log:</span> {data.error_log_path || "-"}</div>
              <div><span className="text-muted-foreground">Confirmed log path:</span> {data.confirmed_log_path || "-"}</div>
              <div><span className="text-muted-foreground">User metrics endpoint:</span> {data.user_metrics_endpoint || "-"}</div>
              <div><span className="text-muted-foreground">JMX exporter port:</span> {data.jmx_exporter_port ?? "-"}</div>
              <div>
                <span className="text-muted-foreground">Detected log paths:</span>
                <ul className="mt-1 list-disc pl-5">
                  {(data.detected_log_paths.length ? data.detected_log_paths : ["-"]).map((p) => (
                    <li key={p} className="break-all">{p}</li>
                  ))}
                </ul>
              </div>
              <div>
                <span className="text-muted-foreground">Effective log paths:</span>
                <ul className="mt-1 list-disc pl-5">
                  {(data.effective_log_paths.length ? data.effective_log_paths : ["-"]).map((p) => (
                    <li key={p} className="break-all">{p}</li>
                  ))}
                </ul>
              </div>
            </div>
          </SectionCard>

          <SectionCard title={data.catalog ? "Catalog Guide" : "Unknown Service"} description={data.catalog ? "Catalog-driven guidance for supported services." : "Confirm the log path and choose how metrics should be collected."}>
            {data.catalog ? (
              <div className="grid gap-4 text-sm">
                {data.port && data.catalog.default_metrics_endpoint ? (
                  <div className="rounded-lg border border-border bg-muted/30 p-3 text-xs text-muted-foreground">
                    Metrics examples below use your detected service port {data.port} where applicable.
                  </div>
                ) : null}
                <div>
                  <div className="font-medium">Enable metrics</div>
                  <pre className="mt-1 whitespace-pre-wrap rounded-lg bg-muted/40 p-3 text-xs leading-relaxed">
                    {metricsGuide}
                  </pre>
                </div>
                <div>
                  <div className="font-medium">Enable logs</div>
                  <pre className="mt-1 whitespace-pre-wrap rounded-lg bg-muted/40 p-3 text-xs leading-relaxed">
                    {data.catalog.logs_guide}
                  </pre>
                </div>
                <div>
                  <div className="font-medium">If journald/syslog is used</div>
                  <pre className="mt-1 whitespace-pre-wrap rounded-lg bg-muted/40 p-3 text-xs leading-relaxed">
                    {data.catalog.journaled_log_guide}
                  </pre>
                </div>
              </div>
            ) : (
              <div className="text-sm text-muted-foreground">
                This service is not yet supported by the catalog.
              </div>
            )}
          </SectionCard>

          <SectionCard
            title="Confirm Configuration"
            description="We prefilled the detected log path and a suggested metrics endpoint. Adjust them if needed, then confirm."
          >
            <div className="grid gap-4 text-sm">
              <div className="rounded-lg border border-border bg-muted/30 p-3 text-xs text-muted-foreground">
                The metrics endpoint suggestion uses the detected service port when one was found.
              </div>

              <label className="grid gap-1">
                <span className="text-muted-foreground">Confirmed log path</span>
                <input
                  className="h-10 rounded-md border border-input bg-background/70 px-3 text-sm outline-none"
                  value={confirmedLogPath}
                  onChange={(e) => setConfirmedLogPath(e.target.value)}
                  placeholder={data.suggested_log_path || data.detected_log_paths[0] || "/var/log/myapp/app.log"}
                />
              </label>

              {data.detected_log_paths.length ? (
                <div className="grid gap-2">
                  <span className="text-muted-foreground">Detected log paths</span>
                  <div className="flex flex-wrap gap-2">
                    {data.detected_log_paths.map((path) => (
                      <button
                        key={path}
                        type="button"
                        className={cn(
                          "rounded-full border px-3 py-1 text-xs transition-colors",
                          confirmedLogPath === path
                            ? "border-foreground bg-foreground text-background"
                            : "border-border bg-background/70 hover:bg-muted"
                        )}
                        onClick={() => setConfirmedLogPath(path)}
                      >
                        {path}
                      </button>
                    ))}
                  </div>
                </div>
              ) : null}

              <label className="grid gap-1">
                <span className="text-muted-foreground">Metrics mode</span>
                <select
                  className="h-10 rounded-md border border-input bg-background/70 px-3 text-sm outline-none"
                  value={metricsMode}
                  onChange={(e) => setMetricsMode(e.target.value)}
                >
                  {data.is_supported ? <option value="catalog">Use catalog defaults</option> : null}
                  <option value="prometheus">Prometheus endpoint</option>
                  <option value="jmx">JMX exporter</option>
                  <option value="none">Logs only</option>
                  {!data.is_supported ? <option value="unknown">Not configured yet</option> : null}
                </select>
              </label>

              {canEditMetricsEndpoint ? (
                <label className="grid gap-1">
                  <span className="text-muted-foreground">Metrics endpoint</span>
                  <input
                    className="h-10 rounded-md border border-input bg-background/70 px-3 text-sm outline-none"
                    value={metricsEndpoint}
                    onChange={(e) => setMetricsEndpoint(e.target.value)}
                    placeholder={data.suggested_metrics_endpoint || "http://127.0.0.1:8080/metrics"}
                  />
                </label>
              ) : null}

              {data.suggested_metrics_endpoint ? (
                <div className="rounded-lg border border-border bg-background/60 p-3 text-xs text-muted-foreground">
                  Suggested endpoint: <span className="font-mono break-all">{data.suggested_metrics_endpoint}</span>
                </div>
              ) : null}

              <label className="flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={isJavaApp}
                  onChange={(e) => setIsJavaApp(e.target.checked)}
                />
                Java application
              </label>

              {metricsMode === "jmx" ? (
                <label className="grid gap-1">
                  <span className="text-muted-foreground">JMX exporter port</span>
                  <input
                    className="h-10 rounded-md border border-input bg-background/70 px-3 text-sm outline-none"
                    value={jmxExporterPort}
                    onChange={(e) => setJMXExporterPort(e.target.value)}
                    placeholder="7070"
                  />
                </label>
              ) : null}

              <div>
                <Button disabled={configMutation.isPending} onClick={() => configMutation.mutate()}>
                  {configMutation.isPending ? "Confirming..." : "Confirm Configuration"}
                </Button>
              </div>
            </div>
          </SectionCard>

          <SectionCard
            title="Version Security"
            description="Current vulnerability findings for the detected service version."
          >
            <div
              className={cn(
                "mb-4 rounded-lg border p-3 text-sm",
                data.vulnerabilities.length
                  ? "border-amber-300/40 bg-amber-500/10 text-amber-700 dark:text-amber-300"
                  : data.vuln_scan_status === "scanned"
                    ? "border-emerald-300/40 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300"
                    : "border-border bg-muted/30 text-muted-foreground"
              )}
            >
              <div>{data.vuln_status_message}</div>
              <div className="mt-1 text-xs opacity-80">
                Last scan: {formatScanTime(data.last_vuln_scan_at)}
              </div>
            </div>

            {data.vulnerabilities.length ? (
              <div className="grid gap-3">
                {data.vulnerabilities.map((item) => (
                  <div key={item.cve_id} className="rounded-xl border border-border bg-card/40 p-4">
                    <div className="flex flex-wrap items-center justify-between gap-3">
                      <div className="font-medium">{item.cve_id}</div>
                      <div
                        className={cn(
                          "text-xs uppercase tracking-wide",
                          item.alert_level === "alert" ? "text-red-500" : "text-amber-500"
                        )}
                      >
                        {item.alert_level}
                      </div>
                    </div>
                    <div className="mt-2 flex flex-wrap gap-4 text-xs text-muted-foreground">
                      <span>Severity {item.severity}</span>
                      <span>CVSS {item.cvss_score.toFixed(1)}</span>
                      <span>Fix {item.fixed_in_version || "-"}</span>
                    </div>
                    <div className="mt-3 text-sm text-muted-foreground">
                      {item.description || "No description available."}
                    </div>
                  </div>
                ))}
              </div>
            ) : null}
          </SectionCard>

          <SectionCard title="Collection Controls">
            <div className="grid gap-3 text-sm">
              <label className="flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={logOn}
                  onChange={(e) => setLogOn(e.target.checked)}
                />
                Enable log collection
              </label>
              <label className="flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={metricsOn}
                  onChange={(e) => setMetricsOn(e.target.checked)}
                />
                Enable metrics collection
              </label>
              <div>
                <Button
                  disabled={saveMutation.isPending}
                  onClick={() => saveMutation.mutate()}
                >
                  {saveMutation.isPending ? "Saving..." : "Save Integration Settings"}
                </Button>
              </div>

              {data.service_name === "nginx" && logOn && metricsOn ? (
                <div>
                  <Link
                    to={`/integrations/nginx-dashboard/${id}`}
                    className="inline-block rounded-md bg-emerald-600 px-3 py-2 text-sm font-semibold text-white hover:bg-emerald-500"
                  >
                    Open Nginx Dashboard
                  </Link>
                </div>
              ) : null}
            </div>
          </SectionCard>

          <SectionCard title="Install Integration Script">
            <div className="grid gap-3 text-sm">
              <div className="text-muted-foreground">
                Generate a host-specific command, then run it on the selected host to activate collection using the confirmed log path and metrics settings above.
              </div>
              <div>
                <Button
                  disabled={installMutation.isPending}
                  onClick={() => installMutation.mutate()}
                >
                  {installMutation.isPending ? "Generating..." : "Generate Install Command"}
                </Button>
              </div>
              {installCommand ? (
                <div className="grid gap-2">
                  <div className="text-xs text-muted-foreground">Run this on host {data.hostname || data.machine_id}:</div>
                  <pre className="whitespace-pre-wrap break-all rounded-lg bg-muted/40 p-3 text-xs leading-relaxed">
                    {installCommand}
                  </pre>
                </div>
              ) : null}
            </div>
          </SectionCard>
        </>
      ) : null}
    </Page>
  )
}
