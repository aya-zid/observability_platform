import { Link } from "react-router-dom"

import ThemeToggle from "@/components/shared/ThemeToggle"

function RequiredMark() {
  return <span className="text-destructive"> *</span>
}

export function AuthLabel({
  children,
  required,
}: {
  children: React.ReactNode
  required?: boolean
}) {
  return (
    <span className="text-sm font-medium">
      {children}
      {required ? <RequiredMark /> : null}
    </span>
  )
}

export default function AuthShell({
  title,
  subtitle,
  children,
  footer,
}: {
  title: string
  subtitle: string
  children: React.ReactNode
  footer: React.ReactNode
}) {
  return (
    <div className="auth-bg min-h-dvh grid place-items-center p-4">
      <div className="w-full max-w-5xl">
        <div className="mb-4 flex items-center justify-between">
          <Link to="/" className="text-xs text-white/70 hover:text-white">
            Back to home
          </Link>
          <ThemeToggle variant="ghost" />
        </div>

        <div className="grid gap-6 md:grid-cols-2 md:gap-0 rounded-3xl border border-white/10 bg-black/20 backdrop-blur-xl overflow-hidden shadow-sm">
          <div className="hidden md:block p-10">
            <div className="flex items-center gap-3">
              <div className="size-12 rounded-2xl bg-white/10 grid place-items-center text-white font-semibold">
                OP
              </div>
              <div className="text-white">
                <div className="font-semibold tracking-tight">Observability Platform</div>
                <div className="text-xs text-white/70">Metrics. Logs. Traces.</div>
              </div>
            </div>

            <div className="mt-10 text-white">
              <div className="text-2xl font-semibold tracking-tight">
                Cloud-grade,
                <span className="text-white/70"> human-friendly</span>.
              </div>
              <div className="mt-2 text-sm text-white/70 max-w-sm">
                Multi-tenant platform with cookie-based auth and agent API keys.
                Built to demo well, and scale further.
              </div>
            </div>

            <div className="mt-10 grid gap-3 text-sm text-white/70">
              <div className="rounded-xl border border-white/10 bg-white/5 p-4">
                Per-tenant isolation (VM, Loki, Tempo)
              </div>
              <div className="rounded-xl border border-white/10 bg-white/5 p-4">
                Secure sessions (HttpOnly cookies + refresh)
              </div>
              <div className="rounded-xl border border-white/10 bg-white/5 p-4">
                Fast UX (React Query + sensible routing)
              </div>
            </div>
          </div>

          <div className="p-6 md:p-10 bg-background/70 backdrop-blur">
            <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
            <p className="mt-1 text-sm text-muted-foreground">{subtitle}</p>
            <div className="mt-6">{children}</div>
            <div className="mt-5 text-sm text-muted-foreground">{footer}</div>
          </div>
        </div>
      </div>
    </div>
  )
}

