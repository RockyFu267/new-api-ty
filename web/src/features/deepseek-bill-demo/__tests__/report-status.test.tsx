import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { beforeEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { DeepSeekBillDemo } from '..'

vi.mock('@/lib/api', () => ({
  api: { get: vi.fn() },
  getStatus: vi.fn(async () => ({ usd_exchange_rate: 7.3 })),
}))

beforeEach(() => {
  useAuthStore.getState().auth.setUser({ id: 1, username: 'alice', role: 1 })
  vi.mocked(api.get).mockReset()
})

function renderStatus(historyComplete: boolean, workerRunning: boolean) {
  vi.mocked(api.get).mockResolvedValue({
    data: {
      success: true,
      data: {
        model: 'deepseek-v4.1-flash',
        snapshot: '2026-11-09',
        totals: {
          requests: 0,
          input: 0,
          output: 0,
          cache_read: 0,
          peak_units: 0,
          off_peak_units: 0,
          estimated: 0,
          unpriced: 0,
        },
        users: [],
        details: [],
        detail_total: 0,
        page: 1,
        status: {
          configured: true,
          history_complete: historyComplete,
          worker_running: workerRunning,
          last_sync: 1794457000,
          imported_through: 100,
          source_through: 1000,
        },
      },
    },
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  })
  render(
    <QueryClientProvider client={client}>
      <DeepSeekBillDemo />
    </QueryClientProvider>
  )
}

test('unfinished background import is explained instead of showing a zero bill', async () => {
  renderStatus(false, true)
  expect(
    await screen.findByText(
      'Billing history is being prepared in the background. AI requests do not wait for it.'
    )
  ).toBeInTheDocument()
  expect(screen.queryByText('¥0.000000')).not.toBeInTheDocument()
  expect(
    screen.queryByRole('heading', { name: /Billing summary/ })
  ).not.toBeInTheDocument()
})

test('paused worker leaves the generated bill visible with a stale-data notice', async () => {
  renderStatus(true, false)
  expect(
    await screen.findByText(
      'Background reporting is paused. Showing the last generated bill.'
    )
  ).toBeInTheDocument()
  expect(screen.getByText('Report updated at')).toBeInTheDocument()
  expect(
    screen.getByRole('heading', { name: /Billing summary/ })
  ).toBeInTheDocument()
})
