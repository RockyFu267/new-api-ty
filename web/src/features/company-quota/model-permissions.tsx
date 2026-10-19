import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { getDepartments } from '@/features/users/api'

import { loadCompanyData } from './api'
import { ModelPermissionEditor } from './model-permission-editor'

type ModelPolicy = { department_id: number; models: string[] }
type PolicyData = { policies: ModelPolicy[]; catalog: string[] }

export function DepartmentModelPermissions() {
  const { t } = useTranslation()
  const [editing, setEditing] = useState<{ id: number; name: string } | null>(
    null
  )
  const departments = useQuery({
    queryKey: ['departments'],
    queryFn: getDepartments,
  })
  const policies = useQuery({
    queryKey: ['department-model-policies'],
    queryFn: () => loadCompanyData<PolicyData>('company-quota/models'),
  })
  return (
    <div className='space-y-3'>
      <p className='text-muted-foreground text-sm'>
        {t(
          'Users without a department, or departments without authorized models, cannot call models. Wallet funds, administrator roles and API keys do not bypass this policy. Quota billing switches do not disable model permissions.'
        )}
      </p>
      {(departments.isPending || policies.isPending) && (
        <p>{t('Loading...')}</p>
      )}
      {(departments.isError || policies.isError) && (
        <Button
          onClick={() => {
            void departments.refetch()
            void policies.refetch()
          }}
        >
          {t('Retry')}
        </Button>
      )}
      {(departments.data ?? []).map((department) => {
        const models =
          policies.data?.policies?.find(
            (policy) => policy.department_id === department.id
          )?.models ?? []
        return (
          <div key={department.id} className='space-y-2 rounded-lg border p-4'>
            <div className='flex flex-wrap items-center gap-3'>
              <span className='flex-1 font-medium'>
                {department.name} · {department.user_count}
              </span>
              <span>
                {models.length
                  ? `${t('Authorized models')}: ${models.length}`
                  : t('All models denied')}
              </span>
              <Button
                variant='outline'
                disabled={!policies.isSuccess}
                onClick={() => setEditing(department)}
              >
                {t('Configure model permissions')}
              </Button>
            </div>
            {models.length > 0 && (
              <p className='text-muted-foreground text-sm break-all'>
                {models.join(', ')}
              </p>
            )}
          </div>
        )
      })}
      {departments.isSuccess && departments.data.length === 0 && (
        <p>{t('Create departments in user management first.')}</p>
      )}
      {editing && policies.data && (
        <ModelPermissionEditor
          key={editing.id}
          department={editing}
          initial={
            policies.data.policies?.find(
              (policy) => policy.department_id === editing.id
            )?.models ?? []
          }
          catalog={policies.data.catalog ?? []}
          onClose={() => setEditing(null)}
        />
      )}
    </div>
  )
}
