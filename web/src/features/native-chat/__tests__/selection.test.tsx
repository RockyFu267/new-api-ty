import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { NativeChat } from '..'

vi.mock('@/lib/api', () => ({
  api: {
    get: vi.fn(),
  },
  getFreshAuthHeaders: vi.fn().mockResolvedValue({}),
}))

function renderNativeChat() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <NativeChat />
    </QueryClientProvider>
  )
}

describe('NativeChat channel model selection', () => {
  beforeEach(() => {
    vi.mocked(api.get).mockImplementation(async (url) => ({
      data: {
        success: true,
        data: url === '/api/user/chat/knowledge-documents' ? [{
          id: 11,
          relative_path: 'k8s/k8s-clusters.md',
          name: 'k8s-clusters.md',
          allow_download: false,
          size: 1024,
        }] : [
          {
            channel_id: 301,
            channel_name: 'Local Qwen',
            channel_type: 33,
            group: 'default',
            model: 'qwen-local',
            prompt_title: '本地免费模型',
            prompt_description: '可用于内部测试。',
          },
          {
            channel_id: 302,
            channel_name: 'AWS Bedrock',
            channel_type: 24,
            group: 'default',
            model: 'claude-bedrock',
            prompt_title: '付费模型提醒',
            prompt_description: '使用会产生费用。',
          },
        ],
      },
    } as never))
  })

  test('opens the exact selected channel and model combination', async () => {
    const user = userEvent.setup()
    renderNativeChat()

    const bedrockButton = await screen.findByRole('button', {
      name: /claude-bedrock AWS Bedrock/,
    })
    expect(screen.getByRole('button', { name: /qwen-local Local Qwen/ })).toBeInTheDocument()

    await user.click(bedrockButton)

    expect(screen.getByText('AWS Bedrock · default · #302')).toBeInTheDocument()
    expect(screen.getByText('付费模型提醒')).toBeInTheDocument()
    expect(screen.getByText('使用会产生费用。')).toBeInTheDocument()
    expect(screen.getByText('选择本次对话使用的知识库')).toBeInTheDocument()
    expect(screen.getByText('k8s-clusters.md')).toBeInTheDocument()
    expect(screen.queryByText('Local Qwen · default · #301')).not.toBeInTheDocument()
  })
})
