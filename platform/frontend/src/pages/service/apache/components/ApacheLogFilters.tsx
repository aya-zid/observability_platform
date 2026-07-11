import type { LogEntryParsed } from './ApacheLogViewerRow'

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
    <div className="rounded-lg border border-border bg-card p-4">
      <h3 className="mb-4 font-semibold">Filters</h3>
      <div className="grid grid-cols-1 gap-4 md:grid-cols-4">
        <div>
          <label className="block text-xs font-semibold text-muted-foreground mb-2">HTTP Method</label>
          <select
            value={filters.method || ''}
            onChange={(e) => onFilterChange({ ...filters, method: e.target.value || null })}
            className="w-full rounded-md border border-border bg-muted px-3 py-2 text-sm"
          >
            <option value="">All</option>
            {uniqueMethods.map((m) => (
              <option key={m} value={m}>{m}</option>
            ))}
          </select>
        </div>

        <div>
          <label className="block text-xs font-semibold text-muted-foreground mb-2">Status Code</label>
          <select
            value={filters.statusCode || ''}
            onChange={(e) => onFilterChange({ ...filters, statusCode: e.target.value || null })}
            className="w-full rounded-md border border-border bg-muted px-3 py-2 text-sm"
          >
            <option value="">All</option>
            {uniqueStatuses.map((s) => (
              <option key={s} value={s}>{s}</option>
            ))}
          </select>
        </div>

        <div>
          <label className="block text-xs font-semibold text-muted-foreground mb-2">Search Path</label>
          <input
            type="text"
            value={filters.searchPath || ''}
            onChange={(e) => onFilterChange({ ...filters, searchPath: e.target.value || null })}
            placeholder="e.g., /api, /home, /admin"
            className="w-full rounded-md border border-border bg-muted px-3 py-2 text-sm"
          />
        </div>

        <div>
          <label className="block text-xs font-semibold text-muted-foreground mb-2">Search IP</label>
          <input
            type="text"
            value={filters.searchIP || ''}
            onChange={(e) => onFilterChange({ ...filters, searchIP: e.target.value || null })}
            placeholder="e.g., 192.168.1"
            className="w-full rounded-md border border-border bg-muted px-3 py-2 text-sm font-mono"
          />
        </div>
      </div>
      <button
        type="button"
        onClick={() => onFilterChange({ method: null, statusCode: null, searchPath: null, searchIP: null })}
        className="mt-4 text-sm text-blue-600 hover:text-blue-500"
      >
        Clear Filters
      </button>
    </div>
  )
}
