import { createFileRoute } from '@tanstack/react-router'

import { Main } from '@/components/layout'
import { NativeChat } from '@/features/native-chat'

export const Route = createFileRoute('/_authenticated/chat/')({
  component: NativeChatPage,
})

function NativeChatPage() {
  return (
    <Main className='p-0'>
      <NativeChat />
    </Main>
  )
}
