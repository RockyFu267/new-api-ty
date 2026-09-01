import { api } from '@/lib/api'

import type { ChannelModel } from './types'

export async function getChannelModels(): Promise<ChannelModel[]> {
  const response = await api.get('/api/user/chat/channel-models')
  if (!response.data?.success || !Array.isArray(response.data.data)) {
    return []
  }
  return response.data.data as ChannelModel[]
}
