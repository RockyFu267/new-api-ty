import { api } from '@/lib/api'

export type PrivateKnowledgeDocument = { id: number; relative_path: string; name: string; size: number }
export type PrivateKnowledgeData = { documents: PrivateKnowledgeDocument[]; directories: string[] }

export async function loadPrivateKnowledgeDocuments(): Promise<PrivateKnowledgeData> {
  const response = await api.get('/api/user/private-knowledge/documents')
  const data = response.data?.data
  if (Array.isArray(data)) return { documents: data, directories: [] }
  return {
    documents: Array.isArray(data?.documents) ? data.documents : [],
    directories: Array.isArray(data?.directories) ? data.directories : [],
  }
}
