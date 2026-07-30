export interface FilterState {
  method: string | null
  statusCode: string | null
  searchPath: string | null
  searchIP: string | null
}

interface Props {
  filters: FilterState
  uniqueMethods: string[]
  uniqueStatuses: number[]
  onFilterChange: (filters: FilterState) => void
}

export function ApacheLogFilters({ filters, uniqueMethods, uniqueStatuses, onFilterChange }: Props) {
  return (
    <div className="rounded-md border border-border bg-card px-3 py-2">
      <div className="flex items-center gap-2 flex-wrap">
        <select
          value={filters.method || ''}
          onChange={(e) => onFilterChange({ ...filters, method: e.target.value || null })}
          className="rounded-md border border-border bg-muted px-2 py-1 text-xs"
        >
          <option value="">All methods</option>
          {uniqueMethods.map((m) => (
            <option key={m} value={m}>{m}</option>
          ))}
        </select>
        <select
          value={filters.statusCode || ''}
          onChange={(e) => onFilterChange({ ...filters, statusCode: e.target.value || null })}
          className="rounded-md border border-border bg-muted px-2 py-1 text-xs"
        >
          <option value="">All statuses</option>
          {uniqueStatuses.map((s) => (
            <option key={s} value={s}>{s}</option>
          ))}
        </select>
        <input
          type="text"
          value={filters.searchPath || ''}
          onChange={(e) => onFilterChange({ ...filters, searchPath: e.target.value || null })}
          placeholder="Path..."
          className="w-28 rounded-md border border-border bg-muted px-2 py-1 text-xs"
        />
        <input
          type="text"
          value={filters.searchIP || ''}
          onChange={(e) => onFilterChange({ ...filters, searchIP: e.target.value || null })}
          placeholder="IP..."
          className="w-24 rounded-md border border-border bg-muted px-2 py-1 text-xs font-mono"
        />
        <button
          type="button"
          onClick={() => onFilterChange({ method: null, statusCode: null, searchPath: null, searchIP: null })}
          className="text-xs text-primary hover:underline"
        >
          Clear
        </button>
      </div>
    </div>
  )
}
