import { useQuery, useQueryClient } from '@tanstack/react-query'
import { FileText, FolderSync } from 'lucide-react'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { api } from '@/lib/api'

import { SettingsSection } from '../components/settings-section'

export type KnowledgeDocument = {
  id: number
  relative_path: string
  name: string
  enabled: boolean
  allow_download: boolean
  available: boolean
  size: number
}

async function loadDocuments(): Promise<KnowledgeDocument[]> {
  const response = await api.get('/api/knowledge-bases/documents')
  return response.data?.data ?? []
}

export function KnowledgeDocumentManagementSection() {
  const queryClient = useQueryClient()
  const query = useQuery({ queryKey: ['knowledge-documents-admin'], queryFn: loadDocuments })

  const refresh = async () => {
    const response = await api.post('/api/knowledge-bases/scan')
    if (response.data?.success) {
      toast.success(`已扫描 ${response.data.data?.count ?? 0} 个 Markdown 文件`)
      await queryClient.invalidateQueries({ queryKey: ['knowledge-documents-admin'] })
      await queryClient.invalidateQueries({ queryKey: ['knowledge-documents-user'] })
    }
  }

  const update = async (document: KnowledgeDocument, values: Partial<KnowledgeDocument>) => {
    const response = await api.put(`/api/knowledge-bases/documents/${document.id}`, {
      enabled: values.enabled ?? document.enabled,
      allow_download: values.allow_download ?? document.allow_download,
    })
    if (response.data?.success) {
      await queryClient.invalidateQueries({ queryKey: ['knowledge-documents-admin'] })
      await queryClient.invalidateQueries({ queryKey: ['knowledge-documents-user'] })
    }
  }

  return (
    <SettingsSection title='知识库文件管理' className='border-t pt-6'>
      <div className='flex items-center justify-between gap-4'>
        <p className='text-muted-foreground text-sm'>扫描服务端 knowledge-base 目录，由管理员逐个开放 Markdown 文件。用户可在 Chat 中按目录或文件多选。</p>
        <Button type='button' variant='outline' onClick={() => void refresh()}><FolderSync className='mr-2 size-4' />扫描目录</Button>
      </div>
      <div className='overflow-hidden rounded-xl border'>
        {query.data?.map((document) => (
          <div key={document.id} className='flex items-center gap-4 border-b p-4 last:border-b-0'>
            <FileText className='text-muted-foreground size-4 shrink-0' />
            <div className='min-w-0 flex-1'><p className='truncate font-medium'>{document.relative_path}</p><p className='text-muted-foreground text-xs'>{Math.ceil(document.size / 1024)} KB{document.available ? '' : ' · 文件已不存在'}</p></div>
            <label className='flex items-center gap-2 text-sm'>用户可用<Switch checked={document.enabled} disabled={!document.available} onCheckedChange={(checked) => void update(document, { enabled: checked })} /></label>
            <label className='flex items-center gap-2 text-sm'>允许下载<Switch checked={document.allow_download} disabled={!document.enabled} onCheckedChange={(checked) => void update(document, { allow_download: checked })} /></label>
          </div>
        ))}
        {!query.isPending && query.data?.length === 0 ? <p className='text-muted-foreground p-6 text-center text-sm'>尚未扫描，请点击“扫描目录”。</p> : null}
      </div>
    </SettingsSection>
  )
}
