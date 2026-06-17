export function ChartBox({
  title,
  children,
  loading,
  empty,
  hint,
  actions,
}: {
  title: string
  children: React.ReactNode
  loading?: boolean
  empty?: boolean
  hint?: string
  actions?: React.ReactNode
}) {
  return (
    <div className="rounded-lg border border-border bg-card p-5 shadow-sm">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <h3 className="text-sm font-semibold text-foreground">{title}</h3>
        {actions}
      </div>
      {loading ? (
        <div className="flex h-64 items-center justify-center text-sm text-muted-foreground">Loading...</div>
      ) : empty ? (
        <div className="flex h-64 flex-col items-center justify-center gap-1 text-sm text-muted-foreground">
          <span>No data for selected range</span>
          {hint ? <span className="text-xs opacity-70">{hint}</span> : null}
        </div>
      ) : (
        children
      )}
    </div>
  )
}
