import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { searchUsers } from '../../api'
import type { User } from '../../types'
import { DepartmentCell, DepartmentsManager } from '../departments'

vi.mock('@/lib/api', () => ({
  api: { get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() },
}))

const departments = [
  { id: 1, name: 'Engineering', user_count: 2 },
  { id: 2, name: 'Research', user_count: 1 },
]
const user: User = {
  id: 5,
  username: 'alice',
  display_name: 'Alice',
  role: 1,
  status: 1,
  group: 'default',
  quota: 0,
  used_quota: 0,
  request_count: 0,
  department_id: 1,
  department_name: 'Engineering',
}
const clients: QueryClient[] = []

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
  vi.mocked(api.get).mockResolvedValue({
    data: { success: true, data: departments },
  })
  vi.mocked(api.post).mockResolvedValue({ data: { success: true } })
  vi.mocked(api.put).mockResolvedValue({ data: { success: true } })
  vi.mocked(api.delete).mockResolvedValue({ data: { success: true } })
  useAuthStore.setState((state) => ({
    auth: { ...state.auth, user: { id: 1, username: 'admin', role: 100 } },
  }))
})

afterEach(() => {
  cleanup()
  clients.splice(0).forEach((client) => client.clear())
  vi.resetAllMocks()
})

test('administrator changes a user department and can clear it', async () => {
  const actor = userEvent.setup()
  renderPage(<DepartmentCell user={user} />)
  const select = screen.getByRole('combobox', { name: 'Department for alice' })
  await waitFor(() => expect(select).toBeEnabled())
  expect(select).toHaveValue('1')
  await actor.selectOptions(select, '2')
  await waitFor(() =>
    expect(api.put).toHaveBeenCalledWith('/api/user/5/department', {
      department_id: 2,
    })
  )
  await waitFor(() => expect(select).toBeEnabled())
  await actor.selectOptions(select, '0')
  await waitFor(() =>
    expect(api.put).toHaveBeenCalledWith('/api/user/5/department', {
      department_id: 0,
    })
  )
})

test('department manager creates and renames departments and requires confirmation before deletion', async () => {
  const actor = userEvent.setup()
  renderPage(<DepartmentsManager />)
  await actor.click(screen.getByRole('button', { name: 'Manage departments' }))
  const input = screen.getByRole('textbox', { name: 'Department name' })
  await actor.type(input, 'Operations')
  await actor.click(screen.getByRole('button', { name: 'Create' }))
  await waitFor(() =>
    expect(api.post).toHaveBeenCalledWith('/api/user/departments', {
      id: 0,
      name: 'Operations',
    })
  )
  await actor.click(screen.getByRole('button', { name: 'Rename Engineering' }))
  await actor.clear(input)
  await actor.type(input, 'Development')
  await actor.click(screen.getByRole('button', { name: 'Save' }))
  await waitFor(() =>
    expect(api.post).toHaveBeenCalledWith('/api/user/departments', {
      id: 1,
      name: 'Development',
    })
  )
  await actor.click(screen.getByRole('button', { name: 'Delete Engineering' }))
  expect(screen.getByRole('alertdialog')).toHaveTextContent(
    'Its 2 users will become unassigned'
  )
  expect(api.delete).not.toHaveBeenCalled()
  await actor.click(screen.getByRole('button', { name: 'Cancel' }))
  expect(api.delete).not.toHaveBeenCalled()
  await actor.click(screen.getByRole('button', { name: 'Delete Engineering' }))
  await actor.click(screen.getByRole('button', { name: /^Delete$/ }))
  await waitFor(() =>
    expect(api.delete).toHaveBeenCalledWith('/api/user/departments/1')
  )
})

test('admin cannot edit departments of peers and sees the department name', () => {
  useAuthStore.setState((state) => ({
    auth: { ...state.auth, user: { id: 1, username: 'admin', role: 10 } },
  }))
  renderPage(<DepartmentCell user={{ ...user, role: 10 }} />)
  expect(screen.queryByRole('combobox')).not.toBeInTheDocument()
  expect(screen.getByText('Engineering')).toBeInTheDocument()
})

test('department search is optional and unassigned zero is sent as a real filter', async () => {
  await searchUsers({ department_id: '0', p: 2 })
  expect(api.get).toHaveBeenLastCalledWith(
    expect.stringContaining('department_id=0')
  )
  await searchUsers({})
  expect(api.get).toHaveBeenLastCalledWith(
    expect.not.stringContaining('department_id=')
  )
})

test('failed department save keeps the editor open and preserves its content', async () => {
  vi.mocked(api.post).mockResolvedValue({
    data: { success: false, message: 'Duplicate department' },
  })
  const actor = userEvent.setup()
  renderPage(<DepartmentsManager />)
  await actor.click(screen.getByRole('button', { name: 'Manage departments' }))
  const input = screen.getByRole('textbox', { name: 'Department name' })
  await actor.type(input, 'Research')
  await actor.click(screen.getByRole('button', { name: 'Create' }))
  await waitFor(() => expect(api.post).toHaveBeenCalled())
  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'Create' })).toBeEnabled()
  )
  expect(input).toHaveValue('Research')
  expect(screen.getByRole('dialog')).toBeInTheDocument()
})
