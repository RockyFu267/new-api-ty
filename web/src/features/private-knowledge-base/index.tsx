import { useMemo, useState, type ReactNode } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { FilePlus2, FileText, Folder, FolderPlus, Pencil, Trash2, Upload } from 'lucide-react'
import { toast } from 'sonner'

import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { buildKnowledgeTree, type KnowledgeTreeNode } from '@/features/knowledge-base/tree'
import { api } from '@/lib/api'
import { loadPrivateKnowledgeDocuments, type PrivateKnowledgeDocument } from './api'

export function PrivateKnowledgeBase() {
  const queryClient = useQueryClient()
  const query = useQuery({ queryKey: ['private-knowledge-documents'], queryFn: loadPrivateKnowledgeDocuments })
  const [editor, setEditor] = useState<{ id?: number; relativePath: string; content: string } | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<PrivateKnowledgeDocument | null>(null)
  const [saving, setSaving] = useState(false)
  const [directoryEditor, setDirectoryEditor] = useState<string | null>(null)
  const [uploadDirectory, setUploadDirectory] = useState('')
  const tree = useMemo(() => buildKnowledgeTree(query.data?.documents ?? [], query.data?.directories ?? []), [query.data])

  const createDirectory = async () => {
    if (!directoryEditor?.trim()) { toast.error('请输入目录路径'); return }
    const response = await api.post('/api/user/private-knowledge/directories', { relative_path: directoryEditor })
    if (response.data?.success) { toast.success('目录已创建'); setDirectoryEditor(null); await queryClient.invalidateQueries({ queryKey: ['private-knowledge-documents'] }) }
  }

  const upload = async (file: File) => {
    const form = new FormData()
    form.append('file', file)
    form.append('directory', uploadDirectory)
    const response = await api.post('/api/user/private-knowledge/upload', form)
    if (response.data?.success) { toast.success('文件已上传'); await queryClient.invalidateQueries({ queryKey: ['private-knowledge-documents'] }) }
  }

  const edit = async (document: PrivateKnowledgeDocument) => {
    const response = await api.get(`/api/user/private-knowledge/documents/${document.id}`)
    if (response.data?.success) setEditor({ id: document.id, relativePath: response.data.data.relative_path, content: response.data.data.content })
  }
  const save = async () => {
    if (!editor?.relativePath.trim()) { toast.error('请输入 Markdown 文件相对路径'); return }
    setSaving(true)
    try {
      const response = editor.id ? await api.put(`/api/user/private-knowledge/documents/${editor.id}`, { content: editor.content }) : await api.post('/api/user/private-knowledge/documents', { relative_path: editor.relativePath, content: editor.content })
      if (response.data?.success) { toast.success(editor.id ? '私有知识已保存' : '私有知识已创建'); setEditor(null); await queryClient.invalidateQueries({ queryKey: ['private-knowledge-documents'] }) }
    } finally { setSaving(false) }
  }
  const remove = async () => {
    if (!deleteTarget) return
    const response = await api.delete(`/api/user/private-knowledge/documents/${deleteTarget.id}`)
    if (response.data?.success) { toast.success('私有知识已删除'); setDeleteTarget(null); await queryClient.invalidateQueries({ queryKey: ['private-knowledge-documents'] }) }
  }
  const renderNode = (node: KnowledgeTreeNode<PrivateKnowledgeDocument>, depth = 0): ReactNode => {
    if (node.kind === 'directory') return <div key={`d:${node.path}`}><div className='bg-muted/30 flex items-center gap-2 border-b py-2' style={{ paddingLeft: `${depth * 24 + 16}px` }}><Folder className='text-primary size-4' /><span className='font-medium'>{node.name}</span></div>{node.children.map((child) => renderNode(child, depth + 1))}</div>
    const document = node.document
    if (!document) return null
    return <div key={document.id} className='flex items-center gap-3 border-b py-3 last:border-0' style={{ paddingLeft: `${depth * 24 + 16}px` }}><FileText className='text-muted-foreground size-4' /><div className='min-w-0 flex-1'><p className='truncate font-medium'>{node.name}</p><p className='text-muted-foreground text-xs'>{node.path} · {Math.ceil(document.size / 1024)} KB</p></div><Button variant='ghost' size='icon' onClick={() => void edit(document)}><Pencil className='size-4' /></Button><Button variant='ghost' size='icon' onClick={() => setDeleteTarget(document)}><Trash2 className='text-destructive size-4' /></Button></div>
  }

  return <div className='mx-auto w-full max-w-6xl space-y-6 p-6'><div className='flex flex-wrap items-center justify-between gap-4'><div><h1 className='text-2xl font-semibold'>我的知识库</h1><p className='text-muted-foreground mt-1 text-sm'>维护仅供你本人在 Chat 中使用的 Markdown 或 TXT 知识文件。</p></div><div className='flex flex-wrap items-center gap-2'><Input className='w-48' value={uploadDirectory} placeholder='上传目录（可留空）' onChange={(event) => setUploadDirectory(event.target.value)} /><Button variant='outline' onClick={() => setDirectoryEditor('')}><FolderPlus className='mr-2 size-4' />新建目录</Button><Button variant='outline' render={<label className='cursor-pointer' />}><Upload className='mr-2 size-4' />上传文件<input className='hidden' type='file' accept='.md,.txt,text/markdown,text/plain' onChange={(event) => { const file = event.target.files?.[0]; if (file) void upload(file); event.target.value = '' }} /></Button><Button onClick={() => setEditor({ relativePath: '', content: '# 新知识文件\n\n' })}><FilePlus2 className='mr-2 size-4' />新建文件</Button></div></div><div className='overflow-hidden rounded-xl border'>{(query.data?.documents.length ?? 0) > 0 || (query.data?.directories.length ?? 0) > 0 ? renderNode(tree) : <p className='text-muted-foreground p-10 text-center text-sm'>还没有私有知识文件或目录。</p>}</div>
    <Dialog open={directoryEditor !== null} onOpenChange={(open) => { if (!open) setDirectoryEditor(null) }}><DialogContent><DialogHeader><DialogTitle>新建目录</DialogTitle><DialogDescription>支持输入嵌套路径，例如 network/switches。</DialogDescription></DialogHeader><div className='space-y-2'><Label>目录相对路径</Label><Input value={directoryEditor ?? ''} placeholder='例如：network/switches' onChange={(event) => setDirectoryEditor(event.target.value)} /></div><DialogFooter><Button variant='outline' onClick={() => setDirectoryEditor(null)}>取消</Button><Button onClick={() => void createDirectory()}>创建</Button></DialogFooter></DialogContent></Dialog>
    <Dialog open={editor !== null} onOpenChange={(open) => { if (!open) setEditor(null) }}><DialogContent className='sm:max-w-3xl'><DialogHeader><DialogTitle>{editor?.id ? '编辑私有知识' : '新建私有知识'}</DialogTitle><DialogDescription>文件将保存到服务端当前用户名对应的私有目录。</DialogDescription></DialogHeader><div className='space-y-2'><Label>相对路径</Label><Input disabled={Boolean(editor?.id)} value={editor?.relativePath ?? ''} placeholder='例如：network/switch.md' onChange={(event) => setEditor((current) => current ? { ...current, relativePath: event.target.value } : current)} /></div><div className='space-y-2'><Label>Markdown 内容</Label><Textarea className='min-h-96 font-mono' value={editor?.content ?? ''} onChange={(event) => setEditor((current) => current ? { ...current, content: event.target.value } : current)} /></div><DialogFooter><Button variant='outline' onClick={() => setEditor(null)}>取消</Button><Button disabled={saving} onClick={() => void save()}>保存</Button></DialogFooter></DialogContent></Dialog>
    <AlertDialog open={deleteTarget !== null} onOpenChange={(open) => { if (!open) setDeleteTarget(null) }}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>删除私有知识文件？</AlertDialogTitle><AlertDialogDescription>将永久删除 {deleteTarget?.relative_path}。</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel>取消</AlertDialogCancel><AlertDialogAction variant='destructive' onClick={() => void remove()}>删除</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog>
  </div>
}
