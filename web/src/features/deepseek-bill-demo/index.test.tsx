import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { DeepSeekBillDemo } from '.'
import type { Report } from './api'

vi.mock('@/lib/api', () => ({
  api: { get: vi.fn() },
  getStatus: vi.fn(async () => ({ usd_exchange_rate: 7.3 })),
}))

function report(): Report {
  const totals = {
    requests: 1,
    input: 1000000,
    output: 1000000,
    cache_read: 500000,
    peak_units: 902000000,
    off_peak_units: 0,
    estimated: 0,
    unpriced: 0,
  }
  return {
    model: 'deepseek-v4.1-flash',
    snapshot: '2026-11-09',
    totals,
    users: [{ user_id: 1, username: 'alice', ...totals }],
    details: [
      {
        id: 1,
        request_id: 'example',
        user_id: 1,
        username: 'alice',
        time: 1794186000,
        time_source: 'request_id',
        peak: true,
        input: 1000000,
        output: 1000000,
        cache_read: 500000,
        cost_units: 902000000,
        estimated: false,
        unpriced: false,
      },
    ],
    detail_total: 1,
    page: 1,
  }
}
function renderPage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  })
  return render(
    <QueryClientProvider client={client}>
      <DeepSeekBillDemo />
    </QueryClientProvider>
  )
}

describe('DeepSeek simulated bill demo', () => {
  beforeEach(() => {
    useAuthStore.getState().auth.setUser({ id: 1, username: 'alice', role: 10 })
    vi.mocked(api.get).mockReset()
    vi.mocked(api.get).mockImplementation(async (url) => ({
      data: {
        success: true,
        data:
          url === '/api/user/deepseek-billing/rules'
            ? {
                store: { revision: 0, rules: [] },
                active: null,
                model: 'deepseek-v4.1-flash',
                defaults: {
                  enabled: false,
                  fx: 7.3,
                  peak: { input: 2, cache: 0.04, output: 8 },
                  off_peak: { input: 1, cache: 0.02, output: 4 },
                  windows: [{ start: 540, end: 720 }],
                  holidays: [],
                },
              }
            : report(),
      },
    }))
  })

  test('ordinary users see their own summary and details without user selectors', async () => {
    useAuthStore.getState().auth.setUser({ id: 1, username: 'alice', role: 1 })
    renderPage()
    expect(
      await screen.findByRole('heading', { name: /alice/ })
    ).toBeInTheDocument()
    expect(screen.queryByLabelText('Detail user')).not.toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'alice · #1' })
    ).not.toBeInTheDocument()
    expect(screen.getAllByText('¥9.020000').length).toBeGreaterThan(0)
    expect(api.get).toHaveBeenLastCalledWith('/api/user/deepseek-bill-demo', {
      params: expect.objectContaining({ user_id: '1' }),
    })
  })

  test('shows CNY and M units, selects a used user and applies a date query', async () => {
    const user = userEvent.setup()
    renderPage()
    const account = await screen.findByRole('button', { name: 'alice · #1' })
    expect(screen.getAllByText('¥9.020000').length).toBeGreaterThan(0)
    expect(screen.getAllByText('0.500 M').length).toBeGreaterThan(0)
    await user.click(account)
    await waitFor(() =>
      expect(api.get).toHaveBeenLastCalledWith('/api/user/deepseek-bill-demo', {
        params: expect.objectContaining({ user_id: '1', page: 1 }),
      })
    )
    fireEvent.change(screen.getByLabelText(/Start date|开始日期/), {
      target: { value: '2026-11-01' },
    })
    fireEvent.change(screen.getByLabelText(/End date|结束日期/), {
      target: { value: '2026-11-08' },
    })
    await user.click(screen.getByRole('button', { name: /Query|查询/ }))
    await waitFor(() =>
      expect(api.get).toHaveBeenLastCalledWith('/api/user/deepseek-bill-demo', {
        params: {
          start: '2026-11-01',
          end: '2026-11-08',
          user_id: '0',
          page: 1,
        },
      })
    )
  })

  test('reports API failure instead of presenting a zero-cost bill', async () => {
    useAuthStore.getState().auth.setUser({ id: 1, username: 'alice', role: 1 })
    vi.mocked(api.get).mockResolvedValue({
      data: { success: false, message: 'select a shorter date range' },
    })
    renderPage()
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'select a shorter date range'
    )
    expect(
      screen.queryByRole('button', { name: 'alice · #1' })
    ).not.toBeInTheDocument()
  })
})
