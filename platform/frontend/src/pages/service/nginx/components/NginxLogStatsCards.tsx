interface NginxLogStatsCardsProps {
  totalLoaded: number
  filteredCount: number
  pagesCount: number
  initialLoading: boolean
}

export function NginxLogStatsCards({
  totalLoaded,
  filteredCount,
  pagesCount,
  initialLoading,
}: NginxLogStatsCardsProps) {
  return (
    <div className="flex gap-4 flex-wrap">
      <div className="rounded-lg border border-border bg-card p-4">
        <p className="text-xs text-muted-foreground">Loaded</p>
        <p className="mt-1 text-2xl font-bold text-primary">{totalLoaded}</p>
      </div>
      <div className="rounded-lg border border-border bg-card p-4">
        <p className="text-xs text-muted-foreground">Matching Filters</p>
        <p className="mt-1 text-2xl font-bold text-blue-500">{filteredCount}</p>
      </div>
      <div className="rounded-lg border border-border bg-card p-4">
        <p className="text-xs text-muted-foreground">Pages loaded</p>
        <p className="mt-1 text-2xl font-bold">{pagesCount}</p>
      </div>
      <div className="rounded-lg border border-border bg-card flex-1 p-4">
        {initialLoading ? (
          <p className="text-sm text-muted-foreground">Updating logs...</p>
        ) : (
          <p className="text-xs text-muted-foreground">
            Last updated {new Date().toLocaleTimeString()} · auto-refresh 15s
          </p>
        )}
      </div>
    </div>
  )
}
