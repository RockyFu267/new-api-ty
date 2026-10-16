import { api } from '@/lib/api'

export type Period = {
  start: number
  reset_at: number
  limit: number
  used: number
  reserved: number
}
export type QuotaUsage = {
  user_id: number
  username: string
  department_id: number
  department_name: string
  configured: boolean
  wallet_monthly_used: number
  wallet: number
  daily: Period
  weekly: Period
  monthly: Period
}
export type QuotaRule = {
  department_id: number
  enabled: boolean
  daily: number
  weekly: number
  monthly: number
}
export type CompanyLog = {
  id: number
  created_at: number
  user_id: number
  username: string
  model_name: string
  quota: number
  prompt_tokens: number
  completion_tokens: number
  request_id: string
  other: string
}
export type Page<T> = { items: T[]; total: number }

export async function loadCompanyData<T>(
  path: string,
  params?: Record<string, string | number>
): Promise<T> {
  const response = await api.get(`/api/user/${path}`, { params })
  if (!response.data.success) {
    throw new Error(response.data.message || 'Unable to load data')
  }
  return response.data.data
}

export async function saveCompanyData(
  path: string,
  data: unknown
): Promise<void> {
  const response = await api.put(`/api/user/${path}`, data)
  if (!response.data.success) {
    throw new Error(response.data.message || 'Unable to save data')
  }
}

export function parseBillingOther(log: CompanyLog): Record<string, unknown> {
  try {
    return JSON.parse(log.other || '{}')
  } catch {
    return {}
  }
}
