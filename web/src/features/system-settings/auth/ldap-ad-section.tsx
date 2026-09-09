import { useState } from 'react'
import { toast } from 'sonner'

import { PasswordInput } from '@/components/password-input'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { api } from '@/lib/api'

import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

export type LDAPADValues = {
  'ldap_ad.enabled': boolean
  'ldap_ad.url': string
  'ldap_ad.base_dn': string
  'ldap_ad.bind_dn': string
  'ldap_ad.bind_secret': string
  'ldap_ad.login_attr': string
  'ldap_ad.user_filter': string
  'ldap_ad.allowed_user_base_dn': string
}

type PreviewUser = {
  object_guid: string
  username: string
  display_name: string
  email: string
  disabled: boolean
  sync_status: 'new' | 'synced' | 'conflict'
}

export function LDAPADSection({ defaultValues }: { defaultValues: LDAPADValues }) {
  const updateOption = useUpdateOption()
  const [values, setValues] = useState(defaultValues)
  const [busy, setBusy] = useState(false)
  const [users, setUsers] = useState<PreviewUser[]>([])

  const set = <K extends keyof LDAPADValues>(key: K, value: LDAPADValues[K]) =>
    setValues((current) => ({ ...current, [key]: value }))

  const save = async () => {
    for (const [key, value] of Object.entries(values)) {
      if (key === 'ldap_ad.bind_secret' && !value) continue
      if (value !== defaultValues[key as keyof LDAPADValues]) {
        await updateOption.mutateAsync({ key, value })
      }
    }
  }

  const testConnection = async () => {
    setBusy(true)
    try {
      const response = await api.post('/api/ldap-ad/test')
      if (response.data?.success) toast.success('域控连接成功')
    } finally { setBusy(false) }
  }

  const preview = async () => {
    setBusy(true)
    try {
      const response = await api.get('/api/ldap-ad/users')
      if (response.data?.success) setUsers(response.data.data || [])
    } finally { setBusy(false) }
  }

  const sync = async () => {
    setBusy(true)
    try {
      const response = await api.post('/api/ldap-ad/sync')
      if (response.data?.success) {
        const result = response.data.data
        toast.success(`同步完成：新增 ${result.created}，跳过 ${result.skipped}`)
        await preview()
      }
    } finally { setBusy(false) }
  }

  const fields: Array<[keyof LDAPADValues, string, string]> = [
    ['ldap_ad.url', 'LDAP 地址', 'ldap://192.168.40.57:389'],
    ['ldap_ad.base_dn', 'Base DN', 'dc=example,dc=com'],
    ['ldap_ad.bind_dn', 'Bind DN', 'CN=service,OU=users,DC=example,DC=com'],
    ['ldap_ad.login_attr', '登录属性', 'sAMAccountName'],
    ['ldap_ad.user_filter', '用户过滤器', '(&(objectClass=user)(sAMAccountName={username}))'],
    ['ldap_ad.allowed_user_base_dn', '允许的用户 Base DN', 'dc=example,dc=com'],
  ]

  return (
    <SettingsSection title='域控接入'>
      <SettingsPageFormActions onSave={save} isSaving={updateOption.isPending} />
      <div className='flex items-center justify-between rounded-lg border p-4'>
        <div><Label>启用域账号登录</Label><p className='text-muted-foreground text-xs'>保留现有本地登录，AD 新用户默认创建为普通用户。</p></div>
        <Switch checked={values['ldap_ad.enabled']} onCheckedChange={(value) => set('ldap_ad.enabled', value)} />
      </div>
      <div className='grid gap-4 lg:grid-cols-2'>
        {fields.map(([key, label, placeholder]) => (
          <div className='space-y-2' key={key}>
            <Label>{label}</Label>
            <Input value={String(values[key])} placeholder={placeholder} onChange={(event) => set(key, event.target.value)} />
          </div>
        ))}
        <div className='space-y-2'>
          <Label>Bind Password</Label>
          <PasswordInput value={values['ldap_ad.bind_secret']} placeholder='留空表示不修改已保存密码' onChange={(event) => set('ldap_ad.bind_secret', event.target.value)} />
        </div>
      </div>
      <div className='flex flex-wrap gap-2'>
        <Button type='button' variant='outline' disabled={busy} onClick={testConnection}>测试连接</Button>
        <Button type='button' variant='outline' disabled={busy} onClick={preview}>预览用户</Button>
        <Button type='button' disabled={busy} onClick={sync}>同步用户</Button>
      </div>
      {users.length > 0 && (
        <div className='max-h-80 overflow-auto rounded-lg border'>
          {users.map((user) => (
            <div key={user.object_guid} className='flex items-center justify-between border-b px-4 py-2 text-sm last:border-0'>
              <span>{user.username}{user.display_name ? ` · ${user.display_name}` : ''}</span>
              <span className='text-muted-foreground'>{user.disabled ? 'AD 已禁用' : user.sync_status}</span>
            </div>
          ))}
        </div>
      )}
    </SettingsSection>
  )
}
