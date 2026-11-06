import { ref } from 'vue'

export type AILaunchKind = 'selection' | 'recent' | 'file'

export interface AILaunchRequest {
  id: number
  kind: AILaunchKind

  text: string
  mode: 'ask' | 'agent'
}

const launch = ref<AILaunchRequest | null>(null)
let sequence = 0

export function requestAILaunch(input: Omit<AILaunchRequest, 'id'>): void {
  sequence += 1
  launch.value = { ...input, id: sequence }
}

export function useAILaunch() {
  return launch
}

export function clearAILaunch(id: number): void {
  if (launch.value?.id === id) launch.value = null
}
