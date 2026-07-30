import { cn } from '@/lib/utils'

interface Props {
  domains: string[]
  selectedDomain: string
  viewMode: 'server' | 'domain'
  domainsLoading: boolean
  onViewModeChange: (viewMode: 'server' | 'domain', domain?: string) => void
  onDomainToggle: (domain: string) => void
}

export function ApacheDomainSelector({
  domains,
  selectedDomain,
  viewMode,
  domainsLoading,
  onViewModeChange,
  onDomainToggle,
}: Props) {
  return (
    <div className="rounded-xl border border-border bg-card p-4">
      <div className="flex flex-col gap-3">
        <div>
          <p className="text-sm font-semibold">Domains</p>
          <p className="mt-1 text-sm text-muted-foreground">
            Same dashboard, same metrics and widgets. Switch tabs to isolate one Apache domain at a time.
          </p>
        </div>
        {domainsLoading ? (
          <p className="text-sm text-muted-foreground">Loading domains...</p>
        ) : domains.length === 0 ? (
          <p className="text-sm text-muted-foreground">No enabled Apache domains found yet.</p>
        ) : (
          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              onClick={() => onViewModeChange('server')}
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
                  onClick={() => onDomainToggle(domain)}
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
  )
}
