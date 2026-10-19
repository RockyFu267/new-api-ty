import { createFileRoute, redirect } from '@tanstack/react-router'

import { UsageAnalysis } from '@/features/usage-analysis'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

export const Route = createFileRoute('/_authenticated/usage-analysis/')({
  beforeLoad: () => {
    if ((useAuthStore.getState().auth.user?.role ?? 0) < ROLE.ADMIN) {
      throw redirect({ to: '/403' })
    }
  },
  component: UsageAnalysis,
})
