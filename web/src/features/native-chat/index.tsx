import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import { ArrowLeft, Bot, Download, ExternalLink, FileText, Folder, Loader2, Server, Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { PlaygroundChat } from '@/features/playground/components/chat/playground-chat'
import { PlaygroundInput } from '@/features/playground/components/input/playground-input'
import { api } from '@/lib/api'
import { Switch } from '@/components/ui/switch'
import { extractShowcaseDirective, inferShowcaseDirective } from '@/features/skill-showcase/directive'
import {
  buildKnowledgeTree,
  collectKnowledgeDocumentIds,
  type KnowledgeTreeNode,
} from '@/features/knowledge-base/tree'
import {
  useChatHandler,
  usePlaygroundConversation,
} from '@/features/playground/hooks'
import {
  applyMessageStateUpdate,
  type MessageStateUpdater,
} from '@/features/playground/lib'
import {
  DEFAULT_CONFIG,
  DEFAULT_PARAMETER_ENABLED,
} from '@/features/playground/constants'
import type {
  Message,
  ParameterEnabled,
  PlaygroundConfig,
} from '@/features/playground/types'

import { getChannelModels } from './api'
import type { ChannelModel, KnowledgeDocument } from './types'

async function getKnowledgeDocuments(): Promise<KnowledgeDocument[]> {
  const response = await api.get('/api/user/chat/knowledge-documents')
  return response.data?.data ?? []
}

function ChannelChatSession(props: {
  target: ChannelModel
  onBack: () => void
}) {
  const { t } = useTranslation()
  const [messages, setMessages] = useState<Message[]>([])
  const [config, setConfig] = useState<PlaygroundConfig>({
    ...DEFAULT_CONFIG,
    group: props.target.group,
    model: props.target.model,
  })
  const [parameterEnabled, setParameterEnabled] = useState<ParameterEnabled>(
    DEFAULT_PARAMETER_ENABLED
  )
  const [selectedKnowledgeIds, setSelectedKnowledgeIds] = useState<number[]>([])
  const [showcaseSkillEnabled, setShowcaseSkillEnabled] = useState(false)
  const appliedSkillMessages = useRef(new Set<string>())
  const knowledgeQuery = useQuery({
    queryKey: ['knowledge-documents-user'],
    queryFn: getKnowledgeDocuments,
  })

  const updateMessages = useCallback((updater: MessageStateUpdater) => {
    setMessages((previous) => applyMessageStateUpdate(previous, updater))
  }, [])

  const updateConfig = useCallback(
    <K extends keyof PlaygroundConfig>(key: K, value: PlaygroundConfig[K]) => {
      setConfig((previous) => ({ ...previous, [key]: value }))
    },
    []
  )

  const updateParameterEnabled = useCallback(
    (key: keyof ParameterEnabled, value: boolean) => {
      setParameterEnabled((previous) => ({ ...previous, [key]: value }))
    },
    []
  )

  const { sendChat, stopGeneration, isGenerating } = useChatHandler({
    channelId: props.target.channel_id,
    knowledgeDocumentIds: selectedKnowledgeIds,
    skillId: showcaseSkillEnabled ? 'showcase-designer' : undefined,
    config,
    parameterEnabled,
    onMessageUpdate: updateMessages,
  })

  useEffect(() => {
    if (!showcaseSkillEnabled) return
    const message = messages.at(-1)
    if (!message || message.from !== 'assistant' || (message.status !== 'complete' && !message.isContentComplete)) return
    const version = message.versions.at(-1)
    const executionKey = `${message.key}:${version?.id ?? ''}`
    if (!version || appliedSkillMessages.current.has(executionKey)) return
    const userMessage = [...messages].reverse().find((item) => item.from === 'user')
    const userContent = userMessage?.versions.at(-1)?.content ?? ''
    const directive = extractShowcaseDirective(version.content) ?? inferShowcaseDirective(userContent, version.content)
    if (!directive) {
      appliedSkillMessages.current.add(executionKey)
      toast.warning('模型没有返回可执行的 Skill 指令，请明确指定展示模板或修改内容')
      return
    }
    appliedSkillMessages.current.add(executionKey)
    void api.post('/api/skill-showcase/apply', directive).then(() => {
      toast.success('Skill 已执行，展示页已实时更新')
    }).catch(() => {
      appliedSkillMessages.current.delete(executionKey)
      toast.error('Skill 指令执行失败')
    })
  }, [messages, showcaseSkillEnabled])

  const conversation = usePlaygroundConversation({
    messages,
    updateMessages,
    sendChat,
  })

  const clearMessages = useCallback(() => {
    conversation.handleEditOpenChange(false)
    updateMessages([])
  }, [conversation, updateMessages])

  const models = useMemo(
    () => [{ label: props.target.model, value: props.target.model }],
    [props.target.model]
  )
  const groups = useMemo(
    () => [
      {
        label: props.target.group,
        value: props.target.group,
        ratio: 1,
      },
    ],
    [props.target.group]
  )

  const downloadKnowledge = async (knowledgeDocument: KnowledgeDocument) => {
    try {
      const response = await api.get(`/api/user/chat/knowledge-documents/${knowledgeDocument.id}/download`, {
        responseType: 'blob',
      })
      const url = URL.createObjectURL(response.data as Blob)
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = knowledgeDocument.name
      anchor.click()
      URL.revokeObjectURL(url)
    } catch {
      toast.error('知识库原文下载失败')
    }
  }

  const knowledgeTree = useMemo(
    () => buildKnowledgeTree(knowledgeQuery.data ?? []),
    [knowledgeQuery.data]
  )

  const toggleDocuments = (ids: number[], checked: boolean) => {
    setSelectedKnowledgeIds((current) => {
      const next = new Set(current)
      for (const id of ids) {
        if (checked) next.add(id)
        else next.delete(id)
      }
      return [...next]
    })
  }

  const renderKnowledgeNode = (
    node: KnowledgeTreeNode<KnowledgeDocument>,
    depth = 0
  ): ReactNode => {
    if (node.kind === 'directory') {
      const ids = collectKnowledgeDocumentIds(node)
      const selectedCount = ids.filter((id) =>
        selectedKnowledgeIds.includes(id)
      ).length
      const checked = selectedCount === ids.length && ids.length > 0
      const indeterminate = selectedCount > 0 && selectedCount < ids.length
      return (
        <div key={`directory:${node.path || 'root'}`}>
          <label
            className='bg-muted/30 flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 font-medium'
            style={{ marginLeft: `${depth * 20}px` }}
          >
            <Checkbox
              checked={checked}
              indeterminate={indeterminate}
              onCheckedChange={(value) => toggleDocuments(ids, value === true)}
            />
            <Folder className='text-primary size-4 shrink-0' />
            <span>{node.name}</span>
            <span className='text-muted-foreground text-xs font-normal'>
              {selectedCount}/{ids.length}
            </span>
          </label>
          <div className='mt-1 space-y-1'>
            {node.children.map((child) =>
              renderKnowledgeNode(child, depth + 1)
            )}
          </div>
        </div>
      )
    }

    const knowledgeDocument = node.document
    if (!knowledgeDocument) return null
    return (
      <div
        key={`file:${node.path}`}
        className='flex items-center gap-2 rounded-md px-2 py-1 text-sm'
        style={{ marginLeft: `${depth * 20}px` }}
      >
        <Checkbox
          checked={selectedKnowledgeIds.includes(knowledgeDocument.id)}
          onCheckedChange={(value) =>
            toggleDocuments([knowledgeDocument.id], value === true)
          }
        />
        <FileText className='text-muted-foreground size-3.5 shrink-0' />
        <span className='min-w-0 flex-1 truncate' title={node.path}>
          {node.name}
        </span>
        {knowledgeDocument.allow_download ? (
          <Button
            type='button'
            variant='ghost'
            size='icon'
            className='size-7'
            aria-label={`下载 ${knowledgeDocument.name}`}
            onClick={() => void downloadKnowledge(knowledgeDocument)}
          >
            <Download className='size-3.5' />
          </Button>
        ) : null}
      </div>
    )
  }

  return (
    <div className='relative flex size-full min-h-0 flex-col overflow-hidden'>
      <div className='border-border flex items-center gap-3 border-b px-4 py-3'>
        <Button
          type='button'
          variant='ghost'
          size='icon'
          aria-label={t('Back to channel models')}
          onClick={props.onBack}
        >
          <ArrowLeft className='size-4' />
        </Button>
        <div className='min-w-0'>
          <p className='truncate font-medium'>{props.target.model}</p>
          <p className='text-muted-foreground truncate text-xs'>
            {props.target.channel_name} · {props.target.group} · #{props.target.channel_id}
          </p>
        </div>
      </div>

      {(knowledgeQuery.data?.length ?? 0) > 0 ? (
        <div className='bg-muted/30 max-h-48 overflow-auto border-b px-4 py-3'>
          <div className='mb-2 flex items-center justify-between'><p className='text-sm font-medium'>选择本次对话使用的知识库</p><span className='text-muted-foreground text-xs'>已选 {selectedKnowledgeIds.length} 个文件</span></div>
          <div className='rounded-lg border bg-background p-2'>
            {renderKnowledgeNode(knowledgeTree)}
          </div>
        </div>
      ) : null}

      <div className='flex items-center justify-between gap-4 border-b bg-violet-500/5 px-4 py-3'>
        <div className='flex items-center gap-3'><div className='rounded-lg bg-violet-500/10 p-2 text-violet-500'><Sparkles className='size-4' /></div><div><p className='text-sm font-medium'>企业展示屏设计师 Skill</p><p className='text-muted-foreground text-xs'>开启后可试试：“切换成管理驾驶舱风格，并把标题改成全球运维中心”</p></div></div>
        <div className='flex items-center gap-3'><Button type='button' variant='outline' size='sm' onClick={() => window.open('/skill-showcase', '_blank')}><ExternalLink className='mr-1 size-3.5' />打开展示页</Button><Switch checked={showcaseSkillEnabled} onCheckedChange={setShowcaseSkillEnabled} aria-label='启用企业展示屏设计师 Skill' /></div>
      </div>

      <div className='flex min-h-0 flex-1 flex-col overflow-hidden'>
        <PlaygroundChat
          messages={messages}
          isLoadingMessages={false}
          onRegenerateMessage={conversation.handleRegenerateMessage}
          onEditMessage={conversation.handleEditMessage}
          onDeleteMessage={conversation.handleDeleteMessage}
          onSelectPrompt={conversation.handleSendMessage}
          isGenerating={isGenerating}
          editingKey={conversation.editingMessageKey}
          onCancelEdit={conversation.handleEditOpenChange}
          onSaveEdit={(content) => conversation.applyEdit(content, false)}
          onSaveEditAndSubmit={(content) => conversation.applyEdit(content, true)}
          emptyStateTitle={props.target.prompt_title}
          emptyStateDescription={props.target.prompt_description}
        />
      </div>

      <div className='mx-auto w-full max-w-4xl'>
        <PlaygroundInput
          config={config}
          disabled={isGenerating}
          groups={groups}
          groupValue={config.group}
          isGenerating={isGenerating}
          isModelLoading={false}
          modelValue={config.model}
          models={models}
          onGroupChange={(value) => updateConfig('group', value)}
          onConfigChange={updateConfig}
          onClearMessages={clearMessages}
          onModelChange={(value) => updateConfig('model', value)}
          onParameterEnabledChange={updateParameterEnabled}
          onStop={stopGeneration}
          onSubmit={conversation.handleSendMessage}
          parameterEnabled={parameterEnabled}
          hasMessages={messages.length > 0}
        />
      </div>
    </div>
  )
}

export function NativeChat() {
  const { t } = useTranslation()
  const [selected, setSelected] = useState<ChannelModel | null>(null)
  const channelModelsQuery = useQuery({
    queryKey: ['native-chat-channel-models'],
    queryFn: getChannelModels,
  })

  if (selected) {
    return (
      <ChannelChatSession
        key={`${selected.channel_id}-${selected.group}-${selected.model}`}
        target={selected}
        onBack={() => setSelected(null)}
      />
    )
  }

  if (channelModelsQuery.isPending) {
    return (
      <div className='flex size-full items-center justify-center'>
        <Loader2 className='text-muted-foreground size-8 animate-spin' />
      </div>
    )
  }

  if (channelModelsQuery.isError) {
    return (
      <div className='flex size-full items-center justify-center p-6 text-center'>
        <div>
          <p className='font-medium'>{t('Unable to load channel models')}</p>
          <Button
            type='button'
            variant='outline'
            className='mt-4'
            onClick={() => void channelModelsQuery.refetch()}
          >
            {t('Retry')}
          </Button>
        </div>
      </div>
    )
  }

  const channelModels = channelModelsQuery.data ?? []

  return (
    <div className='size-full overflow-auto p-6'>
      <div className='mx-auto max-w-6xl'>
        <div className='mb-6'>
          <h1 className='text-2xl font-semibold'>{t('Chat')}</h1>
          <p className='text-muted-foreground mt-1'>
            {t('Select a channel model to start chatting')}
          </p>
        </div>

        {channelModels.length === 0 ? (
          <div className='border-border rounded-xl border border-dashed p-12 text-center'>
            <Bot className='text-muted-foreground mx-auto size-10' />
            <p className='mt-4 font-medium'>{t('No channel models available')}</p>
          </div>
        ) : (
          <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-3'>
            {channelModels.map((item) => (
              <button
                type='button'
                key={`${item.channel_id}-${item.group}-${item.model}`}
                className='text-left'
                onClick={() => setSelected(item)}
              >
                <Card className='hover:border-primary/50 h-full transition-colors'>
                  <CardHeader>
                    <div className='flex items-start gap-3'>
                      <div className='bg-primary/10 text-primary rounded-lg p-2'>
                        <Server className='size-5' />
                      </div>
                      <div className='min-w-0'>
                        <CardTitle className='truncate'>{item.model}</CardTitle>
                        <CardDescription className='mt-1 truncate'>
                          {item.channel_name}
                        </CardDescription>
                      </div>
                    </div>
                    <p className='text-muted-foreground pt-2 text-xs'>
                      {item.group} · #{item.channel_id} · {t('Channel type')} {item.channel_type}
                    </p>
                  </CardHeader>
                </Card>
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
