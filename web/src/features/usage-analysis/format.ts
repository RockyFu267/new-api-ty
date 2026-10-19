// Keep raw token quantities for accounting/export; convert only for display.
export function formatTokenMillions(value: number): string {
  if (value > 0 && value < 10_000) return '<0.01 M'
  return `${(value / 1_000_000).toLocaleString(undefined, {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })} M`
}
