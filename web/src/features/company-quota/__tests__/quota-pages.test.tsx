import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { buildApiParams } from '@/features/usage-logs/lib/utils'
import { api } from '@/lib/api'

import { CompanyQuota } from '../index'
import { CompanyWallet } from '../wallet'
import { WalletAdjustments } from '../wallet-adjustments'

vi.mock('@/lib/api', () => ({ api: { get: vi.fn(), put: vi.fn() } }))

const clients: QueryClient[] = []
const period = {
  start: 1,
  reset_at: 1790000000,
  limit: 5000000,
  used: 1000000,
  reserved: 500000,
}
const usage = {
  user_id: 1,
  username: 'alice',
  department_id: 1,
  department_name: 'Engineering',
  configured: true,
  wallet: 5000000,
  wallet_monthly_used: 500000,
  daily: period,
  weekly: period,
  monthly: period,
}

function renderPage(component: React.ReactNode) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  clients.push(client)
  return render(
    <QueryClientProvider client={client}>{component}</QueryClientProvider>
  )
}

beforeEach(() => {
  vi.mocked(api.get).mockImplementation(async (path) => {
    const url = String(path)
    let data: unknown = { items: [], total: 0 }
    if (url.endsWith('/company-quota/config')) data = { enabled: false }
    if (url.endsWith('/departments')) {
      data = [{ id: 1, name: 'Engineering', user_count: 2 }]
    }
    if (url.endsWith('/company-quota/rules')) data = []
    if (url.endsWith('/company-quota')) data = { enabled: true, usage }
    return { data: { success: true, data } }
  })
  vi.mocked(api.put).mockResolvedValue({ data: { success: true } })
})

afterEach(() => {
  cleanup()
  clients.splice(0).forEach((client) => client.clear())
  vi.resetAllMocks()
})

test('admin configures per-person monthly quota and leaves other periods unset', async () => {
  const actor = userEvent.setup()
  renderPage(<CompanyQuota />)
  const edit = await screen.findByRole('button', { name: 'Edit' })
  await waitFor(() => expect(edit).toBeEnabled())
  await actor.click(edit)
  const save = screen.getByRole('button', { name: 'Save' })
  await actor.click(
    screen.getByRole('switch', { name: 'Enable department quota' })
  )
  expect(save).toBeDisabled()
  await actor.type(screen.getByRole('spinbutton', { name: /^monthly/ }), '10')
  await actor.click(save)
  await waitFor(() =>
    expect(api.put).toHaveBeenCalledWith(
      '/api/user/company-quota/rules',
      expect.objectContaining({
        department_id: 1,
        enabled: true,
        daily: 0,
        weekly: 0,
        monthly: 5000000,
      })
    )
  )
  await waitFor(() =>
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  )
})

test('quota and wallet shows separate bills and hides self-funding controls', async () => {
  const actor = userEvent.setup()
  renderPage(<CompanyWallet />)
  expect(await screen.findByText(/Department: Engineering/)).toBeInTheDocument()
  expect(
    screen.getByRole('heading', { name: 'Quota and wallet' })
  ).toBeInTheDocument()
  expect(screen.queryByText('Redemption Code')).not.toBeInTheDocument()
  expect(screen.queryByText('Affiliate Rewards')).not.toBeInTheDocument()
  expect(screen.queryByText('Add Funds')).not.toBeInTheDocument()
  await actor.click(screen.getByRole('button', { name: 'Wallet bills' }))
  await waitFor(() =>
    expect(api.get).toHaveBeenCalledWith(
      '/api/user/company-bills',
      expect.objectContaining({
        params: expect.objectContaining({ source: 'wallet', p: 1 }),
      })
    )
  )
})

test('failed rule update preserves configuration input for retry', async () => {
  vi.mocked(api.put).mockResolvedValue({
    data: { success: false, message: 'Save failed' },
  })
  const actor = userEvent.setup()
  renderPage(<CompanyQuota />)
  const edit = await screen.findByRole('button', { name: 'Edit' })
  await waitFor(() => expect(edit).toBeEnabled())
  await actor.click(edit)
  await actor.click(
    screen.getByRole('switch', { name: 'Enable department quota' })
  )
  const input = screen.getByRole('spinbutton', { name: /^daily/ })
  await actor.type(input, '5')
  await actor.click(screen.getByRole('button', { name: 'Save' }))
  await waitFor(() => expect(api.put).toHaveBeenCalled())
  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'Save' })).toBeEnabled()
  )
  expect(input).toHaveValue(5)
  expect(screen.getByRole('dialog')).toBeInTheDocument()
})

test('wallet adjustment filters are sent to the audit endpoint', async () => {
  const actor = userEvent.setup()
  renderPage(<WalletAdjustments />)
  await actor.type(
    screen.getByRole('textbox', { name: 'User department or note' }),
    'alice'
  )
  await actor.selectOptions(
    screen.getByRole('combobox', { name: 'Operation filter' }),
    'add'
  )
  await waitFor(() =>
    expect(api.get).toHaveBeenCalledWith(
      '/api/user/wallet-adjustments',
      expect.objectContaining({
        params: expect.objectContaining({ keyword: 'alice', action: 'add' }),
      })
    )
  )
})

test('usage logs and statistics use the same billing-source filter', () => {
  expect(
    buildApiParams({
      page: 1,
      pageSize: 20,
      searchParams: { billingSource: 'department_quota' },
      isAdmin: false,
    })
  ).toMatchObject({ billing_source: 'department_quota' })
})
