import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { DeepSeekBillDemo } from '..'
import type { Report } from '../api'

vi.mock('@/lib/api', () => ({
  api: { get: vi.fn() },
  getStatus: vi.fn(async () => ({ usd_exchange_rate: 7.3 })),
}))

test('next page sends the opaque cursor and previous restores the earlier page', async () => {
  useAuthStore.getState().auth.setUser({ id: 1, username: 'alice', role: 1 })
  const totals = {
    requests: 51,
    input: 5100,
    output: 510,
    cache_read: 0,
    peak_units: 0,
    off_peak_units: 51000,
    estimated: 0,
    unpriced: 0,
  }
  vi.mocked(api.get).mockImplementation(async (_url, config) => {
    const next = config?.params?.cursor === 'opaque-next-page'
    const ids = next ? [1] : Array.from({ length: 50 }, (_, i) => 51 - i)
    const report: Report = {
      model: 'deepseek-v4.1-flash',
      snapshot: '2026-11-09',
      totals,
      users: [{ user_id: 1, username: 'alice', ...totals }],
      details: ids.map((id) => ({
        id,
        request_id: `req-${id}`,
        user_id: 1,
        username: 'alice',
        time: 1794186000,
        time_source: 'billing_snapshot',
        peak: false,
        input: 100,
        output: 10,
        cache_read: 0,
        cost_units: 1000,
        estimated: false,
        unpriced: false,
      })),
      detail_total: 51,
      page: next ? 2 : 1,
      next_cursor: next ? undefined : 'opaque-next-page',
    }
    return { data: { success: true, data: report } }
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  })
  render(
    <QueryClientProvider client={client}>
      <DeepSeekBillDemo />
    </QueryClientProvider>
  )
  const user = userEvent.setup()
  expect(await screen.findByText('req-51')).toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: 'Next' }))
  expect(await screen.findByText('req-1')).toBeInTheDocument()
  await waitFor(() =>
    expect(api.get).toHaveBeenLastCalledWith('/api/user/deepseek-bill-demo', {
      params: expect.objectContaining({
        user_id: '1',
        page: 2,
        cursor: 'opaque-next-page',
      }),
    })
  )
  expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled()
  await user.click(screen.getByRole('button', { name: 'Previous' }))
  expect(await screen.findByText('req-51')).toBeInTheDocument()
  expect(screen.queryByText('req-1')).not.toBeInTheDocument()
})
