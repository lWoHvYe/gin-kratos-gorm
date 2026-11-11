export function formatMoney(cents: number | null | undefined): string {
  const value = Number(cents ?? 0)
  return `¥${(value / 100).toFixed(2)}`
}

export function formatDate(unixSeconds: number | null | undefined): string {
  if (!unixSeconds) return '-'
  return new Date(unixSeconds * 1000).toLocaleString()
}

export function shortId(value: string, keep = 8): string {
  if (value.length <= keep * 2) return value
  return `${value.slice(0, keep)}…${value.slice(-keep)}`
}

export function centsFromInput(value: string): number {
  const normalized = value.trim()
  if (!/^\d+(\.\d{1,2})?$/.test(normalized)) return 0
  return Math.round(Number(normalized) * 100)
}
