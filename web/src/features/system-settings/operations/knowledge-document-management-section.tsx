import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, type ReactNode } from 'react'
import { FilePlus2, FileText, FolderOpen, FolderSync, Pencil, Trash2 } from 'lucide-react'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { api } from '@/lib/api'
import {
  buildKnowledgeTree,
  type KnowledgeTreeNode,
} from '@/features/knowledge-base/tree'

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
  const [editor, setEditor] = useState<{
    id?: number
    relativePath: string
    content: string
  } | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<KnowledgeDocument | null>(null)
  const [saving, setSaving] = useState(false)

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

  const editDocument = async (document: KnowledgeDocument) => {
    const response = await api.get(`/api/knowledge-bases/documents/${document.id}`)
    if (response.data?.success) {
      setEditor({
        id: document.id,
        relativePath: response.data.data.relative_path,
        content: response.data.data.content,
      })
    }
  }

  const saveDocument = async () => {
    if (!editor?.relativePath.trim()) {
      toast.error('请输入 Markdown 文件相对路径')
      return
    }
    setSaving(true)
    try {
      const response = editor.id
        ? await api.put(`/api/knowledge-bases/documents/${editor.id}/content`, {
            content: editor.content,
          })
        : await api.post('/api/knowledge-bases/documents', {
            relative_path: editor.relativePath,
            content: editor.content,
          })
      if (response.data?.success) {
        toast.success(editor.id ? '知识文件已保存' : '知识文件已创建')
        setEditor(null)
        await queryClient.invalidateQueries({ queryKey: ['knowledge-documents-admin'] })
        await queryClient.invalidateQueries({ queryKey: ['knowledge-documents-user'] })
      }
    } finally {
      setSaving(false)
    }
  }

  const deleteDocument = async () => {
    if (!deleteTarget) return
    const response = await api.delete(`/api/knowledge-bases/documents/${deleteTarget.id}`)
    if (response.data?.success) {
      toast.success('知识文件已删除')
      setDeleteTarget(null)
      await queryClient.invalidateQueries({ queryKey: ['knowledge-documents-admin'] })
      await queryClient.invalidateQueries({ queryKey: ['knowledge-documents-user'] })
    }
  }

  const tree = buildKnowledgeTree(query.data ?? [])

  const renderNode = (
    node: KnowledgeTreeNode<KnowledgeDocument>,
    depth = 0
  ): ReactNode => {
    if (node.kind === 'directory') {
      return (
        <div key={`directory:${node.path || 'root'}`}>
          <div
            className='bg-muted/30 flex items-center gap-2 border-b px-4 py-2 font-medium'
            style={{ paddingLeft: `${depth * 24 + 16}px` }}
          >
            <FolderOpen className='text-primary size-4 shrink-0' />
            <span>{node.name}</span>
            <span className='text-muted-foreground text-xs font-normal'>
              {node.children.length} 项
            </span>
          </div>
          {node.children.map((child) => renderNode(child, depth + 1))}
        </div>
      )
    }

    const document = node.document
    if (!document) return null
    return (
      <div
        key={`file:${node.path}`}
        className='flex items-center gap-4 border-b p-3 last:border-b-0'
        style={{ paddingLeft: `${depth * 24 + 16}px` }}
      >
        <FileText className='text-muted-foreground size-4 shrink-0' />
        <div className='min-w-0 flex-1'>
          <p className='truncate font-medium'>{node.name}</p>
          <p className='text-muted-foreground truncate text-xs'>
            {node.path} · {Math.ceil(document.size / 1024)} KB
            {document.available ? '' : ' · 文件已不存在'}
          </p>
        </div>
        <Button type='button' variant='ghost' size='icon' aria-label={`编辑 ${node.path}`} onClick={() => void editDocument(document)}>
          <Pencil className='size-4' />
        </Button>
        <Button type='button' variant='ghost' size='icon' aria-label={`删除 ${node.path}`} onClick={() => setDeleteTarget(document)}>
          <Trash2 className='text-destructive size-4' />
        </Button>
        <label className='flex items-center gap-2 text-sm'>
          用户可用
          <Switch
            checked={document.enabled}
            disabled={!document.available}
            onCheckedChange={(checked) =>
              void update(document, { enabled: checked })
            }
          />
        </label>
        <label className='flex items-center gap-2 text-sm'>
          允许下载
          <Switch
            checked={document.allow_download}
            disabled={!document.enabled}
            onCheckedChange={(checked) =>
              void update(document, { allow_download: checked })
            }
          />
        </label>
      </div>
    )
  }

  return (
    <SettingsSection title='知识库'>
      <div className='flex items-center justify-between gap-4'>
        <p className='text-muted-foreground text-sm'>扫描服务端 knowledge-base 目录，由管理员逐个开放 Markdown 文件。用户可在 Chat 中按目录或文件多选。</p>
        <div className='flex gap-2'>
          <Button type='button' onClick={() => setEditor({ relativePath: '', content: '# 新知识文件\n\n' })}><FilePlus2 className='mr-2 size-4' />新建文件</Button>
          <Button type='button' variant='outline' onClick={() => void refresh()}><FolderSync className='mr-2 size-4' />扫描目录</Button>
        </div>
      </div>
      <div className='overflow-hidden rounded-xl border'>
        {(query.data?.length ?? 0) > 0 ? renderNode(tree) : null}
        {!query.isPending && query.data?.length === 0 ? <p className='text-muted-foreground p-6 text-center text-sm'>尚未扫描，请点击“扫描目录”。</p> : null}
      </div>
      <Dialog open={editor !== null} onOpenChange={(open) => { if (!open) setEditor(null) }}>
        <DialogContent className='sm:max-w-3xl'>
          <DialogHeader>
            <DialogTitle>{editor?.id ? '编辑知识文件' : '新建知识文件'}</DialogTitle>
            <DialogDescription>内容会直接保存到服务端 knowledge-base 目录中的 Markdown 原文件。</DialogDescription>
          </DialogHeader>
          <div className='space-y-2'>
            <Label>相对路径</Label>
            <Input
              value={editor?.relativePath ?? ''}
              disabled={Boolean(editor?.id)}
              placeholder='例如：k8s/new-cluster.md'
              onChange={(event) => setEditor((current) => current ? { ...current, relativePath: event.target.value } : current)}
            />
          </div>
          <div className='space-y-2'>
            <Label>Markdown 内容</Label>
            <Textarea
              value={editor?.content ?? ''}
              className='min-h-96 font-mono text-sm'
              onChange={(event) => setEditor((current) => current ? { ...current, content: event.target.value } : current)}
            />
          </div>
          <DialogFooter>
            <Button type='button' variant='outline' onClick={() => setEditor(null)}>取消</Button>
            <Button type='button' disabled={saving} onClick={() => void saveDocument()}>保存</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={deleteTarget !== null} onOpenChange={(open) => { if (!open) setDeleteTarget(null) }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认删除知识文件？</AlertDialogTitle>
            <AlertDialogDescription>将永久删除 knowledge-base/{deleteTarget?.relative_path}，此操作无法撤销。</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction variant='destructive' onClick={() => void deleteDocument()}>删除文件</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </SettingsSection>
  )
}
