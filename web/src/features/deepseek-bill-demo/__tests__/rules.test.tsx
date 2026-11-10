import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { DeepSeekRules } from '../rules'

vi.mock('@/lib/api', () => ({ api: { get: vi.fn(), put: vi.fn() } }))

beforeEach(() => {
  vi.mocked(api.get).mockReset()
  vi.mocked(api.put).mockReset()
  vi.mocked(api.get).mockResolvedValue({
    data: {
      success: true,
      data: {
        store: { revision: 0, rules: [] },
        active: null,
        model: 'deepseek-v4.1-flash',
        defaults: {
          enabled: false,
          fx: 7.3,
          peak: { input: 2, cache: 0.04, output: 8 },
          off_peak: { input: 1, cache: 0.02, output: 4 },
          windows: [
            { start: 540, end: 720 },
            { start: 840, end: 1080 },
          ],
          holidays: ['2026-10-01'],
        },
      },
    },
  })
})

function show() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  })
  return render(
    <QueryClientProvider client={client}>
      <DeepSeekRules />
    </QueryClientProvider>
  )
}

test('administrator publishes a new version with explicit FX and default original-provider prices', async () => {
  vi.mocked(api.put).mockResolvedValue({
    data: { success: true, data: { revision: 1, rules: [] } },
  })
  const user = userEvent.setup()
  show()
  await user.click(await screen.findByRole('checkbox'))
  fireEvent.change(screen.getByLabelText(/CNY per USD|配额换算汇率/), {
    target: { value: '7' },
  })
  await user.click(
    screen.getByRole('button', {
      name: /Publish new pricing version|发布新计价版本/,
    })
  )
  await waitFor(() =>
    expect(api.put).toHaveBeenCalledWith('/api/user/deepseek-billing/rules', {
      revision: 0,
      rule: {
        enabled: true,
        fx: 7,
        effective_at: 0,
        peak: { input: 2, cache: 0.04, output: 8 },
        off_peak: { input: 1, cache: 0.02, output: 4 },
        windows: [
          { start: 540, end: 720 },
          { start: 840, end: 1080 },
        ],
        holidays: ['2026-10-01'],
      },
    })
  )
})

test('rejected pricing version stays visible with the server error', async () => {
  vi.mocked(api.put).mockResolvedValue({
    data: { success: false, message: 'rules changed; refresh before saving' },
  })
  const user = userEvent.setup()
  show()
  await user.click(
    await screen.findByRole('button', {
      name: /Publish new pricing version|发布新计价版本/,
    })
  )
  expect(await screen.findByRole('alert')).toHaveTextContent(
    'rules changed; refresh before saving'
  )
})
