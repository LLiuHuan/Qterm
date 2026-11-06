import { ref } from 'vue'

let handler: ((content: string) => void) | null = null
const pending = ref<string | null>(null)

export function registerSnippetCreator(next: (content: string) => void): void {
  handler = next
  if (pending.value !== null) {
    const content = pending.value
    pending.value = null
    handler(content)
  }
}

export function requestSnippetCreator(content: string): boolean {
  const trimmed = content.trim()
  if (!trimmed) return false
  if (handler) {
    handler(trimmed)
    return true
  }
  pending.value = trimmed
  return false
}
