export type ChannelModel = {
  channel_id: number
  channel_name: string
  channel_type: number
  group: string
  model: string
  prompt_title: string
  prompt_description: string
}

export type KnowledgeDocument = {
  id: number
  relative_path: string
  name: string
  allow_download: boolean
  size: number
}
