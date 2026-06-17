import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, useParams, useSearchParams } from 'react-router-dom'

import { getDiscoveryService } from '@/api/discovery'
import { getNginxSlowRequests } from '@/api/nginx'
import type { NginxSlowRequest } from '@/api/nginx'
import {
  formatNumber,
  formatTimestamp,
  getNowSeconds,
  getStartSeconds,
  getStatusBadgeClass,
  nanosToSeconds,
  secondsToNanos,
} from '@/lib/queryHelpers'
import { cn } from '@/lib/utils'
import { RefreshCw } from 'lucide-react'

const PER_PAGE = 100

function methodBadgeColor(method: string) {
  switch (method) {
    case 'GET':
      return '#22c55e'
    case 'POST':
      return '#3b82f6'
    case 'DELETE':
      return '#ef4444'
    default:
      return '#8b5cf6'
  }
}

function formatWindowLabel(startNs: string, endNs: string) {
  const start = nanosToSeconds(startNs)
  const end = nanosToSeconds(endNs)
  if (start === null || end === null) {
    return 'Selected range'
  }
  return `${formatTimestamp(start)} - ${formatTimestamp(end)}`
}

function SlowRequestRow({
  req,
  expanded,
  onToggle,
}: {
  req: NginxSlowRequest
  expanded: boolean
  onToggle: () => void
}) {
  return (
    <div className="rounded-lg border border-border bg-card">
      <button
        type="button"
        onClick={onToggle}
        className="grid w-full grid-cols-1 gap-2 px-3 py-3 text-left text-xs hover:bg-muted/40 md:grid-cols-[180px_132px_76px_78px_minmax(0,1fr)_96px] md:items-center md:gap-3"
      >
        <span className="font-mono text-muted-foreground" title={req.timestamp || undefined}>
          {req.timestamp || '-'}
        </span>
        <span className="truncate font-mono text-muted-foreground" title={req.ip || undefined}>
          {req.ip || '-'}
        </span>
        <span
          className="w-fit rounded px-1.5 py-0.5 font-semibold text-white"
          style={{ background: methodBadgeColor(req.method) }}
        >
          {req.method || '-'}
        </span>
        <span className={cn('w-fit rounded px-1.5 py-0.5 font-semibold', getStatusBadgeClass(req.status))}>
          {req.status || '-'}
        </span>
        <span className="min-w-0 truncate font-mono" title={req.path}>
          {req.path}
        </span>
        <span
          className={cn(
            'text-right font-semibold tabular-nums',
            req.rt >= 5 ? 'text-red-500' : req.rt >= 2 ? 'text-amber-500' : 'text-foreground',
          )}
        >
          {(req.rt * 1000).toFixed(0)}ms
        </span>
      </button>
      {expanded ? (
        <div className="border-t border-border bg-muted/30 px-3 py-3 text-xs">
          <div className="grid grid-cols-1 gap-2 md:grid-cols-2">
            <div>
              <span className="text-muted-foreground">Client IP:</span>{' '}
              <span className="font-mono">{req.ip || '-'}</span>
            </div>
            <div>
              <span className="text-muted-foreground">Response Time:</span>{' '}
              <span className="font-mono">{req.rt.toFixed(3)}s</span>
            </div>
            <div className="md:col-span-2">
              <span className="text-muted-foreground">Path:</span>{' '}
              <span className="font-mono break-all">{req.path}</span>
            </div>
            <div className="md:col-span-2">
              <span className="text-muted-foreground">Normalized Path:</span>{' '}
              <span className="font-mono break-all">{req.normalized_path || '-'}</span>
            </div>
            <div className="md:col-span-2">
              <span className="text-muted-foreground">Referer:</span>{' '}
              <span className="font-mono break-all">{req.referer || '-'}</span>
            </div>
            <div className="md:col-span-2">
              <span className="text-muted-foreground">User Agent:</span>{' '}
              <span className="font-mono break-all">{req.user_agent || '-'}</span>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  )
}

export function NginxSlowRequestsPage() {
  const { id = '' } = useParams<{ id?: string }>()
  const [searchParams, setSearchParams] = useSearchParams()
  const [expanded, setExpanded] = useState<number | null>(null)

  const defaultEnd = getNowSeconds()
  const defaultStart = getStartSeconds(3600)
  const start = searchParams.get('start') || secondsToNanos(defaultStart)
  const end = searchParams.get('end') || secondsToNanos(defaultEnd)
  const domain = searchParams.get('domain') || ''
  const minRT = searchParams.get('min_rt') || '1'
  const pageParam = Number.parseInt(searchParams.get('page') || '1', 10)
  const page = Number.isFinite(pageParam) && pageParam > 0 ? pageParam : 1

  const serviceQuery = useQuery({
    queryKey: ['discovery-service', id],
    queryFn: () => getDiscoveryService(id),
    enabled: !!id,
  })

  const slowQuery = useQuery({
    queryKey: ['nginx-slow-requests', id, start, end, domain, minRT, page],
    queryFn: () =>
      getNginxSlowRequests(id, {
        start,
        end,
        domain: domain || undefined,
        min_rt: minRT,
        page: String(page),
        per_page: String(PER_PAGE),
      }),
    enabled: !!id && !!start && !!end,
    staleTime: 20_000,
    refetchOnWindowFocus: false,
  })

  const rows = slowQuery.data?.items ?? []
  const total = slowQuery.data?.total ?? 0
  const totalPages = slowQuery.data?.total_pages ?? 1
  const serviceName = serviceQuery.data?.service_name ?? 'nginx'

  const dashboardHref = useMemo(() => {
    const params = new URLSearchParams()
    if (domain) params.set('domain', domain)
    params.set('min_rt', minRT)
    return `/integrations/nginx-dashboard/${id}?${params.toString()}`
  }, [domain, id, minRT])

  function setPage(nextPage: number) {
    const next = new URLSearchParams(searchParams)
    next.set('page', String(Math.max(1, nextPage)))
    setExpanded(null)
    setSearchParams(next)
  }

  if (!id) return <div className="p-6 text-sm text-red-600">Missing service id.</div>

  return (
    <div className="space-y-6 p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-3xl font-bold">Slow Nginx Requests</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Service: <span className="font-mono font-semibold">{serviceName}</span>
            {domain ? (
              <>
                {' '}• Domain: <span className="font-mono font-semibold">{domain}</span>
              </>
            ) : null}
          </p>
          <p className="mt-1 text-xs text-muted-foreground">
            {formatWindowLabel(start, end)} • RT &gt; {slowQuery.data?.min_rt ?? minRT}s
          </p>
        </div>
        <div className="flex items-center gap-3">
          <button
            type="button"
            onClick={() => slowQuery.refetch()}
            className="rounded-md border border-border bg-background p-2 text-foreground hover:bg-muted"
            title="Refresh"
          >
            <RefreshCw className={`h-4 w-4 ${slowQuery.isRefetching ? 'animate-spin' : ''}`} />
          </button>
          <Link
            to={dashboardHref}
            className="rounded-md border border-border bg-background px-4 py-2 text-sm font-semibold text-foreground hover:bg-muted"
          >
            Back to Dashboard
          </Link>
        </div>
      </div>

      <div className="rounded-lg border border-border bg-card p-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <p className="text-sm text-muted-foreground">
            Showing page {formatNumber(page, 0)} of {formatNumber(totalPages, 0)} • {formatNumber(total, 0)} total slow requests
          </p>
          <div className="flex items-center gap-2">
            <button
              type="button"
              disabled={page <= 1 || slowQuery.isFetching}
              onClick={() => setPage(page - 1)}
              className="rounded-md border border-border px-3 py-1.5 text-sm font-medium disabled:cursor-not-allowed disabled:opacity-50"
            >
              Previous
            </button>
            <button
              type="button"
              disabled={page >= totalPages || slowQuery.isFetching}
              onClick={() => setPage(page + 1)}
              className="rounded-md border border-border px-3 py-1.5 text-sm font-medium disabled:cursor-not-allowed disabled:opacity-50"
            >
              Next
            </button>
          </div>
        </div>
      </div>

      {slowQuery.isLoading ? (
        <div className="rounded-lg border border-border bg-card p-8 text-center text-sm text-muted-foreground">Loading slow requests...</div>
      ) : slowQuery.isError ? (
        <div className="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-950/30 dark:text-red-300">
          Failed to load slow requests.
        </div>
      ) : rows.length === 0 ? (
        <div className="rounded-lg border border-border bg-card p-8 text-center text-sm text-muted-foreground">
          No slow requests found for this range.
        </div>
      ) : (
        <div className="space-y-2">
          {rows.map((req, index) => (
            <SlowRequestRow
              key={`${req.timestamp}-${req.ip}-${req.path}-${index}`}
              req={req}
              expanded={expanded === index}
              onToggle={() => setExpanded(expanded === index ? null : index)}
            />
          ))}
        </div>
      )}
    </div>
  )
}
