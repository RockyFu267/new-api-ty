import { useEffect, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ExternalLink, ShieldCheck, Sparkles } from 'lucide-react'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { api } from '@/lib/api'

import { SettingsSection } from '../components/settings-section'

type SkillDefinition = {
  id: string
  name: string
  description: string
  system_prompt: string
  enabled: boolean
}

async function loadSkill(): Promise<SkillDefinition> {
  const response = await api.get('/api/skills/showcase/')
  return response.data.data
}

export function SkillManagementSection() {
  const queryClient = useQueryClient()
  const query = useQuery({ queryKey: ['showcase-skill-admin'], queryFn: loadSkill })
  const [draft, setDraft] = useState<SkillDefinition | null>(null)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (query.data) setDraft(query.data)
  }, [query.data])

  const update = <K extends keyof SkillDefinition>(key: K, value: SkillDefinition[K]) => {
    setDraft((current) => current ? { ...current, [key]: value } : current)
  }

  const save = async () => {
    if (!draft?.name.trim() || !draft.system_prompt.trim()) {
      toast.error('Skill 名称和系统提示词不能为空')
      return
    }
    setSaving(true)
    try {
      const response = await api.put('/api/skills/showcase/', draft)
      if (response.data?.success) {
        toast.success('Skill 配置已保存，后续新对话立即生效')
        await queryClient.invalidateQueries({ queryKey: ['showcase-skill-admin'] })
        await queryClient.invalidateQueries({ queryKey: ['showcase-skill-user'] })
      }
    } finally {
      setSaving(false)
    }
  }

  return (
    <SettingsSection title='Skill 管理'>
      <div className='space-y-6'>
        <div className='flex flex-wrap items-center justify-between gap-4 rounded-xl border bg-violet-500/5 p-4'>
          <div className='flex items-center gap-3'><div className='rounded-lg bg-violet-500/10 p-2 text-violet-500'><Sparkles className='size-5' /></div><div><p className='font-medium'>{draft?.name ?? '正在加载…'}</p><p className='text-muted-foreground text-xs'>受控动作：更新固定展示配置 · Skill ID：showcase-designer</p></div></div>
          <div className='flex items-center gap-4'><label className='flex items-center gap-2 text-sm'>用户可用<Switch checked={draft?.enabled ?? false} disabled={!draft} onCheckedChange={(checked) => update('enabled', checked)} /></label><Button type='button' variant='outline' size='sm' onClick={() => window.open('/skill-showcase', '_blank')}><ExternalLink className='mr-1 size-4' />预览展示页</Button></div>
        </div>

        <div className='grid gap-5 rounded-xl border p-5'>
          <div className='space-y-2'><Label>Skill 名称</Label><Input value={draft?.name ?? ''} maxLength={60} onChange={(event) => update('name', event.target.value)} /></div>
          <div className='space-y-2'><Label>用户说明</Label><Textarea value={draft?.description ?? ''} maxLength={300} rows={3} onChange={(event) => update('description', event.target.value)} /><p className='text-muted-foreground text-xs'>这段说明会展示在用户 Chat 页面中。</p></div>
          <div className='space-y-2'><Label>系统提示词</Label><Textarea className='min-h-48 font-mono text-sm' value={draft?.system_prompt ?? ''} maxLength={4000} onChange={(event) => update('system_prompt', event.target.value)} /><p className='text-muted-foreground text-xs'>用于约束模型的角色、内容风格和处理方式；平台安全执行协议由后端固定追加，不能在此绕过。</p></div>
          <div className='flex items-center justify-between gap-4'><div className='text-muted-foreground flex items-center gap-2 text-xs'><ShieldCheck className='size-4 text-emerald-500' />只允许更新 demo/skill-showcase.json，不执行任意命令或路径。</div><Button type='button' disabled={!draft || saving} onClick={() => void save()}>保存 Skill</Button></div>
        </div>

        <div className='rounded-xl border p-5'><h3 className='font-medium'>可用预设模板</h3><p className='text-muted-foreground mt-1 text-sm'>科技蓝（cyber-blue）、管理驾驶舱（executive-dark）、绿色运维（operations-green）、AI 创新（aurora-purple）。模板与执行协议属于当前 Demo 的受控能力。</p></div>
      </div>
    </SettingsSection>
  )
}
