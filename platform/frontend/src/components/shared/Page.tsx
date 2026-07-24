import { cn } from "@/lib/utils"

export function Page({
  children,
  className,
}: {
  children: React.ReactNode
  className?: string
}) {
  return (
    <div className={cn("mx-auto w-full max-w-6xl grid gap-6", className)}>
      {children}
    </div>
  )
}

export function PageHeader({
  title,
  description,
  actions,
  icon: Icon,
}: {
  title: string
  description?: string
  actions?: React.ReactNode
  icon?: React.ComponentType<{ className?: string }>
}) {
  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
      <div className="min-w-0">
        <div className="flex items-center gap-2.5">
          {Icon && <Icon className="w-6 h-6 text-accent shrink-0" />}
          <h1 className="text-2xl font-bold text-foreground tracking-tight">
            {title}
          </h1>
        </div>
        {description ? (
          <p className="mt-1 text-sm text-muted-foreground max-w-2xl">
            {description}
          </p>
        ) : null}
      </div>
      {actions ? <div className="shrink-0">{actions}</div> : null}
    </div>
  )
}

export function SectionCard({
  title,
  description,
  children,
  className,
}: {
  title?: string
  description?: string
  children: React.ReactNode
  className?: string
}) {
  return (
    <section
      className={cn(
        "rounded-xl border border-border bg-card shadow-card p-4 md:p-6",
        className
      )}
    >
      {title ? (
        <div className="mb-4">
          <div className="font-semibold text-foreground">{title}</div>
          {description ? (
            <div className="mt-1 text-sm text-muted-foreground">{description}</div>
          ) : null}
        </div>
      ) : null}
      {children}
    </section>
  )
}
