import type { InjectionKey, Ref } from 'vue'
import { inject } from 'vue'

export interface ReasoningContextValue {
  isStreaming: Ref<boolean>
  duration: Ref<number | undefined>
}

export const ReasoningKey: InjectionKey<ReasoningContextValue> = Symbol('ReasoningContext')

export function useReasoningContext(): ReasoningContextValue {
  const context = inject(ReasoningKey)
  if (!context) throw new Error('Reasoning components must be used within <Reasoning>')
  return context
}
