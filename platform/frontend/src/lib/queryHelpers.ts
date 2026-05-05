// src/lib/queryHelpers.ts

/**
 * Query response parsing and formatting helpers for Loki and VictoriaMetrics
 */

export interface TimeSeriesPoint {
  time: number  // unix seconds
  value: number
}

export interface TimeSeries {
  metric: Record<string, string>
  values: TimeSeriesPoint[]
}

export interface LogStream {
  stream: Record<string, string>
  values: Array<[string, string]> // [timestamp_ns, log_line]
}

/**
 * Parse Loki/Prometheus matrix response into chart-ready time series data.
 * Handles both Loki and VictoriaMetrics (Prometheus) responses.
 *
 * @param data - Raw response from /api/query/logs/range or /api/query/metrics
 * @returns Array of time series with metric labels and values
 */
export function parseMatrixResponse(data: any): TimeSeries[] {
  if (!data?.data?.result) return []

  return data.data.result.map((series: any) => ({
    metric: series.metric || {},
    values: (series.values || []).map(([t, v]: [number, string]) => ({
      time: t,
      value: parseFloat(v) || 0,
    }))
  }))
}

/**
 * Parse Loki streams response (used for log lines).
 *
 * @param data - Raw response from /api/query/logs
 * @returns Array of log streams with parsed entries
 */
export function parseStreamsResponse(data: any): LogStream[] {
  if (!data?.data?.result) return []

  return data.data.result.map((stream: any) => ({
    stream: stream.stream || {},
    values: stream.values || []
  }))
}

/**
 * Extract the latest value from a matrix response (for stat cards).
 * Returns the last value from the first time series.
 *
 * @param data - Raw response from query
 * @returns Latest numeric value or null if not found
 */
export function extractLatestValue(data: any): number | null {
  const series = parseMatrixResponse(data)
  if (!series.length || !series[0].values.length) return null

  const vals = series[0].values
  return vals[vals.length - 1].value
}

/**
 * Extract the first (latest in backward order) value from a matrix response.
 *
 * @param data - Raw response from query
 * @returns First value or null if not found
 */
export function extractFirstValue(data: any): number | null {
  const series = parseMatrixResponse(data)
  if (!series.length || !series[0].values.length) return null

  return series[0].values[0].value
}

/**
 * Calculate average of all values in a matrix response.
 *
 * @param data - Raw response from query
 * @returns Average value or null if empty
 */
export function calculateAverage(data: any): number | null {
  const series = parseMatrixResponse(data)
  if (!series.length || !series[0].values.length) return null

  const vals = series[0].values
  const sum = vals.reduce((acc, point) => acc + point.value, 0)
  return Math.round((sum / vals.length) * 100) / 100
}

/**
 * Parse raw log lines from Loki streams response.
 * Extracts just the log line text from each entry.
 *
 * @param data - Raw response from /api/query/logs
 * @returns Array of log line strings, ordered newest first
 */
export function parseLogLines(data: any): string[] {
  const streams = parseStreamsResponse(data)

  return streams
    .flatMap((stream) =>
      (stream.values || []).map(([timestamp, line]: [string, string]) => ({
        timestamp,
        line,
      })),
    )
    .sort((a, b) => {
      try {
        const aNs = BigInt(a.timestamp)
        const bNs = BigInt(b.timestamp)
        if (aNs === bNs) return 0
        return aNs > bNs ? -1 : 1
      } catch {
        return 0
      }
    })
    .map((entry) => entry.line)
}

/**
 * Format Unix timestamp to readable datetime string.
 *
 * @param unixSeconds - Unix timestamp in seconds
 * @returns Formatted string like "Mar 27, 11:05:32"
 */
export function formatTimestamp(unixSeconds: number): string {
  const date = new Date(unixSeconds * 1000)
  return date.toLocaleString('en-US', {
    month: 'short',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false
  })
}

/**
 * Format Unix nanoseconds to readable datetime string.
 *
 * @param unixNano - Unix timestamp in nanoseconds
 * @returns Formatted string
 */
export function formatTimestampNano(unixNano: string | number): string {
  const ns = typeof unixNano === 'string' ? BigInt(unixNano) : BigInt(unixNano)
  const seconds = Number(ns / BigInt(1e9))
  return formatTimestamp(seconds)
}

/**
 * Format a number with commas and appropriate units.
 *
 * @param value - Numeric value
 * @param decimals - Number of decimal places (default 2)
 * @returns Formatted string (e.g., "1,234.56")
 */
export function formatNumber(value: number, decimals = 2): string {
  return value.toLocaleString('en-US', {
    minimumFractionDigits: 0,
    maximumFractionDigits: decimals
  })
}

/**
 * Format a number as a percentage.
 *
 * @param value - Numeric value (0-100 or 0-1)
 * @param decimals - Number of decimal places (default 2)
 * @returns Formatted string like "95.12%"
 */
export function formatPercent(value: number, decimals = 2): string {
  const normalized = value > 1 ? value : value * 100
  return normalized.toLocaleString('en-US', {
    minimumFractionDigits: 0,
    maximumFractionDigits: decimals
  }) + '%'
}

/**
 * Get status color based on error rate percentage.
 *
 * @param errorRatePercent - Error rate as percentage 0-100
 * @returns Status class name: 'green' | 'yellow' | 'red'
 */
export function getErrorRateStatus(errorRatePercent?: number | null): 'green' | 'yellow' | 'red' {
  if (errorRatePercent === null || errorRatePercent === undefined) return 'green'

  if (errorRatePercent < 1) return 'green'
  if (errorRatePercent < 5) return 'yellow'
  return 'red'
}

/**
 * Get status color based on response time (milliseconds).
 *
 * @param ms - Response time in milliseconds
 * @returns Status class name: 'green' | 'yellow' | 'red'
 */
export function getResponseTimeStatus(ms?: number | null): 'green' | 'yellow' | 'red' {
  if (ms === null || ms === undefined) return 'green'

  if (ms < 100) return 'green'
  if (ms < 500) return 'yellow'
  return 'red'
}

/**
 * Parse query counter/count response.
 * Used for queries that return a single count value.
 *
 * @param data - Raw response from a count query
 * @returns Count value or 0 if not found
 */
export function parseCountResponse(data: any): number {
  const series = parseMatrixResponse(data)

  if (!series.length || !series[0].values.length) return 0

  // Sum all values in the response
  return series.reduce((total, s) =>
    total + s.values.reduce((sum, point) => sum + point.value, 0), 0
  )
}

/**
 * Convert seconds to a human-readable duration string.
 *
 * @param seconds - Duration in seconds
 * @returns String like "1h 23m 45s" or "45s"
 */
export function formatDuration(seconds: number): string {
  if (seconds < 60) {
    return `${Math.round(seconds)}s`
  }

  if (seconds < 3600) {
    const minutes = Math.floor(seconds / 60)
    const secs = Math.round(seconds % 60)
    return secs > 0 ? `${minutes}m ${secs}s` : `${minutes}m`
  }

  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  return minutes > 0 ? `${hours}h ${minutes}m` : `${hours}h`
}

function pad2(value: number): string {
  return String(value).padStart(2, '0')
}

/**
 * Format a Date as DD-MM-YYYY for dashboard date inputs.
 */
export function formatDateInputValue(date: Date): string {
  return `${pad2(date.getDate())}-${pad2(date.getMonth() + 1)}-${date.getFullYear()}`
}

/**
 * Format a Date as HH:mm for dashboard time inputs.
 */
export function formatTimeInputValue(date: Date): string {
  return `${pad2(date.getHours())}:${pad2(date.getMinutes())}`
}

/**
 * Parse DD-MM-YYYY plus HH:mm into local unix seconds.
 */
export function dateTimeInputToSeconds(dateValue: string, timeValue: string): number | null {
  const dateMatch = dateValue.trim().match(/^(\d{2})-(\d{2})-(\d{4})$/)
  const timeMatch = timeValue.trim().match(/^(\d{2}):(\d{2})$/)

  if (!dateMatch || !timeMatch) {
    return null
  }

  const day = Number(dateMatch[1])
  const month = Number(dateMatch[2])
  const year = Number(dateMatch[3])
  const hour = Number(timeMatch[1])
  const minute = Number(timeMatch[2])

  if (
    month < 1 || month > 12 ||
    day < 1 || day > 31 ||
    hour < 0 || hour > 23 ||
    minute < 0 || minute > 59
  ) {
    return null
  }

  const parsed = new Date(year, month - 1, day, hour, minute, 0, 0)
  if (
    parsed.getFullYear() !== year ||
    parsed.getMonth() !== month - 1 ||
    parsed.getDate() !== day ||
    parsed.getHours() !== hour ||
    parsed.getMinutes() !== minute
  ) {
    return null
  }

  return Math.floor(parsed.getTime() / 1000)
}

/**
 * Convert bytes to human-readable format.
 *
 * @param bytes - Size in bytes
 * @returns String like "1.5 MB"
 */
export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'

  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))

  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

// ─────────────────────────────────────────────────────────────────────────────
// Nginx-specific parsing and analysis functions
// ─────────────────────────────────────────────────────────────────────────────

/**
 * Extract structured fields from nginx access log line.
 * Parses nginx combined log format:
 * IP - USER [TIMESTAMP] "METHOD PATH HTTP/VERSION" STATUS BYTES "REFERER" "USER_AGENT"
 *
 * @param line - Raw log line from nginx
 * @returns Parsed object or null if parsing fails
 */
export function parseNginxLogLine(line: string) {
  const regex = /^([^ ]+) [^ ]+ [^ ]+ \[([^\]]+)\] "([A-Z]+) ([^ ]+) [^"]*" (\d{3}) ([\d-]+) "([^"]*)" "([^"]*)"/
  const match = line.match(regex)
  if (!match) return null

  const rtMatch = line.match(/\brt=([0-9.]+)\s*$/)

  return {
    ip: match[1],
    timestamp: match[2],
    method: match[3],
    path: match[4],
    status: parseInt(match[5], 10),
    bytes: match[6] === '-' ? 0 : parseInt(match[6], 10),
    referer: match[7],
    user_agent: match[8],
    rt: rtMatch ? parseFloat(rtMatch[1]) || 0 : 0,
  }
}

/**
 * Get HTTP status category
 */
export function getStatusCategory(status: number): 'info' | 'success' | 'redirect' | 'client_error' | 'server_error' {
  if (status < 200) return 'info'
  if (status < 300) return 'success'
  if (status < 400) return 'redirect'
  if (status < 500) return 'client_error'
  return 'server_error'
}

/**
 * Get Tailwind color class for HTTP status
 */
export function getStatusColor(status: number): string {
  const category = getStatusCategory(status)
  const colors: Record<string, string> = {
    info: 'text-blue-500',
    success: 'text-green-500',
    redirect: 'text-yellow-500',
    client_error: 'text-orange-500',
    server_error: 'text-red-500',
  }
  return colors[category]
}

/**
 * Get badge style for HTTP status
 */
export function getStatusBadgeClass(status: number): string {
  const category = getStatusCategory(status)
  const classes: Record<string, string> = {
    info: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300',
    success: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300',
    redirect: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-300',
    client_error: 'bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-300',
    server_error: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-300',
  }
  return classes[category]
}

/**
 * Extract top N paths from nginx logs
 */
export function extractTopPaths(lines: string[], limit = 10): { path: string; count: number; avgBytes: number }[] {
  const paths = new Map<string, { count: number; totalBytes: number }>()

  lines.forEach((line) => {
    const parsed = parseNginxLogLine(line)
    if (!parsed) return

    const key = parsed.path
    const existing = paths.get(key) || { count: 0, totalBytes: 0 }
    paths.set(key, {
      count: existing.count + 1,
      totalBytes: existing.totalBytes + parsed.bytes
    })
  })

  return Array.from(paths.entries())
    .map(([path, { count, totalBytes }]) => ({
      path,
      count,
      avgBytes: Math.round(totalBytes / count)
    }))
    .sort((a, b) => b.count - a.count)
    .slice(0, limit)
}

/**
 * Extract top N status codes from nginx logs
 */
export function extractTopStatuses(lines: string[], limit = 10): { status: number; count: number; percent: number }[] {
  const total = lines.length
  const statuses = new Map<number, number>()

  lines.forEach((line) => {
    const parsed = parseNginxLogLine(line)
    if (!parsed) return

    statuses.set(parsed.status, (statuses.get(parsed.status) ?? 0) + 1)
  })

  return Array.from(statuses.entries())
    .map(([status, count]) => ({
      status,
      count,
      percent: (count / total) * 100
    }))
    .sort((a, b) => b.count - a.count)
    .slice(0, limit)
}

/**
 * Extract top N IP addresses from nginx logs
 */
export function extractTopIPs(lines: string[], limit = 10): { ip: string; count: number }[] {
  const ips = new Map<string, number>()

  lines.forEach((line) => {
    const parsed = parseNginxLogLine(line)
    if (!parsed) return

    ips.set(parsed.ip, (ips.get(parsed.ip) ?? 0) + 1)
  })

  return Array.from(ips.entries())
    .map(([ip, count]) => ({ ip, count }))
    .sort((a, b) => b.count - a.count)
    .slice(0, limit)
}

/**
 * Calculate aggregate statistics from nginx logs
 */
export function calculateNginxStats(lines: string[]) {
  const stats = {
    total: lines.length,
    success: 0,
    redirect: 0,
    client_error: 0,
    server_error: 0,
    total_bytes: 0,
    methods: new Map<string, number>(),
    status_codes: new Map<number, number>(),
  }

  let parsed_count = 0

  lines.forEach((line) => {
    const parsed = parseNginxLogLine(line)
    if (!parsed) return

    parsed_count++
    const category = getStatusCategory(parsed.status)
    stats[category as keyof typeof stats]++
    stats.total_bytes += parsed.bytes
    stats.methods.set(parsed.method, (stats.methods.get(parsed.method) ?? 0) + 1)
    stats.status_codes.set(parsed.status, (stats.status_codes.get(parsed.status) ?? 0) + 1)
  })

  const error_rate = parsed_count > 0
    ? ((stats.client_error + stats.server_error) / parsed_count) * 100
    : 0

  return {
    ...stats,
    parsed_count,
    error_rate: Math.round(error_rate * 100) / 100,
    avg_bytes: parsed_count > 0 ? Math.round(stats.total_bytes / parsed_count) : 0,
  }
}

/**
 * ────────────────────────────────────────────────────────────────────────────
 * Time Calculation & Query Helpers for Loki/Prometheus
 * ────────────────────────────────────────────────────────────────────────────
 */

/**
 * Compute adaptive step for PromQL/LogQL range queries.
 *
 * Aims for ~120 data points regardless of time range.
 * Returns a string like "15s", "5m", "1h" suitable for the step parameter.
 *
 * @param rangeSeconds - Time range in seconds (e.g., 3600 for 1h)
 * @returns Step interval as string (e.g., "60s", "30m", "1h")
 */
export function getStep(rangeSeconds: number): string {
  // Fixed clean steps that are aligned with the dashboard's own getStep().
  // This ensures metrics and logs always use the same step so chart buckets line up.
  if (rangeSeconds <= 900) return '15s'   // 15m → 60 points
  if (rangeSeconds <= 3600) return '30s'  // 1h  → 120 points
  if (rangeSeconds <= 21600) return '2m'  // 6h  → 180 points
  return '5m'                             // 24h → 288 points
}

/**
 * Get current Unix timestamp in seconds (integer).
 * Always call this inside the queryFn — never at component render time —
 * so each fetch gets a fresh "now".
 *
 * @returns Current Unix timestamp in whole seconds
 */
export function getNowSeconds(): number {
  return Math.floor(Date.now() / 1000)
}

/**
 * Compute start time for a range query given a duration.
 * Must be called inside query functions, not at component render time.
 *
 * @param durationSeconds - How far back to query (e.g., 3600 for 1h)
 * @returns Start Unix timestamp in whole seconds
 */
export function getStartSeconds(durationSeconds: number): number {
  return getNowSeconds() - durationSeconds
}

/**
 * Convert Unix seconds to a nanosecond string safe for Loki.
 *
 * WHY A STRING:
 * JS `number` is IEEE-754 float64 with 53-bit mantissa (~15 significant digits).
 * A nanosecond timestamp is ~19 digits (e.g. 1_745_000_000_000_000_000).
 * Doing `seconds * 1e9` as a JS number silently drops the last 4–6 digits,
 * giving Loki a window that is off by up to ±1 000 000 ns (1 ms), which causes
 * the boundary of 15 m / 1 h / 6 h / 24 h windows to be subtly wrong.
 *
 * Fix: perform the multiplication in integer arithmetic using BigInt, then
 * stringify. The HTTP layer sends this as a plain decimal string, which Loki
 * and VictoriaMetrics both accept.
 *
 * @param seconds - Unix timestamp in whole seconds (integer)
 * @returns Nanosecond timestamp as a decimal string (e.g. "1745000000000000000")
 */
export function secondsToNanos(seconds: number): string {
  // BigInt multiplication is exact — no floating-point rounding.
  return (BigInt(Math.floor(seconds)) * 1_000_000_000n).toString()
}
