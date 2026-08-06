import type { ReactNode } from "react"
import type { LucideIcon } from "lucide-react"

export function ReportSection({
  title,
  eyebrow,
  icon: Icon,
  children,
  action,
}: {
  title: string
  eyebrow?: string
  icon: LucideIcon
  children: ReactNode
  action?: ReactNode
}) {
  return (
    <section className="rounded-lg border border-border bg-card shadow-card">
      <div className="flex flex-wrap items-center justify-between gap-2 border-b border-border px-4 py-2">
        <div className="flex min-w-0 items-center gap-2">
          <span className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md border border-border bg-muted/35 text-primary">
            <Icon className="h-3.5 w-3.5" />
          </span>
          <div className="min-w-0">
            <h2 className="truncate text-base font-semibold tracking-tight text-foreground">{title}</h2>
            {eyebrow ? <p className="mt-0.5 text-xs text-muted-foreground">{eyebrow}</p> : null}
          </div>
        </div>
        {action}
      </div>
      <div className="p-3">{children}</div>
    </section>
  )
}

export function Badge({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <span className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md border text-xs font-medium ${className ?? ""}`}>
      {children}
    </span>
  )
}
