<script setup lang="ts">
import { QtermButton } from '../../components/common'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Ban, RotateCcw, Keyboard, Power, TriangleAlert } from '@lucide/vue'
import GlobalShortcutSettings from './GlobalShortcutSettings.vue'
import ShortcutKeycaps from './ShortcutKeycaps.vue'
import {
  DEFAULT_SHORTCUTS,
  FIXED_SHORTCUT_ACTIONS,
  SHORTCUT_ACTION_GROUPS,
  UNBOUND_SHORTCUT,
  shortcutConflicts,
  shortcutDisplay,
  shortcutFromKeyboardEvent,
  shortcutTokenForAction,
  type ShortcutAction,
  type ShortcutBindings,
  type ShortcutConflict,
} from './shortcutStore'

import { isMacPlatform, refreshIsMacPlatform } from '@/lib/platform.ts'
import { useI18n } from '@/locales'

const props = withDefaults(
  defineProps<{
    shortcuts?: ShortcutBindings
    globalShortcut?: string
    globalShortcutEffective?: string
    globalShortcutError?: string
  }>(),
  {
    shortcuts: () => ({ ...DEFAULT_SHORTCUTS }),
    globalShortcut: '',
    globalShortcutEffective: '',
    globalShortcutError: '',
  },
)

const emit = defineEmits<{
  'update:shortcut': [action: ShortcutAction, binding: string]
  'update:globalShortcut': [binding: string]
}>()

const recording = ref<ShortcutAction | null>(null)
const recordingError = ref('')

const isMac = isMacPlatform
const { t } = useI18n()

const currentShortcuts = computed(() => props.shortcuts ?? DEFAULT_SHORTCUTS)

function display(binding: string): string {
  return shortcutDisplay(binding, isMac.value)
}

function displayTokens(binding: string): string[] {
  return display(binding).split(' + ')
}

function isBound(action: ShortcutAction): boolean {
  return (currentShortcuts.value[action] ?? UNBOUND_SHORTCUT) !== ''
}

function bindingLabel(action: ShortcutAction): string {
  return isBound(action) ? display(currentShortcuts.value[action]) : t('settings.shortcuts.binding.unbound')
}

function bindingTokens(action: ShortcutAction): string[] {
  return isBound(action) ? displayTokens(currentShortcuts.value[action]) : []
}

function isFixedAction(action: ShortcutAction): boolean {
  return FIXED_SHORTCUT_ACTIONS.has(action)
}

function startRecording(action: ShortcutAction): void {
  if (isFixedAction(action)) return

  if (globalShortcutRecording.value) {
    globalShortcutRecording.value = false
    globalShortcutControl.value?.cancelRecording()
  }
  recordingError.value = ''
  recording.value = action
}

function toggleShortcut(action: ShortcutAction): void {
  if (isFixedAction(action)) return
  const bound = isBound(action)
  if (recording.value === action) cancelRecording()
  recordingError.value = ''
  emit('update:shortcut', action, bound ? UNBOUND_SHORTCUT : DEFAULT_SHORTCUTS[action])
}

function cancelRecording(): void {
  recording.value = null
  recordingError.value = ''
}

function resetShortcut(action: ShortcutAction): void {
  if (isFixedAction(action)) return
  recordingError.value = ''
  emit('update:shortcut', action, DEFAULT_SHORTCUTS[action])
}

function recordedBinding(action: ShortcutAction, actualBinding: string): string | null {
  const token = shortcutTokenForAction(action)
  if (!token) return actualBinding

  const separator = actualBinding.lastIndexOf('+')
  if (separator <= 0) return null
  return `${actualBinding.slice(0, separator)}+${token}`
}

function handleRecordingKeydown(event: KeyboardEvent): void {
  const action = recording.value
  if (!action) return
  event.preventDefault()
  event.stopPropagation()
  if (event.key === 'Escape') {
    cancelRecording()
    return
  }
  const actualBinding = shortcutFromKeyboardEvent(event, isMac.value)
  if (!actualBinding) {
    recordingError.value = t('settings.shortcuts.error.modifierRequired')
    return
  }
  const binding = recordedBinding(action, actualBinding)
  if (!binding) {
    recordingError.value =
      shortcutTokenForAction(action) === 'TabNumber'
        ? t('settings.shortcuts.error.switchTabNumber')
        : t('settings.shortcuts.error.modifierRequired')
    return
  }
  const conflict = Object.entries(currentShortcuts.value).find(([key, value]) => key !== action && value === binding)
  if (conflict) {
    recordingError.value = t('settings.shortcuts.error.conflict', { name: labelFor(conflict[0] as ShortcutAction) })
    return
  }
  emit('update:shortcut', action, binding)
  recording.value = null
  recordingError.value = ''
}

function labelFor(action: ShortcutAction): string {
  return t(`settings.shortcuts.action.${action}`)
}

function groupLabel(groupId: string): string {
  return t(`settings.shortcuts.group.${groupId}`)
}

function groupDescription(groupId: string): string {
  return t(`settings.shortcuts.group.${groupId}Description`)
}

function actionLabel(action: ShortcutAction): string {
  return t(`settings.shortcuts.action.${action}`)
}

function actionDescription(action: ShortcutAction): string {
  return t(`settings.shortcuts.action.${action}Description`)
}

function tokenHint(action: ShortcutAction): string {
  const token = shortcutTokenForAction(action)
  if (!token) return ''
  return shortcutDisplay(token, isMac.value)
}

const conflicts = computed(() => shortcutConflicts(currentShortcuts.value))

const globalShortcutRecording = ref(false)
const globalShortcutControl = ref<{ cancelRecording: () => void } | null>(null)

const acknowledgedGlobalShortcut = ref(props.globalShortcut ?? '')
let globalShortcutQosTimer: number | null = null

const globalShortcutSettling = computed(
  () => Boolean(props.globalShortcut) && props.globalShortcut !== acknowledgedGlobalShortcut.value,
)

const globalShortcutRolledBack = computed(
  () =>
    !globalShortcutSettling.value &&
    Boolean(props.globalShortcutEffective) &&
    Boolean(props.globalShortcut) &&
    props.globalShortcutEffective !== props.globalShortcut,
)

const GLOBAL_SHORTCUT_SETTLE_MS = 1200

function scheduleGlobalShortcutSettle(): void {
  if (globalShortcutQosTimer !== null) window.clearTimeout(globalShortcutQosTimer)
  globalShortcutQosTimer = window.setTimeout(() => {
    globalShortcutQosTimer = null
    acknowledgedGlobalShortcut.value = props.globalShortcut ?? ''
  }, GLOBAL_SHORTCUT_SETTLE_MS)
}

watch(
  () => props.globalShortcutEffective,
  (effective) => {
    if (effective && effective === props.globalShortcut) {
      if (globalShortcutQosTimer !== null) {
        window.clearTimeout(globalShortcutQosTimer)
        globalShortcutQosTimer = null
      }
      acknowledgedGlobalShortcut.value = effective
      return
    }
    scheduleGlobalShortcutSettle()
  },
)

watch(
  () => props.globalShortcut,
  (value, previous) => {
    if (value && value !== previous) scheduleGlobalShortcutSettle()
  },
)

function onGlobalShortcutRecordingChange(value: boolean): void {
  globalShortcutRecording.value = value
  if (value && recording.value) cancelRecording()
}

function conflictFor(action: ShortcutAction): ShortcutConflict | undefined {
  return conflicts.value[action]
}

function conflictMessage(action: ShortcutAction): string {
  const conflict = conflictFor(action)
  if (!conflict) return ''
  if (conflict.kind === 'duplicate') {
    const names = (conflict.peers ?? []).map((peer) => labelFor(peer)).join(' / ')
    return t('settings.shortcuts.conflict.duplicate', { names })
  }
  return conflict.reason === 'xtermMeta'
    ? t('settings.shortcuts.conflict.xtermMeta')
    : t('settings.shortcuts.conflict.browserReserved')
}

onMounted(() => {
  refreshIsMacPlatform()
  window.addEventListener('keydown', handleRecordingKeydown, true)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleRecordingKeydown, true)
  if (globalShortcutQosTimer !== null) window.clearTimeout(globalShortcutQosTimer)
})
</script>

<template>
  <section
    class="settings-panel shortcuts-settings flex flex-col gap-4"
    :aria-label="t('settings.shortcuts.panel.aria')"
  >
    <section v-for="group in SHORTCUT_ACTION_GROUPS" :key="group.id" class="shortcuts-group gap-4 flex flex-col">
      <header class="settings-section-header shortcuts-group-header mt-2">
        <div>
          <h3 class="m-0 text-sm">{{ groupLabel(group.id) }}</h3>
          <p class="mt-0.75 mb-0 text-xs">
            {{ groupDescription(group.id) }}
          </p>
        </div>
        <span class="shortcuts-count text-xs">{{ group.items.length }}</span>
      </header>
      <div
        class="settings-card shortcuts-card border border-solid border-qt-border rounded-[8px] [background:color-mix(in_srgb,_var(--qt-panel)_32%,_transparent)] [-webkit-backdrop-filter:blur(16px)_saturate(135%)] [backdrop-filter:blur(16px)_saturate(135%)]"
      >
        <div
          v-for="item in group.items"
          :key="item.id"
          class="settings-card-row shortcut-row flex-cb gap-[16px] pt-[12px] pr-[14px] pb-[12px] pl-[14px] border-b border-b-qt-divider"
          :class="{ 'shortcut-row-conflicted': conflictFor(item.id) }"
          :data-shortcut-conflict="conflictFor(item.id)?.kind"
        >
          <div class="shortcut-description">
            <strong class="text-sm">{{ actionLabel(item.id) }}</strong>

            <small
              class="min-h-8 text-xs"
              :class="{ 'shortcut-description-error': recording === item.id && recordingError }"
            >
              <template v-if="recording === item.id && recordingError">{{ recordingError }}</template>
              <template v-else-if="recording === item.id && tokenHint(item.id)">{{
                t('settings.shortcuts.recording.tokenHint', { token: tokenHint(item.id) })
              }}</template>
              <template v-else-if="recording === item.id">{{ t('settings.shortcuts.recording.hint') }}</template>
              <template v-else-if="isFixedAction(item.id)">{{ t('settings.shortcuts.fixedHint') }}</template>
              <template v-else-if="conflictFor(item.id)">
                <span class="shortcut-conflict-message text-[var(--destructive,#dc2626)]">
                  <TriangleAlert
                    :size="14"
                    class="mr-1 inline-block size-3.5 shrink-0 align-[-2px]"
                    aria-hidden="true"
                  />
                  {{ conflictMessage(item.id) }}
                </span>
              </template>
              <template v-else>{{ actionDescription(item.id) }}</template>
            </small>
          </div>
          <div class="shortcut-actions">
            <span
              v-if="conflictFor(item.id)"
              class="shortcut-deactivated-badge rounded border border-[var(--destructive,#dc2626)] px-1.5 py-px text-xs text-[var(--destructive,#dc2626)]"
              >{{ t('settings.shortcuts.conflict.deactivated') }}</span
            >
            <div
              class="shortcut-key-display"
              :aria-label="
                t('settings.shortcuts.binding.aria', {
                  name: actionLabel(item.id),
                  binding: bindingLabel(item.id),
                })
              "
            >
              <span v-if="!isBound(item.id)" class="text-xs text-qt-muted">{{
                t('settings.shortcuts.binding.unbound')
              }}</span>
              <ShortcutKeycaps :tokens="bindingTokens(item.id)" />
            </div>
            <QtermButton
              variant="outline"
              size="xs"
              class="shortcut-record-button min-w-[112px] transition"
              :class="{ recording: recording === item.id }"
              type="button"
              :disabled="isFixedAction(item.id)"
              :data-shortcut-capture="recording === item.id ? 'true' : undefined"
              :data-shortcut-fixed="isFixedAction(item.id) ? 'true' : undefined"
              @click="recording === item.id ? cancelRecording() : startRecording(item.id)"
            >
              <Keyboard :size="14" aria-hidden="true" />
              {{
                recording === item.id
                  ? t('settings.shortcuts.recording.active')
                  : t('settings.shortcuts.recording.start')
              }}
            </QtermButton>
            <QtermButton
              variant="ghost"
              size="icon-xs"
              type="button"
              :disabled="isFixedAction(item.id)"
              :aria-label="t('settings.shortcuts.action.reset', { name: actionLabel(item.id) })"
              :title="t('settings.shortcuts.action.resetTitle')"
              data-shortcut-reset
              @click="resetShortcut(item.id)"
            >
              <RotateCcw :size="14" aria-hidden="true" />
            </QtermButton>
            <QtermButton
              v-if="!isFixedAction(item.id)"
              variant="ghost"
              size="icon-xs"
              type="button"
              :aria-label="
                isBound(item.id)
                  ? t('settings.shortcuts.action.disable', { name: actionLabel(item.id) })
                  : t('settings.shortcuts.action.enable', { name: actionLabel(item.id) })
              "
              :title="
                isBound(item.id)
                  ? t('settings.shortcuts.action.disableTitle')
                  : t('settings.shortcuts.action.enableTitle')
              "
              :data-shortcut-disable="isBound(item.id) ? 'true' : undefined"
              :data-shortcut-enable="isBound(item.id) ? undefined : 'true'"
              @click="toggleShortcut(item.id)"
            >
              <Ban v-if="isBound(item.id)" :size="14" aria-hidden="true" />
              <Power v-else :size="14" aria-hidden="true" />
            </QtermButton>
          </div>
        </div>
      </div>
    </section>

    <section class="shortcuts-group gap-4 flex flex-col" data-global-shortcut-group>
      <header class="settings-section-header shortcuts-group-header mt-2">
        <div>
          <h3 class="m-0 text-sm">{{ t('settings.shortcuts.group.global') }}</h3>
          <p class="mt-0.75 mb-0 text-xs">{{ t('settings.shortcuts.group.globalDescription') }}</p>
        </div>
        <span class="shortcuts-count text-xs">1</span>
      </header>
      <div
        class="settings-card shortcuts-card border border-solid border-qt-border rounded-[8px] [background:color-mix(in_srgb,_var(--qt-panel)_32%,_transparent)] [-webkit-backdrop-filter:blur(16px)_saturate(135%)] [backdrop-filter:blur(16px)_saturate(135%)]"
      >
        <div
          class="settings-card-row shortcut-row flex-cb gap-[16px] pt-[12px] pr-[14px] pb-[12px] pl-[14px] border-b border-b-qt-divider"
          data-global-shortcut-row
        >
          <div class="shortcut-description">
            <strong class="text-sm">{{ t('application.window.globalShortcut.label') }}</strong>
            <small
              class="min-h-8 text-xs"
              :class="{ 'shortcut-description-error': Boolean(props.globalShortcutError) }"
            >
              <template v-if="globalShortcutRecording">{{ t('settings.shortcuts.recording.hint') }}</template>
              <template v-else-if="props.globalShortcutError">
                <TriangleAlert :size="12" aria-hidden="true" />
                <span data-global-shortcut-error>{{ props.globalShortcutError }}</span>
              </template>
              <span v-else-if="globalShortcutRolledBack" data-global-shortcut-rollback>
                {{ t('application.window.globalShortcut.rolledBack', { binding: props.globalShortcutEffective }) }}
              </span>
              <template v-else>
                {{ t('application.window.globalShortcut.help') }}
              </template>
            </small>
          </div>
          <GlobalShortcutSettings
            ref="globalShortcutControl"
            :binding="props.globalShortcut ?? ''"
            :name="t('application.window.globalShortcut.label')"
            @update:binding="emit('update:globalShortcut', $event)"
            @recording-change="onGlobalShortcutRecordingChange"
          />
        </div>
      </div>
    </section>
  </section>
</template>

<style scoped>
.shortcuts-group-header {
  align-items: center;
  display: flex;
  justify-content: space-between;
}

.shortcuts-group:first-of-type .shortcuts-group-header {
  margin-top: 0;
}

.shortcuts-count {
  align-items: center;
  border: 1px solid var(--qt-border);
  border-radius: 999px;
  color: var(--qt-muted);
  display: inline-flex;
  height: 22px;
  justify-content: center;
  min-width: 22px;
  padding: 0 6px;
}

.shortcut-row {
  align-items: center;
  gap: 20px;
}

.shortcut-description {
  min-width: 0;
}

.shortcut-description small {
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  overflow: hidden;

  line-height: 16px;
}

.shortcut-description-error {
  color: var(--destructive, #dc2626);
}

.shortcut-row-conflicted {
  background: color-mix(in oklab, var(--destructive, #dc2626) 6%, transparent);
  box-shadow: inset 2px 0 0 var(--destructive, #dc2626);
}

.shortcut-actions {
  align-items: center;
  display: flex;
  flex: 0 0 auto;
  gap: 8px;
}

.shortcut-key-display {
  align-items: center;
  display: flex;
  gap: 4px;
  min-width: 145px;
  justify-content: flex-end;
}

.shortcut-key-display kbd {
  border: 1px solid var(--qt-border);
  border-radius: 5px;
  color: var(--qt-text);
  font-family: inherit;
  line-height: 24px;
  min-width: 26px;
  padding: 0 7px;
  text-align: center;
}

.shortcut-record-button {
  min-width: 112px;
}

.shortcut-record-button.recording {
  background-color: color-mix(in srgb, var(--qt-accent) 14%, transparent) !important;
  border-color: var(--qt-accent) !important;
  color: var(--qt-accent) !important;
}

.shortcut-recording-hint,
.shortcut-recording-error {
  margin: 0;
}

.shortcut-recording-hint {
  color: var(--qt-muted);
}

.shortcut-recording-error {
  color: var(--destructive, #dc2626);
}

@media (max-width: 860px) {
  .shortcut-row {
    align-items: flex-start;
    flex-direction: column;
    gap: 10px;
  }

  .shortcut-actions {
    align-self: stretch;
    justify-content: flex-end;
    width: 100%;
  }
}
</style>
