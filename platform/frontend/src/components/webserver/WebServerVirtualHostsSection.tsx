// src/components/webserver/WebServerVirtualHostsSection.tsx
//
// Drop-in replacement for the virtual hosts SectionCard in ServiceConfigPage.tsx
// Usage:
//   import WebServerVirtualHostsSection from "@/components/webserver/WebServerVirtualHostsSection"
//   <WebServerVirtualHostsSection service="nginx" hosts={virtualHosts} onChange={setVirtualHosts} />

import { Globe, FileText, Trash2, Plus, AlertTriangle, CheckCircle2, Share2 } from "lucide-react"
import { cn } from "@/lib/utils"

export type VirtualHostEntry = {
  id?: number
  domain: string
  access_log: string
  error_log: string
  enabled: boolean
  is_shared_log: boolean
  is_default: boolean
}

type Props = {
  hosts: VirtualHostEntry[]
  onChange: (hosts: VirtualHostEntry[]) => void
  service?: "nginx" | "apache"
  hideGuide?: boolean
}

function LogFormatWarning({ hasSharedLog, service }: { hasSharedLog: boolean; service: "nginx" | "apache" }) {
  const isApache = service === "apache"
  const displayName = isApache ? "Apache" : "Nginx"

  return (
    <div className="rounded-lg border border-amber-300/40 bg-amber-500/8 p-4 text-sm">
      <div className="flex items-start gap-3">
        <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-amber-500" />
        <div className="grid gap-2">
          <div className="font-medium text-amber-700 dark:text-amber-300">
            Required: Add <code className="rounded bg-amber-100/60 px-1 dark:bg-amber-900/40">host=</code> to your {displayName} log format
          </div>
          <div className="text-muted-foreground leading-relaxed">
            By default, {displayName} does not log which domain a request belongs to. You need to add a custom log format.
          </div>

          {/* Step 1 */}
          <div className="grid gap-1 mt-1">
            <div className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              {isApache ? "Step 1 — Add to apache2.conf below LogFormat definitions" : 'Step 1 — Add to nginx.conf inside http {}'}
            </div>
            <pre className="rounded-lg bg-muted/50 border border-border p-3 text-xs leading-relaxed overflow-x-auto">
{isApache ? `LogFormat "%a - %u %t \\"%r\\" %>s %B \\"%{Referer}i\\" \\"%{User-Agent}i\\" host=%v rt=%{ms}T" platform` : `log_format observability '$remote_addr - $remote_user [$time_local] '
                         '"$request" $status $body_bytes_sent '
                         '"$http_referer" "$http_user_agent" '
                         'host=$host '
                         'rt=$request_time '
                         'urt=$upstream_response_time';`}
            </pre>
          </div>

          {/* Step 2 */}
          <div className="grid gap-1">
            <div className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              {isApache ? "Step 2 — Apply to each VirtualHost" : "Step 2 — Apply to each site in sites-enabled/"}
            </div>
            <pre className="rounded-lg bg-muted/50 border border-border p-3 text-xs leading-relaxed overflow-x-auto">
{isApache ? `<VirtualHost *:80>
    ServerName shop.example.com
    CustomLog \${APACHE_LOG_DIR}/shop.access.log platform
    ErrorLog  \${APACHE_LOG_DIR}/shop.error.log
</VirtualHost>` : `server {
    server_name shop.example.com;
    access_log /var/log/nginx/shop.access.log observability;
}`}
            </pre>
          </div>

          {/* Step 3 */}
          <div className="grid gap-1">
            <div className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">Step 3 — Reload {displayName}</div>
            <pre className="rounded-lg bg-muted/50 border border-border p-3 text-xs leading-relaxed overflow-x-auto">
{isApache ? `sudo apache2ctl configtest && sudo systemctl restart apache2` : `sudo nginx -t && sudo systemctl reload nginx`}
            </pre>
          </div>

          {hasSharedLog && (
            <div className="flex items-start gap-2 rounded-md border border-amber-300/30 bg-amber-500/5 p-3 text-xs text-amber-700 dark:text-amber-300 mt-1">
              <Share2 className="mt-0.5 h-3 w-3 shrink-0" />
              <span>
                You have rows marked as <strong>Shared log</strong>. The <code className="rounded bg-amber-100/50 px-1 dark:bg-amber-900/30">host=</code> field
                is especially critical here — without it we cannot split traffic between domains that share one file.
              </span>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

function VirtualHostRow({
  vh,
  index,
  service,
  onChange,
  onRemove,
}: {
  vh: VirtualHostEntry
  index: number
  service: "nginx" | "apache"
  onChange: (index: number, updated: VirtualHostEntry) => void
  onRemove: (index: number) => void
}) {
  const update = (patch: Partial<VirtualHostEntry>) => onChange(index, { ...vh, ...patch })

  return (
    <div
      className={cn(
        "grid gap-3 rounded-xl border p-4 transition-colors",
        vh.enabled
          ? "border-border bg-card/40"
          : "border-border/50 bg-muted/20 opacity-60"
      )}
    >
      {/* Header row: domain + badges + controls */}
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex flex-1 items-center gap-2 min-w-[200px]">
          <Globe className="h-4 w-4 shrink-0 text-muted-foreground" />
          <input
            type="text"
            className="flex-1 rounded-md border border-input bg-background/70 px-3 py-1.5 text-sm font-medium outline-none focus:border-foreground/40 transition-colors"
            value={vh.domain}
            onChange={(e) => update({ domain: e.target.value })}
            placeholder="shop.example.com"
          />
        </div>

        <div className="flex items-center gap-3 flex-wrap">
          {/* Enabled toggle */}
          <label className="flex cursor-pointer items-center gap-1.5 text-xs text-muted-foreground select-none">
            <div
              className={cn(
                "relative h-5 w-9 rounded-full transition-colors",
                vh.enabled ? "bg-emerald-500/80" : "bg-muted"
              )}
              onClick={() => update({ enabled: !vh.enabled })}
            >
              <div
                className={cn(
                  "absolute top-0.5 h-4 w-4 rounded-full bg-white shadow transition-transform",
                  vh.enabled ? "translate-x-4" : "translate-x-0.5"
                )}
              />
            </div>
            Collect
          </label>

          {/* Shared log toggle */}
          <label className="flex cursor-pointer items-center gap-1.5 text-xs text-muted-foreground select-none" title="Multiple domains write to this same log file. We'll parse host= to separate them.">
            <input
              type="checkbox"
              className="h-3.5 w-3.5 rounded"
              checked={vh.is_shared_log}
              onChange={(e) => update({ is_shared_log: e.target.checked })}
            />
            Shared log
          </label>

          {/* Default server badge */}
          <label className="flex cursor-pointer items-center gap-1.5 text-xs text-muted-foreground select-none" title="This is the catch-all server block (no matching domain)">
            <input
              type="checkbox"
              className="h-3.5 w-3.5 rounded"
              checked={vh.is_default}
              onChange={(e) => update({ is_default: e.target.checked })}
            />
            Default
          </label>

          {/* Remove */}
          <button
            type="button"
            onClick={() => onRemove(index)}
            className="rounded-md p-1.5 text-muted-foreground hover:bg-destructive/10 hover:text-destructive transition-colors"
            title="Remove this virtual host"
          >
            <Trash2 className="h-3.5 w-3.5" />
          </button>
        </div>
      </div>

      {/* Log paths */}
      <div className="grid gap-2 pl-6 sm:grid-cols-2">
        <label className="grid gap-1">
          <span className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <FileText className="h-3 w-3" />
            Access log
          </span>
          <input
            type="text"
            className="h-9 rounded-md border border-input bg-background/60 px-3 text-xs font-mono outline-none focus:border-foreground/40 transition-colors"
            value={vh.access_log}
            onChange={(e) => update({ access_log: e.target.value })}
            placeholder={service === "apache" ? "/var/log/apache2/shop.access.log" : "/var/log/nginx/shop.access.log"}
          />
        </label>
        <label className="grid gap-1">
          <span className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <FileText className="h-3 w-3" />
            Error log
          </span>
          <input
            type="text"
            className="h-9 rounded-md border border-input bg-background/60 px-3 text-xs font-mono outline-none focus:border-foreground/40 transition-colors"
            value={vh.error_log}
            onChange={(e) => update({ error_log: e.target.value })}
            placeholder={service === "apache" ? "/var/log/apache2/shop.error.log" : "/var/log/nginx/shop.error.log"}
          />
        </label>
      </div>

      {/* Badges */}
      <div className="flex flex-wrap gap-2 pl-6">
        {vh.is_shared_log && (
          <span className="inline-flex items-center gap-1 rounded-full border border-amber-300/40 bg-amber-500/10 px-2 py-0.5 text-xs text-amber-700 dark:text-amber-300">
            <Share2 className="h-3 w-3" /> Shared — requires host= field
          </span>
        )}
        {vh.is_default && (
          <span className="inline-flex items-center gap-1 rounded-full border border-border bg-muted/60 px-2 py-0.5 text-xs text-muted-foreground">
            Catch-all default
          </span>
        )}
        {!vh.access_log && !vh.error_log && (
          <span className="inline-flex items-center gap-1 rounded-full border border-orange-300/40 bg-orange-500/10 px-2 py-0.5 text-xs text-orange-600 dark:text-orange-400">
            <AlertTriangle className="h-3 w-3" /> No log paths set
          </span>
        )}
        {(vh.access_log || vh.error_log) && vh.enabled && (
          <span className="inline-flex items-center gap-1 rounded-full border border-emerald-300/40 bg-emerald-500/10 px-2 py-0.5 text-xs text-emerald-700 dark:text-emerald-300">
            <CheckCircle2 className="h-3 w-3" /> Ready to collect
          </span>
        )}
      </div>
    </div>
  )
}

export default function WebServerVirtualHostsSection({ hosts, onChange, service = "nginx", hideGuide = false }: Props) {
  const hasSharedLog = hosts.some((h) => h.is_shared_log)
  const displayName = service === "apache" ? "Apache" : "Nginx"

  const handleChange = (index: number, updated: VirtualHostEntry) => {
    const next = [...hosts]
    next[index] = updated
    onChange(next)
  }

  const handleRemove = (index: number) => {
    onChange(hosts.filter((_, i) => i !== index))
  }

  const handleAdd = () => {
    onChange([
      ...hosts,
      {
        domain: "",
        access_log: "",
        error_log: "",
        enabled: true,
        is_shared_log: false,
        is_default: false,
      },
    ])
  }

  return (
    <div className="grid gap-4">
      {!hideGuide && <LogFormatWarning hasSharedLog={hasSharedLog} service={service} />}

      {hosts.length === 0 ? (
        <div className="rounded-xl border border-dashed border-border bg-muted/10 py-10 text-center text-sm text-muted-foreground">
          <Globe className="mx-auto mb-3 h-8 w-8 opacity-30" />
          <div>No virtual hosts detected yet.</div>
          <div className="mt-1 text-xs">The agent reads your {displayName} site configs on the next discovery run, or add one manually below.</div>
        </div>
      ) : (
        <div className="grid gap-3">
          {hosts.map((vh, idx) => (
            <VirtualHostRow
              key={idx}
              vh={vh}
              index={idx}
              service={service}
              onChange={handleChange}
              onRemove={handleRemove}
            />
          ))}
        </div>
      )}

      <button
        type="button"
        onClick={handleAdd}
        className="flex items-center gap-2 rounded-lg border border-dashed border-border bg-background/50 px-4 py-3 text-sm text-muted-foreground transition-colors hover:border-foreground/30 hover:bg-muted/30 hover:text-foreground"
      >
        <Plus className="h-4 w-4" />
        Add virtual host manually
      </button>

      {!hideGuide && (
        <div className="rounded-lg border border-border bg-muted/20 p-3 text-xs text-muted-foreground leading-relaxed">
          <strong>How this works:</strong> Each row is one domain (virtual host). Per-domain log files let us show you separate dashboards per domain.
          If multiple domains share one log file, mark it as "Shared log" — we'll parse the <code className="rounded bg-muted px-1">host=</code> field to split traffic.
          The agent auto-detects this table by reading your {displayName} site configs.
        </div>
      )}
    </div>
  )
}

export { LogFormatWarning }
