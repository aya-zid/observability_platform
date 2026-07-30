import { useState } from "react"
import { useQuery } from "@tanstack/react-query"
import {
  BookOpen, Globe, Server, Database, Cpu, Zap, Box, ChevronRight, FileText,
} from "lucide-react"
import { getCatalog } from "@/api/discovery"
import type { CatalogService } from "@/api/discovery"
import MarkdownRenderer from "@/components/MarkdownRenderer"
import { cn } from "@/lib/utils"

const serviceMeta: Record<string, { icon: React.ComponentType<{ className?: string }>; desc: string }> = {
  nginx: { icon: Globe, desc: "Web server & reverse proxy" },
  apache: { icon: Server, desc: "Web server" },
  postgresql: { icon: Database, desc: "Relational database" },
  mariadb: { icon: Box, desc: "MySQL-compatible database" },
  phpfpm: { icon: Cpu, desc: "PHP application server" },
  redis: { icon: Zap, desc: "In-memory cache & store" },
}

const categoryOrder = ["web", "database", "app", "cache"]

export default function DocsPage() {
  const [activeDoc, setActiveDoc] = useState("introduction")

  const { data: catalog } = useQuery({
    queryKey: ["catalog"],
    queryFn: getCatalog,
  })

  const services = (catalog || [])
    .filter((s) => serviceMeta[s.name])
    .sort((a, b) => {
      const aIdx = categoryOrder.indexOf(a.category)
      const bIdx = categoryOrder.indexOf(b.category)
      return (aIdx === -1 ? 99 : aIdx) - (bIdx === -1 ? 99 : bIdx)
    })

  const activeService = services.find((s) => s.name === activeDoc) || null

  return (
    <div className="flex-1 overflow-hidden flex flex-col">
      <header className="border-b border-border bg-card/50 backdrop-blur-sm px-6 py-4 shrink-0">
        <h1 className="text-xl font-bold text-foreground flex items-center gap-2">
          <BookOpen className="w-5 h-5 text-primary" /> Documentation
        </h1>
        <p className="text-sm text-muted-foreground mt-0.5">
          How to configure your services for the observability platform
        </p>
      </header>

      <div className="flex-1 flex overflow-hidden">
        <nav className="w-56 shrink-0 border-r border-border overflow-y-auto p-3 space-y-1">
          <button
            onClick={() => setActiveDoc("introduction")}
            className={cn(
              "w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm text-left transition-colors",
              activeDoc === "introduction"
                ? "bg-primary/10 text-primary font-semibold"
                : "text-muted-foreground hover:text-foreground hover:bg-muted/50"
            )}
          >
            <BookOpen className={cn("w-4 h-4 shrink-0", activeDoc === "introduction" ? "text-primary" : "text-muted-foreground")} />
            <div className="flex-1 min-w-0">
              <div className={cn("truncate", activeDoc === "introduction" && "text-primary")}>
                Introduction
              </div>
              <div className="text-sm text-muted-foreground/70 truncate">
                Overview & prerequisites
              </div>
            </div>
            {activeDoc === "introduction" && <ChevronRight className="w-3.5 h-3.5 text-primary shrink-0" />}
          </button>

          <div className="h-px bg-border my-2" />

          {services.map((s) => {
            const meta = serviceMeta[s.name]
            const isActive = activeDoc === s.name
            return (
              <button
                key={s.name}
                onClick={() => setActiveDoc(s.name)}
                className={cn(
                  "w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm text-left transition-colors",
                  isActive
                    ? "bg-primary/10 text-primary font-semibold"
                    : "text-muted-foreground hover:text-foreground hover:bg-muted/50"
                )}
              >
                <meta.icon className={cn("w-4 h-4 shrink-0", isActive ? "text-primary" : "text-muted-foreground")} />
                <div className="flex-1 min-w-0">
                  <div className={cn("truncate", isActive && "text-primary")}>
                    {s.display_name}
                  </div>
                  <div className="text-sm text-muted-foreground/70 truncate">
                    {meta.desc}
                  </div>
                </div>
                {isActive && <ChevronRight className="w-3.5 h-3.5 text-primary shrink-0" />}
              </button>
            )
          })}
        </nav>

        <div className="flex-1 overflow-y-auto">
          <div className="max-w-4xl mx-auto p-6 md:p-8">
            {activeDoc === "introduction" ? (
              <Introduction />
            ) : activeService ? (
              <ServiceDoc service={activeService} />
            ) : null}
          </div>
        </div>
      </div>
    </div>
  )
}

function Introduction() {
  return (
    <div className="space-y-8">
      <div>
        <div className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full bg-primary/10 text-primary text-sm font-semibold mb-4">
          <BookOpen className="w-3 h-3" /> Getting Started
        </div>
        <h2 className="text-2xl font-bold text-foreground tracking-tight mb-3">
          Before you install the agent
        </h2>
        <p className="text-sm text-muted-foreground leading-relaxed">
          The observability platform automatically discovers services running on your hosts and collects their
          metrics and logs. But for the platform to show you meaningful data — dashboards, alerts, logs — each
          service must be configured so the agent can reach it.
        </p>
        <p className="text-sm text-muted-foreground leading-relaxed mt-3">
          This documentation walks you through every service the platform supports. Each guide explains what
          needs to be enabled, which files to configure, and what commands to run. Follow the guide for your
          service <strong className="text-foreground font-semibold">before</strong> installing the agent so
          that everything works on the first scan.
        </p>
      </div>

      <div className="rounded-xl border border-border bg-card shadow-card overflow-hidden">
        <div className="bg-primary/5 px-5 py-3 border-b border-border">
          <h3 className="text-sm font-semibold text-foreground">What each guide covers</h3>
        </div>
        <div className="p-5">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            {[
              { icon: Globe, label: "Nginx", desc: "Enable stub_status, configure per-vhost logs" },
              { icon: Server, label: "Apache", desc: "Enable mod_status, add custom log format, per-vhost logs" },
              { icon: Database, label: "PostgreSQL", desc: "Configure logging, pg_stat_statements, exporter setup" },
              { icon: Box, label: "MariaDB", desc: "Enable performance_schema, slow query log, exporter setup" },
              { icon: Cpu, label: "PHP-FPM", desc: "Configure pool status endpoints, access/slow logs, exporter" },
              { icon: Zap, label: "Redis", desc: "Set log path, log level, slowlog, exporter setup" },
            ].map((item) => (
              <div key={item.label} className="flex items-center gap-3 bg-muted/50 rounded-lg p-3">
                <div className="w-8 h-8 rounded-lg bg-primary/10 flex items-center justify-center shrink-0">
                  <item.icon className="w-4 h-4 text-primary" />
                </div>
                <div>
                  <span className="text-sm font-semibold text-foreground">{item.label}</span>
                  <p className="text-sm text-muted-foreground">{item.desc}</p>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>

      <div className="rounded-xl border border-border bg-card shadow-card p-5">
        <h3 className="text-sm font-semibold text-foreground mb-2">Why follow the guides?</h3>
        <ul className="space-y-2 text-sm text-muted-foreground">
          <li className="flex items-start gap-2">
            <ChevronRight className="w-3.5 h-3.5 text-primary shrink-0 mt-0.5" />
            The agent discovers services automatically but cannot enable internal modules or edit config files
          </li>
          <li className="flex items-start gap-2">
            <ChevronRight className="w-3.5 h-3.5 text-primary shrink-0 mt-0.5" />
            Some metrics endpoints (stub_status, mod_status) must be explicitly enabled and locked to localhost
          </li>
          <li className="flex items-start gap-2">
            <ChevronRight className="w-3.5 h-3.5 text-primary shrink-0 mt-0.5" />
            Per-service log files let the platform route logs to the correct dashboard automatically
          </li>
          <li className="flex items-start gap-2">
            <ChevronRight className="w-3.5 h-3.5 text-primary shrink-0 mt-0.5" />
            Exporters (for databases, cache, PHP-FPM) must be installed and running before the agent scans
          </li>
        </ul>
      </div>
    </div>
  )
}

function ServiceDoc({ service }: { service: CatalogService }) {
  const meta = serviceMeta[service.name]
  const hasMetricsGuide = service.metrics_guide?.trim().length > 0
  const hasLogsGuide = service.logs_guide?.trim().length > 0

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3 pb-4 border-b border-border">
        <div className="w-10 h-10 rounded-xl bg-primary/10 flex items-center justify-center shrink-0">
          {meta ? <meta.icon className="w-5 h-5 text-primary" /> : <FileText className="w-5 h-5 text-primary" />}
        </div>
        <div>
          <h2 className="text-xl font-bold text-foreground tracking-tight">{service.display_name}</h2>
          <p className="text-sm text-muted-foreground">{service.category} {meta?.desc ? `— ${meta.desc}` : ""}</p>
        </div>
      </div>

      {hasMetricsGuide && (
        <div className="rounded-xl border border-border bg-card shadow-card p-4 md:p-6">
          <div className="flex items-center gap-2 mb-4 pb-3 border-b border-border">
            <div className="w-7 h-7 rounded-lg bg-primary/10 flex items-center justify-center">
              <svg className="w-3.5 h-3.5 text-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
              </svg>
            </div>
            <h3 className="text-sm font-semibold text-foreground">Metrics Configuration</h3>
          </div>
          <MarkdownRenderer content={service.metrics_guide} />
        </div>
      )}

      {hasLogsGuide && (
        <div className="rounded-xl border border-border bg-card shadow-card p-4 md:p-6">
          <div className="flex items-center gap-2 mb-4 pb-3 border-b border-border">
            <div className="w-7 h-7 rounded-lg bg-primary/10 flex items-center justify-center">
              <svg className="w-3.5 h-3.5 text-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
            </div>
            <h3 className="text-sm font-semibold text-foreground">Log Collection</h3>
          </div>
          <MarkdownRenderer content={service.logs_guide} />
        </div>
      )}

      {!hasMetricsGuide && !hasLogsGuide && (
        <div className="rounded-xl border border-dashed border-border bg-card/50 p-8 text-center">
          <FileText className="w-8 h-8 text-muted-foreground/50 mx-auto mb-2" />
          <p className="text-sm text-muted-foreground">No guide available for this service yet.</p>
        </div>
      )}
    </div>
  )
}
