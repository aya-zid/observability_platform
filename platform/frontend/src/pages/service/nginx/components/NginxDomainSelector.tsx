import { cn } from '@/lib/utils'

interface NginxDomainSelectorProps {
  domains: string[]
  selectedDomain: string | null
  viewMode: 'server' | 'domain'
  onSelect: (domain: string | null) => void
  loading: boolean
  configuredMetricsEndpoint: string
}

export function NginxDomainSelector({
  domains,
  selectedDomain,
  viewMode,
  onSelect,
  loading,
  configuredMetricsEndpoint,
}: NginxDomainSelectorProps) {
  return (
    <>
      <div className="rounded-xl border border-border bg-card p-4">
        <div className="flex flex-col gap-3">
          <div>
            <p className="text-sm font-semibold">Domains</p>
            <p className="mt-1 text-xs text-muted-foreground">
              Same dashboard, same metrics and widgets. Switch tabs to isolate one Nginx domain at a time.
            </p>
          </div>
          {loading ? (
            <p className="text-sm text-muted-foreground">Loading domains...</p>
          ) : domains.length === 0 ? (
            <p className="text-sm text-muted-foreground">No enabled Nginx domains found yet.</p>
          ) : (
            <div className="flex flex-wrap gap-2">
              <button
                type="button"
                onClick={() => onSelect(null)}
                className={cn(
                  'rounded-lg border px-3 py-2 text-sm font-medium transition-colors',
                  viewMode === 'server'
                    ? 'border-slate-900 bg-slate-900 text-white dark:border-slate-100 dark:bg-slate-100 dark:text-slate-900'
                    : 'border-border bg-muted/50 text-foreground hover:bg-muted',
                )}
              >
                Server Metrics
              </button>
              {domains.map((domain) => {
                const active = viewMode === 'domain' && domain === selectedDomain
                return (
                  <button
                    key={domain}
                    type="button"
                    onClick={() => onSelect(domain)}
                    className={cn(
                      'rounded-lg border px-3 py-2 text-sm font-medium transition-colors',
                      active
                        ? 'border-blue-600 bg-blue-600 text-white'
                        : 'border-border bg-muted/50 text-foreground hover:bg-muted',
                    )}
                  >
                    <span className="font-mono">{domain}</span>
                  </button>
                )
              })}
            </div>
          )}
        </div>
      </div>

      {selectedDomain ? (
        <div className="rounded-lg border border-border bg-muted/30 px-4 py-3 text-sm text-muted-foreground">
          {viewMode === 'server' ? (
            <>
              Viewing nginx-wide status metrics. Domain logs remain available under{' '}
              <span className="font-mono font-semibold text-foreground">{selectedDomain}</span>.
            </>
          ) : (
            <>
              Active domain: <span className="font-mono font-semibold text-foreground">{selectedDomain}</span>
            </>
          )}
        </div>
      ) : null}

      <div className="rounded-xl border border-border bg-card p-4">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <p className="text-sm font-semibold">Metrics source</p>
            <p className="mt-1 text-xs text-muted-foreground">
              The integration script scrapes the endpoint saved for this service in the database. That saved endpoint is client-specific.
            </p>
          </div>
          <div className="rounded-lg border border-border bg-muted/40 px-3 py-2 text-xs">
            <span className="text-muted-foreground">Configured endpoint:</span>{' '}
            <span className="font-mono break-all">{configuredMetricsEndpoint || 'Not configured'}</span>
          </div>
        </div>
      </div>
    </>
  )
}
