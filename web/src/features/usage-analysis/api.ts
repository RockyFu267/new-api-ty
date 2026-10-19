import { api } from '@/lib/api'

export type Totals = {
  requests: number
  input: number
  output: number
  cache_read: number
  cache_write: number
  cache_reported: number
  measured: number
  estimated: number
  unknown: number
}
export type Bucket = Totals & {
  name: string
  avoided_usd: number
  priced_requests: number
}
export type Price = {
  id: number
  local_model: string
  reference_model: string
  effective_day: string
  input_price: number
  output_price: number
}
export type Report = {
  totals: Bucket
  periods: Record<string, Bucket>
  trend: Bucket[]
  rows: Bucket[]
  row_count: number
  prices: Price[]
  coverage_start: string
  imported: boolean
  last_sync: number
  pending: number
  month_count: number
  day_count: number
  users: number
  models: number
}
export type Filters = {
  start: string
  end: string
  department_id: string
  department_basis?: string
  user_id: string
  model: string
  local: string
  source?: string
  dimension: string
  page: number
}
export type FilterOptions = {
  departments: { value: string; label: string }[]
  users: { value: string; label: string }[]
  models: { value: string; label: string }[]
}
export async function getFilterOptions(
  filters: Filters
): Promise<FilterOptions> {
  const response = await api.get('/api/user/usage-analysis/options', {
    params: filters,
  })
  if (!response.data.success) throw new Error(response.data.message)
  return response.data.data
}
export async function getReport(filters: Filters): Promise<Report> {
  const response = await api.get('/api/user/usage-analysis', {
    params: filters,
  })
  if (!response.data.success) throw new Error(response.data.message)
  return response.data.data
}
export async function createPrice(price: Omit<Price, 'id'>): Promise<void> {
  const response = await api.post('/api/user/usage-analysis/prices', price)
  if (!response.data.success) throw new Error(response.data.message)
}
export async function exportReport(filters: Filters): Promise<void> {
  const response = await api.get('/api/user/usage-analysis/export', {
    params: filters,
    responseType: 'blob',
  })
  if (String(response.headers['content-type']).includes('application/json')) {
    const result = JSON.parse(await response.data.text()) as { message: string }
    throw new Error(result.message)
  }
  const url = URL.createObjectURL(response.data)
  const link = document.createElement('a')
  link.href = url
  link.download = `usage-analysis-${filters.department_basis ?? 'historical'}-${filters.start}-${filters.end}.csv`
  link.click()
  URL.revokeObjectURL(url)
}

export function beijingDay(date = new Date()): string {
  return new Date(date.getTime() + 8 * 3600_000).toISOString().slice(0, 10)
}

export function groupTrend(rows: Bucket[], period: string): Bucket[] {
  const groups = new Map<string, Bucket>()
  for (const row of rows) {
    let name = row.name
    if (period === 'month') name = name.slice(0, 7)
    if (period === 'week') {
      const day = new Date(`${name}T00:00:00Z`)
      day.setUTCDate(day.getUTCDate() - day.getUTCDay())
      name = day.toISOString().slice(0, 10)
    }
    const existing = groups.get(name)
    if (!existing) {
      groups.set(name, { ...row, name })
      continue
    }
    for (const key of Object.keys(row) as (keyof Bucket)[]) {
      if (key !== 'name') existing[key] += row[key]
    }
  }
  return [...groups.values()]
}
