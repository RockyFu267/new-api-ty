import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, test } from 'vitest'

import { appendUserMessagePair } from '@/features/playground/lib/message/conversation-message-utils'
import { formatMessageForAPI } from '@/features/playground/lib/message/message-utils'

import { ChatContextSection } from '../chat-context-section'
import {
  readTemporaryAttachment,
  useTemporaryAttachments,
} from '../temporary-attachments'

const png = new Uint8Array([137, 80, 78, 71, 13, 10, 26, 10])

function ComposerFixture() {
  const attachments = useTemporaryAttachments()
  return (
    <>
      {attachments.panel}
      <textarea aria-label='Message' onPaste={attachments.onPaste} />
      <button type='button' onClick={attachments.clear}>
        Clear conversation
      </button>
    </>
  )
}

describe('temporary chat attachments', () => {
  test('reads allowed text, checks contents and rejects unsupported or oversized files', async () => {
    for (const name of ['trace.log', 'notes.txt', 'readme.md']) {
      expect(
        await readTemporaryAttachment(new File(['日志 error'], name))
      ).toMatchObject({ name, text: '日志 error' })
    }
    await expect(
      readTemporaryAttachment(new File(['%PDF'], 'report.pdf'))
    ).rejects.toThrow('Only LOG')
    await expect(
      readTemporaryAttachment(new File(['abc\0'], 'binary.txt'))
    ).rejects.toThrow('UTF-8')
    await expect(
      readTemporaryAttachment(new File(['x'.repeat(100001)], 'large.log'))
    ).rejects.toThrow('100,000')
    await expect(
      readTemporaryAttachment(
        new File([new Uint8Array(5 * 1024 * 1024 + 1)], 'large.png')
      )
    ).rejects.toThrow('5 MB')
  })

  test('sniffs pasted image contents and sends images in message history and retries', async () => {
    const attachment = await readTemporaryAttachment(
      new File([png], 'clipboard.png', { type: 'image/png' })
    )
    expect(attachment.image).toBe('data:image/png;base64,iVBORw0KGgo=')
    await expect(
      readTemporaryAttachment(
        new File(['fake'], 'fake.png', { type: 'image/png' })
      )
    ).rejects.toThrow('Only LOG')
    const messages = appendUserMessagePair([], 'What is shown?', [
      attachment.image ?? '',
    ])
    expect(formatMessageForAPI(messages[0])).toEqual({
      role: 'user',
      content: [
        { type: 'text', text: 'What is shown?' },
        { type: 'image_url', image_url: { url: attachment.image } },
      ],
    })
    expect(
      formatMessageForAPI(appendUserMessagePair(messages, 'Explain more')[0])
    ).toEqual(formatMessageForAPI(messages[0]))
  })

  test('uploads, removes, pastes and clears attachments without storing them', async () => {
    const user = userEvent.setup()
    render(<ComposerFixture />)
    const input = screen.getByLabelText(/Temporary attachments|临时附件/)
    await user.upload(input, new File(['hello'], 'notes.txt'))
    expect(await screen.findByText('notes.txt')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: /notes.txt/ }))
    expect(screen.queryByText('notes.txt')).not.toBeInTheDocument()
    fireEvent.paste(screen.getByRole('textbox'), {
      clipboardData: { files: [new File([png], 'screen.png')] },
    })
    expect(await screen.findByAltText('screen.png')).toHaveAttribute(
      'src',
      'data:image/png;base64,iVBORw0KGgo='
    )
    await user.click(screen.getByRole('button', { name: 'Clear conversation' }))
    await waitFor(() =>
      expect(screen.queryByAltText('screen.png')).not.toBeInTheDocument()
    )
  })

  test('collapses context without clearing its selection', async () => {
    const user = userEvent.setup()
    render(
      <ChatContextSection title='Skill' count={1}>
        <input type='checkbox' aria-label='Enable skill' defaultChecked />
      </ChatContextSection>
    )
    const toggle = screen.getByRole('button', { name: /Skill/ })
    await user.click(toggle)
    expect(toggle).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument()
    await user.click(toggle)
    expect(screen.getByRole('checkbox')).toBeChecked()
  })
})
