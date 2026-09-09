import { createFileRoute } from '@tanstack/react-router'

import { Main } from '@/components/layout'
import { SkillShowcase } from '@/features/skill-showcase'

export const Route = createFileRoute('/_authenticated/skill-showcase/')({
  component: SkillShowcasePage,
})

function SkillShowcasePage() {
  return <Main className='p-0'><SkillShowcase /></Main>
}
