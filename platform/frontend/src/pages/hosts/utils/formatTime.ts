export function formatTimePoints<T extends Record<string, any>>(
  data: T[] | undefined | null,
): T[] {
  if (!data) return []
  return data.map((pt) => ({
    ...pt,
    time: new Date((pt.time as number) * 1000).toLocaleTimeString([], {
      hour: "2-digit",
      minute: "2-digit",
    }),
  }))
}
