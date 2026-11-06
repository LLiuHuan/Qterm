import { inject, provide, ref, type InjectionKey, type Ref } from 'vue'

export interface RemoteEditorFocusRegistry {
  readonly hasActive: Ref<boolean>

  register(save: () => void): () => void

  requestSave(): boolean
}

export const remoteEditorFocusKey: InjectionKey<RemoteEditorFocusRegistry> = Symbol('qterm.remoteEditorFocus')

export function provideRemoteEditorFocus(): RemoteEditorFocusRegistry {
  const active = ref<(() => void) | null>(null)
  const hasActive = ref(false)
  const registry: RemoteEditorFocusRegistry = {
    hasActive,
    register(save) {
      active.value = save
      hasActive.value = true
      let released = false
      return () => {
        if (released) return
        released = true

        if (active.value === save) {
          active.value = null
          hasActive.value = false
        }
      }
    },
    requestSave() {
      const save = active.value
      if (!save) return false
      save()
      return true
    },
  }
  provide(remoteEditorFocusKey, registry)
  return registry
}

export function useRemoteEditorFocus(): RemoteEditorFocusRegistry | null {
  return inject(remoteEditorFocusKey, null)
}
