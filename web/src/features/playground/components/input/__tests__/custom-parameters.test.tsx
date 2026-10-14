import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { afterEach, expect, test } from 'vitest'

import { useChatParameterStore } from '@/stores/chat-parameter-store'

import { DEFAULT_CONFIG, DEFAULT_PARAMETER_ENABLED } from '../../../constants'
import { buildChatCompletionPayload } from '../../../lib/streaming/payload-builder'
import { PlaygroundInputTools } from '../playground-input-tools'

afterEach(() => {
  cleanup()
  localStorage.clear()
  useChatParameterStore.setState({ enabledByTarget: {} })
})

function Tools() {
  const [enabled, setEnabled] = useState(true)
  return (
    <PlaygroundInputTools
      config={DEFAULT_CONFIG}
      parameterEnabled={DEFAULT_PARAMETER_ENABLED}
      onConfigChange={() => {}}
      onParameterEnabledChange={() => {}}
      customParametersEnabled={enabled}
      onCustomParametersEnabledChange={setEnabled}
      showSearch={false}
    />
  )
}

test('turning off custom parameters disables individual controls and turning on restores selections', async () => {
  const user = userEvent.setup()
  render(<Tools />)
  expect(
    screen.queryByRole('button', { name: 'Search' })
  ).not.toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: 'Parameters' }))
  const master = screen.getByRole('switch', {
    name: 'Enable custom parameters',
  })
  const temperature = screen.getByRole('switch', { name: 'Enable Temperature' })
  expect(temperature).toBeChecked()
  await user.click(master)
  expect(temperature).toHaveAttribute('aria-disabled', 'true')
  expect(temperature).toBeChecked()
  await user.click(master)
  expect(temperature).not.toHaveAttribute('aria-disabled', 'true')
  expect(temperature).toBeChecked()
})

test('disabled custom parameters are omitted in streaming and nonstreaming requests', () => {
  for (const stream of [true, false]) {
    const config = { ...DEFAULT_CONFIG, stream, seed: 42 }
    const enabled = {
      ...DEFAULT_PARAMETER_ENABLED,
      max_tokens: true,
      seed: true,
    }
    const payload = buildChatCompletionPayload([], config, enabled, false)
    expect(payload).toEqual({
      model: config.model,
      group: config.group,
      stream,
      messages: [],
    })
    expect(buildChatCompletionPayload([], config, enabled, true)).toMatchObject(
      {
        temperature: config.temperature,
        seed: 42,
        max_tokens: config.max_tokens,
      }
    )
  }
})

test('saved switches survive reload and remain separate for each user channel and model', async () => {
  const store = useChatParameterStore.getState()
  store.setEnabled('[1,1,"qwen"]', false)
  store.setEnabled('[1,2,"qwen"]', true)
  store.setEnabled('[2,1,"qwen"]', true)
  store.setEnabled('[1,1,"deepseek"]', true)
  const saved = localStorage.getItem('native-chat-custom-parameters') ?? ''
  useChatParameterStore.setState({ enabledByTarget: {} })
  localStorage.setItem('native-chat-custom-parameters', saved)
  await useChatParameterStore.persist.rehydrate()
  expect(useChatParameterStore.getState().enabledByTarget).toEqual({
    '[1,1,"qwen"]': false,
    '[1,2,"qwen"]': true,
    '[2,1,"qwen"]': true,
    '[1,1,"deepseek"]': true,
  })
})
