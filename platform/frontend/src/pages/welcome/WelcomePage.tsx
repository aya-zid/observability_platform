import { useState } from "react"
import { Link } from "react-router-dom"
import {
  Activity, ArrowRight, Server, Box, Bell, ShieldCheck, GitBranch,
  LineChart, Database, Eye, Cpu, Layers, Sparkles, Check, Terminal,
  AlertTriangle, Github, Twitter, Linkedin,
  Cloud, HardDrive, BarChart3, Search,
  Menu, X, Radio,
} from "lucide-react"
import ThemeToggle from "@/components/shared/ThemeToggle"
import { Button } from "@/components/ui/button"
import "@/styles/landing.css"

function NavLink({ href, label }: { href: string; label: string }) {
  return (
    <a
      href={href}
      className="relative text-sm text-muted-foreground hover:text-foreground transition-colors after:absolute after:-bottom-1 after:left-0 after:h-[2px] after:w-0 after:rounded-full after:bg-primary after:transition-all hover:after:w-full"
    >
      {label}
    </a>
  )
}

function FeatureCard({
  icon: Icon,
  title,
  desc,
}: {
  icon: typeof Activity
  title: string
  desc: string
}) {
  return (
    <div className="group relative overflow-hidden rounded-xl border border-border bg-card p-6 transition-all duration-300 hover:border-accent/40 hover:shadow-elevated hover:-translate-y-0.5">
      <div className="absolute inset-0 bg-gradient-to-br from-accent/5 via-transparent to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-500" />
      <div className="relative">
        <div className="w-10 h-10 rounded-lg bg-accent/10 flex items-center justify-center mb-4 group-hover:bg-accent/15 group-hover:scale-110 transition-all duration-300">
          <Icon className="w-5 h-5 text-accent" />
        </div>
        <h3 className="font-semibold text-base">{title}</h3>
        <p className="mt-2 text-sm text-muted-foreground leading-relaxed">{desc}</p>
      </div>
    </div>
  )
}

function StepCard({
  number,
  icon: Icon,
  title,
  desc,
  code,
}: {
  number: string
  icon: typeof Terminal
  title: string
  desc: string
  code: string
}) {
  return (
    <div className="relative rounded-xl border border-border bg-card p-6 transition-all duration-300 hover:border-primary/30 hover:shadow-elevated">
      <div className="flex items-center justify-between mb-4">
        <span className="text-xs font-mono text-muted-foreground/60">{number}</span>
        <div className="w-9 h-9 rounded-lg bg-primary/10 flex items-center justify-center">
          <Icon className="w-4.5 h-4.5 text-primary" />
        </div>
      </div>
      <h3 className="font-semibold text-lg">{title}</h3>
      <p className="mt-2 text-sm text-muted-foreground leading-relaxed">{desc}</p>
      <div className="mt-5 rounded-lg bg-background border border-border px-3 py-2.5 font-mono text-[11px] text-foreground/80 overflow-x-auto">
        {code}
      </div>
    </div>
  )
}

const THROUGHPUT_PATH = "M0,80 L40,70 L80,75 L120,50 L160,60 L200,35 L240,45 L280,25 L320,40 L360,20 L400,30"
const THROUGHPUT_AREA = `${THROUGHPUT_PATH} L400,120 L0,120 Z`

export default function WelcomePage() {
  const [mobileOpen, setMobileOpen] = useState(false)

  return (
    <div className="landing min-h-dvh bg-background">

      {/* ===================== NAV ===================== */}
      <header className="sticky top-0 z-50 backdrop-blur-xl bg-background/70 border-b border-border/50">
        <div className="max-w-7xl mx-auto px-6 h-16 flex items-center justify-between">
          <Link to="/" className="flex items-center gap-2.5 group shrink-0">
            <div className="w-9 h-9 rounded-md bg-primary/15 flex items-center justify-center ring-1 ring-primary/30 group-hover:bg-primary/20 transition-colors">
              <Activity className="w-4 h-4 text-primary" />
            </div>
            <div className="leading-tight">
              <div className="text-[15px] font-bold tracking-tight">
                <span className="text-accent">Media</span><span className="text-primary">net</span>
              </div>
              <div className="text-[10px] uppercase tracking-[0.18em] text-muted-foreground">Observability</div>
            </div>
          </Link>

          <nav className="hidden md:flex items-center gap-8">
            <NavLink href="#features" label="Features" />
            <NavLink href="#how" label="How it works" />
            <NavLink href="#correlation" label="Correlation" />
            <NavLink href="#pricing" label="Pricing" />
          </nav>

          <div className="flex items-center gap-2">
            <ThemeToggle variant="ghost" />
            <Button render={<Link to="/login" />} variant="ghost" className="hidden sm:inline-flex">
              Sign in
            </Button>
            <Button render={<Link to="/signup" />} className="hidden sm:inline-flex">
              Start free <ArrowRight className="ml-1 size-3.5" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="md:hidden"
              onClick={() => setMobileOpen(!mobileOpen)}
              aria-label="Toggle menu"
            >
              {mobileOpen ? <X className="size-5" /> : <Menu className="size-5" />}
            </Button>
          </div>
        </div>

        {mobileOpen && (
          <div className="md:hidden border-t border-border/50 bg-background/95 backdrop-blur-xl">
            <div className="px-6 py-4 space-y-3">
              {[
                { href: "#features", label: "Features" },
                { href: "#how", label: "How it works" },
                { href: "#correlation", label: "Correlation" },
                { href: "#pricing", label: "Pricing" },
              ].map((n) => (
                <a
                  key={n.href}
                  href={n.href}
                  onClick={() => setMobileOpen(false)}
                  className="block text-sm text-muted-foreground hover:text-foreground transition-colors py-1.5"
                >
                  {n.label}
                </a>
              ))}
              <div className="pt-2 flex gap-2">
                <Button render={<Link to="/login" />} variant="outline" className="flex-1">
                  Sign in
                </Button>
                <Button render={<Link to="/signup" />} className="flex-1">
                  Start free
                </Button>
              </div>
            </div>
          </div>
        )}
      </header>

      {/* ===================== HERO ===================== */}
      <section className="relative overflow-hidden">
        <div className="absolute inset-0 bg-grid opacity-60" />
        <div className="relative max-w-7xl mx-auto px-6 pt-20 pb-24 md:pb-32">
          <div className="max-w-3xl">
            <div className="inline-flex items-center gap-2 px-3 py-1.5 rounded-full border border-border bg-card/60 backdrop-blur text-xs text-muted-foreground mb-6">
              <span className="status-dot-green" />
              <span>Now with cross-tier correlation engine</span>
              <span className="hidden sm:inline text-muted-foreground/50">&mdash;</span>
              <span className="hidden sm:inline text-primary font-medium">30+ diagnostic rules</span>
            </div>

            <h1 className="text-5xl md:text-7xl font-bold leading-[1.02] tracking-tighter">
              One pane of glass for
              <br />
              <span className="text-gradient-brand">your entire stack</span>.
            </h1>

            <p className="mt-6 text-lg md:text-xl text-muted-foreground max-w-2xl leading-relaxed">
              Auto-discovers your hosts, containers, and services. Collects logs and metrics.
              Correlates failures across tiers so you find the root cause in seconds, not hours.
            </p>

            <div className="mt-8 flex flex-wrap items-center gap-3">
              <Button
                render={<Link to="/signup" />}
                size="lg"
                className="shadow-accent-glow"
              >
                Start monitoring free <ArrowRight className="ml-1 size-4" />
              </Button>
              <Button
                render={<a href="#how" />}
                size="lg"
                variant="outline"
              >
                <Terminal className="size-4" /> See it in action
              </Button>
            </div>

            <div className="mt-8 flex flex-wrap items-center gap-x-6 gap-y-2 text-xs text-muted-foreground">
              <span className="flex items-center gap-1.5">
                <Check className="w-3.5 h-3.5 text-status-green" /> No credit card required
              </span>
              <span className="flex items-center gap-1.5">
                <Check className="w-3.5 h-3.5 text-status-green" /> 5-minute install
              </span>
              <span className="flex items-center gap-1.5">
                <Check className="w-3.5 h-3.5 text-status-green" /> Free for up to 5 hosts
              </span>
            </div>
          </div>

          {/* Dashboard mockup */}
          <div className="mt-16 md:mt-20 relative">
            <div className="absolute -inset-4 gradient-brand opacity-15 blur-3xl rounded-3xl" />
            <div className="relative rounded-xl border border-border bg-card shadow-elevated overflow-hidden">
              <div className="h-9 bg-secondary/60 border-b border-border flex items-center px-4 gap-1.5">
                <span className="w-2.5 h-2.5 rounded-full bg-destructive/70" />
                <span className="w-2.5 h-2.5 rounded-full bg-status-amber/70" />
                <span className="w-2.5 h-2.5 rounded-full bg-status-green/70" />
                <span className="ml-4 text-[11px] text-muted-foreground font-mono">medianet.observability / overview</span>
              </div>
              <div className="p-4 md:p-5 grid grid-cols-12 gap-3 md:gap-4">
                {/* KPI row */}
                {[
                  { label: "Hosts online", v: "24", c: "text-status-green" },
                  { label: "Services healthy", v: "87", c: "text-status-green" },
                  { label: "Degraded", v: "3", c: "text-status-amber" },
                  { label: "Down", v: "1", c: "text-destructive" },
                  { label: "Active issues", v: "5", c: "text-status-amber" },
                  { label: "Findings", v: "12", c: "text-destructive" },
                ].map((k) => (
                  <div key={k.label} className="col-span-4 md:col-span-2 rounded-lg border border-border bg-background/50 p-3">
                    <div className="text-[10px] uppercase tracking-wider text-muted-foreground">{k.label}</div>
                    <div className={`text-xl md:text-2xl font-bold mt-1 ${k.c}`}>{k.v}</div>
                  </div>
                ))}
                {/* Chart panel */}
                <div className="col-span-12 md:col-span-8 rounded-lg border border-border bg-background/50 p-4">
                  <div className="flex items-center justify-between mb-3">
                    <div className="text-xs font-semibold">Ingest throughput</div>
                    <div className="text-[10px] text-muted-foreground font-mono">last 1h</div>
                  </div>
                  <svg viewBox="0 0 400 120" className="w-full h-20 md:h-24" preserveAspectRatio="none">
                    <defs>
                      <linearGradient id="chartGradient" x1="0" x2="0" y1="0" y2="1">
                        <stop offset="0%" stopColor="var(--accent)" stopOpacity="0.35" />
                        <stop offset="100%" stopColor="var(--accent)" stopOpacity="0" />
                      </linearGradient>
                    </defs>
                    <path d={THROUGHPUT_AREA} fill="url(#chartGradient)" />
                    <path d={THROUGHPUT_PATH} fill="none" stroke="var(--accent)" strokeWidth="2" />
                  </svg>
                </div>
                {/* Findings panel */}
                <div className="col-span-12 md:col-span-4 rounded-lg border border-border bg-background/50 p-4">
                  <div className="text-xs font-semibold mb-3 flex items-center gap-1.5">
                    <Sparkles className="w-3.5 h-3.5 text-primary" /> Correlation findings
                  </div>
                  <div className="space-y-2">
                    {[
                      { t: "Web \u2192 App exhaustion", c: "red" },
                      { t: "Cache miss cascade", c: "amber" },
                      { t: "DB slow query storm", c: "amber" },
                    ].map((f) => (
                      <div key={f.t} className="flex items-center gap-2 text-[11px]">
                        <span className={`status-dot-${f.c}`} />
                        <span className="text-foreground/85 truncate">{f.t}</span>
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ===================== TRUSTED BY ===================== */}
      <section className="border-y border-border bg-secondary/20">
        <div className="max-w-7xl mx-auto px-6 py-10">
          <div className="text-center text-[10px] uppercase tracking-[0.25em] text-muted-foreground mb-6">
            Trusted by teams running production on
          </div>
          <div className="flex flex-wrap items-center justify-center gap-x-10 gap-y-4 text-muted-foreground/60 font-mono text-sm">
            {["NGINX", "APACHE", "POSTGRESQL", "MARIADB", "REDIS", "PHP-FPM", "DOCKER", "LINUX"].map((name) => (
              <span key={name} className="hover:text-foreground/80 transition-colors">{name}</span>
            ))}
          </div>
        </div>
      </section>

      {/* ===================== FEATURES ===================== */}
      <section id="features" className="max-w-7xl mx-auto px-6 py-24 md:py-28">
        <div className="max-w-2xl mb-14">
          <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-accent/10 border border-accent/20 text-xs font-semibold text-accent mb-4">
            <Sparkles className="w-3 h-3" /> Features
          </div>
          <h2 className="text-4xl md:text-5xl font-bold tracking-tight">
            Everything you need.
            <br />
            <span className="text-muted-foreground">Nothing you don&apos;t.</span>
          </h2>
          <p className="mt-4 text-muted-foreground text-lg max-w-xl">
            A focused observability suite built around one goal: making your infrastructure explain itself.
          </p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {[
            { icon: Server, title: "Host & container discovery", desc: "The agent auto-detects every host, container and running service. No manual inventory." },
            { icon: LineChart, title: "Metrics that matter", desc: "CPU, memory, disk, RPS, latency, error rates \u2014 pre-wired dashboards per service type." },
            { icon: Database, title: "Structured logs", desc: "Log paths detected automatically. Confirm, edit, or add your own with one click." },
            { icon: GitBranch, title: "Cross-tier correlation", desc: "9 deterministic rules chain-detect failures across Host \u2192 Web \u2192 App \u2192 DB \u2192 Cache." },
            { icon: ShieldCheck, title: "CVE scanning", desc: "Every service version checked against the CVE database with safe upgrade paths." },
            { icon: Bell, title: "Smart alerting", desc: "Template-based rules with per-severity thresholds and email delivery. No alert fatigue." },
          ].map((f) => (
            <FeatureCard key={f.title} icon={f.icon} title={f.title} desc={f.desc} />
          ))}
        </div>
      </section>

      {/* ===================== HOW IT WORKS ===================== */}
      <section id="how" className="border-y border-border bg-secondary/15">
        <div className="max-w-7xl mx-auto px-6 py-24 md:py-28">
          <div className="max-w-2xl mb-14">
            <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-primary/10 border border-primary/20 text-xs font-semibold text-primary mb-4">
              <Terminal className="w-3 h-3" /> How it works
            </div>
            <h2 className="text-4xl md:text-5xl font-bold tracking-tight">
              From zero to full visibility
              <br />
              <span className="text-muted-foreground">in 5 minutes.</span>
            </h2>
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
            <StepCard
              number="01"
              icon={Terminal}
              title="Install the agent"
              desc="One curl command per host. The agent starts discovering services, containers, and log paths immediately."
              code="curl -sSL https://get.medianet.io | sudo bash"
            />
            <StepCard
              number="02"
              icon={Layers}
              title="Confirm & enable"
              desc="Review the detected services. Confirm log paths and metrics endpoints, or override them if needed."
              code="\u2713 nginx  \u2713 mariadb  \u2713 redis  \u2713 php-fpm"
            />
            <StepCard
              number="03"
              icon={Eye}
              title="Watch it work"
              desc="Live dashboards, correlated findings and vulnerability alerts populate as data flows in."
              code="\u2192 12 hosts \u00b7 87 services \u00b7 0 unknowns"
            />
          </div>
        </div>
      </section>

      {/* ===================== CORRELATION SHOWCASE ===================== */}
      <section id="correlation" className="max-w-7xl mx-auto px-6 py-24 md:py-28">
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-12 items-center">
          <div>
            <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-accent/10 border border-accent/20 text-xs font-semibold text-accent mb-4">
              <GitBranch className="w-3 h-3" /> Correlation engine
            </div>
            <h2 className="text-4xl md:text-5xl font-bold tracking-tight leading-tight">
              Stop chasing symptoms.
              <br />
              <span className="text-gradient-brand">See the chain reaction.</span>
            </h2>
            <p className="mt-5 text-muted-foreground text-lg leading-relaxed">
              When your web tier starts throwing 502s, most tools show you 502s.
              We show you the exhausted PHP-FPM pool, the slow MariaDB query behind it,
              and the missing Redis cache that started it all \u2014 as one incident.
            </p>
            <ul className="mt-6 space-y-3">
              {[
                "Deterministic PromQL/LogQL rules \u2014 fully explainable",
                "One active finding per host/rule \u2014 no noise",
                "Deep links to the exact service dashboard causing it",
                "Recommended remediation for every finding",
              ].map((li) => (
                <li key={li} className="flex items-start gap-2.5 text-sm">
                  <Check className="w-4 h-4 text-status-green shrink-0 mt-0.5" />
                  <span>{li}</span>
                </li>
              ))}
            </ul>
          </div>

          {/* Enhanced dependency graph */}
          <div className="rounded-xl border border-border bg-card p-6 shadow-elevated">
            <div className="flex items-center justify-between mb-5">
              <div className="text-sm font-semibold flex items-center gap-1.5">
                <GitBranch className="w-4 h-4 text-accent" /> Live dependency graph
              </div>
              <span className="text-[10px] font-mono text-destructive inline-flex items-center gap-1">
                <AlertTriangle className="w-3 h-3" /> 1 finding active
              </span>
            </div>
            <svg viewBox="0 0 520 260" className="w-full">
              <defs>
                <radialGradient id="nodeHost" cx="50%" cy="50%" r="50%">
                  <stop offset="0%" stopColor="color-mix(in oklch, var(--accent) 30%, transparent)" />
                  <stop offset="100%" stopColor="transparent" />
                </radialGradient>
                <radialGradient id="nodeOk" cx="50%" cy="50%" r="50%">
                  <stop offset="0%" stopColor="color-mix(in oklch, var(--status-green) 25%, transparent)" />
                  <stop offset="100%" stopColor="transparent" />
                </radialGradient>
                <radialGradient id="nodeFail" cx="50%" cy="50%" r="50%">
                  <stop offset="0%" stopColor="color-mix(in oklch, var(--destructive) 30%, transparent)" />
                  <stop offset="100%" stopColor="transparent" />
                </radialGradient>
              </defs>

              {/* Glow halos behind nodes */}
              <circle cx={60} cy={115} r={38} fill="url(#nodeOk)" />
              <circle cx={175} cy={115} r={38} fill="url(#nodeFail)" />
              <circle cx={290} cy={115} r={38} fill="url(#nodeFail)" />
              <circle cx={405} cy={115} r={38} fill="url(#nodeOk)" />
              <circle cx={460} cy={115} r={38} fill="url(#nodeOk)" />

              {/* Connection lines */}
              {[[60, 175, true], [175, 290, true], [290, 405, false], [405, 460, false]].map(([x1, x2, failing], i) => {
                const color = failing ? "var(--destructive)" : "var(--border)"
                const width = failing ? 2.5 : 1.5
                return (
                  <g key={i}>
                    {/* Glow on failing lines */}
                    {failing && (
                      <line x1={x1} y1={115} x2={x2} y2={115}
                        stroke="var(--destructive)" strokeWidth={6} strokeOpacity={0.15} strokeLinecap="round" />
                    )}
                    <line x1={x1} y1={115} x2={x2} y2={115}
                      stroke={color} strokeWidth={width} strokeDasharray={failing ? "0" : "4 3"}
                      strokeLinecap="round" />
                    {/* Traffic direction arrow */}
                    <polygon
                      points={`${(x1 + x2) / 2},109 ${(x1 + x2) / 2 + 6},115 ${(x1 + x2) / 2},121`}
                      fill={color} opacity={0.6}
                    />
                  </g>
                )
              })}

              {/* Connection labels */}
              {[
                { x: 117, label: "502 spike" },
                { x: 232, label: "pool exhausted" },
                { x: 347, label: "200ms \u2192 4.2s" },
                { x: 432, label: "miss \u2192 DB" },
              ].map((l) => (
                <text key={l.x} x={l.x} y={92} textAnchor="middle" fontSize="8"
                  fill="var(--muted-foreground)" fontFamily="monospace">
                  {l.label}
                </text>
              ))}

              {/* Nodes */}
              {[
                { x: 60, label: "Host", sub: "host-01", health: "ok" },
                { x: 175, label: "Web", sub: "nginx", health: "failing" },
                { x: 290, label: "App", sub: "php-fpm", health: "failing" },
                { x: 405, label: "DB", sub: "mariadb", health: "warn" },
                { x: 460, label: "Cache", sub: "redis", health: "ok" },
              ].map((n) => {
                const borderColor = n.health === "failing" ? "var(--destructive)" :
                  n.health === "warn" ? "var(--status-amber)" : "var(--border)"
                const dotColor = n.health === "failing" ? "var(--destructive)" :
                  n.health === "warn" ? "var(--status-amber)" : "var(--status-green)"
                return (
                  <g key={n.label}>
                    <circle cx={n.x} cy={115} r={28}
                      fill="var(--card)" stroke={borderColor} strokeWidth={n.health === "failing" ? 2 : 1.5} />
                    {/* Health dot */}
                    <circle cx={n.x + 18} cy={96} r={4} fill={dotColor}
                      stroke="var(--card)" strokeWidth={1.5} />
                    <text x={n.x} y={119} textAnchor="middle" fontSize="12"
                      fill="var(--foreground)" fontWeight="700">{n.label}</text>
                    <text x={n.x} y={158} textAnchor="middle" fontSize="9"
                      fill="var(--muted-foreground)" fontFamily="monospace">{n.sub}</text>
                  </g>
                )
              })}

              {/* Zone labels */}
              <text x={60} y={210} textAnchor="middle" fontSize="8"
                fill="var(--muted-foreground)" fontFamily="monospace" opacity={0.5}>
                INFRASTRUCTURE
              </text>
              <text x={232} y={210} textAnchor="middle" fontSize="8"
                fill="var(--muted-foreground)" fontFamily="monospace" opacity={0.5}>
                APPLICATION TIER
              </text>
              <text x={432} y={210} textAnchor="middle" fontSize="8"
                fill="var(--muted-foreground)" fontFamily="monospace" opacity={0.5}>
                DATA LAYER
              </text>

              {/* Zone dividers */}
              {[100, 365].map((x) => (
                <line key={x} x1={x} y1={196} x2={x} y2={218}
                  stroke="var(--border)" strokeWidth={1} strokeDasharray="2 2" opacity={0.4} />
              ))}
            </svg>
            <div className="mt-5 rounded-lg border border-destructive/30 bg-destructive/5 p-3">
              <div className="flex items-center gap-2 text-xs font-semibold text-destructive">
                <AlertTriangle className="w-3.5 h-3.5" /> Web tier failures caused by app-tier exhaustion
              </div>
              <p className="mt-1.5 text-[11px] text-muted-foreground leading-relaxed">
                nginx 5xx spike correlated with php-fpm queue saturation. Root cause: slow MariaDB query on <span className="font-mono text-foreground/80">/api/orders</span>.
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* ===================== STATS STRIP ===================== */}
      <section className="border-y border-border bg-secondary/15">
        <div className="max-w-7xl mx-auto px-6 py-14 grid grid-cols-2 md:grid-cols-4 gap-6">
          {[
            { v: "&lt;5min", l: "From install to first metric" },
            { v: "30+", l: "Diagnostic rules out of the box" },
            { v: "99.99%", l: "Ingest pipeline uptime" },
            { v: "7&times;", l: "Faster mean time to resolution" },
          ].map((s) => (
            <div key={s.l} className="text-center">
              <div className="text-3xl md:text-4xl font-bold text-gradient-brand" dangerouslySetInnerHTML={{ __html: s.v }} />
              <div className="mt-1 text-xs text-muted-foreground">{s.l}</div>
            </div>
          ))}
        </div>
      </section>

      {/* ===================== PRICING ===================== */}
      <section id="pricing" className="max-w-7xl mx-auto px-6 py-24 md:py-28">
        <div className="text-center max-w-2xl mx-auto mb-14">
          <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-primary/10 border border-primary/20 text-xs font-semibold text-primary mb-4">
            <BarChart3 className="w-3 h-3" /> Pricing
          </div>
          <h2 className="text-4xl md:text-5xl font-bold tracking-tight">Simple, per-host pricing.</h2>
          <p className="mt-4 text-muted-foreground text-lg">Start free. Scale when you&apos;re ready. Cancel anytime.</p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-5 max-w-5xl mx-auto">
          {[
            {
              name: "Free", price: "$0", tag: "Perfect for side projects",
              features: ["Up to 5 hosts", "3 days log retention", "7 days metrics retention", "Email alerts", "Community support"],
              cta: "Start free", highlight: false,
            },
            {
              name: "Pro", price: "$12", tag: "Per host / month",
              features: ["Unlimited hosts", "30 days log retention", "90 days metrics retention", "Correlation engine", "CVE scanning", "Priority support"],
              cta: "Start Pro trial", highlight: true,
            },
            {
              name: "Enterprise", price: "Custom", tag: "For regulated workloads",
              features: ["Everything in Pro", "SSO / SAML", "1 year retention", "Dedicated infrastructure", "SLA & DPA", "Solution architect"],
              cta: "Contact sales", highlight: false,
            },
          ].map((p) => (
            <div
              key={p.name}
              className={`relative rounded-2xl border p-8 flex flex-col transition-all duration-300 ${
                p.highlight
                  ? "border-accent/60 bg-card shadow-elevated ring-1 ring-accent/20 scale-[1.02]"
                  : "border-border bg-card hover:border-accent/30 hover:shadow-elevated"
              }`}
            >
              {p.highlight && (
                <div className="absolute -top-3 left-8 px-2.5 py-0.5 rounded-full text-[10px] font-bold uppercase tracking-wider gradient-accent text-accent-foreground">
                  Most popular
                </div>
              )}
              <div className="text-sm font-semibold">{p.name}</div>
              <div className="mt-3 flex items-baseline gap-1">
                <span className="text-4xl font-bold">{p.price}</span>
                {p.name === "Pro" && <span className="text-sm text-muted-foreground">/host/mo</span>}
              </div>
              <div className="text-xs text-muted-foreground mt-1">{p.tag}</div>
              <ul className="mt-6 space-y-2.5 flex-1">
                {p.features.map((f) => (
                  <li key={f} className="flex items-start gap-2 text-sm">
                    <Check className="w-4 h-4 text-status-green shrink-0 mt-0.5" />
                    <span className="text-foreground/85">{f}</span>
                  </li>
                ))}
              </ul>
              <Button
                render={<Link to="/signup" />}
                variant={p.highlight ? "default" : "outline"}
                className={`mt-8 w-full ${p.highlight ? "shadow-accent-glow" : ""}`}
              >
                {p.cta}
              </Button>
            </div>
          ))}
        </div>
      </section>

      {/* ===================== CTA ===================== */}
      <section className="max-w-7xl mx-auto px-6 pb-24 md:pb-28">
        <div className="relative overflow-hidden rounded-2xl border border-border bg-card p-10 md:p-16">
          <div className="absolute -top-24 -right-24 w-96 h-96 rounded-full bg-primary/10 blur-3xl" />
          <div className="absolute -bottom-24 -left-24 w-96 h-96 rounded-full bg-accent/10 blur-3xl" />
          <div className="relative max-w-2xl">
            <h2 className="text-4xl md:text-5xl font-bold tracking-tight">Ready to see your infrastructure clearly?</h2>
            <p className="mt-4 text-muted-foreground text-lg">
              Install the agent on your first host and get a full observability suite in under 5 minutes.
            </p>
            <div className="mt-8 flex flex-wrap gap-3">
              <Button
                render={<Link to="/signup" />}
                size="lg"
                className="shadow-accent-glow"
              >
                Create your workspace <ArrowRight className="ml-1 size-4" />
              </Button>
              <Button
                render={<Link to="/login" />}
                size="lg"
                variant="outline"
              >
                Sign in
              </Button>
            </div>
          </div>
        </div>
      </section>

      {/* ===================== FOOTER ===================== */}
      <footer className="border-t border-border">
        <div className="max-w-7xl mx-auto px-6 py-12 grid grid-cols-2 md:grid-cols-5 gap-8">
          <div className="col-span-2">
            <Link to="/" className="flex items-center gap-2.5 mb-3">
              <div className="w-9 h-9 rounded-md bg-primary/15 flex items-center justify-center ring-1 ring-primary/30">
                <Activity className="w-4 h-4 text-primary" />
              </div>
              <div className="leading-tight">
                <div className="text-[15px] font-bold tracking-tight">
                  <span className="text-accent">Media</span><span className="text-primary">net</span>
                </div>
                <div className="text-[10px] uppercase tracking-[0.18em] text-muted-foreground">Observability</div>
              </div>
            </Link>
            <p className="text-sm text-muted-foreground max-w-sm">
              The single pane of glass for hosts, containers, services, logs, metrics and CVEs.
            </p>
            <div className="flex items-center gap-3 mt-5 text-muted-foreground">
              <a href="#" className="hover:text-foreground transition-colors"><Github className="w-4 h-4" /></a>
              <a href="#" className="hover:text-foreground transition-colors"><Twitter className="w-4 h-4" /></a>
              <a href="#" className="hover:text-foreground transition-colors"><Linkedin className="w-4 h-4" /></a>
            </div>
          </div>
          {[
            { title: "Product", links: ["Features", "Correlation", "Pricing", "Changelog"] },
            { title: "Company", links: ["About", "Blog", "Careers", "Contact"] },
            { title: "Resources", links: ["Docs", "Guides", "Status", "Support"] },
          ].map((col) => (
            <div key={col.title}>
              <div className="text-xs font-semibold uppercase tracking-wider text-foreground mb-3">{col.title}</div>
              <ul className="space-y-2 text-sm text-muted-foreground">
                {col.links.map((l) => (
                  <li key={l}><a href="#" className="hover:text-foreground transition-colors">{l}</a></li>
                ))}
              </ul>
            </div>
          ))}
        </div>
        <div className="border-t border-border">
          <div className="max-w-7xl mx-auto px-6 py-5 flex flex-wrap items-center justify-between gap-3 text-xs text-muted-foreground">
            <div>&copy; {new Date().getFullYear()} Medianet Observability. All rights reserved.</div>
            <div className="flex gap-5">
              <a href="#" className="hover:text-foreground transition-colors">Privacy</a>
              <a href="#" className="hover:text-foreground transition-colors">Terms</a>
              <a href="#" className="hover:text-foreground transition-colors">Security</a>
            </div>
          </div>
        </div>
      </footer>
    </div>
  )
}