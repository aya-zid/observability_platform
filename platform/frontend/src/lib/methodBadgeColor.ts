export function methodBadgeColor(method: string) {
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
