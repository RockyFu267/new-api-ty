import { useCallback, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { ArrowLeft, Bot, Download, FileText, Folder, Loader2, Server } from 'lucide-react'
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
    config,
    parameterEnabled,
    onMessageUpdate: updateMessages,
  })

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

  const knowledgeByDirectory = useMemo(() => {
    const grouped = new Map<string, KnowledgeDocument[]>()
    for (const document of knowledgeQuery.data ?? []) {
      const directory = document.relative_path.includes('/')
        ? document.relative_path.slice(0, document.relative_path.lastIndexOf('/'))
        : '根目录'
      grouped.set(directory, [...(grouped.get(directory) ?? []), document])
    }
    return [...grouped.entries()]
  }, [knowledgeQuery.data])

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
          <div className='grid gap-2 md:grid-cols-2'>
            {knowledgeByDirectory.map(([directory, documents]) => {
              const ids = documents.map((document) => document.id)
              const directoryChecked = ids.every((id) => selectedKnowledgeIds.includes(id))
              return <div key={directory} className='rounded-lg border bg-background p-3'>
                <label className='flex cursor-pointer items-center gap-2 font-medium'><Checkbox checked={directoryChecked} onCheckedChange={(value) => toggleDocuments(ids, value === true)} /><Folder className='size-4' />{directory}</label>
                <div className='mt-2 space-y-2 pl-6'>{documents.map((document) => (
                  <div key={document.id} className='flex items-center gap-2 text-sm'>
                    <Checkbox checked={selectedKnowledgeIds.includes(document.id)} onCheckedChange={(value) => toggleDocuments([document.id], value === true)} />
                    <FileText className='size-3.5' /><span className='min-w-0 flex-1 truncate'>{document.name}</span>
                    {document.allow_download ? <Button type='button' variant='ghost' size='icon' className='size-7' aria-label={`下载 ${document.name}`} onClick={() => void downloadKnowledge(document)}><Download className='size-3.5' /></Button> : null}
                  </div>
                ))}</div>
              </div>
            })}
          </div>
        </div>
      ) : null}

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
