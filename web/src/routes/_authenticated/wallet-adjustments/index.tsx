import { createFileRoute, redirect } from '@tanstack/react-router'

import { WalletAdjustments } from '@/features/company-quota/wallet-adjustments'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

export const Route = createFileRoute('/_authenticated/wallet-adjustments/')({
  beforeLoad: () => {
    if ((useAuthStore.getState().auth.user?.role ?? 0) < ROLE.ADMIN)
      {throw redirect({ to: '/403' })}
  },
  component: WalletAdjustments,
})
