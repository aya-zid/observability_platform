import { useMemo } from "react"
import type { MariaDBDashboardSummary, MariaDBTimePoint } from "@/api/mariadb"
import { formatDuration, formatNumber, formatPercent, formatBytes } from "@/lib/queryHelpers"
import { cn } from "@/lib/utils"

/* ═══════════════════════════════════════════════════════════════
   MariaDB Summary — decision interface
   Answers in 5 seconds:
     1) Is the system healthy?
     2) If not, what is broken?
     3) Why is it happening?
     4) Where should I click next?
   ═══════════════════════════════════════════════════════════════ */

/* ─── Instance profile (controls all gating) ───────────────── */

interface Profile {
  isReplica: boolean
  isGalera: boolean
  digestsAvailable: boolean
  binlogEnabled: boolean
  qcacheActive: boolean
  isUp: boolean
}

function classify(data: MariaDBDashboardSummary): Profile {
  return {
    isReplica: data.replication_lag_seconds != null,
    isGalera: data.wsrep_cluster_size != null,
    digestsAvailable: (data.top_query_digests?.length ?? 0) > 0,
    binlogEnabled: (data.binlog_size_bytes ?? 0) > 0 || (data.binlog_files ?? 0) > 0,
    qcacheActive: data.qcache_hit_ratio_pct != null,
    isUp: data.up === 1,
  }
}

/* ─── Types ─────────────────────────────────────────────────── */

type Severity = "critical" | "warning" | "notice"

interface Finding {
  severity: Severity
  title: string
  detail: string
  category?: string
}

interface Insight {
  title: string
  detail: string
}

/* ─── Trend helpers ────────────────────────────────────────── */

function tsStartEnd(series: MariaDBTimePoint[], key: string): { first: number | null; last: number | null } {
  if (!series || series.length < 2) return { first: null, last: null }
  let first: number | null = null
  let last: number | null = null
  for (const pt of series) {
    const v = pt[key]
    if (v != null && first == null) first = v as number
  }
  for (let i = series.length - 1; i >= 0; i--) {
    const v = series[i][key]
    if (v != null && last == null) last = v as number
  }
  return { first, last }
}

function trendWord(series: MariaDBTimePoint[], key: string, higherIsBad: boolean): string {
  const { first, last } = tsStartEnd(series, key)
  if (first == null || last == null) return ""
  const pct = ((last - first) / Math.abs(first || 1)) * 100
  if (Math.abs(pct) < 5) return ""
  const going = pct > 0 ? "up" : "down"
  if (higherIsBad) return going === "up" ? ` (rising: ${formatNumber(first, 1)} → ${formatNumber(last, 1)})` : ""
  return ` (${going}: ${formatNumber(first, 1)} → ${formatNumber(last, 1)})`
}

/* ─── Severity tokens (dark-theme tuned) ──────────────────── */

type Tone = "ok" | "warn" | "critical" | "na" | "info"

const TONE = {
  ok:       { dot: "bg-emerald-400",   text: "text-emerald-300",  ring: "ring-emerald-400/30",  bar: "bg-emerald-400",  glow: "shadow-[0_0_24px_-8px_rgba(52,211,153,0.45)]",  surface: "bg-emerald-400/[0.04] border-emerald-400/15" },
  warn:     { dot: "bg-amber-400",     text: "text-amber-300",    ring: "ring-amber-400/30",    bar: "bg-amber-400",    glow: "shadow-[0_0_24px_-8px_rgba(251,191,36,0.5)]",   surface: "bg-amber-400/[0.04] border-amber-400/15" },
  critical: { dot: "bg-rose-500",      text: "text-rose-300",     ring: "ring-rose-500/30",     bar: "bg-rose-500",     glow: "shadow-[0_0_28px_-8px_rgba(244,63,94,0.55)]",   surface: "bg-rose-500/[0.05] border-rose-500/20" },
  info:     { dot: "bg-sky-400",       text: "text-sky-300",      ring: "ring-sky-400/30",      bar: "bg-sky-400",      glow: "shadow-[0_0_24px_-8px_rgba(56,189,248,0.45)]",  surface: "bg-sky-400/[0.04] border-sky-400/15" },
  na:       { dot: "bg-zinc-600",      text: "text-zinc-500",     ring: "ring-zinc-700/40",     bar: "bg-zinc-600",     glow: "",                                              surface: "bg-zinc-500/[0.03] border-zinc-700/40" },
} as const

const SEV_BY_OVERALL: Record<Severity, Tone> = { critical: "critical", warning: "warn", notice: "ok" }

/* ─── Inline iconography (no extra deps) ──────────────────── */

const Icon = {
  Pulse:  (p: any) => (<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" {...p}><path d="M3 12h4l3-8 4 16 3-8h4"/></svg>),
  Bolt:   (p: any) => (<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" {...p}><path d="M13 2 4 14h7l-1 8 9-12h-7l1-8z"/></svg>),
  Chip:   (p: any) => (<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" {...p}><rect x="4" y="4" width="16" height="16" rx="2"/><path d="M9 4v-2M15 4v-2M9 22v-2M15 22v-2M4 9h-2M4 15h-2M22 9h-2M22 15h-2"/><rect x="8" y="8" width="8" height="8" rx="1"/></svg>),
  Db:     (p: any) => (<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" {...p}><ellipse cx="12" cy="5" rx="8" ry="3"/><path d="M4 5v6c0 1.7 3.6 3 8 3s8-1.3 8-3V5"/><path d="M4 11v6c0 1.7 3.6 3 8 3s8-1.3 8-3v-6"/></svg>),
  Link:   (p: any) => (<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" {...p}><path d="M10 13a5 5 0 0 0 7 0l3-3a5 5 0 0 0-7-7l-1 1"/><path d="M14 11a5 5 0 0 0-7 0l-3 3a5 5 0 0 0 7 7l1-1"/></svg>),
  Flame:  (p: any) => (<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" {...p}><path d="M12 2s4 4 4 8a4 4 0 0 1-8 0c0-1.5 1-2.5 1-2.5S5 11 5 15a7 7 0 0 0 14 0c0-6-7-13-7-13z"/></svg>),
  Eye:    (p: any) => (<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" {...p}><path d="M1 12s4-7 11-7 11 7 11 7-4 7-11 7S1 12 1 12z"/><circle cx="12" cy="12" r="3"/></svg>),
  Check:  (p: any) => (<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" {...p}><path d="m5 12 5 5L20 7"/></svg>),
  Alert:  (p: any) => (<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" {...p}><path d="M12 9v4M12 17h.01"/><path d="M10.3 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/></svg>),
  Spark:  (p: any) => (<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" {...p}><path d="M12 3v3M12 18v3M3 12h3M18 12h3M5.6 5.6l2.1 2.1M16.3 16.3l2.1 2.1M5.6 18.4l2.1-2.1M16.3 7.7l2.1-2.1"/></svg>),
  Diamond:(p: any) => (<svg viewBox="0 0 24 24" fill="currentColor" {...p}><path d="M12 2 22 12 12 22 2 12z"/></svg>),
}

/* ═══════════════════════════════════════════════════════════════
   Main component
   ═══════════════════════════════════════════════════════════════ */

export function MariaDBSummaryTab({ data }: { data: MariaDBDashboardSummary }) {
  const profile = useMemo(() => classify(data), [data])
  const uptimeMin = (data.uptime_seconds ?? 0) / 60
  const sqpm = data.slow_queries_per_min ?? 0
  const qps = data.qps ?? 0
  const writes = (data.inserts_per_sec ?? 0) + (data.updates_per_sec ?? 0) + (data.deletes_per_sec ?? 0)
  const reads = data.selects_per_sec ?? 0

  /* ─── Pre-computed scalars used in both findings & overview ── */
  const connSat = data.connection_saturation_pct ?? 0
  const bpHit   = data.buffer_pool_hit_ratio_pct ?? 100
  const bpDirty = data.buffer_pool_dirty_pct ?? 0
  const bpWF    = data.buffer_pool_wait_free_per_sec ?? 0
  const hll     = data.innodb_history_list_length ?? 0
  const lw      = data.innodb_log_waits_per_sec ?? 0
  const fs      = data.innodb_fsyncs_per_sec ?? 0
  const sr      = data.table_scan_ratio_pct ?? 0
  const dl      = data.innodb_deadlocks_per_min ?? 0
  const rlw     = data.innodb_row_lock_waits_per_min ?? 0
  const rlt     = data.innodb_row_lock_time_avg_ms ?? 0
  const tlw     = data.table_locks_waited_per_min ?? 0
  const tdt     = data.tmp_disk_tables_per_sec ?? 0
  const bs      = data.binlog_size_bytes ?? 0
  const tch     = data.table_cache_hit_ratio_pct ?? 100
  const ec      = data.log_summary.errors ?? 0
  const slowRate = qps > 0 ? (sqpm / 60 / qps) * 100 : 0

  /* ────────────────────────────────────────────────────────────
     FINDINGS — only what's wrong
     ──────────────────────────────────────────────────────────── */

  const findings = useMemo<Finding[]>(() => {
    const out: Finding[] = []
    if (!profile.isUp) {
      out.push({ severity: "critical", category: "Availability", title: "Database unreachable", detail: "mysql_up is 0 — the server is not responding." })
      return out
    }
    if (data.scrape_error === 1) {
      out.push({ severity: "notice", category: "Monitoring", title: "Metrics scrape error", detail: "Last exporter scrape failed. Data may be stale — monitoring issue, not database." })
    }
    if (uptimeMin < 15 && (data.log_summary.startup ?? 0) > 0) {
      out.push({ severity: "notice", category: "Availability", title: "Server restarted", detail: `MariaDB restarted ${formatDuration(data.uptime_seconds ?? 0)} ago.` })
    }

    if (profile.isReplica) {
      if (data.slave_running === 0) {
        out.push({ severity: "critical", category: "Replication", title: "Replication stopped", detail: "This replica is not receiving updates. Check SHOW SLAVE STATUS." })
      } else {
        const lag = data.replication_lag_seconds ?? 0
        if (lag > 30) out.push({ severity: "critical", category: "Replication", title: `Replica lag ${formatDuration(lag)}`, detail: "The replica is critically behind its source." })
        else if (lag > 5) out.push({ severity: "warning", category: "Replication", title: `Replica lag ${formatDuration(lag)}`, detail: "The replica is falling behind." })
      }
    }

    if (profile.isGalera) {
      if (data.wsrep_connected === 0) out.push({ severity: "critical", category: "Cluster", title: "Galera node disconnected", detail: "This node is not connected to the cluster." })
      if (data.wsrep_ready === 0) out.push({ severity: "critical", category: "Cluster", title: "Galera node not ready", detail: "Not accepting queries (likely non-Primary after network partition)." })
    }

    if (connSat > 85) {
      out.push({ severity: "critical", category: "Connections", title: `Connection pool ${formatPercent(connSat)} full`, detail: `${data.threads_connected ?? "?"}/${data.max_connections ?? "?"} threads used. Increase max_connections or investigate leaks.` })
    } else if (connSat > 70) {
      out.push({ severity: "warning", category: "Connections", title: `Connection pool ${formatPercent(connSat)} full`, detail: `${data.threads_connected ?? "?"}/${data.max_connections ?? "?"} threads used.` })
    }

    const aborted = data.aborted_connects_per_min ?? 0
    const authLog = data.log_summary.auth_failures ?? 0
    if (aborted > 1 || authLog > 0) {
      const parts: string[] = []
      if (aborted > 1) parts.push(`${formatNumber(aborted, 1)} aborted connects/min`)
      if (authLog > 0) parts.push(`${authLog} auth failures in logs`)
      out.push({
        severity: aborted > 5 || authLog > 5 ? "warning" : "notice",
        category: "Connections",
        title: "Connection or auth failures",
        detail: parts.join("; ") + (aborted > 5 && authLog > 5 ? ". Check connection_errors_by_type." : ""),
      })
    }

    if (sqpm > 0) {
      out.push({
        severity: slowRate > 5 ? "critical" : slowRate > 1 ? "warning" : "notice",
        category: "Queries",
        title: `Slow queries ${formatNumber(sqpm, 2)}/min`,
        detail: `${formatPercent(slowRate)} of all queries.${slowRate < 1 ? " Negligible rate." : slowRate < 5 ? "" : " Significant share of traffic."}`,
      })
    }

    const bpTrend = trendWord(data.buffer_pool_timeseries ?? [], "hit_ratio", true)
    if (bpHit < 90) {
      out.push({ severity: "critical", category: "InnoDB", title: `Buffer pool hit ratio ${formatPercent(bpHit)}`, detail: `${bpWF > 0 ? `Free-page waits (${formatNumber(bpWF, 2)}/sec) — InnoDB is stalling. ` : ""}Increase innodb_buffer_pool_size.${bpTrend}` })
    } else if (bpHit < 99) {
      const warn = bpWF > 0 || bpDirty > 75
      out.push({
        severity: warn ? "warning" : "notice",
        category: "InnoDB",
        title: `Buffer pool hit ratio ${formatPercent(bpHit)}`,
        detail: `${warn ? (bpWF > 0 ? "Free-page waits present. " : "High dirty page ratio. ") : ""}${uptimeMin < 30 ? "Server recently started — cache may not be warm yet." : "Increase innodb_buffer_pool_size."}${bpTrend}`,
      })
    }

    if (hll > 100000) out.push({ severity: "critical", category: "InnoDB", title: `History list ${formatNumber(hll, 0)}`, detail: "Long-running transaction blocking purge. Look for idle-in-transaction sessions." })
    else if (hll > 10000) out.push({ severity: "warning", category: "InnoDB", title: `History list ${formatNumber(hll, 0)}`, detail: "Elevated undo records. Check for long-running transactions." })

    if (lw > 0) out.push({ severity: "warning", category: "InnoDB", title: "Log buffer waits", detail: `${formatNumber(lw, 2)}/sec — innodb_log_buffer_size may need increasing.` })

    const pr = data.innodb_pending_reads ?? 0
    const pw = data.innodb_pending_writes ?? 0
    if (pr > 5 || pw > 5) out.push({ severity: "warning", category: "I/O", title: "Pending I/O operations", detail: `${pr} pending reads, ${pw} pending writes — storage I/O pressure.` })

    const noIdxTxt = (d: typeof data.no_index_digests[0] | undefined) =>
      d ? ` Top unindexed query: "${(d.digest_text || d.digest).slice(0, 60)}…".` : ""
    if (sr > 50) out.push({ severity: "critical", category: "Queries", title: `Full table scans ${formatPercent(sr)} of reads`, detail: `Add indexes.${noIdxTxt(data.no_index_digests?.[0])}` })
    else if (sr > 20) out.push({ severity: "warning", category: "Queries", title: `Full table scans ${formatPercent(sr)} of reads`, detail: noIdxTxt(data.no_index_digests?.[0]) || "Check no-index digest table for missing indexes." })

    const smp = data.sort_merge_passes_per_sec ?? 0
    if (smp > 1) out.push({ severity: "warning", category: "Queries", title: "Disk-based sort merges", detail: `${formatNumber(smp, 2)}/sec — sort_buffer_size undersized.` })
    else if (smp > 0.5) out.push({ severity: "notice", category: "Queries", title: "Occasional disk sorts", detail: `${formatNumber(smp, 2)}/sec — not sustained.` })

    if (dl > 0) {
      const ineff = data.inefficient_digests?.[0]
      const q = ineff ? ` Suspect: "${(ineff.digest_text || ineff.digest).slice(0, 60)}…" (rows/sent ${formatNumber(ineff.rows_examined_per_sent, 0)}).` : ""
      out.push({ severity: dl > 1 ? "critical" : "warning", category: "Locking", title: `Deadlocks ${formatNumber(dl, 2)}/min`, detail: `Review inefficient-query patterns.${q}` })
    }

    if (rlw > 10 && rlt > 50) out.push({ severity: "warning", category: "Locking", title: "Row lock contention", detail: `${formatNumber(rlw, 1)} waits/min, avg ${formatNumber(rlt, 0)} ms — frequent expensive waits.` })
    else if (rlw > 10) out.push({ severity: "notice", category: "Locking", title: "Frequent row lock waits", detail: `${formatNumber(rlw, 1)}/min, avg ${formatNumber(rlt, 0)} ms.` })
    else if (rlt > 50) out.push({ severity: "notice", category: "Locking", title: "High row lock wait time", detail: `${formatNumber(rlt, 0)} ms avg — occasional severe waits.` })

    if (tlw > 0) out.push({ severity: "notice", category: "Locking", title: "Table-level lock waits", detail: `${formatNumber(tlw, 2)}/min — unusual in all-InnoDB; check for MyISAM or LOCK TABLES.` })

    if (tdt > 1) {
      const d = data.inefficient_digests?.[0]
      out.push({ severity: "warning", category: "Memory", title: "Excessive disk temp tables", detail: `${formatNumber(tdt, 2)}/sec. Increase tmp_table_size / max_heap_table_size.${d ? ` Suspect: "${(d.digest_text || d.digest).slice(0, 60)}…"` : ""}` })
    } else if (tdt > 0.5) out.push({ severity: "notice", category: "Memory", title: "Disk temp tables elevated", detail: `${formatNumber(tdt, 2)}/sec.` })

    if (tch < 90) out.push({ severity: "warning", category: "Memory", title: `Table cache hit ratio ${formatPercent(tch)}`, detail: `Increase table_open_cache if open_tables (${data.open_tables ?? "?"}) is near the limit.` })

    if (profile.binlogEnabled && bs > 1_000_000_000) {
      out.push({ severity: "notice", category: "Storage", title: "Binlog size large", detail: `${formatBytes(bs)} across ${data.binlog_files ?? "?"} files. Confirm expire_logs_days / binlog_expire_logs_seconds is configured.` })
    }

    if (ec > 5) out.push({ severity: "warning", category: "Logs", title: "Errors in MariaDB logs", detail: `${ec} error events in this window. Check recent_logs.` })
    else if (ec > 0) out.push({ severity: "notice", category: "Logs", title: "Minor log errors", detail: `${ec} event(s).` })

    if (!profile.digestsAvailable) {
      out.push({ severity: "notice", category: "Observability", title: "Query analysis unavailable", detail: "No digest data — performance_schema may be off or the collector hasn't run. Not the same as having no slow queries." })
    }

    if (fs > 200) out.push({ severity: "notice", category: "I/O", title: "High fsync rate", detail: `${formatNumber(fs, 0)}/sec — check storage latency if writes are slow.` })

    return out
  }, [data, profile, uptimeMin, sqpm, qps, slowRate, connSat, bpHit, bpDirty, bpWF, hll, lw, fs, sr, dl, rlw, rlt, tlw, tdt, bs, tch, ec])

  /* ────────────────────────────────────────────────────────────
     CORRELATED INSIGHTS
     ──────────────────────────────────────────────────────────── */

  const insights = useMemo<Insight[]>(() => {
    const out: Insight[] = []
    const ioReads = data.innodb_data_reads_per_sec ?? 0
    if (bpHit < 95 && ioReads > 100000) {
      out.push({
        title: "Buffer pool pressure driving disk I/O",
        detail: `Cache miss rate (${formatPercent(bpHit)} hit ratio) and physical reads (${formatBytes(ioReads)}/s) are both elevated.${bpWF > 0 ? " Free-page waits confirm InnoDB is stalling." : ""} Increasing innodb_buffer_pool_size should reduce both.`,
      })
    }
    if (sr > 20 && profile.digestsAvailable) {
      const q = data.no_index_digests?.[0]
      if (q) out.push({
        title: "Full table scans linked to unindexed query",
        detail: `"${(q.digest_text || q.digest).slice(0, 70)}…" is the top no-index query and table scans are ${formatPercent(sr)} of reads. Adding an index for this pattern would reduce scan overhead.`,
      })
    }
    if (dl > 0) {
      const q = data.inefficient_digests?.[0]
      if (q) out.push({
        title: "Deadlocks likely caused by inefficient queries",
        detail: `Deadlocks (${formatNumber(dl, 2)}/min) and inefficient queries overlap. "${(q.digest_text || q.digest).slice(0, 70)}…" has a rows-examined-per-sent ratio of ${formatNumber(q.rows_examined_per_sent, 0)} — poor index coverage forces long-held locks.`,
      })
    }
    if (connSat > 60 && rlw > 5) {
      out.push({
        title: "High connection usage coinciding with row lock waits",
        detail: `${formatPercent(connSat)} connection saturation with ${formatNumber(rlw, 1)} row lock waits/min — threads may be accumulating while waiting on locks. Review the inefficient-queries table for lock-heavy patterns.`,
      })
    }
    return out
  }, [data, profile, bpHit, bpWF, sr, dl, connSat, rlw])

  /* ────────────────────────────────────────────────────────────
     CONTEXT SIGNALS
     ──────────────────────────────────────────────────────────── */

  const contextSignals = useMemo<Insight[]>(() => {
    const out: Insight[] = []
    if (reads + writes > 0) {
      const rwRatio = writes > 0 ? (reads / writes).toFixed(1) : "∞"
      out.push({ title: "Workload profile", detail: `${formatNumber(reads, 1)} reads/s · ${formatNumber(writes, 1)} writes/s · ratio ${rwRatio}:1` })
    }
    const td = data.top_query_digests?.[0]
    const tdtTotal = data.query_digest_total_time_seconds ?? 0
    if (td && tdtTotal > 0 && td.total_time_seconds / tdtTotal > 0.35) {
      out.push({ title: "Hot query dominates", detail: `"${(td.digest_text || td.digest).slice(0, 70)}…" — ${formatPercent((td.total_time_seconds / tdtTotal) * 100)} of digest time (${formatDuration(td.total_time_seconds)}).` })
    }
    const dbStor = data.db_storage_sizes ?? []
    const totalStor = dbStor.reduce((s: number, d: { count: number }) => s + d.count, 0)
    const topDb = dbStor[0]
    if (topDb && totalStor > 0 && topDb.count / totalStor > 0.6) {
      out.push({ title: "Storage concentrated", detail: `"${topDb.label}" holds ${formatPercent((topDb.count / totalStor) * 100)} of total storage.` })
    }
    if (profile.digestsAvailable && data.inefficient_digests?.length === 0 && data.no_index_digests?.length === 0) {
      out.push({ title: "Clean query patterns", detail: "Digest analysis shows no inefficient or unindexed patterns." })
    }
    if (profile.qcacheActive) {
      const qh = data.qcache_hit_ratio_pct ?? 0
      if (qh < 50) out.push({ title: "Query cache underused", detail: `${formatPercent(qh)} hit ratio — feature is deprecated; consider disabling.` })
    }
    return out
  }, [data, profile, reads, writes])

  /* ────────────────────────────────────────────────────────────
     SORTED FINDINGS + OVERALL SEVERITY + HEALTH SCORE
     ──────────────────────────────────────────────────────────── */

  const sorted = useMemo(() => {
    const bySev: Record<Severity, Finding[]> = { critical: [], warning: [], notice: [] }
    for (const f of findings) bySev[f.severity].push(f)
    return bySev
  }, [findings])

  const overall: Severity = sorted.critical.length ? "critical" : sorted.warning.length ? "warning" : "notice"
  const overallTone = SEV_BY_OVERALL[overall]

  const healthScore = useMemo(() => {
    if (!profile.isUp) return 0
    const penalty = sorted.critical.length * 18 + sorted.warning.length * 7 + sorted.notice.length * 2
    return Math.max(5, Math.min(100, 100 - penalty))
  }, [profile.isUp, sorted])

  const verdict = !profile.isUp
    ? { headline: "Instance is DOWN", sub: "MariaDB is unreachable. Restore the server before further triage." }
    : overall === "critical" ? { headline: "Action required", sub: `${sorted.critical.length} critical issue${sorted.critical.length > 1 ? "s" : ""} blocking healthy operation.` }
    : overall === "warning"  ? { headline: "Operating degraded", sub: `${sorted.warning.length} warning${sorted.warning.length > 1 ? "s" : ""} — performance impacted but stable.` }
    : sorted.notice.length    ? { headline: "All systems nominal", sub: `${sorted.notice.length} informational notice${sorted.notice.length > 1 ? "s" : ""} — no action required.` }
    :                            { headline: "All systems nominal", sub: "Every health check passed in this window." }

  /* ────────────────────────────────────────────────────────────
     PULSE STRIP — 6 KPIs
     ──────────────────────────────────────────────────────────── */

  const pulse = useMemo(() => {
    const items: Array<{ label: string; value: string; sub: string; tone: Tone; icon: any }> = []

    items.push({
      label: "Throughput",
      value: `${formatNumber(qps, 1)}`,
      sub: "queries / sec",
      tone: profile.isUp ? "info" : "critical",
      icon: Icon.Bolt,
    })

    items.push({
      label: "Connections",
      value: formatPercent(connSat),
      sub: `${data.threads_connected ?? "?"} of ${data.max_connections ?? "?"}`,
      tone: connSat > 85 ? "critical" : connSat > 70 ? "warn" : "ok",
      icon: Icon.Link,
    })

    items.push({
      label: "Buffer Pool",
      value: formatPercent(bpHit),
      sub: `hit · ${formatPercent(bpDirty)} dirty`,
      tone: bpHit < 90 ? "critical" : bpHit < 99 ? "warn" : "ok",
      icon: Icon.Chip,
    })

    items.push({
      label: "Slow Queries",
      value: sqpm > 0 ? `${formatNumber(sqpm, 2)}` : "0",
      sub: sqpm > 0 ? `${formatPercent(slowRate)} of traffic` : "none / min",
      tone: slowRate > 5 ? "critical" : slowRate > 1 ? "warn" : sqpm > 0 ? "info" : "ok",
      icon: Icon.Flame,
    })

    if (profile.isReplica) {
      const lag = data.replication_lag_seconds ?? 0
      items.push({
        label: "Replica Lag",
        value: data.slave_running === 0 ? "STOPPED" : formatDuration(lag),
        sub: data.slave_running === 0 ? "replication halted" : "behind source",
        tone: data.slave_running === 0 || lag > 30 ? "critical" : lag > 5 ? "warn" : "ok",
        icon: Icon.Pulse,
      })
    } else if (profile.isGalera) {
      items.push({
        label: "Cluster",
        value: `${data.wsrep_cluster_size ?? 0} nodes`,
        sub: data.wsrep_ready === 0 ? "not ready" : data.wsrep_connected === 0 ? "disconnected" : "synced",
        tone: data.wsrep_connected === 0 || data.wsrep_ready === 0 ? "critical" : "ok",
        icon: Icon.Pulse,
      })
    } else {
      items.push({
        label: "Uptime",
        value: formatDuration(data.uptime_seconds ?? 0),
        sub: "since last restart",
        tone: uptimeMin < 15 ? "info" : "ok",
        icon: Icon.Pulse,
      })
    }

    const total = reads + writes
    const readPct = total > 0 ? (reads / total) * 100 : 0
    items.push({
      label: "Workload Mix",
      value: total > 0 ? `${readPct.toFixed(0)}% R` : "—",
      sub: total > 0 ? `${(100 - readPct).toFixed(0)}% W · ${formatNumber(total, 0)} ops/s` : "no traffic",
      tone: "ok",
      icon: Icon.Eye,
    })

    return items
  }, [data, profile, qps, connSat, bpHit, bpDirty, sqpm, slowRate, uptimeMin, reads, writes])

  /* ────────────────────────────────────────────────────────────
     SYSTEM MAP — 15 rows
     ──────────────────────────────────────────────────────────── */

  const overview = useMemo(() => {
    const rows: Array<{ key: string; label: string; status: Tone; value: string; group: string }> = [
      { key: "status", group: "Service", label: "Availability", status: profile.isUp ? (data.scrape_error === 1 ? "info" : "ok") : "critical", value: profile.isUp ? `Up ${formatDuration(data.uptime_seconds ?? 0)}` : "Down" },
      { key: "replication", group: "Service", label: "Replication", status: profile.isReplica ? (data.slave_running === 0 ? "critical" : (data.replication_lag_seconds ?? 0) > 5 ? "warn" : "ok") : "na", value: profile.isReplica ? (data.slave_running === 0 ? "Stopped" : `Lag ${formatDuration(data.replication_lag_seconds ?? 0)}`) : "Not configured" },
      { key: "galera", group: "Service", label: "Cluster", status: profile.isGalera ? (data.wsrep_connected === 0 || data.wsrep_ready === 0 ? "critical" : "ok") : "na", value: profile.isGalera ? `${data.wsrep_cluster_size} nodes` : "Not clustered" },
      { key: "connections", group: "Workload", label: "Connections", status: connSat > 85 ? "critical" : connSat > 70 ? "warn" : "ok", value: `${formatPercent(connSat)} · ${data.threads_connected ?? "?"}/${data.max_connections ?? "?"}` },
      { key: "throughput", group: "Workload", label: "Throughput", status: slowRate > 5 ? "critical" : slowRate > 1 ? "warn" : sqpm > 0 ? "info" : "ok", value: `${formatNumber(qps, 1)} QPS · ${formatNumber(sqpm, 2)} slow/min` },
      { key: "scans", group: "Workload", label: "Full Scans", status: sr > 50 ? "critical" : sr > 20 ? "warn" : "ok", value: `${formatPercent(sr)} of reads` },
      { key: "buffer_pool", group: "InnoDB", label: "Buffer Pool", status: bpHit < 90 ? "critical" : bpHit < 99 ? "warn" : "ok", value: `${formatPercent(bpHit)} hit · ${formatPercent(bpDirty)} dirty` },
      { key: "innodb_health", group: "InnoDB", label: "InnoDB Health", status: hll > 100000 ? "critical" : hll > 10000 || lw > 0 ? "warn" : "ok", value: `HLL ${formatNumber(hll, 0)}${lw > 0 ? ` · log waits ${formatNumber(lw, 2)}/s` : ""}` },
      { key: "io", group: "InnoDB", label: "Disk I/O", status: fs > 200 ? "info" : "ok", value: `${formatBytes(data.innodb_data_reads_per_sec ?? 0)}/s reads` },
      { key: "locks", group: "Contention", label: "Locks", status: dl > 1 ? "critical" : dl > 0 || rlw > 10 ? "warn" : tlw > 0 ? "info" : "ok", value: `${formatNumber(dl, 2)} dl/min · ${formatNumber(rlw, 1)} waits/min` },
      { key: "tmp", group: "Contention", label: "Temp Tables", status: tdt > 1 ? "warn" : tdt > 0.5 ? "info" : "ok", value: `${formatNumber(tdt, 2)} disk/s` },
      { key: "cache", group: "Contention", label: "Table Cache", status: tch < 90 ? "warn" : "ok", value: `${formatPercent(tch)} hit` },
      { key: "storage", group: "Platform", label: "Storage", status: profile.binlogEnabled && bs > 1_000_000_000 ? "info" : "ok", value: profile.binlogEnabled ? `${formatBytes(bs)} binlog` : "Binlog off" },
      { key: "digests", group: "Platform", label: "Query Analysis", status: profile.digestsAvailable ? "ok" : "na", value: profile.digestsAvailable ? `${data.top_query_digests?.length ?? 0} patterns` : "Unavailable" },
      { key: "logs", group: "Platform", label: "Logs", status: ec > 5 ? "warn" : ec > 0 ? "info" : "ok", value: `${ec} err · ${data.log_summary.warnings ?? 0} warn` },
    ]
    return rows
  }, [data, profile, sqpm, qps, slowRate, bpHit, bpDirty, hll, lw, fs, sr, dl, rlw, tlw, tdt, bs, tch, ec, connSat])

  /* ────────────────────────────────────────────────────────────
     HOTSPOTS — top 3 query digests by total time
     ──────────────────────────────────────────────────────────── */

  const hotspots = useMemo(() => {
    const tot = data.query_digest_total_time_seconds ?? 0
    const list = data.top_query_digests ?? []
    return list.slice(0, 3).map((d) => ({
      text: (d.digest_text || d.digest).slice(0, 90),
      pct: tot > 0 ? (d.total_time_seconds / tot) * 100 : 0,
      time: d.total_time_seconds,
      execs: d.exec_count,
      db: d.database,
    }))
  }, [data])

  /* ─── Render ───────────────────────────────────────────────── */

  const total = reads + writes
  const readPct = total > 0 ? (reads / total) * 100 : 0

  return (
    <div className="space-y-6">

      {/* ═══════════════════════════════════════════════════════════════
          1 · HEALTH HERO
          ═══════════════════════════════════════════════════════════════ */}
      <section
        className={cn(
          "relative overflow-hidden rounded-2xl border bg-gradient-to-br from-zinc-950 via-zinc-900 to-zinc-950 p-6 md:p-7",
          TONE[overallTone].surface.split(" ")[1],
        )}
      >
        <div className={cn("pointer-events-none absolute -top-24 -right-24 h-72 w-72 rounded-full blur-3xl opacity-30", overall === "critical" ? "bg-rose-500" : overall === "warning" ? "bg-amber-400" : "bg-emerald-400")} />
        <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_30%_0%,rgba(255,255,255,0.04),transparent_50%)]" />

        <div className="relative grid gap-6 md:grid-cols-[auto,1fr,auto] md:items-center">
          <ScoreRing score={healthScore} tone={overallTone} />

          <div className="min-w-0">
            <div className="flex items-center gap-2 text-[10px] font-semibold uppercase tracking-[0.18em] text-zinc-500">
              <span className={cn("h-1.5 w-1.5 rounded-full animate-pulse", TONE[overallTone].dot)} />
              MariaDB · Summary
            </div>
            <h2 className="mt-2 text-2xl md:text-3xl font-semibold tracking-tight text-zinc-50">
              {verdict.headline}
            </h2>
            <p className="mt-1.5 text-sm text-zinc-400 leading-relaxed max-w-xl">{verdict.sub}</p>

            <div className="mt-4 flex flex-wrap items-center gap-1.5">
              <Badge>{profile.isReplica ? "Replica" : profile.isGalera ? "Galera" : "Standalone"}</Badge>
              <Badge>{profile.digestsAvailable ? "Digests on" : "Digests off"}</Badge>
              {profile.binlogEnabled && <Badge>Binlog</Badge>}
              {profile.qcacheActive && <Badge>QCache</Badge>}
              <Badge>Uptime {formatDuration(data.uptime_seconds ?? 0)}</Badge>
            </div>
          </div>

          <div className="grid grid-cols-3 gap-2 md:w-56">
            <Tally tone="critical" label="Critical" count={sorted.critical.length} />
            <Tally tone="warn"     label="Warning"  count={sorted.warning.length} />
            <Tally tone="info"     label="Notice"   count={sorted.notice.length} />
          </div>
        </div>
      </section>

      {/* ═══════════════════════════════════════════════════════════════
          2 · PULSE STRIP
          ═══════════════════════════════════════════════════════════════ */}
      <section className="grid gap-3 grid-cols-2 md:grid-cols-3 xl:grid-cols-6">
        {pulse.map((p, i) => (
          <KpiCard key={i} {...p} />
        ))}
      </section>

      {/* ═══════════════════════════════════════════════════════════════
          3 · ACTION CENTER  |  DIAGNOSTICS
          ═══════════════════════════════════════════════════════════════ */}
      <section className="grid gap-4 lg:grid-cols-5">

        {/* ── Action Center ── */}
        <div className="lg:col-span-3 space-y-4">
          <Panel
            title="Action Center"
            sub={findings.length ? `${findings.length} findings · sorted by severity` : "No findings"}
            icon={<Icon.Alert className="h-4 w-4 text-zinc-400" />}
          >
            {findings.length === 0 ? (
              <EmptyState
                icon={<Icon.Check className="h-6 w-6 text-emerald-400" />}
                title="Nothing to act on"
                detail="All health checks passed in the current window."
              />
            ) : (
              <div className="space-y-5">
                {(["critical", "warning", "notice"] as Severity[]).map((sev) => {
                  const list = sorted[sev]
                  if (!list.length) return null
                  const tone = SEV_BY_OVERALL[sev]
                  const heading = sev === "critical" ? "Critical" : sev === "warning" ? "Warnings" : "Notices"
                  return (
                    <div key={sev}>
                      <div className="mb-2 flex items-center gap-2 text-[10px] font-semibold uppercase tracking-[0.2em]">
                        <span className={cn("h-1.5 w-1.5 rounded-full", TONE[tone].dot)} />
                        <span className={TONE[tone].text}>{heading}</span>
                        <span className="text-zinc-600">· {list.length}</span>
                      </div>
                      <ol className="space-y-1.5">
                        {list.map((f, i) => (
                          <FindingRow key={i} index={i + 1} finding={f} tone={tone} />
                        ))}
                      </ol>
                    </div>
                  )
                })}
              </div>
            )}
          </Panel>
        </div>

        {/* ── Diagnostics ── */}
        <div className="lg:col-span-2 space-y-4">

          <Panel
            title="Correlated Insights"
            sub={insights.length ? `${insights.length} cross-signal pattern${insights.length > 1 ? "s" : ""}` : "Cross-signal analysis"}
            icon={<Icon.Diamond className="h-3.5 w-3.5 text-violet-400" />}
          >
            {insights.length === 0 ? (
              <p className="text-sm text-zinc-500">No correlations detected between independent signals.</p>
            ) : (
              <ul className="space-y-2.5">
                {insights.map((ins, i) => (
                  <li key={i} className="rounded-lg border border-violet-400/15 bg-violet-400/[0.04] p-3.5">
                    <div className="flex items-start gap-2.5">
                      <Icon.Diamond className="mt-1 h-2.5 w-2.5 shrink-0 text-violet-400" />
                      <div className="min-w-0">
                        <div className="text-sm font-medium text-zinc-100">{ins.title}</div>
                        <p className="mt-1 text-[13px] leading-relaxed text-zinc-400">{ins.detail}</p>
                      </div>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </Panel>

          <Panel
            title="Query Hotspots"
            sub="Top patterns by total execution time"
            icon={<Icon.Flame className="h-4 w-4 text-orange-400" />}
          >
            {hotspots.length === 0 ? (
              <p className="text-sm text-zinc-500">
                {profile.digestsAvailable ? "No query digest data in this window." : "Query digests unavailable — enable performance_schema."}
              </p>
            ) : (
              <ul className="space-y-3">
                {hotspots.map((h: { text: string; pct: number; time: number; execs: number; db: string }, i: number) => (
                  <li key={i} className="space-y-1.5">
                    <div className="flex items-baseline justify-between gap-3">
                      <span className="font-mono text-[12px] text-zinc-200 truncate" title={h.text}>{h.text}{h.text.length >= 90 ? "…" : ""}</span>
                      <span className="shrink-0 text-xs tabular-nums text-zinc-400">{formatPercent(h.pct)}</span>
                    </div>
                    <div className="h-1 w-full overflow-hidden rounded-full bg-zinc-800">
                      <div className="h-full rounded-full bg-gradient-to-r from-orange-500 to-rose-500" style={{ width: `${Math.max(2, h.pct)}%` }} />
                    </div>
                    <div className="flex items-center gap-2 text-[11px] text-zinc-500">
                      <span className="font-mono">{h.db}</span>
                      <span>·</span>
                      <span>{formatNumber(h.execs, 0)} execs</span>
                      <span>·</span>
                      <span>{formatDuration(h.time)}</span>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </Panel>

          {total > 0 && (
            <Panel
              title="Workload Mix"
              sub={`${formatNumber(total, 0)} ops/s · ratio ${(writes > 0 ? reads / writes : Infinity).toFixed(1)}:1`}
              icon={<Icon.Eye className="h-4 w-4 text-sky-400" />}
            >
              <div className="flex h-2 w-full overflow-hidden rounded-full bg-zinc-800">
                <div className="bg-sky-500" style={{ width: `${readPct}%` }} />
                <div className="bg-emerald-500" style={{ width: `${100 - readPct}%` }} />
              </div>
              <div className="mt-2 flex justify-between text-[11px] text-zinc-400">
                <span><span className="inline-block h-1.5 w-1.5 rounded-full bg-sky-500 mr-1.5" />Reads {formatNumber(reads, 1)}/s</span>
                <span><span className="inline-block h-1.5 w-1.5 rounded-full bg-emerald-500 mr-1.5" />Writes {formatNumber(writes, 1)}/s</span>
              </div>
            </Panel>
          )}

          {contextSignals.length > 0 && (
            <Panel
              title="Context Signals"
              sub="Non-alert intelligence"
              icon={<Icon.Spark className="h-4 w-4 text-zinc-400" />}
            >
              <div className="flex flex-wrap gap-1.5">
                {contextSignals.map((c, i) => (
                  <span key={i} className="rounded-md border border-zinc-800 bg-zinc-900/60 px-2.5 py-1.5 text-[12px] text-zinc-400">
                    <span className="font-medium text-zinc-200">{c.title}:</span> {c.detail}
                  </span>
                ))}
              </div>
            </Panel>
          )}
        </div>
      </section>

      {/* ═══════════════════════════════════════════════════════════════
          4 · SYSTEM MAP
          ═══════════════════════════════════════════════════════════════ */}
      <Panel
        title="System Map"
        sub="Full subsystem state — 15 checks"
        icon={<Icon.Db className="h-4 w-4 text-zinc-400" />}
      >
        <div className="grid gap-px overflow-hidden rounded-lg border border-zinc-800 bg-zinc-800 sm:grid-cols-2 lg:grid-cols-3">
          {overview.map((row) => (
            <div key={row.key} className="group flex items-center justify-between gap-3 bg-zinc-950 px-3.5 py-3 transition-colors hover:bg-zinc-900">
              <div className="flex min-w-0 items-center gap-2.5">
                <span className={cn("h-1.5 w-1.5 shrink-0 rounded-full", TONE[row.status].dot)} />
                <div className="min-w-0">
                  <div className="text-[10px] font-medium uppercase tracking-wider text-zinc-500">{row.group}</div>
                  <div className="truncate text-sm text-zinc-200">{row.label}</div>
                </div>
              </div>
              <span className={cn("shrink-0 text-right text-xs font-medium tabular-nums", TONE[row.status].text)}>
                {row.status === "na" ? "—" : row.value}
              </span>
            </div>
          ))}
        </div>
      </Panel>
    </div>
  )
}

/* ═══════════════════════════════════════════════════════════════
   Local presentational components
   ═══════════════════════════════════════════════════════════════ */

function Panel({ title, sub, icon, children }: { title: string; sub?: string; icon?: React.ReactNode; children: React.ReactNode }) {
  return (
    <section className="rounded-2xl border border-zinc-800/80 bg-zinc-950/40 p-5">
      <header className="mb-4 flex items-center justify-between gap-3">
        <div className="flex items-center gap-2.5">
          {icon}
          <div>
            <h3 className="text-sm font-semibold text-zinc-100">{title}</h3>
            {sub && <p className="text-[11px] text-zinc-500">{sub}</p>}
          </div>
        </div>
      </header>
      {children}
    </section>
  )
}

function Badge({ children }: { children: React.ReactNode }) {
  return (
    <span className="inline-flex items-center rounded-md border border-zinc-800 bg-zinc-900/70 px-2 py-0.5 text-[10px] font-medium uppercase tracking-wider text-zinc-400">
      {children}
    </span>
  )
}

function Tally({ tone, label, count }: { tone: Tone; label: string; count: number }) {
  const active = count > 0
  return (
    <div className={cn(
      "rounded-xl border bg-zinc-950/60 px-3 py-3 text-center transition-colors",
      active ? TONE[tone].surface : "border-zinc-800/60",
    )}>
      <div className={cn("text-2xl font-semibold tabular-nums", active ? TONE[tone].text : "text-zinc-600")}>
        {count}
      </div>
      <div className="mt-0.5 text-[10px] font-medium uppercase tracking-wider text-zinc-500">{label}</div>
    </div>
  )
}

function KpiCard({ label, value, sub, tone, icon: I }: { label: string; value: string; sub: string; tone: Tone; icon: any }) {
  const t = TONE[tone]
  return (
    <div className={cn(
      "group relative overflow-hidden rounded-xl border border-zinc-800/80 bg-zinc-950/60 p-4 transition-all hover:border-zinc-700",
      tone !== "ok" && tone !== "na" && t.glow,
    )}>
      <div className="flex items-center justify-between">
        <span className="text-[10px] font-semibold uppercase tracking-[0.14em] text-zinc-500">{label}</span>
        <I className={cn("h-3.5 w-3.5", t.text)} />
      </div>
      <div className={cn("mt-2 text-2xl font-semibold tabular-nums tracking-tight", tone === "ok" ? "text-zinc-100" : t.text)}>
        {value}
      </div>
      <div className="mt-0.5 text-[11px] text-zinc-500">{sub}</div>
      <div className={cn("pointer-events-none absolute inset-x-0 bottom-0 h-px", t.bar, "opacity-60")} />
    </div>
  )
}

function FindingRow({ index, finding, tone }: { index: number; finding: Finding; tone: Tone }) {
  const t = TONE[tone]
  return (
    <li className={cn("group relative overflow-hidden rounded-lg border bg-zinc-950/40 pl-4 pr-4 py-3 transition-colors hover:bg-zinc-900/40", t.surface)}>
      <span className={cn("absolute inset-y-0 left-0 w-[3px]", t.bar)} />
      <div className="flex items-start gap-3">
        <span className={cn("mt-0.5 grid h-5 w-5 shrink-0 place-items-center rounded-md text-[10px] font-semibold tabular-nums", t.text, "bg-zinc-900/80 border border-zinc-800")}>
          {index}
        </span>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-baseline gap-x-2">
            <span className={cn("text-sm font-medium", "text-zinc-100")}>{finding.title}</span>
            {finding.category && (
              <span className="text-[10px] uppercase tracking-wider text-zinc-500">· {finding.category}</span>
            )}
          </div>
          <p className="mt-1 text-[13px] leading-relaxed text-zinc-400">{finding.detail}</p>
        </div>
      </div>
    </li>
  )
}

function EmptyState({ icon, title, detail }: { icon: React.ReactNode; title: string; detail: string }) {
  return (
    <div className="flex flex-col items-center justify-center gap-2 py-10 text-center">
      <div className="grid h-12 w-12 place-items-center rounded-full border border-emerald-400/20 bg-emerald-400/5">
        {icon}
      </div>
      <div className="text-sm font-medium text-zinc-200">{title}</div>
      <p className="max-w-xs text-[13px] text-zinc-500">{detail}</p>
    </div>
  )
}

function ScoreRing({ score, tone }: { score: number; tone: Tone }) {
  const r = 44
  const c = 2 * Math.PI * r
  const offset = c - (score / 100) * c
  const stroke = tone === "critical" ? "#f43f5e" : tone === "warn" ? "#fbbf24" : tone === "info" ? "#38bdf8" : "#34d399"
  return (
    <div className="relative h-[112px] w-[112px] shrink-0">
      <svg viewBox="0 0 100 100" className="h-full w-full -rotate-90">
        <circle cx="50" cy="50" r={r} stroke="rgba(255,255,255,0.06)" strokeWidth="6" fill="none" />
        <circle
          cx="50" cy="50" r={r}
          stroke={stroke}
          strokeWidth="6"
          strokeLinecap="round"
          fill="none"
          strokeDasharray={c}
          strokeDashoffset={offset}
          style={{ transition: "stroke-dashoffset 600ms ease, stroke 300ms" }}
        />
      </svg>
      <div className="absolute inset-0 flex flex-col items-center justify-center">
        <div className="text-2xl font-semibold tabular-nums text-zinc-100">{score}</div>
        <div className="text-[9px] font-semibold uppercase tracking-[0.18em] text-zinc-500">Health</div>
      </div>
    </div>
  )
}
