import { create } from 'zustand'
import { persist } from 'zustand/middleware'

type ChatParameterState = {
  enabledByTarget: Record<string, boolean>
  setEnabled: (target: string, enabled: boolean) => void
}

export const useChatParameterStore = create<ChatParameterState>()(
  persist(
    (set) => ({
      enabledByTarget: {},
      setEnabled: (target, enabled) =>
        set((state) => ({
          enabledByTarget: { ...state.enabledByTarget, [target]: enabled },
        })),
    }),
    { name: 'native-chat-custom-parameters' }
  )
)
