import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import { groupTrend, type Bucket } from './api'
import { ComparisonPrices } from './prices'
import { UsageTable } from './usage-table'

const session = vi.hoisted(() => ({ role: 10 }))
vi.mock('@/stores/auth-store', () => ({
  useAuthStore: (
    selector: (s: { auth: { user: { role: number } } }) => unknown
  ) => selector({ auth: { user: { role: session.role } } }),
}))
vi.mock('@/lib/api', () => ({ api: { get: vi.fn(), post: vi.fn() } }))
afterEach(cleanup)

function bucket(name: string, input = 100): Bucket {
  return {
    name,
    requests: 1,
    input,
    output: 20,
    cache_read: 0,
    cache_write: 0,
    cache_reported: 0,
    measured: 1,
    estimated: 0,
    unknown: 0,
    avoided_usd: 2,
    priced_requests: 1,
  }
}

test('weekly trends use Sunday boundaries and preserve cross-month weeks', () => {
  const rows = [
    bucket('2026-10-31'),
    bucket('2026-11-01'),
    bucket('2026-11-02'),
  ]
  const result = groupTrend(rows, 'week')
  expect(result.map((r) => r.name)).toEqual(['2026-10-25', '2026-11-01'])
  expect(result[1]).toMatchObject({
    input: 200,
    output: 40,
    requests: 2,
    avoided_usd: 4,
  })
  expect(groupTrend(rows, 'month').map((r) => r.input)).toEqual([100, 200])
  expect(rows[1].input).toBe(100)
})

test('missing cache data is displayed as not provided rather than a measured zero', () => {
  render(<UsageTable rows={[bucket('local-model')]} />)
  expect(screen.getAllByText('Not provided')).toHaveLength(2)
  expect(screen.getByText('local-model')).toBeInTheDocument()
})

test('comparison prices are readable by admins but only root sees the editor', () => {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const prices = [
    {
      id: 1,
      local_model: 'local-model',
      reference_model: 'commercial-model',
      effective_day: '2026-10-19',
      input_price: 1,
      output_price: 2,
    },
  ]
  const view = render(
    <QueryClientProvider client={client}>
      <ComparisonPrices prices={prices} />
    </QueryClientProvider>
  )
  expect(screen.getByText('commercial-model')).toBeInTheDocument()
  expect(
    screen.queryByRole('button', { name: 'Add price version' })
  ).not.toBeInTheDocument()
  session.role = 100
  view.rerender(
    <QueryClientProvider client={client}>
      <ComparisonPrices prices={prices} />
    </QueryClientProvider>
  )
  expect(
    screen.getByRole('button', { name: 'Add price version' })
  ).toBeInTheDocument()
  session.role = 10
  client.clear()
})
