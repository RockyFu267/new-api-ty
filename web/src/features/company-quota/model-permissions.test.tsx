import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { DepartmentModelPermissions } from './model-permissions'

vi.mock('@/lib/api', () => ({ api: { get: vi.fn(), put: vi.fn() } }))
const clients: QueryClient[] = []
beforeEach(() => {
  vi.mocked(api.get).mockImplementation(async (path) => ({
    data: {
      success: true,
      data: String(path).endsWith('/departments')
        ? [{ id: 1, name: 'Engineering', user_count: 2 }]
        : { policies: [], catalog: ['qwen', 'deepseek'] },
    },
  }))
  vi.mocked(api.put).mockResolvedValue({ data: { success: true } })
})
afterEach(() => {
  cleanup()
  clients.splice(0).forEach((client) => client.clear())
  vi.resetAllMocks()
})
function renderPermissions() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  clients.push(client)
  render(
    <QueryClientProvider client={client}>
      <DepartmentModelPermissions />
    </QueryClientProvider>
  )
}

test('unconfigured departments deny models and selecting a search result grants only that model', async () => {
  const user = userEvent.setup()
  renderPermissions()
  expect(await screen.findByText('All models denied')).toBeInTheDocument()
  await user.click(
    await screen.findByRole('button', { name: 'Configure model permissions' })
  )
  await user.type(
    screen.getByRole('textbox', { name: 'Search models' }),
    'qwen'
  )
  expect(
    screen.queryByRole('checkbox', { name: 'deepseek' })
  ).not.toBeInTheDocument()
  await user.click(
    screen.getByRole('button', { name: 'Select search results' })
  )
  expect(screen.getByRole('checkbox', { name: 'qwen' })).toBeChecked()
  await user.click(
    screen.getByRole('button', { name: 'Save model permissions' })
  )
  await waitFor(() =>
    expect(api.put).toHaveBeenCalledWith('/api/user/company-quota/models', {
      department_id: 1,
      models: ['qwen'],
    })
  )
  await waitFor(() =>
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  )
})

test('clearing all models saves an empty deny list and failed saves preserve the editor', async () => {
  const user = userEvent.setup()
  renderPermissions()
  await user.click(
    await screen.findByRole('button', { name: 'Configure model permissions' })
  )
  await user.click(screen.getByRole('checkbox', { name: 'qwen' }))
  vi.mocked(api.put).mockResolvedValueOnce({
    data: { success: false, message: 'Save failed' },
  })
  await user.click(
    screen.getByRole('button', { name: 'Save model permissions' })
  )
  expect(await screen.findByRole('alert')).toHaveTextContent('Save failed')
  expect(screen.getByRole('checkbox', { name: 'qwen' })).toBeChecked()
  await user.click(screen.getByRole('button', { name: 'Clear all models' }))
  expect(screen.getByRole('status')).toHaveTextContent(
    'This department will be unable to call any model.'
  )
  await user.click(
    screen.getByRole('button', { name: 'Save model permissions' })
  )
  await waitFor(() =>
    expect(api.put).toHaveBeenLastCalledWith('/api/user/company-quota/models', {
      department_id: 1,
      models: [],
    })
  )
})
