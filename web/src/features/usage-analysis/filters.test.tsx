import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { beijingDay, type Filters } from './api'
import { UsageFilters } from './filters'

vi.mock('@/lib/api', () => ({ api: { get: vi.fn() } }))
afterEach(() => {
  cleanup()
  vi.resetAllMocks()
})

const initial: Filters = {
  start: `${beijingDay().slice(0, 7)}-01`,
  end: beijingDay(),
  department_id: '',
  department_basis: 'current',
  user_id: '',
  model: '',
  local: '',
  dimension: 'department',
  page: 1,
}

test('department, named user and model choices are submitted together only when Search is clicked', async () => {
  vi.mocked(api.get).mockResolvedValue({
    data: {
      success: true,
      data: {
        departments: [{ value: '7', label: 'Engineering' }],
        users: [{ value: '12', label: 'Alice (alice) · #12' }],
        models: [{ value: 'qwen3.8-max', label: 'qwen3.8-max' }],
      },
    },
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const onApply = vi.fn()
  const user = userEvent.setup()
  render(
    <QueryClientProvider client={client}>
      <UsageFilters filters={initial} onApply={onApply} onReset={vi.fn()} />
    </QueryClientProvider>
  )
  await screen.findByRole('option', { name: 'Engineering' })
  await user.selectOptions(
    screen.getByRole('combobox', { name: 'Department' }),
    '7'
  )
  const person = screen.getByRole('combobox', { name: 'User' })
  await user.click(person)
  await user.type(person, 'alice')
  await user.click(
    await screen.findByRole('option', { name: 'Alice (alice) · #12' })
  )
  const model = screen.getByRole('combobox', { name: 'Model filter' })
  await user.click(model)
  await user.type(model, '3.8')
  await user.click(await screen.findByRole('option', { name: 'qwen3.8-max' }))
  expect(onApply).not.toHaveBeenCalled()
  await user.click(screen.getByRole('button', { name: 'Search usage' }))
  expect(onApply).toHaveBeenCalledWith(
    expect.objectContaining({
      department_id: '7',
      department_basis: 'current',
      user_id: '12',
      model: 'qwen3.8-max',
      dimension: 'user',
      page: 1,
    })
  )
  await waitFor(() =>
    expect(api.get).toHaveBeenCalledWith(
      '/api/user/usage-analysis/options',
      expect.objectContaining({
        params: expect.objectContaining({ department_id: '7', user_id: '12' }),
      })
    )
  )
  client.clear()
})

test('changing departments clears a previous person and model instead of querying an impossible combination', async () => {
  vi.mocked(api.get).mockResolvedValue({
    data: {
      success: true,
      data: {
        departments: [{ value: '8', label: 'Research' }],
        users: [],
        models: [],
      },
    },
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const onApply = vi.fn()
  const user = userEvent.setup()
  render(
    <QueryClientProvider client={client}>
      <UsageFilters
        filters={{ ...initial, user_id: '12', model: 'old-model' }}
        onApply={onApply}
        onReset={vi.fn()}
      />
    </QueryClientProvider>
  )
  await screen.findByRole('option', { name: 'Research' })
  await user.selectOptions(
    screen.getByRole('combobox', { name: 'Department' }),
    '8'
  )
  await user.click(screen.getByRole('button', { name: 'Search usage' }))
  expect(onApply).toHaveBeenCalledWith(
    expect.objectContaining({ department_id: '8', user_id: '', model: '' })
  )
  client.clear()
})
