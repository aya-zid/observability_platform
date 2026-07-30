interface Props {
  totalLoaded: number
  filteredEntriesLength: number
  pagesLength: number
  initialLoading: boolean
}

export function ApacheLogStatsCards({ totalLoaded, filteredEntriesLength, pagesLength, initialLoading }: Props) {
  return (
    <div className="flex gap-2 flex-wrap">
      <div className="rounded-md border border-border bg-card px-2.5 py-1.5">
        <p className="text-xs text-muted-foreground">Loaded</p>
        <p className="text-sm font-bold text-primary">{totalLoaded}</p>
      </div>
      <div className="rounded-md border border-border bg-card px-2.5 py-1.5">
        <p className="text-xs text-muted-foreground">Matching</p>
        <p className="text-sm font-bold text-blue-500">{filteredEntriesLength}</p>
      </div>
      <div className="rounded-md border border-border bg-card px-2.5 py-1.5">
        <p className="text-xs text-muted-foreground">Pages</p>
        <p className="text-sm font-bold">{pagesLength}</p>
      </div>
      <div className="rounded-md border border-border bg-card flex-1 px-2.5 py-1.5 flex items-center">
        {initialLoading ? (
          <span className="text-xs text-muted-foreground">Updating logs...</span>
        ) : (
          <span className="text-xs text-muted-foreground">
            {new Date().toLocaleTimeString()} · auto-refresh 15s
          </span>
        )}
      </div>
    </div>
  )
}
