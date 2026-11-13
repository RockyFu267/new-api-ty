import { api } from '@/lib/api'

export type Totals = {
  actual?: number
  actual_quota?: number
  requests: number
  input: number
  output: number
  cache_read: number
  peak_units: number
  off_peak_units: number
  estimated: number
  unpriced: number
}
export type BillUser = Totals & { user_id: number; username: string }
export type Detail = {
  report_key?: string
  actual?: {
    snapshot: {
      rule: { version: number; fx: number }
      rates: { input: number; cache: number; output: number }
    }
    cny: number
    usd: number
    quota: number
  }
  id: number
  request_id: string
  user_id: number
  username: string
  time: number
  time_source: string
  peak: boolean
  input: number
  output: number
  cache_read: number
  cost_units: number
  estimated: boolean
  unpriced: boolean
}
export type Report = {
  status?: {
    configured: boolean
    history_complete: boolean
    worker_running: boolean
    last_sync: number
    imported_through: number
    source_through: number
  }
  next_cursor?: string
  model: string
  snapshot: string
  totals: Totals
  users: BillUser[]
  details: Detail[]
  detail_total: number
  page: number
}
export type Filters = {
  cursor?: string
  start: string
  end: string
  user_id: string
  page: number
}
export async function getBill(filters: Filters): Promise<Report> {
  const response = await api.get('/api/user/deepseek-bill-demo', {
    params: filters,
  })
  if (!response.data?.success) {
    throw new Error(response.data?.message ?? 'Unable to load bill')
  }
  return response.data.data as Report
}
export function yuan(units: number): string {
  return `¥${(units / 1e8).toFixed(6)}`
}
export function millions(tokens: number): string {
  return `${(tokens / 1e6).toFixed(3)} M`
}
