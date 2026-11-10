import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import { getStatus } from '@/lib/api'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { ExchangeRateNotice } from '../exchange-rate-notice'

vi.mock('@/lib/api', () => ({ getStatus: vi.fn() }))
const original = useSystemConfigStore.getState().config
const clients: QueryClient[] = []
afterEach(() => {
  cleanup()
  clients.splice(0).forEach((client) => client.clear())
  useSystemConfigStore.getState().setConfig(original)
  localStorage.clear()
  vi.resetAllMocks()
})

function show(deepseek = false) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  clients.push(client)
  return render(
    <QueryClientProvider client={client}>
      <ExchangeRateNotice deepseek={deepseek} />
    </QueryClientProvider>
  )
}

test('wallet explanation shows the current 7.3 exchange rate and USD accounting units', async () => {
  vi.mocked(getStatus).mockResolvedValue({ usd_exchange_rate: 7.3 })
  show()
  expect(
    await screen.findByText(/1 USD = 7.3 CNY|1 美元＝7.3 元人民币/)
  ).toBeInTheDocument()
})

test('explanation follows the configured rate instead of hardcoding 7.3', async () => {
  vi.mocked(getStatus).mockResolvedValue({ usd_exchange_rate: 7 })
  show()
  expect(
    await screen.findByText(/1 USD = 7 CNY|1 美元＝7 元人民币/)
  ).toBeInTheDocument()
})

test('DeepSeek explanation states that historical actual bills retain their recorded rate', async () => {
  vi.mocked(getStatus).mockResolvedValue({ usd_exchange_rate: 7.3 })
  show(true)
  expect(
    await screen.findByText(
      /Historical DeepSeek actual bills retain|DeepSeek 历史实际账单保留/
    )
  ).toBeInTheDocument()
})
