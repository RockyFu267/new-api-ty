import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { SSE } from 'sse.js'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { NativeChat } from '..'

vi.mock('sse.js', () => ({
  SSE: vi.fn(function () {
    return { addEventListener: vi.fn(), stream: vi.fn(), close: vi.fn() }
  }),
}))

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
    vi.mocked(SSE).mockClear()
    const responses: Record<string, unknown> = {
      '/api/user/chat/knowledge-documents': [
        {
          id: 11,
          relative_path: 'k8s/k8s-clusters.md',
          name: 'k8s-clusters.md',
          allow_download: false,
          size: 1024,
        },
      ],
      '/api/user/chat/channel-models': [
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
      '/api/user/private-knowledge/documents': {
        documents: [],
        directories: [],
      },
      '/api/user/company-quota': { enabled: false },
    }
    vi.mocked(api.get).mockImplementation(
      async (url) =>
        ({
          data: { success: true, data: responses[String(url)] ?? null },
        }) as never
    )
  })

  test('opens the exact selected channel and model combination', async () => {
    const user = userEvent.setup()
    renderNativeChat()

    const bedrockButton = await screen.findByRole('button', {
      name: /claude-bedrock AWS Bedrock/,
    })
    expect(
      screen.getByRole('button', { name: /qwen-local Local Qwen/ })
    ).toBeInTheDocument()

    await user.click(bedrockButton)

    expect(screen.getByText('AWS Bedrock · default · #302')).toBeInTheDocument()
    expect(screen.getByText('付费模型提醒')).toBeInTheDocument()
    expect(screen.getByText('使用会产生费用。')).toBeInTheDocument()
    expect(screen.getByText('选择本次对话使用的知识库')).toBeInTheDocument()
    expect(screen.getByText('k8s-clusters.md')).toBeInTheDocument()
    expect(
      screen.queryByText('Local Qwen · default · #301')
    ).not.toBeInTheDocument()
  })

  test('sends text files and images through the real chat composer', async () => {
    const user = userEvent.setup()
    renderNativeChat()
    await user.click(
      await screen.findByRole('button', { name: /claude-bedrock AWS Bedrock/ })
    )
    await user.upload(screen.getByLabelText(/Temporary attachments|临时附件/), [
      new File(['database connection refused'], 'server.log'),
    ])
    expect(await screen.findByText('server.log')).toBeInTheDocument()
    fireEvent.paste(screen.getByRole('textbox'), {
      clipboardData: {
        files: [
          new File(
            [new Uint8Array([137, 80, 78, 71, 13, 10, 26, 10])],
            'screen.png',
            { type: 'image/png' }
          ),
        ],
      },
    })
    expect(await screen.findByAltText('screen.png')).toBeInTheDocument()
    await user.click(screen.getAllByRole('button', { name: /Send|发送/ })[0])
    await waitFor(() => expect(SSE).toHaveBeenCalledOnce())
    const options = vi.mocked(SSE).mock.calls[0][1]
    const payload = JSON.parse(String(options?.payload))
    expect(payload.channel_id).toBe(302)
    expect(payload.model).toBe('claude-bedrock')
    expect(payload.messages[0].content).toEqual([
      {
        type: 'text',
        text: '\n--- server.log ---\ndatabase connection refused\n---',
      },
      {
        type: 'image_url',
        image_url: { url: 'data:image/png;base64,iVBORw0KGgo=' },
      },
    ])
    expect(screen.getByText('server.log')).toBeInTheDocument()
    expect(
      screen.queryByText(/database connection refused/)
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('button', {
        name: /移除附件.*screen.png|Remove attachment.*screen.png/,
      })
    ).not.toBeInTheDocument()
  })
})
