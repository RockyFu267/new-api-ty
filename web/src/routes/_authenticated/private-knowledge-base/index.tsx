import { createFileRoute } from '@tanstack/react-router'
import { Main } from '@/components/layout'
import { PrivateKnowledgeBase } from '@/features/private-knowledge-base'

export const Route = createFileRoute('/_authenticated/private-knowledge-base/')({ component: Page })
function Page() { return <Main className='p-0'><PrivateKnowledgeBase /></Main> }
