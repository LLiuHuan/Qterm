import { computed, onBeforeUnmount, onMounted, ref, watch, type MaybeRefOrGetter, toValue } from 'vue'
import {
  createTerminalControlGateway,
  type TerminalControlGateway,
  type TerminalSyncRequest,
} from '../../../services/terminalControl'
import { whenWailsRuntime } from '../../../services/runtime'
import { translate } from '../../../locales'
import { decodeAppError, translateAppError } from '../../../services/errors'
import type { BatchLogEntry, BatchSendRequest, BatchTargetSnapshot, BatchTaskView, SyncGroupView } from '../types'

export function useTerminalControl(
  sessionIds: MaybeRefOrGetter<string[]>,
  gateway: TerminalControlGateway = createTerminalControlGateway(),
) {
  const group = ref<SyncGroupView | null>(null)
  const task = ref<BatchTaskView | null>(null)

  const log = ref<BatchLogEntry[]>([])
  const eligibleIds = ref<string[]>([])
  const error = ref('')
  const busy = ref(false)
  const pickerOpen = ref(false)
  const pickerMode = ref<'create' | 'manage'>('create')
  const sourceId = ref('')
  const batchOpen = ref(false)
  const restoreActive = ref(false)
  let editingGroupId = ''
  let editingRevision = 0
  let alive = true
  let readVersion = 0
  let timer: ReturnType<typeof setTimeout> | undefined
  let cancelRuntime = () => {}

  const running = computed(() => task.value?.status === 'running')
  const fail = (cause?: unknown) => {
    const appError = decodeAppError(cause)
    error.value = appError ? translateAppError(appError) : translate('terminalControl.error')
  }

  async function refresh(): Promise<void> {
    const version = ++readVersion
    try {
      const state = await gateway.get()
      if (!alive || busy.value || version !== readVersion) return
      group.value = state.group
      task.value = state.task
      if (state.error) error.value = translate('terminalControl.error')
      eligibleIds.value = state.sessions.map((session) => session.id).filter((id) => toValue(sessionIds).includes(id))
    } catch (cause) {
      if (alive) fail(cause)
    }
  }

  async function mutate(request: TerminalSyncRequest): Promise<boolean> {
    if (busy.value) return false
    busy.value = true
    ++readVersion
    error.value = ''
    try {
      const state = await gateway.sync(request)
      if (!alive) return false
      group.value = state.group
      task.value = state.task
      return true
    } catch (cause) {
      fail(cause)
      return false
    } finally {
      busy.value = false
    }
  }

  async function action(action: TerminalSyncRequest['action'] | 'manage', sessionId = ''): Promise<void> {
    if (action === 'create' || action === 'manage') {
      sourceId.value = sessionId
      pickerMode.value = action
      restoreActive.value = action === 'manage' && group.value?.status === 'active'
      editingGroupId = group.value?.id ?? ''
      if (
        restoreActive.value &&
        !(await mutate({ action: 'pause', sessionIds: [], revision: group.value?.revision ?? 0 }))
      )
        return
      editingRevision = group.value?.revision ?? 0
      pickerOpen.value = true
      return
    }
    await mutate({ action, sessionIds: sessionId ? [sessionId] : [], revision: group.value?.revision ?? 0 })
  }

  async function applyMembers(ids: string[]): Promise<void> {
    const manage = pickerMode.value === 'manage'
    if (manage && group.value?.id !== editingGroupId) {
      fail()
      return
    }
    if (!(await mutate({ action: manage ? 'members' : 'create', sessionIds: ids, revision: editingRevision }))) return
    if (manage && restoreActive.value && group.value?.status === 'paused') {
      await mutate({ action: 'resume', sessionIds: [], revision: group.value.revision })
    }
    pickerOpen.value = false
    restoreActive.value = false
  }

  async function cancelPicker(): Promise<void> {
    pickerOpen.value = false

    if (restoreActive.value && group.value?.id === editingGroupId && group.value.revision === editingRevision) {
      await mutate({ action: 'resume', sessionIds: [], revision: editingRevision })
    }
    restoreActive.value = false
  }

  function openBatch(sessionId: string): void {
    sourceId.value = sessionId
    batchOpen.value = true
  }

  async function startBatch(request: BatchSendRequest, targets: BatchTargetSnapshot[] = []): Promise<void> {
    if (busy.value || running.value) return
    busy.value = true
    ++readVersion
    error.value = ''
    try {
      const started = await gateway.start(request)
      task.value = started
      if (started?.id) {
        log.value = [
          {
            id: started.id,
            content: request.content,
            sendMode: request.sendMode,
            repeatCount: request.repeatCount,
            intervalMs: request.intervalMs,
            targets: targets.length
              ? targets.map((target) => ({ ...target }))
              : request.sessionIds.map((id) => ({ id, title: id })),
            at: Date.now(),
            status: started.status,
            results: started.results ?? [],
            completedRounds: started.completedRounds ?? 0,
          },
          ...log.value.filter((entry) => entry.id !== started.id),
        ].slice(0, 20)
      }
    } catch (cause) {
      fail(cause)
    } finally {
      busy.value = false
    }
    await refresh()
  }

  async function cancelBatch(): Promise<void> {
    if (!task.value || busy.value) return
    busy.value = true
    ++readVersion
    try {
      task.value = await gateway.cancel(task.value.id)
    } catch (cause) {
      fail(cause)
    } finally {
      busy.value = false
    }
  }

  watch(task, (value) => {
    if (!value) return
    const entry = log.value.find((item) => item.id === value.id)
    if (!entry) return
    entry.status = value.status
    entry.results = value.results
    entry.completedRounds = value.completedRounds ?? 0
  })

  async function poll(): Promise<void> {
    await refresh()
    if (alive) timer = setTimeout(() => void poll(), running.value || group.value ? 250 : 1000)
  }
  onMounted(() => {
    cancelRuntime = whenWailsRuntime(() => void refresh())
    void poll()
  })
  onBeforeUnmount(() => {
    alive = false
    ++readVersion
    clearTimeout(timer)
    cancelRuntime()
  })
  return {
    resumeAfterManage: restoreActive,
    group,
    task,
    log,
    eligibleIds,
    error,
    busy,
    pickerOpen,
    pickerMode,
    sourceId,
    batchOpen,
    action,
    applyMembers,
    cancelPicker,
    openBatch,
    startBatch,
    cancelBatch,
    refresh,
  }
}
