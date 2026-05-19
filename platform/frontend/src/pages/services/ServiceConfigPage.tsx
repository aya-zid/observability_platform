import { useEffect, useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Link, useParams, useNavigate } from "react-router-dom"
import {
  createIntegrationInstallCommand,
  getDiscoveryService,
  updateDiscoveryServiceConfig,
  updateDiscoveryServiceCollection,
} from "@/api/discovery"
import { Button } from "@/components/ui/button"
import NginxVirtualHostsSection from "@/components/nginx/NginxVirtualHostsSection"
import ApacheVirtualHostsSection from "@/components/apache/ApacheVirtualHostsSection"
import type { VirtualHostEntry } from "@/components/webserver/WebServerVirtualHostsSection"
import MarkdownRenderer from "@/components/MarkdownRenderer"
import { cn } from "@/lib/utils"
import {
  ArrowLeft,
  Copy,
  Check,
  Terminal,
  BookOpen,
  AlertTriangle,
  Shield,
  CheckCircle,
  XCircle,
  Clock,
  ChevronDown,
  Activity,
  FileText,
  Server,
  Globe,
  Zap,
} from "lucide-react"

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

function isPostgresService(name: string) {
  return name.toLowerCase().includes("postgres")
}

function groupWebServerVirtualHostsFromAPI(raw: any[]): VirtualHostEntry[] {
  if (!raw?.length) return []
  const grouped: Record<string, VirtualHostEntry> = {}
  const order: string[] = []

  for (const vh of raw) {
    const key = `${vh.domain}::${String(vh.is_default)}`
    if (!grouped[key]) {
      grouped[key] = {
        id: vh.id,
        domain: vh.domain,
        access_log: vh.access_log ?? "",
        error_log: vh.error_log ?? "",
        enabled: vh.enabled ?? true,
        is_shared_log: vh.is_shared_log ?? false,
        is_default: vh.is_default ?? false,
      }
      order.push(key)
    } else {
      if (!grouped[key].access_log && vh.access_log) grouped[key].access_log = vh.access_log
      if (!grouped[key].error_log && vh.error_log) grouped[key].error_log = vh.error_log
    }
  }

  return order.map((k) => grouped[k])
}

interface GuideStep {
  title: string
  description?: string
  code?: string
  text?: string   // rich pre-formatted content (not styled as a terminal block)
  note?: string
}

interface GuideData {
  intro: string
  steps?: GuideStep[]
  result?: string
}

const CollapsibleGuide = ({ title, icon: Icon, guide, content }: { title: string; icon: React.ComponentType<any>; guide: GuideData; content?: string }) => {
  const [open, setOpen] = useState(false)
  return (
    <div className="rounded-xl border border-border bg-card overflow-hidden transition-all">
      <button
        onClick={() => setOpen(!open)}
        className="w-full flex items-center gap-3 px-5 py-4 text-left hover:bg-muted/50 transition-colors"
      >
        <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-primary/80 to-primary flex items-center justify-center shrink-0">
          <Icon className="w-4 h-4 text-primary-foreground" />
        </div>
        <div className="flex-1">
          <h3 className="text-sm font-semibold text-foreground">{title}</h3>
          <p className="text-xs text-muted-foreground mt-0.5">{guide.intro}</p>
        </div>
        <ChevronDown
          className={`w-4 h-4 text-muted-foreground transition-transform duration-200 ${open ? "rotate-180" : ""}`}
        />
      </button>

      {open && (
        <div className="px-5 pb-5 pt-3 border-t border-border">
          {content ? (
            <MarkdownRenderer content={content} />
          ) : (
            <div className="space-y-4">
              {guide.steps?.map((step, i) => (
                <div key={i}>
                  <div className="flex items-center gap-3 mb-2">
                    <span className="flex items-center justify-center w-6 h-6 rounded-full bg-gradient-to-br from-primary/80 to-primary text-primary-foreground text-xs font-bold shrink-0">
                      {i + 1}
                    </span>
                    <h4 className="text-sm font-semibold text-foreground">{step.title}</h4>
                  </div>
                  {step.description && (
                    <p className="text-xs text-muted-foreground ml-9 mb-2">{step.description}</p>
                  )}
                  {step.text && (
                    <pre className="ml-9 text-xs text-foreground whitespace-pre-wrap leading-relaxed font-sans overflow-x-auto rounded-lg border border-border bg-background/50 p-3">
                      {step.text}
                    </pre>
                  )}
                  {step.code && (
                    <div className="ml-9 rounded-lg overflow-hidden border border-primary/20">
                      <div className="px-3 py-1.5 border-b border-primary/10 bg-primary/5 flex items-center gap-1.5">
                        <Terminal className="w-3 h-3 text-primary" />
                        <span className="text-[10px] text-primary uppercase tracking-wider font-semibold">Terminal</span>
                      </div>
                      <pre className="p-3 text-xs font-mono text-primary whitespace-pre-wrap overflow-x-auto leading-relaxed bg-[hsl(240,15%,5%)]">
                        {step.code}
                      </pre>
                    </div>
                  )}
                  {step.note && (
                    <p className="text-xs ml-9 mt-2 flex items-center gap-1.5 text-emerald-600 dark:text-emerald-400">
                      <CheckCircle className="w-3 h-3" />
                      {step.note}
                    </p>
                  )}
                </div>
              ))}
              {guide.result && (
                <div className="ml-9">
                  <p className="text-xs text-muted-foreground mb-2 font-medium">Expected output:</p>
                  <div className="rounded-lg overflow-hidden border border-emerald-600/20">
                    <div className="px-3 py-1.5 border-b border-emerald-600/10 bg-emerald-600/5">
                      <span className="text-[10px] text-emerald-600 dark:text-emerald-400 uppercase tracking-wider font-semibold">
                        Output
                      </span>
                    </div>
                    <pre className="p-3 text-xs font-mono text-emerald-600 dark:text-emerald-400 whitespace-pre-wrap overflow-x-auto bg-[hsl(240,15%,5%)]">
                      {guide.result}
                    </pre>
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  )
}

export default function ServiceConfigPage() {
  const { id = "" } = useParams()
  const navigate = useNavigate()
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
  const [nginxVirtualHosts, setNginxVirtualHosts] = useState<VirtualHostEntry[]>([])
  const [apacheVirtualHosts, setApacheVirtualHosts] = useState<VirtualHostEntry[]>([])
  const [copied, setCopied] = useState(false)
  const [configSaved, setConfigSaved] = useState(false)

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

      if (serviceQuery.data.service_name === "nginx") {
        setNginxVirtualHosts(
          groupWebServerVirtualHostsFromAPI((serviceQuery.data as any).nginx_virtual_hosts ?? [])
        )
      }
      if (serviceQuery.data.service_name === "apache") {
        setApacheVirtualHosts(
          groupWebServerVirtualHostsFromAPI((serviceQuery.data as any).apache_virtual_hosts ?? [])
        )
      }
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
    mutationFn: () => {
      const payload: any = {
        confirmed_log_path: confirmedLogPath,
        user_metrics_endpoint: metricsEndpoint,
        metrics_mode: metricsMode,
        is_java_app: isJavaApp,
        jmx_exporter_port: jmxExporterPort ? Number(jmxExporterPort) : undefined,
      }

      if (data?.service_name === "nginx") {
        payload.nginx_virtual_hosts = nginxVirtualHosts.map((vh) => ({
          id: vh.id,
          domain: vh.domain || "(default)",
          access_log: vh.access_log,
          error_log: vh.error_log,
          enabled: vh.enabled,
          is_shared_log: vh.is_shared_log,
          is_default: vh.is_default,
        }))
      }
      if (data?.service_name === "apache") {
        payload.apache_virtual_hosts = apacheVirtualHosts.map((vh) => ({
          id: vh.id,
          domain: vh.domain || "(default)",
          access_log: vh.access_log,
          error_log: vh.error_log,
          enabled: vh.enabled,
          is_shared_log: vh.is_shared_log,
          is_default: vh.is_default,
        }))
      }

      return updateDiscoveryServiceConfig(id, payload)
    },
    onSuccess: async () => {
      setConfigSaved(true)
      setTimeout(() => setConfigSaved(false), 2000)
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
  const canEditMetricsEndpoint =
    metricsMode === "prometheus" || metricsMode === "catalog" || metricsMode === "unknown"

  const metricsGuide = data?.catalog
    ? data.catalog.metrics_pattern === "builtin"
      ? adaptGuideToPort(data.catalog.metrics_guide, data.catalog.default_metrics_endpoint, data.port)
      : data.catalog.metrics_guide
    : ""

  const logsGuide = data?.catalog?.logs_guide ?? ""

  const handleCopy = () => {
    navigator.clipboard.writeText(installCommand)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const getStatusConfig = (status?: string) => {
    switch (status) {
      case "installed":
        return {
          label: "Integration Installed",
          icon: CheckCircle,
          dotClass: "bg-emerald-600",
          textClass: "text-emerald-600 dark:text-emerald-400",
          borderClass: "border-emerald-600/30 bg-emerald-600/10",
        }
      case "pending":
        return {
          label: "Integration Pending",
          icon: Clock,
          dotClass: "bg-amber-600",
          textClass: "text-amber-600 dark:text-amber-400",
          borderClass: "border-amber-600/30 bg-amber-600/10",
        }
      case "error":
        return {
          label: "Integration Error",
          icon: AlertTriangle,
          dotClass: "bg-red-600",
          textClass: "text-red-600 dark:text-red-400",
          borderClass: "border-red-600/30 bg-red-600/10",
        }
      default:
        return {
          label: "Not Installed",
          icon: XCircle,
          dotClass: "bg-muted-foreground",
          textClass: "text-muted-foreground",
          borderClass: "border-border bg-secondary",
        }
    }
  }

  const statusConfig = getStatusConfig(data?.integration_status)

  const infoItems = [
    { label: "Service", value: data?.service_name || "-", icon: Globe },
    { label: "Category", value: data?.category || "-", icon: Server },
    { label: "Hostname", value: data?.hostname || "-", icon: Server },
    { label: "Port", value: data?.port ? String(data.port) : "-", icon: Zap },
    { label: "Version", value: data?.version || "-", icon: FileText },
    { label: "Logging Mode", value: data?.logging_mode || "-", icon: Activity },
  ]

  if (serviceQuery.isLoading) {
    return (
      <div className="min-h-screen bg-background flex items-center justify-center">
        <div className="text-muted-foreground">Loading service details...</div>
      </div>
    )
  }

  if (!data) {
    return (
      <div className="min-h-screen bg-background flex items-center justify-center">
        <div className="text-center">
          <div className="text-muted-foreground">Service not found</div>
          <Link to="/integrations" className="text-sm text-primary mt-4 inline-block">
            Back to integrations
          </Link>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-background">
      {/* Header */}
      <header className="border-b border-border bg-gradient-to-r from-primary/5 via-card/80 to-card/80 backdrop-blur-sm px-6 py-5">
        <div className="max-w-4xl mx-auto flex items-center gap-4">
          <button
            onClick={() => navigate("/integrations")}
            className="p-2 rounded-lg text-muted-foreground hover:text-primary hover:bg-primary/10 transition-colors"
          >
            <ArrowLeft className="w-5 h-5" />
          </button>
          <div className="flex-1">
            <h1 className="text-xl font-bold text-foreground">{data.service_name} Configuration</h1>
            <p className="text-sm text-muted-foreground mt-0.5">
              {data.hostname} · <span className="font-mono text-xs text-primary/70">{data.machine_id.slice(0, 12)}...</span>
            </p>
          </div>
          <div className={`inline-flex items-center gap-2 px-3 py-1.5 rounded-full text-xs font-semibold border ${statusConfig.borderClass}`}>
            <span className={`w-2 h-2 rounded-full ${statusConfig.dotClass}`} />
            <span className={statusConfig.textClass}>{statusConfig.label}</span>
          </div>
        </div>
      </header>

      <main className="max-w-4xl mx-auto p-6 space-y-6">
        {/* Service Info — read-only display */}
        <div className="rounded-xl border border-border bg-gradient-to-br from-card via-card to-primary/[0.03] p-5">
          <h2 className="text-sm font-semibold text-foreground mb-4 flex items-center gap-2">
            <Server className="w-4 h-4 text-primary" />
            Service Information
          </h2>
          <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
            {infoItems.map((item) => (
              <div key={item.label} className="bg-background/50 rounded-lg px-4 py-3 border border-border/50">
                <p className="text-[11px] uppercase tracking-wider text-muted-foreground font-medium">{item.label}</p>
                <p className="text-sm text-foreground font-semibold mt-1 capitalize">{item.value}</p>
              </div>
            ))}
          </div>
        </div>

        {/* Configuration — editable inputs */}
        <div className="rounded-xl border border-primary/15 bg-card p-5 space-y-5">
          <h2 className="text-sm font-semibold text-foreground flex items-center gap-2">
            <Activity className="w-4 h-4 text-primary" />
            Configuration
          </h2>

          {/* Log path - only for services without virtual-host log mapping */}
          {data.service_name !== "nginx" && data.service_name !== "apache" ? (
            <div>
              <label className="text-xs font-medium text-foreground mb-1.5 block">Confirmed Log Path</label>
              <input
                value={confirmedLogPath}
                onChange={(e) => setConfirmedLogPath(e.target.value)}
                placeholder={data.suggested_log_path || data.detected_log_paths[0] || "/var/log/myapp/app.log"}
                className="w-full bg-background border border-border rounded-lg px-3 py-2.5 text-sm text-foreground font-mono focus:outline-none focus:border-primary focus:ring-1 focus:ring-primary/30 transition-colors"
              />
            </div>
          ) : null}

          {data.detected_log_paths.length && data.service_name !== "nginx" && data.service_name !== "apache" ? (
            <div>
              <label className="text-xs font-medium text-foreground mb-1.5 block">Detected Log Paths</label>
              <p className="text-xs text-muted-foreground mb-2">Click to select</p>
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

          {/* Metrics mode + endpoint */}
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="text-xs font-medium text-foreground mb-1.5 block">Metrics Mode</label>
              <select
                className="w-full bg-background border border-border rounded-lg px-3 py-2.5 text-sm text-foreground focus:outline-none focus:border-primary focus:ring-1 focus:ring-primary/30 transition-colors"
                value={metricsMode}
                onChange={(e) => setMetricsMode(e.target.value)}
              >
                {data.is_supported ? <option value="catalog">Use Catalog Defaults</option> : null}
                <option value="prometheus">Prometheus Endpoint</option>
                <option value="jmx">JMX Exporter</option>
                <option value="none">Logs Only</option>
                {!data.is_supported ? <option value="unknown">Not Configured Yet</option> : null}
              </select>
            </div>
            {canEditMetricsEndpoint ? (
              <div>
                <label className="text-xs font-medium text-foreground mb-1.5 block">Metrics Endpoint</label>
                <input
                  value={metricsEndpoint}
                  onChange={(e) => setMetricsEndpoint(e.target.value)}
                  placeholder={data.suggested_metrics_endpoint || "http://127.0.0.1:8080/metrics"}
                  className="w-full bg-background border border-border rounded-lg px-3 py-2.5 font-mono text-xs text-foreground focus:outline-none focus:border-primary focus:ring-1 focus:ring-primary/30 transition-colors placeholder:text-muted-foreground"
                />
              </div>
            ) : null}
          </div>

          {metricsMode === "jmx" ? (
            <div>
              <label className="text-xs font-medium text-foreground mb-1.5 block">JMX Exporter Port</label>
              <input
                value={jmxExporterPort}
                onChange={(e) => setJMXExporterPort(e.target.value)}
                placeholder="7070"
                className="w-full bg-background border border-border rounded-lg px-3 py-2.5 text-sm text-foreground font-mono focus:outline-none focus:border-primary focus:ring-1 focus:ring-primary/30 transition-colors"
              />
            </div>
          ) : null}

          <label className="flex items-center justify-between cursor-pointer bg-background rounded-lg px-4 py-3 border border-border hover:border-primary/30 transition-colors">
            <span className="text-sm text-foreground font-medium">Java Application</span>
            <button
              onClick={() => setIsJavaApp(!isJavaApp)}
              className={`w-11 h-6 rounded-full relative transition-colors ${
                isJavaApp ? "bg-gradient-to-r from-primary/80 to-primary" : "bg-secondary"
              }`}
            >
              <span
                className={`absolute top-0.5 left-0.5 w-5 h-5 rounded-full bg-foreground shadow-lg transition-transform ${
                  isJavaApp ? "translate-x-5" : ""
                }`}
              />
            </button>
          </label>

          {/* Collection toggles */}
          <div className="grid grid-cols-2 gap-4">
            <label className="flex items-center justify-between cursor-pointer bg-background rounded-lg px-4 py-3 border border-border hover:border-primary/30 transition-colors">
              <span className="text-sm text-foreground font-medium">Log Collection</span>
              <button
                onClick={() => setLogOn(!logOn)}
                className={`w-11 h-6 rounded-full relative transition-colors ${
                  logOn ? "bg-gradient-to-r from-primary/80 to-primary" : "bg-secondary"
                }`}
              >
                <span className={`absolute top-0.5 left-0.5 w-5 h-5 rounded-full bg-foreground shadow-lg transition-transform ${logOn ? "translate-x-5" : ""}`} />
              </button>
            </label>
            <label className="flex items-center justify-between cursor-pointer bg-background rounded-lg px-4 py-3 border border-border hover:border-primary/30 transition-colors">
              <span className="text-sm text-foreground font-medium">Metrics Collection</span>
              <button
                onClick={() => setMetricsOn(!metricsOn)}
                className={`w-11 h-6 rounded-full relative transition-colors ${
                  metricsOn ? "bg-gradient-to-r from-primary/80 to-primary" : "bg-secondary"
                }`}
              >
                <span className={`absolute top-0.5 left-0.5 w-5 h-5 rounded-full bg-foreground shadow-lg transition-transform ${metricsOn ? "translate-x-5" : ""}`} />
              </button>
            </label>
          </div>

          <button
            disabled={configMutation.isPending}
            onClick={() => configMutation.mutate()}
            className="w-full bg-gradient-to-r from-primary/80 to-primary text-primary-foreground py-3 rounded-lg text-sm font-bold hover:opacity-90 transition-opacity disabled:opacity-50"
          >
            {configMutation.isPending ? "Saving..." : configSaved ? "✓ Configuration Saved" : "Save Configuration"}
          </button>
        </div>

        {/* Nginx Virtual Hosts — only shown for nginx services */}
        {data.service_name === "nginx" ? (
          <div className="rounded-xl border border-border bg-card p-5">
            <h2 className="text-sm font-semibold text-foreground mb-4 flex items-center gap-2">
              <Globe className="w-4 h-4 text-primary" />
              Virtual Hosts (Domains)
            </h2>
            <p className="text-xs text-muted-foreground mb-4">
              Map each domain to its log files. The agent auto-detects these from your Nginx config.
            </p>
            <NginxVirtualHostsSection hosts={nginxVirtualHosts} onChange={setNginxVirtualHosts} />
          </div>
        ) : null}

        {/* Apache Virtual Hosts — only shown for apache services */}
        {data.service_name === "apache" ? (
          <div className="rounded-xl border border-border bg-card p-5">
            <h2 className="text-sm font-semibold text-foreground mb-4 flex items-center gap-2">
              <Globe className="w-4 h-4 text-primary" />
              Virtual Hosts (Domains)
            </h2>
            <p className="text-xs text-muted-foreground mb-4">
              Map each Apache virtual host to its access and error log files.
            </p>
            <ApacheVirtualHostsSection hosts={apacheVirtualHosts} onChange={setApacheVirtualHosts} />
          </div>
        ) : null}

        {/* Guide — collapsible */}
        {(logsGuide || metricsGuide) && (
          <div className="space-y-3">
            <h2 className="text-sm font-semibold text-foreground flex items-center gap-2">
              <BookOpen className="w-4 h-4 text-primary" />
              Setup Guide
            </h2>
            <CollapsibleGuide
              title={logsGuide && metricsGuide ? "Configure Logs & Metrics" : logsGuide ? "Configure Logs" : "Configure Metrics"}
              icon={BookOpen}
              guide={{
                intro: logsGuide && metricsGuide
                  ? "Enable log collection and metrics exposure for this service"
                  : logsGuide
                    ? "Configure log collection for this service"
                    : "Configure metrics exposure for this service",
              }}
              content={[logsGuide, metricsGuide].filter(Boolean).join("\n\n---\n\n")}
            />
          </div>
        )}

        {/* Install Integration Script */}
        <div className="rounded-xl border border-primary/20 bg-gradient-to-br from-card via-card to-primary/[0.05] p-5">
          <h2 className="text-sm font-semibold text-foreground mb-1 flex items-center gap-2">
            <Terminal className="w-4 h-4 text-primary" />
            Install Integration Script
          </h2>
          <p className="text-xs text-muted-foreground mb-4">
            Generate a host-specific command, then run it on <span className="font-semibold text-primary">{data.hostname}</span> to
            activate collection using the confirmed log paths and metrics settings above.
          </p>
          {!installCommand ? (
            <button
              onClick={() => installMutation.mutate()}
              className="w-full py-3 rounded-lg border-2 border-dashed border-primary/30 text-primary font-semibold text-sm hover:border-primary/60 hover:bg-primary/5 transition-all flex items-center justify-center gap-2"
            >
              <Terminal className="w-4 h-4" />
              {installMutation.isPending ? "Generating..." : "Generate Integration Command"}
            </button>
          ) : null}
          {installCommand && (
            <div className="relative rounded-lg overflow-hidden border border-primary/20">
              <div className="px-3 py-1.5 border-b border-primary/10 bg-primary/5 flex items-center gap-1.5">
                <Terminal className="w-3 h-3 text-primary" />
                <span className="text-[10px] text-primary uppercase tracking-wider font-semibold">Run on {data.hostname}</span>
              </div>
              <pre className="p-4 text-xs text-primary font-mono whitespace-pre-wrap break-all overflow-x-auto bg-[hsl(240,15%,5%)] leading-relaxed">
                {installCommand}
              </pre>
              <button
                onClick={handleCopy}
                className="absolute top-9 right-2 px-3 py-1.5 rounded-md bg-gradient-to-r from-primary/80 to-primary text-primary-foreground text-xs font-semibold hover:opacity-90 transition-opacity flex items-center gap-1.5"
              >
                {copied ? (
                  <>
                    <Check className="w-3 h-3" /> Copied
                  </>
                ) : (
                  <>
                    <Copy className="w-3 h-3" /> Copy
                  </>
                )}
              </button>
            </div>
          )}
        </div>

        {/* Vulnerability Scan Status + CVE */}
        <div className="rounded-xl border border-border bg-card p-5 space-y-4">
          <h2 className="text-sm font-semibold text-foreground flex items-center gap-2">
            <Shield className="w-4 h-4 text-primary" />
            Version Security
          </h2>

          {/* Scan status */}
          <div className="flex items-center gap-4 bg-background/50 rounded-lg px-4 py-3 border border-border/50">
            <div>
              <p className="text-[11px] uppercase tracking-wider text-muted-foreground font-medium">Scan Status</p>
              <p className="text-sm font-semibold text-emerald-600 dark:text-emerald-400 flex items-center gap-1.5 mt-0.5">
                <CheckCircle className="w-3.5 h-3.5" />
                {data.vuln_scan_status === "scanned" ? "Scanned" : "Not scanned"}
              </p>
            </div>
            {data.last_vuln_scan_at && (
              <div className="border-l border-border pl-4">
                <p className="text-[11px] uppercase tracking-wider text-muted-foreground font-medium">Last Scan</p>
                <p className="text-sm text-foreground mt-0.5">{formatScanTime(data.last_vuln_scan_at)}</p>
              </div>
            )}
          </div>

          {/* Vulnerabilities list */}
          {data.vulnerabilities && data.vulnerabilities.length > 0 ? (
            <div className="space-y-3">
              {data.vulnerabilities.map((cve) => {
                const isHigh = cve.alert_level === "alert"
                return (
                  <div
                    key={cve.cve_id}
                    className={`rounded-lg p-4 border-l-4 ${
                      isHigh
                        ? "border-l-red-600 bg-red-600/[0.04]"
                        : "border-l-amber-600 bg-amber-600/[0.04]"
                    }`}
                  >
                    <div className="flex items-center justify-between mb-2">
                      <span className="font-mono text-sm text-primary font-bold">{cve.cve_id}</span>
                      <div className="flex items-center gap-2">
                        <span
                          className={`text-xs font-bold uppercase px-2.5 py-1 rounded-full ${
                            isHigh
                              ? "bg-red-600/15 text-red-600 dark:text-red-400"
                              : "bg-amber-600/15 text-amber-600 dark:text-amber-400"
                          }`}
                        >
                          {cve.alert_level}
                        </span>
                        <span className={`text-xs font-bold ${isHigh ? "text-red-600 dark:text-red-400" : "text-amber-600 dark:text-amber-400"}`}>
                          CVSS {cve.cvss_score.toFixed(1)}
                        </span>
                        <span className="text-xs text-emerald-600 dark:text-emerald-400 font-semibold">
                          Fix {cve.fixed_in_version || "-"}
                        </span>
                      </div>
                    </div>
                    <p className="text-xs text-muted-foreground leading-relaxed">{cve.description || "No description available."}</p>
                  </div>
                )
              })}
            </div>
          ) : null}

          {data.vuln_scan_status === "scanned" && (!data.vulnerabilities || data.vulnerabilities.length === 0) ? (
            <div className="flex items-center gap-2 text-emerald-600 dark:text-emerald-400 text-sm font-semibold bg-emerald-600/10 rounded-lg px-4 py-3">
              <CheckCircle className="w-4 h-4" />
              No known vulnerabilities found for v{data.version}
            </div>
          ) : null}
        </div>

        {/* Collection Controls */}
        <div className="rounded-xl border border-border bg-card p-5">
          <h2 className="text-sm font-semibold text-foreground mb-4 flex items-center gap-2">
            <Activity className="w-4 h-4 text-primary" />
            Collection Controls
          </h2>
          <div className="space-y-3">
            <Button disabled={saveMutation.isPending} onClick={() => saveMutation.mutate()} className="w-full">
              {saveMutation.isPending ? "Saving..." : "Save Integration Settings"}
            </Button>

            {data.service_name === "nginx" && logOn && metricsOn ? (
              <Link
                to={`/integrations/nginx-dashboard/${id}`}
                className="inline-block w-full text-center rounded-md bg-emerald-600 px-3 py-2 text-sm font-semibold text-white hover:bg-emerald-500 transition-colors"
              >
                Open Nginx Dashboard
              </Link>
            ) : null}
            {data.service_name === "apache" && logOn && metricsOn ? (
              <Link
                to={`/integrations/apache-dashboard/${id}`}
                className="inline-block w-full text-center rounded-md bg-emerald-600 px-3 py-2 text-sm font-semibold text-white hover:bg-emerald-500 transition-colors"
              >
                Open Apache Dashboard
              </Link>
            ) : null}
            {isPostgresService(data.service_name) && logOn && metricsOn ? (
              <div className="grid gap-2 sm:grid-cols-2">
                <Link
                  to={`/integrations/postgres-dashboard/${id}`}
                  className="inline-block w-full text-center rounded-md bg-sky-600 px-3 py-2 text-sm font-semibold text-white hover:bg-sky-500 transition-colors"
                >
                  Open PostgreSQL Dashboard
                </Link>
                <Link
                  to={`/integrations/postgres-logs/${id}`}
                  className="inline-block w-full text-center rounded-md border border-border px-3 py-2 text-sm font-semibold hover:bg-muted transition-colors"
                >
                  Open PostgreSQL Logs
                </Link>
              </div>
            ) : null}
          </div>
        </div>
      </main>
    </div>
  )
}
