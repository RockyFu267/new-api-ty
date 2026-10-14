import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogCancel,
  AlertDialogAction,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { useAuthStore } from '@/stores/auth-store'

import {
  getDepartments,
  saveDepartment,
  deleteDepartment,
  assignUserDepartment,
} from '../api'
import type { Department, User } from '../types'

export function DepartmentCell(props: { user: User }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const role = useAuthStore((state) => state.auth.user?.role ?? 0)
  const query = useQuery({ queryKey: ['departments'], queryFn: getDepartments })
  const mutation = useMutation({
    mutationFn: (id: number) => assignUserDepartment(props.user.id, id),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['users'] }),
        queryClient.invalidateQueries({ queryKey: ['departments'] }),
      ])
      toast.success(t('Department updated'))
    },
  })
  if ((role !== 100 && role <= props.user.role) || props.user.DeletedAt) {
    return <span>{props.user.department_name || t('Unassigned')}</span>
  }
  return (
    <select
      className='bg-background w-40 rounded-md border px-2 py-1 text-sm'
      aria-label={t('Department for {{username}}', {
        username: props.user.username,
      })}
      value={props.user.department_id ?? 0}
      disabled={mutation.isPending || query.isPending || query.isError}
      onChange={(event) => mutation.mutate(Number(event.target.value))}
    >
      <option value='0'>{t('Unassigned')}</option>
      {(query.data ?? []).map((department) => (
        <option key={department.id} value={department.id}>
          {department.name}
        </option>
      ))}
    </select>
  )
}

export function DepartmentsManager() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [name, setName] = useState('')
  const [editingId, setEditingId] = useState(0)
  const [deleteTarget, setDeleteTarget] = useState<Department | null>(null)
  const query = useQuery({ queryKey: ['departments'], queryFn: getDepartments })
  const refresh = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['departments'] }),
      queryClient.invalidateQueries({ queryKey: ['users'] }),
    ])
  }
  const save = useMutation({
    mutationFn: () => saveDepartment(editingId, name.trim()),
    onSuccess: async () => {
      setName('')
      setEditingId(0)
      await refresh()
      toast.success(t('Department saved'))
    },
  })
  const remove = useMutation({
    mutationFn: (id: number) => deleteDepartment(id),
    onSuccess: async (_, id) => {
      setDeleteTarget(null)
      if (editingId === id) {
        setEditingId(0)
        setName('')
      }
      await refresh()
      toast.success(t('Department deleted'))
    },
  })
  const pending = save.isPending || remove.isPending
  return (
    <>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogTrigger render={<Button variant='outline' size='sm' />}>
          {t('Manage departments')}
        </DialogTrigger>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Manage departments')}</DialogTitle>
            <DialogDescription>
              {t(
                'Create flat departments and assign users from the users list.'
              )}
            </DialogDescription>
          </DialogHeader>
          <form
            className='flex gap-2'
            onSubmit={(event) => {
              event.preventDefault()
              save.mutate()
            }}
          >
            <Input
              aria-label={t('Department name')}
              placeholder={t('Department name')}
              maxLength={64}
              value={name}
              disabled={pending}
              onChange={(event) => setName(event.target.value)}
            />
            <Button type='submit' disabled={pending || !name.trim()}>
              {editingId ? t('Save') : t('Create')}
            </Button>
            {editingId > 0 && (
              <Button
                type='button'
                variant='outline'
                disabled={pending}
                onClick={() => {
                  setEditingId(0)
                  setName('')
                }}
              >
                {t('Cancel')}
              </Button>
            )}
          </form>
          <div className='max-h-80 space-y-2 overflow-auto'>
            {query.isPending && <p>{t('Loading...')}</p>}
            {query.isError && (
              <Button variant='outline' onClick={() => void query.refetch()}>
                {t('Retry')}
              </Button>
            )}
            {query.isSuccess && query.data.length === 0 && (
              <p className='text-muted-foreground text-sm'>
                {t('No departments yet')}
              </p>
            )}
            {(query.data ?? []).map((department) => (
              <div
                key={department.id}
                className='flex items-center gap-2 rounded-md border p-2'
              >
                <span className='min-w-0 flex-1 break-words'>
                  {department.name} ·{' '}
                  {t('{{count}} users', { count: department.user_count })}
                </span>
                <Button
                  variant='outline'
                  size='sm'
                  disabled={pending}
                  aria-label={t('Rename {{name}}', { name: department.name })}
                  onClick={() => {
                    setEditingId(department.id)
                    setName(department.name)
                  }}
                >
                  {t('Rename')}
                </Button>
                <Button
                  variant='destructive'
                  size='sm'
                  disabled={pending}
                  aria-label={t('Delete {{name}}', { name: department.name })}
                  onClick={() => setDeleteTarget(department)}
                >
                  {t('Delete')}
                </Button>
              </div>
            ))}
          </div>
        </DialogContent>
      </Dialog>
      <AlertDialog
        open={deleteTarget !== null}
        onOpenChange={(value) => {
          if (!value && !remove.isPending) setDeleteTarget(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Delete department')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'Delete {{name}}? Its {{count}} users will become unassigned. User accounts will be kept.',
                {
                  name: deleteTarget?.name,
                  count: deleteTarget?.user_count ?? 0,
                }
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={remove.isPending}>
              {t('Cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={remove.isPending}
              onClick={(event) => {
                event.preventDefault()
                if (deleteTarget) remove.mutate(deleteTarget.id)
              }}
            >
              {t('Delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
