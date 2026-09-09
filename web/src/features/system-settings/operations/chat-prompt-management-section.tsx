import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { api } from '@/lib/api'

import { SettingsSection } from '../components/settings-section'

type ChatPromptItem = {
  channel_id: number
  channel_name: string
  channel_type: number
  model: string
  title: string
  description: string
  configured: boolean
}

type Draft = { title: string; description: string }

const DEFAULT_TITLE = '开始与该模型对话'
const DEFAULT_DESCRIPTION =
  '请在发送消息前确认当前渠道、模型及相关使用说明，并根据实际需要合理使用。'

async function loadChatPrompts(): Promise<ChatPromptItem[]> {
  const response = await api.get('/api/chat-prompts/')
  return response.data?.success && Array.isArray(response.data.data)
    ? response.data.data
    : []
}

export function ChatPromptManagementSection() {
  const queryClient = useQueryClient()
  const query = useQuery({ queryKey: ['chat-prompts-admin'], queryFn: loadChatPrompts })
  const [drafts, setDrafts] = useState<Record<string, Draft>>({})
  const [savingKey, setSavingKey] = useState<string | null>(null)

  useEffect(() => {
    if (!query.data) return
    const next: Record<string, Draft> = {}
    for (const item of query.data) {
      next[`${item.channel_id}:${item.model}`] = {
        title: item.title || DEFAULT_TITLE,
        description: item.description || DEFAULT_DESCRIPTION,
      }
    }
    setDrafts(next)
  }, [query.data])

  const updateDraft = (key: string, field: keyof Draft, value: string) => {
    setDrafts((current) => ({
      ...current,
      [key]: { ...current[key], [field]: value },
    }))
  }

  const save = async (item: ChatPromptItem) => {
    const key = `${item.channel_id}:${item.model}`
    const draft = drafts[key]
    if (!draft?.title.trim()) { toast.error('提示标题不能为空'); return }
    setSavingKey(key)
    try {
      const response = await api.put('/api/chat-prompts/', {
        channel_id: item.channel_id,
        model: item.model,
        title: draft.title,
        description: draft.description,
      })
      if (response.data?.success) {
        toast.success('聊天提示已保存')
        await queryClient.invalidateQueries({ queryKey: ['chat-prompts-admin'] })
        await queryClient.invalidateQueries({ queryKey: ['native-chat-channel-models'] })
      }
    } finally { setSavingKey(null) }
  }

  const reset = async (item: ChatPromptItem) => {
    const key = `${item.channel_id}:${item.model}`
    setSavingKey(key)
    try {
      const response = await api.delete('/api/chat-prompts/', {
        params: { channel_id: item.channel_id, model: item.model },
      })
      if (response.data?.success) {
        toast.success('已恢复默认提示')
        await queryClient.invalidateQueries({ queryKey: ['chat-prompts-admin'] })
        await queryClient.invalidateQueries({ queryKey: ['native-chat-channel-models'] })
      }
    } finally { setSavingKey(null) }
  }

  return (
    <SettingsSection title='聊天提示管理'>
      <p className='text-muted-foreground text-sm'>按渠道和模型配置用户进入 Chat 后首先看到的费用、用途与风险提示。</p>
      {query.isPending ? <p className='text-muted-foreground text-sm'>正在加载渠道模型…</p> : null}
      {query.data?.length === 0 ? <p className='text-muted-foreground text-sm'>暂无可配置的渠道模型。</p> : null}
      <div className='grid gap-4'>
        {query.data?.map((item) => {
          const key = `${item.channel_id}:${item.model}`
          const draft = drafts[key] || { title: DEFAULT_TITLE, description: DEFAULT_DESCRIPTION }
          return (
            <div key={key} className='space-y-4 rounded-xl border p-4'>
              <div>
                <h3 className='font-medium'>{item.model}</h3>
                <p className='text-muted-foreground text-xs'>{item.channel_name} · #{item.channel_id} · 渠道类型 {item.channel_type}</p>
              </div>
              <div className='space-y-2'>
                <Label>提示标题</Label>
                <Input value={draft.title} maxLength={200} onChange={(event) => updateDraft(key, 'title', event.target.value)} />
              </div>
              <div className='space-y-2'>
                <Label>提示说明</Label>
                <Textarea value={draft.description} maxLength={4000} rows={4} onChange={(event) => updateDraft(key, 'description', event.target.value)} />
              </div>
              <div className='flex gap-2'>
                <Button type='button' disabled={savingKey === key} onClick={() => save(item)}>保存提示</Button>
                {item.configured ? <Button type='button' variant='outline' disabled={savingKey === key} onClick={() => reset(item)}>恢复默认</Button> : null}
              </div>
            </div>
          )
        })}
      </div>
    </SettingsSection>
  )
}
