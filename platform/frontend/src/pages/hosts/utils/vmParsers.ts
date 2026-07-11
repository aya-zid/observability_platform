function formatChartTime(unixSeconds: number) {
  return new Date(unixSeconds * 1000).toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
  })
}

export function parseVMSingle(data: any): Array<{ time: string; value: number }> {
  if (!data?.data?.result?.[0]?.values) return []
  return data.data.result[0].values.map(([t, v]: [number, string]) => ({
    time: formatChartTime(t),
    value: parseFloat(v) || 0,
  }))
}

export function parseVMMulti(
  data: any,
  labelKey: string,
): Array<Record<string, string | number>> {
  if (!data?.data?.result) return []
  const timeMap: Record<number, Record<string, string | number>> = {}
  for (const series of data.data.result) {
    const label = series.metric?.[labelKey] ?? "unknown"
    if (!series.values) continue
    for (const [t, v] of series.values) {
      const numT = t as number
      if (!timeMap[numT]) timeMap[numT] = { time: formatChartTime(numT) }
      timeMap[numT][label] = parseFloat(v as string) || 0
    }
  }
  return Object.values(timeMap)
}

export function latestVMValue(data: any): number | null {
  if (!data?.data?.result?.[0]?.values) return null
  const values = data.data.result[0].values
  const last = values[values.length - 1]
  return parseFloat(last[1]) ?? null
}

export function latestVMValueMulti(data: any, labelKey: string): Record<string, number> {
  if (!data?.data?.result) return {}
  const result: Record<string, number> = {}
  for (const series of data.data.result) {
    const label = series.metric?.[labelKey] ?? "unknown"
    const values = series.values
    if (!values?.length) continue
    const last = parseFloat(values[values.length - 1][1])
    if (!isNaN(last)) result[label] = last
  }
  return result
}

export function sumLatestVMValues(data: any, labelFilter?: (label: string) => boolean): number | null {
  const values = latestVMValueMulti(data, labelFilter ? "_dummy" : "_dummy")
  if (!data?.data?.result) return null
  let sum = 0
  let count = 0
  for (const series of data.data.result) {
    const label = series.metric?.[Object.keys(series.metric ?? {})[0]] ?? ""
    if (labelFilter && !labelFilter(label)) continue
    const vals = series.values
    if (!vals?.length) continue
    const last = parseFloat(vals[vals.length - 1][1])
    if (!isNaN(last)) { sum += last; count++ }
  }
  return count > 0 ? sum : null
}
