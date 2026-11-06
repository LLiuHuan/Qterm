<script setup lang="ts">
import {
  IconButton,
  QtermButton,
  QtermFormMessage,
  QtermHint,
  QtermInput,
  QtermNumberField,
  QtermSecretInput,
  QtermSwitch,
  QtermTagsInput,
  QtermTextarea,
} from '../common'
import { SegmentedTabs } from '../common'
import { ChevronDown, Plus } from '@lucide/vue'
import { PopoverContent, PopoverPortal, PopoverRoot, PopoverTrigger } from 'reka-ui'
import HostIconPicker from './HostIconPicker.vue'
import CredentialCreateDialog from './CredentialCreateDialog.vue'
import CredentialQuickCreateMenu from './CredentialQuickCreateMenu.vue'
import ProxyEditorDialog from './ProxyEditorDialog.vue'
import ProxyQuickCreateButton from './ProxyQuickCreateButton.vue'
import ColorTagField from '../common/form-controls/ColorTagField.vue'
import {
  QtermDialog,
  QtermDialogContent,
  QtermDialogFooter,
  QtermDialogHeader,
  dialogThemeContextKey,
} from '../common/dialog'
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
  Label,
  ScrollArea,
} from '../common/control-primitives'
import {
  QtermCombobox,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  type QtermComboboxOption,
} from '../common/form-controls'
import type { OtpEntry } from '../../services/otp'
import { defaultTerminalTypeFor } from '@/services/hosts.ts'
import { testRouteDraft, type RouteTestHopView } from '../../services/hostRoute'
import { decodeAppError, translateErrorKey } from '../../services/errors'
import { hostIconPickerOptions } from '../../features/hosts/builtinIcons'
import {
  HOST_PROTOCOLS,
  hostProtocolAuthModes,
  hostProtocolDefinition,
  normalizeVncTransport,
  shouldAdoptDefaultPort,
  vncUsesWebSocket,
  vncWsPathError,
  vncWsPathErrorKey,
  type HostEditorDraft,
  type HostFormSection,
} from '../../features/hosts/hostProtocols'
import {
  sortCredentialsByCreatedAtDesc,
  type CredentialGateway,
  type CredentialRecord,
  type CredentialVaultStatus,
} from '../../services/credentials'
import {
  normalizeProxyMode,
  type GroupRecord,
  type HostDraft,
  type HostProxyMode,
  type HostRecord,
} from '@/services/hosts.ts'
import { listSerialPorts } from '../../services/serialSession'
import type { ProxyGateway, ProxyRecord } from '../../services/proxies'
import type { HostAuthMode } from '../../types/qterm'
import { computed, inject, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from '@/locales'

const dialogTheme = inject(dialogThemeContextKey, null)

const UNGROUPED_VALUE = '__qterm_ungrouped__'

const credentialOptions = computed<QtermComboboxOption[]>(() => {
  const options: QtermComboboxOption[] = []
  const current = props.draft.credentialId
  if (current && !props.credentials.some((item) => item.id === current)) {
    options.push({ value: current, label: t('shell.hostEditor.currentCredential') })
  }

  for (const credential of sortCredentialsByCreatedAtDesc(props.credentials)) {
    options.push({ value: credential.id, label: `${credential.name} · ${props.credentialLabel(credential)}` })
  }
  return options
})

const groupOptions = computed<QtermComboboxOption[]>(() => {
  const sorted = [...props.groups].sort((left, right) => {
    const delta = Date.parse(right.createdAt || '') - Date.parse(left.createdAt || '')
    if (Number.isFinite(delta) && delta !== 0) return delta
    return left.name.localeCompare(right.name)
  })
  return [
    { value: UNGROUPED_VALUE, label: t('shell.hostEditor.ungrouped') },
    ...sorted.map((group) => ({ value: group.name, label: group.name })),
  ]
})

const props = defineProps<{
  open: boolean
  editing: boolean
  persistent: boolean
  draft: HostEditorDraft
  groups: GroupRecord[]

  hosts?: HostRecord[]

  proxies?: ProxyRecord[]

  proxyGateway?: ProxyGateway

  editingHostId?: string
  authMode: HostAuthMode
  directSecret: string
  credentials: CredentialRecord[]
  otpEntries?: OtpEntry[]

  credentialGateway?: CredentialGateway
  vaultStatus: CredentialVaultStatus | null
  credentialsLoading: boolean
  credentialsError: string
  formError: string
  testingConnection: boolean

  detectingIcon?: boolean
  saving: boolean
  credentialLabel: (credential: CredentialRecord) => string

  layout?: 'modal' | 'page'
}>()

const emit = defineEmits<{
  close: []
  submit: []
  testConnection: []
  detectIcon: []
  openSecurity: []

  createGroup: []

  credentialCreated: [credentialId: string]

  proxyCreated: [proxyId: string]
  updateDraft: [patch: Partial<HostEditorDraft>]
  updateAuthMode: [mode: HostAuthMode]
  updatePasswordSource: [source: 'prompt' | 'saved']
  updateSecret: [value: string]
}>()
const { t } = useI18n()
const advancedSections = ref<string[]>([])
const autoSyncedName = ref('')

const pageLayout = computed(() => props.layout === 'page')

function handlePageEscape(event: KeyboardEvent): void {
  if (event.key !== 'Escape' || event.defaultPrevented) return
  if (
    document.querySelector(
      '[role="dialog"][data-state="open"], [data-slot="popover-content"][data-state="open"], [data-slot="select-content"][data-state="open"]',
    )
  ) {
    return
  }
  emit('close')
}

watch(
  [() => props.open, pageLayout],
  ([open, page]) => {
    if (open && page) document.addEventListener('keydown', handlePageEscape)
    else document.removeEventListener('keydown', handlePageEscape)
  },
  { immediate: true },
)

onBeforeUnmount(() => document.removeEventListener('keydown', handlePageEscape))

const credentialCreateOpen = ref(false)
const credentialCreateKind = ref<'password' | 'privateKey' | 'sshAgent'>('password')
const credentialCreateKeyMode = ref<'generate' | 'upload' | 'manual'>('generate')

const credentialCreateDisabled = computed(
  () => !props.credentialGateway || props.credentialsLoading || props.vaultStatus?.state !== 'unlocked',
)

function openCredentialCreate(
  kind: 'password' | 'privateKey' | 'sshAgent',
  keyMode?: 'generate' | 'upload' | 'manual',
): void {
  if (credentialCreateDisabled.value) return
  credentialCreateKind.value = kind
  credentialCreateKeyMode.value = keyMode ?? 'generate'
  credentialCreateOpen.value = true
}

function handleCredentialCreated(credentialId: string): void {
  credentialCreateOpen.value = false
  emit('credentialCreated', credentialId)
}

const proxyCreateOpen = ref(false)

function openProxyCreate(): void {
  if (!props.proxyGateway) return
  proxyCreateOpen.value = true
}

function handleProxyCreated(proxy: ProxyRecord): void {
  proxyCreateOpen.value = false
  emit('proxyCreated', proxy.id)
}

watch(
  () => props.open,
  (open) => {
    if (open) {
      advancedSections.value = []
      autoSyncedName.value = ''
    } else {
      credentialCreateOpen.value = false
      proxyCreateOpen.value = false
    }
  },
)

const serialBaudOpen = ref(false)

function chooseBaudPreset(value: number): void {
  updateDraft({ serialBaud: value })
  serialBaudOpen.value = false
}

function applyCustomBaud(value: number): void {
  if (!Number.isFinite(value) || value < 1 || value > 4294967295) return
  updateDraft({ serialBaud: value })
  serialBaudOpen.value = false
}

const serialBaudPresets = [9600, 19200, 38400, 57600, 74880, 115200, 230400, 250000, 460800, 921600]

const serialDataBitsOptions = [
  { value: '5', label: '5' },
  { value: '6', label: '6' },
  { value: '7', label: '7' },
  { value: '8', label: '8' },
]

const serialParityOptions = computed(() => [
  { value: 'none', label: t('shell.hostEditor.serialParityNone') },
  { value: 'odd', label: t('shell.hostEditor.serialParityOdd') },
  { value: 'even', label: t('shell.hostEditor.serialParityEven') },
  { value: 'mark', label: t('shell.hostEditor.serialParityMark') },
  { value: 'space', label: t('shell.hostEditor.serialParitySpace') },
])

const serialStopBitsOptions = [
  { value: '1', label: '1' },
  { value: '1.5', label: '1.5' },
  { value: '2', label: '2' },
]

const serialPorts = ref<string[]>([])
const serialDeviceOptions = computed(() => serialPorts.value.map((name) => ({ value: name, label: name })))

async function refreshSerialPorts(): Promise<void> {
  for (let attempt = 0; attempt < 3; attempt += 1) {
    const ports = await listSerialPorts()
    if (ports.length || attempt === 2) {
      serialPorts.value = ports.map((port) => port.name)
      return
    }
    await new Promise((resolve) => setTimeout(resolve, 300))
  }
}

watch(
  () => [props.open, props.draft.protocol] as const,
  ([open, protocol]) => {
    if (open && protocol === 'serial') void refreshSerialPorts()
  },
  { immediate: true },
)

const protocolOptions = computed(() =>
  HOST_PROTOCOLS.map((item) => ({
    value: item.id,
    label: item.labelKey ? t(item.labelKey) : item.label,
    disabled: !item.enabled,
    ...(item.disabledHintKey ? { hint: t(item.disabledHintKey) } : {}),
  })),
)

const protocolDefinition = computed(() => hostProtocolDefinition(props.draft.protocol))

const sshHost = computed(() => (props.draft.protocol ?? 'ssh') === 'ssh')
const otpOptions = computed<QtermComboboxOption[]>(() => {
  const options: QtermComboboxOption[] = [{ value: '__manual__', label: t('shell.hostEditor.twoFactor.manual') }]
  options.push(
    ...(props.otpEntries ?? [])
      .filter((entry) => entry.type === 'totp')
      .map((entry) => ({
        value: entry.id,
        label: [entry.name, entry.username].filter(Boolean).join(' · '),
      })),
  )
  if (props.draft.sshOtpEntryId && !options.some((item) => item.value === props.draft.sshOtpEntryId))
    options.push({
      value: props.draft.sshOtpEntryId,
      label: t('shell.hostEditor.twoFactor.unavailable'),
      disabled: true,
    })
  return options
})
function updateTwoFactor(enabled: boolean): void {
  updateDraft(
    enabled ? { sshTwoFactorEnabled: true } : { sshTwoFactorEnabled: false, sshOtpEntryId: '', sshOtpAutoFill: false },
  )
}
function updateOtpSelection(value: string | number): void {
  const id = value === '__manual__' ? '' : String(value)
  updateDraft({ sshOtpEntryId: id, sshOtpAutoFill: id ? props.draft.sshOtpAutoFill : false })
}

const advancedDescriptionKey = computed(() => {
  switch (props.draft.protocol) {
    case 'telnet':
      return 'shell.hostEditor.advanced.descriptionTelnet'
    case 'rdp':
      return 'shell.hostEditor.advanced.descriptionRdp'
    case 'vnc':
      return 'shell.hostEditor.advanced.descriptionVnc'
    case 'serial':
      return 'shell.hostEditor.advanced.descriptionSerial'
    default:
      return 'shell.hostEditor.advanced.description'
  }
})

const protocolHint = computed(() => (protocolDefinition.value.hintKey ? t(protocolDefinition.value.hintKey) : ''))
const usernameHint = computed(() =>
  protocolDefinition.value.usernameHintKey ? t(protocolDefinition.value.usernameHintKey) : '',
)

function shows(section: HostFormSection): boolean {
  return protocolDefinition.value.sections.includes(section)
}

const credentialsRowSingle = computed(
  () =>
    !shows('identity-name') ||
    !(
      shows('auth') &&
      (props.authMode === 'password' ? props.draft.passwordSource === 'saved' : props.authMode === 'credential')
    ),
)

const authOptions = computed(() => {
  const labels: Record<string, string> = {
    none: t('shell.hostEditor.noPasswordAuth'),
    password: t('shell.hostEditor.passwordAuth'),
    credential: t('shell.hostEditor.credentialAuth'),
  }
  return hostProtocolAuthModes(props.draft.protocol).map((mode) => ({
    value: mode,
    label: labels[mode] ?? mode,
  }))
})

const passwordSourceOptions = computed(() => [
  { value: 'prompt', label: t('shell.hostEditor.passwordPrompt') },
  { value: 'saved', label: t('shell.hostEditor.passwordSaved') },
])

const rdpDisplayModeOptions = computed(() => [
  { value: 'fit', label: t('shell.hostEditor.display.modeFit') },
  { value: 'fixed', label: t('shell.hostEditor.display.modeFixed') },
])

const vncToggleOptions = computed(() => [
  { value: 'on', label: t('shell.hostEditor.vnc.toggleOn') },
  { value: 'off', label: t('shell.hostEditor.vnc.toggleOff') },
])

const vncScaleOptions = computed(() => [
  { value: 'fit', label: t('shell.hostEditor.vnc.scaleFit') },
  { value: 'actual', label: t('shell.hostEditor.vnc.scaleActual') },
])

const vncAutoReconnectEnabled = computed(() => props.draft.vncAutoReconnect === true)

const vncTransport = computed(() => normalizeVncTransport(props.draft.vncTransport))
const vncWssTransport = computed(() => vncUsesWebSocket(props.draft.vncTransport))
const vncTransportOptions = computed(() => [
  { value: 'tcp', label: t('shell.hostEditor.vnc.transportTcp') },
  { value: 'wss', label: t('shell.hostEditor.vnc.transportWss') },
])

const vncWsPathErrorText = computed(() => {
  if (!vncWssTransport.value) return ''
  const error = vncWsPathError(props.draft.vncWsPath)
  return error ? t(vncWsPathErrorKey(error)) : ''
})

const vncWssHintsVisible = computed(() => props.draft.protocol === 'vnc' && vncWssTransport.value)

const vncRfbPasswordMode = computed(() => (props.draft.vncRfbPasswordMode === 'same' ? 'same' : 'auto'))
const vncRfbPasswordModeOptions = computed(() => [
  { value: 'auto', label: t('shell.hostEditor.vnc.rfbPasswordAuto'), disabled: !vncWssTransport.value },
  { value: 'same', label: t('shell.hostEditor.vnc.rfbPasswordSame'), disabled: !vncWssTransport.value },
])

const rdpClipboardModeOptions = computed(() => [
  { value: 'both', label: t('shell.hostEditor.clipboard.modeBoth') },
  { value: 'local-to-remote', label: t('shell.hostEditor.clipboard.modeLocalToRemote') },
  { value: 'remote-to-local', label: t('shell.hostEditor.clipboard.modeRemoteToLocal') },
  { value: 'off', label: t('shell.hostEditor.clipboard.modeOff') },
])

const rdpClipboardContentOptions = computed(() => [
  { value: 'text', label: t('shell.hostEditor.clipboard.contentText') },
  {
    value: 'text-and-files',
    label: t('shell.hostEditor.clipboard.contentTextAndFiles'),
    disabled: true,
    hint: t('shell.hostEditor.clipboard.contentFilesUnsupported'),
  },
])

function clampRdpSize(value: unknown, fallback: number, min: number, max: number): number {
  const parsed = Number.parseInt(String(value ?? ''), 10)
  if (!Number.isFinite(parsed)) return fallback
  return Math.min(max, Math.max(min, parsed))
}

const JUMP_HOST_NONE = '__qterm_no_jump_host__'

function draftConnectionMode(): 'direct' | 'jump' | 'proxy' {
  if (props.draft.jumpHostId) return 'jump'

  if ((props.draft.proxyId ?? '').trim() !== '') return 'proxy'
  if (normalizeProxyMode(props.draft.proxyMode) !== 'none') return 'proxy'
  return 'direct'
}

const connectionMode = ref<'direct' | 'jump' | 'proxy'>(draftConnectionMode())

watch(
  () => [props.open, props.editingHostId] as const,
  () => {
    if (props.open) connectionMode.value = draftConnectionMode()
  },
)

const connectionModeOptions = computed(() => [
  { value: 'direct', label: t('shell.hostEditor.direct') },
  { value: 'jump', label: t('shell.hostEditor.jump') },
  { value: 'proxy', label: t('shell.hostEditor.proxy') },
])

const PROXY_GROUP_DEFAULTS = {
  proxyId: '',
  proxyMode: 'none' as HostProxyMode,
  proxyHost: '',
  proxyPort: 0,
  proxyUsername: '',
  proxyPassword: '',
  clearProxyPassword: false,
  proxyPasswordConfigured: false,
}

const libraryProxies = computed<ProxyRecord[]>(() => props.proxies ?? [])

const proxyCustomSelected = ref(draftSelectsCustomProxy())

function draftSelectsCustomProxy(): boolean {
  if ((props.draft.proxyId ?? '').trim()) return false
  if (!libraryProxies.value.length) return true
  return normalizeProxyMode(props.draft.proxyMode) !== 'none'
}

watch(
  () => [props.open, props.editingHostId] as const,
  () => {
    if (props.open) proxyCustomSelected.value = draftSelectsCustomProxy()
  },
)

watch(
  () => (props.draft.proxyId ?? '').trim(),
  (proxyId) => {
    if (proxyId) proxyCustomSelected.value = false
  },
)

watch(
  () => libraryProxies.value.length,
  () => {
    if (!props.open) return
    if ((props.draft.proxyId ?? '').trim()) return
    if (normalizeProxyMode(props.draft.proxyMode) !== 'none') return
    proxyCustomSelected.value = libraryProxies.value.length === 0
  },
)

function switchConnectionMode(value: string | number): void {
  const next = String(value)
  connectionMode.value = next === 'proxy' ? 'proxy' : next === 'jump' ? 'jump' : 'direct'
  if (next === 'direct') {
    proxyCustomSelected.value = false
    updateDraft({ jumpHostId: '', ...PROXY_GROUP_DEFAULTS })
    return
  }
  if (next === 'jump') {
    proxyCustomSelected.value = false
    updateDraft({ ...PROXY_GROUP_DEFAULTS })
    return
  }

  const hasLibraryProxy = (props.draft.proxyId ?? '').trim() !== ''
  const currentProxy = normalizeProxyMode(props.draft.proxyMode)

  proxyCustomSelected.value = !hasLibraryProxy && (currentProxy !== 'none' || libraryProxies.value.length === 0)
  updateDraft({
    jumpHostId: '',
    ...(!hasLibraryProxy && currentProxy === 'none' ? { proxyMode: 'socks5' as HostProxyMode } : {}),
  })
}

function jumpChainReaches(startId: string, selfId: string, byId: Map<string, HostRecord>): boolean {
  if (!selfId) return false
  const visited = new Set<string>()
  let currentId = startId
  while (currentId) {
    if (currentId === selfId) return true
    if (visited.has(currentId)) return false
    visited.add(currentId)
    currentId = byId.get(currentId)?.jumpHostId || ''
  }
  return false
}

const routeTesting = ref(false)
const routeTestHops = ref<RouteTestHopView[]>([])
const routeTestUnavailable = ref(false)

const routeTestError = ref('')

let routeTestVersion = 0

function resetRouteTest(): void {
  routeTestVersion += 1
  routeTestHops.value = []
  routeTestUnavailable.value = false
  routeTestError.value = ''
}

watch(
  () => [
    props.open,
    props.editingHostId,
    props.draft.address,
    props.draft.port,
    props.draft.username,
    props.draft.jumpHostId,
    props.draft.proxyId,
    props.draft.proxyMode,
    props.draft.proxyHost,
    props.draft.proxyPort,
    props.draft.proxyUsername,
    props.draft.proxyPassword,
    props.draft.clearProxyPassword,
    props.draft.credentialId,
    props.authMode,
    props.directSecret,
  ],
  () => resetRouteTest(),
)

const hasDraftRoute = computed(
  () =>
    Boolean(props.draft.jumpHostId) ||
    (props.draft.proxyId ?? '').trim() !== '' ||
    normalizeProxyMode(props.draft.proxyMode) !== 'none',
)

async function runConnectionTest(): Promise<void> {
  if (props.saving || props.testingConnection || routeTesting.value) return
  if (!hasDraftRoute.value) {
    emit('testConnection')
    return
  }
  const version = (routeTestVersion += 1)
  routeTesting.value = true
  routeTestHops.value = []
  routeTestUnavailable.value = false
  routeTestError.value = ''
  try {
    const result = await testRouteDraft({
      hostId: props.editingHostId,

      protocol: props.draft.protocol || 'ssh',
      address: props.draft.address.trim(),
      port: Number(props.draft.port),
      username: props.draft.username.trim(),
      jumpHostId: props.draft.jumpHostId || '',

      proxyId: props.draft.proxyId || '',
      proxyMode: normalizeProxyMode(props.draft.proxyMode),
      proxyHost: props.draft.proxyHost || '',
      proxyPort: props.draft.proxyPort || 0,
      proxyUsername: props.draft.proxyUsername || '',
      proxyPassword: props.draft.proxyPassword || '',
      clearProxyPassword: props.draft.clearProxyPassword === true,
      credentialId: props.authMode === 'credential' ? props.draft.credentialId : '',
      kind: props.authMode === 'password' ? 'password' : '',
      secret: props.authMode === 'password' ? props.directSecret : '',
    })
    if (version !== routeTestVersion) return
    if (!result) {
      routeTestUnavailable.value = true
      return
    }
    routeTestHops.value = result.hops
  } catch (error) {
    if (version !== routeTestVersion) return
    const messageKey = decodeAppError(error)?.messageKey
    routeTestError.value = messageKey ? translateErrorKey(messageKey, t('error.unknown')) : t('error.unknown')
  } finally {
    if (version === routeTestVersion) routeTesting.value = false
  }
}

const jumpHostCandidates = computed<HostRecord[]>(() => {
  const selfId = props.editingHostId ?? ''
  const hosts = props.hosts ?? []
  const byId = new Map(hosts.map((host) => [host.id, host]))
  return hosts
    .filter((host) => host.id !== selfId)

    .filter((host) => (host.protocol || 'ssh') === 'ssh')
    .filter((host) => !jumpChainReaches(host.id, selfId, byId))
    .sort((a, b) => a.name.localeCompare(b.name))
})

const jumpHostOptions = computed<QtermComboboxOption[]>(() => [
  { value: JUMP_HOST_NONE, label: t('shell.hostEditor.jumpHostNone') },
  ...jumpHostCandidates.value.map((host) => ({
    value: host.id,

    label: `${host.name} · ${host.address}`,
  })),
])

function updateJumpHost(value: string | number): void {
  const next = String(value)
  const cleared = next === JUMP_HOST_NONE

  if (cleared) connectionMode.value = 'direct'
  updateDraft({ jumpHostId: cleared ? '' : next })
}

const proxyTypeOptions = computed(() => [
  { value: 'socks5', label: t('shell.hostEditor.proxySocks5') },
  { value: 'http', label: t('shell.hostEditor.proxyHttp') },
])

const proxyTypeValue = computed<HostProxyMode>(() => (props.draft.proxyMode === 'http' ? 'http' : 'socks5'))

const PROXY_CUSTOM = '__qterm_custom_proxy__'

const proxyTargetOptions = computed<QtermComboboxOption[]>(() => {
  const options: QtermComboboxOption[] = []
  const current = (props.draft.proxyId ?? '').trim()
  if (current && !libraryProxies.value.some((item) => item.id === current)) {
    options.push({ value: current, label: t('shell.hostEditor.proxyUnavailable') })
  }
  for (const proxy of libraryProxies.value) {
    options.push({
      value: proxy.id,

      label: `${proxy.name} · ${proxy.mode.toUpperCase()} ${proxy.host}:${proxy.port}`,
    })
  }
  options.push({ value: PROXY_CUSTOM, label: t('shell.hostEditor.proxyCustom') })
  return options
})

const proxyTargetValue = computed(() => {
  const current = (props.draft.proxyId ?? '').trim()
  if (current) return current
  return proxyCustomSelected.value ? PROXY_CUSTOM : ''
})

const selectedLibraryProxy = computed<ProxyRecord | null>(
  () => libraryProxies.value.find((item) => item.id === (props.draft.proxyId ?? '').trim()) ?? null,
)

const proxyReferenceMissing = computed(
  () => (props.draft.proxyId ?? '').trim() !== '' && selectedLibraryProxy.value === null,
)

function updateProxyTarget(value: string | number): void {
  const next = String(value)
  if (next === PROXY_CUSTOM) {
    proxyCustomSelected.value = true
    updateDraft({ proxyId: '', proxyMode: 'socks5' as HostProxyMode })
    return
  }
  proxyCustomSelected.value = false
  updateDraft({
    proxyId: next,
    proxyMode: 'none' as HostProxyMode,
    proxyHost: '',
    proxyPort: 0,
    proxyUsername: '',
    proxyPassword: '',
    clearProxyPassword: false,
    proxyPasswordConfigured: false,
  })
}

const usernamePlaceholder = computed(() => {
  if (props.draft.protocol === 'rdp') return t('shell.hostEditor.usernamePlaceholderRdp')
  if (props.draft.protocol === 'vnc') return t('shell.hostEditor.usernamePlaceholderVnc')
  return t('shell.hostEditor.usernamePlaceholder')
})

const networkDevice = computed(() => (props.draft.connectionType ?? 'standard') === 'network')

const connectionTypeOptions = computed(() => [
  { value: 'standard', label: t('shell.hostEditor.connection.typeStandard') },
  { value: 'network', label: t('shell.hostEditor.connection.typeNetwork') },
])

const terminalTypeOptions = computed(() =>
  ['vt100', 'vt200', 'xterm', 'xterm-256color', 'ansi', 'linux'].map((value) => ({ value, label: value })),
)

function updateConnectionType(value: string): void {
  const next = value === 'network' ? 'network' : 'standard'
  const previous = props.draft.connectionType ?? 'standard'
  const patch: Partial<HostDraft> = { connectionType: next }
  const current = props.draft.terminalType || defaultTerminalTypeFor(previous)
  if (current === defaultTerminalTypeFor(previous)) {
    patch.terminalType = defaultTerminalTypeFor(next)
  }
  updateDraft(patch)
}

const x11TrustOptions = computed(() => [
  { value: 'trusted', label: t('shell.hostEditor.x11.trusted') },
  { value: 'untrusted', label: t('shell.hostEditor.x11.untrusted') },
])

const backspaceModeOptions = computed(() => [
  { value: 'del', label: t('shell.hostEditor.terminal.backspaceDel') },
  { value: 'bs', label: t('shell.hostEditor.terminal.backspaceBs') },
])

const encodingOptions = computed(() => [
  { value: 'utf-8', label: 'UTF-8' },
  { value: 'gbk', label: 'GBK' },
])

const sftpShowHiddenFilesOptions = computed(() => [
  { value: 'inherit', label: t('shell.hostEditor.sftpHidden.inherit') },
  { value: 'enabled', label: t('shell.hostEditor.sftpHidden.enabled') },
  { value: 'disabled', label: t('shell.hostEditor.sftpHidden.disabled') },
])

const directoryFollowOptions = computed(() => [
  { value: 'inherit', label: t('shell.hostEditor.directoryFollow.inherit') },
  { value: 'enabled', label: t('shell.hostEditor.directoryFollow.enabled') },
  { value: 'disabled', label: t('shell.hostEditor.directoryFollow.disabled') },
])

function iconDisplayName(value: string | undefined): string {
  if (!value) return t('shell.hostEditor.iconDetect.unknown')
  return hostIconPickerOptions.find((option) => option.value === value)?.label ?? value
}

const currentIconLabel = computed(() => iconDisplayName(props.draft.icon))

function addEnvironmentVariable(): void {
  updateDraft({ environmentVariables: [...(props.draft.environmentVariables ?? []), { name: '', value: '' }] })
}

function updateEnvironmentVariable(index: number, patch: Partial<{ name: string; value: string }>): void {
  const next = (props.draft.environmentVariables ?? []).map((item, position) =>
    position === index ? { ...item, ...patch } : item,
  )
  updateDraft({ environmentVariables: next })
}

function removeEnvironmentVariable(index: number): void {
  updateDraft({
    environmentVariables: (props.draft.environmentVariables ?? []).filter((_, position) => position !== index),
  })
}

function updateDraft(patch: Partial<HostEditorDraft>): void {
  emit('updateDraft', patch)
}

function updateProxyPassword(value: string | number): void {
  const password = String(value)
  updateDraft({
    proxyPassword: password,
    ...(password ? { clearProxyPassword: false } : {}),
  })
}

function updateProtocol(value: string): void {
  const next = HOST_PROTOCOLS.find((item) => item.id === value)
  if (!next || !next.enabled) {
    return
  }
  const patch: Partial<HostDraft> = { protocol: next.id }

  if (next.id === 'serial' && !props.draft.serialBaud) {
    patch.serialBaud = 115200
  }

  if (shouldAdoptDefaultPort(props.draft.port, next)) {
    patch.port = next.defaultPort
  }
  updateDraft(patch)

  const allowedModes = hostProtocolAuthModes(next.id)
  if (!allowedModes.includes(props.authMode)) {
    emit('updateAuthMode', allowedModes.includes('password') ? 'password' : (allowedModes[0] ?? 'password'))
  }
}

function updateAddress(value: unknown): void {
  const address = String(value)
  const patch: Partial<HostDraft> = { address }
  const currentName = props.draft.name.trim()
  if (!currentName || currentName === autoSyncedName.value) {
    patch.name = address
    autoSyncedName.value = address
  }
  updateDraft(patch)
}
</script>

<template>
  <QtermDialog :open="props.open" @update:open="() => undefined">
    <component
      :is="pageLayout ? 'div' : QtermDialogContent"
      v-if="pageLayout || props.open"
      class="qterm-host-dialog"
      :class="[pageLayout ? ['qterm-dialog', 'host-editor-page', dialogTheme?.className] : undefined]"
      :style="pageLayout ? dialogTheme?.style : undefined"
      v-bind="pageLayout ? {} : { size: 'xl' }"
      @pointer-down-outside.prevent
      @interact-outside.prevent
    >
      <form :aria-label="t('shell.hostEditor.formAria')" @submit.prevent="emit('submit')">
        <QtermDialogHeader
          v-if="!pageLayout"
          :title="props.editing ? t('shell.hostEditor.editTitle') : t('shell.hostEditor.createTitle')"
          :description="
            props.persistent ? t('shell.hostEditor.persistentDescription') : t('shell.hostEditor.previewDescription')
          "
          :show-close="!pageLayout"
          @close="emit('close')"
        />

        <ScrollArea type="hover" :scroll-hide-delay="600" class="host-editor-scroll">
          <div class="dialog-body">
            <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
              <Label for="host-name"
                >{{ t('shell.hostEditor.name') }} <span class="required-mark text-qt-danger">*</span></Label
              >
              <div class="host-identity-field grid grid-cols-[auto_minmax(0,_1fr)] gap-[8px] items-center">
                <HostIconPicker
                  :model-value="props.draft.icon"
                  @update:model-value="(value) => updateDraft({ icon: value })"
                />
                <QtermInput
                  id="host-name"
                  class="qterm-reference-input"
                  :model-value="props.draft.name"
                  name="name"
                  autocomplete="off"
                  autocorrect="off"
                  autocapitalize="off"
                  spellcheck="false"
                  :placeholder="t('shell.hostEditor.namePlaceholder')"
                  autofocus
                  @update:model-value="(value) => updateDraft({ name: String(value) })"
                />
              </div>
              <QtermFormMessage variant="hint">{{ t('shell.hostEditor.iconHint') }}</QtermFormMessage>
            </div>

            <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
              <Label>{{ t('shell.hostEditor.colorTag') }}</Label>
              <ColorTagField
                :model-value="props.draft.color"
                :label="t('shell.hostEditor.colorTag')"
                :none-label="t('shell.hostEditor.colorNone')"
                :custom-label="t('shell.hostEditor.customColor')"
                :color-label="(value) => t('shell.hostEditor.colorValue', { color: value })"
                @update:model-value="(value) => updateDraft({ color: value })"
              />
              <QtermFormMessage variant="hint">{{ t('shell.hostEditor.colorHint') }}</QtermFormMessage>
            </div>

            <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
              <Label>{{ t('shell.hostEditor.protocol') }}</Label>
              <SegmentedTabs
                :model-value="props.draft.protocol || 'ssh'"
                :options="protocolOptions"
                :label="t('shell.hostEditor.connectionProtocol')"
                @update:model-value="updateProtocol"
              />
              <QtermFormMessage v-if="protocolHint" variant="hint">{{ protocolHint }}</QtermFormMessage>
            </div>

            <div v-if="shows('serial')" class="flex flex-col gap-[10px]">
              <div class="form-grid grid grid-cols-3 gap-[10px]">
                <div class="col-span-2 form-field min-w-0 grid gap-[6px] text-qt-muted">
                  <Label for="host-serial-device"
                    >{{ t('shell.hostEditor.serialDevice') }} <span class="required-mark text-qt-danger">*</span></Label
                  >
                  <QtermCombobox
                    id="host-serial-device"
                    class="w-full min-w-0"
                    :model-value="props.draft.address"
                    :options="serialDeviceOptions"
                    :placeholder="t('shell.hostEditor.serialDevicePlaceholder')"
                    :empty-text="t('shell.hostEditor.serialDeviceEmpty')"
                    menu-width="trigger"
                    menu-class="z-50"
                    :aria-label="t('shell.hostEditor.serialDevice')"
                    @update:model-value="(value) => updateAddress(String(value))"
                  />
                </div>
                <div class="form-field grid min-w-0 gap-[6px] text-qt-muted">
                  <Label>{{ t('shell.hostEditor.serialBaud') }}</Label>
                  <PopoverRoot v-model:open="serialBaudOpen">
                    <PopoverTrigger as-child>
                      <QtermButton variant="outline" class="w-full justify-between" data-serial-baud>
                        <span>{{ props.draft.serialBaud ?? 115200 }}</span>
                        <ChevronDown :size="14" aria-hidden="true" />
                      </QtermButton>
                    </PopoverTrigger>
                    <PopoverPortal>
                      <PopoverContent align="end" class="z-50 w-[220px] rounded-lg border-d bg-qt-panel p-2 shadow-lg">
                        <div class="grid grid-cols-2 gap-1">
                          <QtermButton
                            v-for="value in serialBaudPresets"
                            :key="value"
                            variant="ghost"
                            size="xs"
                            class="justify-center px-0 text-xs"
                            :class="
                              props.draft.serialBaud === value
                                ? 'bg-qt-accent/15 text-qt-text ring-1 ring-qt-accent/40'
                                : ''
                            "
                            :data-serial-baud-option="value"
                            @click="chooseBaudPreset(value)"
                          >
                            {{ value }}
                          </QtermButton>
                        </div>
                        <div class="mt-2 flex flex-col gap-1 border-t border-qt-border/60 pt-2">
                          <span class="text-[11px] text-qt-muted">{{ t('shell.hostEditor.serialBaudCustom') }}</span>
                          <QtermNumberField
                            id="host-serial-baud-custom"
                            :model-value="props.draft.serialBaud ?? 115200"
                            :min="1"
                            :max="4294967295"
                            :step="1"
                            :step-snapping="false"
                            class="w-full"
                            @keydown.enter.prevent="applyCustomBaud(props.draft.serialBaud ?? 115200)"
                            :aria-label="t('shell.hostEditor.serialBaudCustom')"
                            @update:model-value="(value) => value !== undefined && applyCustomBaud(value)"
                          />
                        </div>
                      </PopoverContent>
                    </PopoverPortal>
                  </PopoverRoot>
                </div>
              </div>

              <div class="form-grid grid grid-cols-[repeat(3,minmax(0,1fr))] gap-[10px] overflow-hidden">
                <div class="form-field grid min-w-0 max-w-full gap-[6px] overflow-hidden text-qt-muted">
                  <Label>{{ t('shell.hostEditor.serialDataBits') }}</Label>
                  <Select
                    :model-value="String(props.draft.serialDataBits ?? 8)"
                    @update:model-value="(value) => updateDraft({ serialDataBits: Number(value) })"
                  >
                    <SelectTrigger
                      id="host-serial-data-bits"
                      class="w-full"
                      :aria-label="t('shell.hostEditor.serialDataBits')"
                    >
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent position="popper" align="start">
                      <SelectItem v-for="option in serialDataBitsOptions" :key="option.value" :value="option.value">
                        {{ option.label }}
                      </SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div class="form-field grid min-w-0 max-w-full gap-[6px] overflow-hidden text-qt-muted">
                  <Label>{{ t('shell.hostEditor.serialParity') }}</Label>
                  <Select
                    :model-value="props.draft.serialParity ?? 'none'"
                    @update:model-value="(value) => updateDraft({ serialParity: String(value) })"
                  >
                    <SelectTrigger
                      id="host-serial-parity"
                      class="w-full"
                      :aria-label="t('shell.hostEditor.serialParity')"
                    >
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent position="popper" align="start">
                      <SelectItem v-for="option in serialParityOptions" :key="option.value" :value="option.value">
                        {{ option.label }}
                      </SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div class="form-field grid min-w-0 max-w-full gap-[6px] overflow-hidden text-qt-muted">
                  <Label>{{ t('shell.hostEditor.serialStopBits') }}</Label>
                  <Select
                    :model-value="props.draft.serialStopBits ?? '1'"
                    @update:model-value="(value) => updateDraft({ serialStopBits: String(value) })"
                  >
                    <SelectTrigger
                      id="host-serial-stop-bits"
                      class="w-full"
                      :aria-label="t('shell.hostEditor.serialStopBits')"
                    >
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent position="popper" align="start">
                      <SelectItem v-for="option in serialStopBitsOptions" :key="option.value" :value="option.value">
                        {{ option.label }}
                      </SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>
            </div>

            <div
              v-if="!shows('serial')"
              class="form-grid grid grid-cols-2 gap-[10px] form-grid--address grid-cols-[minmax(0,_1fr)_104px]"
            >
              <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
                <Label for="host-address"
                  >{{ t('shell.hostEditor.address') }} <span class="required-mark text-qt-danger">*</span></Label
                >
                <QtermInput
                  id="host-address"
                  :model-value="props.draft.address"
                  name="address"
                  autocomplete="off"
                  autocorrect="off"
                  autocapitalize="off"
                  spellcheck="false"
                  :placeholder="t('shell.hostEditor.addressPlaceholder')"
                  @update:model-value="updateAddress"
                />
              </div>
              <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
                <Label for="host-port">{{ t('shell.hostEditor.port') }}</Label>
                <QtermNumberField
                  id="host-port"
                  :model-value="props.draft.port"
                  name="port"
                  :min="1"
                  :max="65535"
                  @update:model-value="(value) => value !== undefined && updateDraft({ port: value })"
                />
              </div>
            </div>

            <div v-if="shows('auth')" class="form-field min-w-0 grid gap-[6px] text-qt-muted">
              <Label>{{ t('shell.hostEditor.authMethod') }}</Label>
              <SegmentedTabs
                :model-value="props.authMode"
                :options="authOptions"
                :label="t('shell.hostEditor.authMethod')"
                @update:model-value="(value) => emit('updateAuthMode', value as HostAuthMode)"
              />
            </div>

            <div
              v-if="shows('auth') && props.authMode === 'password'"
              class="form-field min-w-0 grid gap-[6px] text-qt-muted"
            >
              <Label>{{ t('shell.hostEditor.passwordSource') }}</Label>
              <SegmentedTabs
                :model-value="props.draft.passwordSource === 'saved' ? 'saved' : 'prompt'"
                :options="passwordSourceOptions"
                :label="t('shell.hostEditor.passwordSource')"
                @update:model-value="(value) => emit('updatePasswordSource', value as 'prompt' | 'saved')"
              />
              <QtermHint v-if="props.draft.passwordSource !== 'saved'">
                {{ t('shell.hostEditor.passwordPromptHint') }}
              </QtermHint>
            </div>

            <div class="form-grid grid grid-cols-2 gap-[10px] form-grid--credentials">
              <div
                v-if="shows('identity-name')"
                class="form-field min-w-0 grid gap-[6px] text-qt-muted"
                :class="{ 'col-span-2': credentialsRowSingle }"
              >
                <Label for="host-username"
                  >{{ t('shell.hostEditor.username') }}
                  <span class="optional-label text-qt-faint font-normal">{{
                    t('shell.hostEditor.optional')
                  }}</span></Label
                >

                <QtermInput
                  id="host-username"
                  :model-value="props.draft.username"
                  name="username"
                  autocomplete="off"
                  autocorrect="off"
                  autocapitalize="off"
                  spellcheck="false"
                  :placeholder="usernamePlaceholder"
                  :title="usernameHint || undefined"
                  @update:model-value="(value) => updateDraft({ username: String(value) })"
                />

                <small v-if="vncWssHintsVisible" class="text-xs text-qt-muted" data-vnc-wss-username-hint>{{
                  t('shell.hostEditor.vnc.wssUsernameHint')
                }}</small>
              </div>
              <div
                v-if="shows('auth') && props.authMode === 'password' && props.draft.passwordSource === 'saved'"
                class="form-field min-w-0 grid gap-[6px] text-qt-muted"
                :class="{ 'col-span-2': credentialsRowSingle }"
              >
                <Label for="host-password"
                  >{{ t('shell.hostEditor.password') }}
                  <span class="optional-label text-qt-faint font-normal">{{
                    t('shell.hostEditor.optional')
                  }}</span></Label
                >
                <QtermSecretInput
                  id="host-password"
                  :model-value="props.directSecret"
                  name="password"
                  :placeholder="
                    props.editing ? t('shell.hostEditor.keepPassword') : t('shell.hostEditor.passwordPlaceholder')
                  "
                  @update:model-value="(value) => emit('updateSecret', String(value))"
                />
              </div>
              <div
                v-else-if="shows('auth') && props.authMode === 'credential'"
                class="form-field min-w-0 grid gap-[6px] text-qt-muted"
                :class="{ 'col-span-2': credentialsRowSingle }"
              >
                <Label for="host-credential"
                  >{{ t('shell.hostEditor.savedCredential') }}
                  <span class="optional-label text-qt-faint font-normal">{{
                    t('shell.hostEditor.optional')
                  }}</span></Label
                >
                <div class="flex w-full min-w-0 items-center gap-2">
                  <QtermCombobox
                    id="host-credential"
                    class="min-w-0 w-full flex-1"
                    :model-value="props.draft.credentialId"
                    :options="credentialOptions"
                    :disabled="props.credentialsLoading || props.vaultStatus?.state !== 'unlocked'"
                    :placeholder="t('shell.hostEditor.selectCredential')"
                    :aria-label="t('shell.hostEditor.savedCredential')"
                    :empty-text="t('settings.credentials.empty.matching')"
                    @update:model-value="(value) => updateDraft({ credentialId: String(value) })"
                  />

                  <CredentialQuickCreateMenu :disabled="credentialCreateDisabled" @create="openCredentialCreate" />
                </div>
              </div>
            </div>

            <QtermHint v-if="vncWssHintsVisible" data-vnc-wss-password-hint>
              {{
                vncRfbPasswordMode === 'same'
                  ? t('shell.hostEditor.vnc.wssPasswordHintSame')
                  : t('shell.hostEditor.vnc.wssPasswordHint')
              }}
            </QtermHint>

            <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
              <Label for="host-group">{{ t('shell.hostEditor.group') }}</Label>
              <div class="flex w-full min-w-0 items-center gap-2">
                <QtermCombobox
                  id="host-group"
                  class="min-w-0 w-full flex-1"
                  :model-value="props.draft.group || UNGROUPED_VALUE"
                  :options="groupOptions"
                  :placeholder="t('shell.hostEditor.selectGroup')"
                  :aria-label="t('shell.hostEditor.group')"
                  @update:model-value="
                    (value) => updateDraft({ group: value === UNGROUPED_VALUE ? '' : String(value) })
                  "
                />

                <IconButton
                  variant="outline"
                  :label="t('shell.asset.createGroup')"
                  class="shrink-0"
                  @click="emit('createGroup')"
                >
                  <Plus :size="16" aria-hidden="true" />
                </IconButton>
              </div>
            </div>

            <div v-if="shows('connection')" class="flex flex-col gap-3">
              <div class="flex flex-col gap-1.5">
                <Label>{{ t('shell.hostEditor.connectionMode') }}</Label>

                <SegmentedTabs
                  data-connection-mode
                  :model-value="connectionMode"
                  :options="connectionModeOptions"
                  :label="t('shell.hostEditor.connectionMode')"
                  @update:model-value="switchConnectionMode"
                />
              </div>

              <QtermFormMessage v-if="connectionMode === 'direct'" variant="hint">
                {{ t('shell.hostEditor.connectionDirectHint') }}
              </QtermFormMessage>

              <div v-else-if="connectionMode === 'jump'" class="flex flex-col gap-1.5">
                <Label for="host-jump-host">{{ t('shell.hostEditor.jumpHost') }}</Label>
                <QtermCombobox
                  id="host-jump-host"
                  :model-value="props.draft.jumpHostId || JUMP_HOST_NONE"
                  :options="jumpHostOptions"
                  :placeholder="t('shell.hostEditor.jumpHostPlaceholder')"
                  :aria-label="t('shell.hostEditor.jumpHost')"
                  :empty-text="t('shell.hostEditor.jumpHostEmpty')"
                  @update:model-value="updateJumpHost"
                />
                <QtermFormMessage v-if="jumpHostCandidates.length === 0" variant="hint">
                  {{ t('shell.hostEditor.jumpHostEmpty') }}
                </QtermFormMessage>
                <QtermFormMessage v-else variant="hint">
                  {{ t('shell.hostEditor.connectionJumpHint') }}
                </QtermFormMessage>
                <QtermFormMessage variant="hint">{{ t('shell.hostEditor.jumpHostHint') }}</QtermFormMessage>
              </div>

              <div v-else class="flex flex-col gap-3" role="group" :aria-label="t('shell.hostEditor.proxyMode')">
                <QtermFormMessage variant="hint">{{ t('shell.hostEditor.connectionProxyHint') }}</QtermFormMessage>

                <div v-if="libraryProxies.length || proxyReferenceMissing" class="flex flex-col gap-1.5">
                  <Label for="host-proxy-library">{{ t('shell.hostEditor.proxyLibrary') }}</Label>
                  <div class="flex min-w-0 items-center gap-2">
                    <QtermCombobox
                      id="host-proxy-library"
                      class="min-w-0 flex-1"
                      :model-value="proxyTargetValue"
                      :options="proxyTargetOptions"
                      :placeholder="t('shell.hostEditor.proxyLibraryPlaceholder')"
                      :aria-label="t('shell.hostEditor.proxyLibrary')"
                      @update:model-value="updateProxyTarget"
                    />
                    <ProxyQuickCreateButton :disabled="!props.proxyGateway" @click="openProxyCreate" />
                  </div>

                  <QtermFormMessage v-if="proxyReferenceMissing" variant="error" data-proxy-missing>
                    {{ t('shell.hostEditor.proxyUnavailableHint') }}
                  </QtermFormMessage>
                </div>

                <div
                  v-if="selectedLibraryProxy"
                  class="flex flex-col gap-2 rounded-md border-d bg-qt-subtle p-3"
                  data-proxy-summary
                >
                  <strong class="text-xs font-medium">{{ t('shell.hostEditor.proxySummaryTitle') }}</strong>
                  <dl class="m-0 grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-xs">
                    <dt class="text-qt-muted">{{ t('shell.hostEditor.proxyType') }}</dt>
                    <dd class="m-0">{{ selectedLibraryProxy.mode.toUpperCase() }}</dd>
                    <dt class="text-qt-muted">{{ t('shell.hostEditor.proxyHost') }}</dt>
                    <dd class="m-0 break-all">{{ selectedLibraryProxy.host }}:{{ selectedLibraryProxy.port }}</dd>
                    <dt class="text-qt-muted">{{ t('shell.hostEditor.proxyUsername') }}</dt>
                    <dd class="m-0 break-all">{{ selectedLibraryProxy.username || '—' }}</dd>
                    <dt class="text-qt-muted">{{ t('shell.hostEditor.proxySummaryPassword') }}</dt>
                    <dd class="m-0 break-all">
                      {{
                        selectedLibraryProxy.passwordConfigured ? t('shell.hostEditor.proxyPasswordConfigured') : '—'
                      }}
                    </dd>
                  </dl>
                </div>

                <template v-else-if="proxyCustomSelected">
                  <div class="flex flex-col gap-1.5">
                    <Label>{{ t('shell.hostEditor.proxyType') }}</Label>
                    <SegmentedTabs
                      :model-value="proxyTypeValue"
                      :options="proxyTypeOptions"
                      :label="t('shell.hostEditor.proxyType')"
                      @update:model-value="(value) => updateDraft({ proxyMode: value === 'http' ? 'http' : 'socks5' })"
                    />
                  </div>

                  <div
                    class="form-grid grid grid-cols-2 gap-[10px] form-grid--address grid-cols-[minmax(0,_1fr)_104px]"
                  >
                    <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
                      <Label for="host-proxy-host">{{ t('shell.hostEditor.proxyHost') }}</Label>
                      <QtermInput
                        id="host-proxy-host"
                        :model-value="props.draft.proxyHost || ''"
                        name="proxyHost"
                        autocomplete="off"
                        autocorrect="off"
                        autocapitalize="off"
                        spellcheck="false"
                        :placeholder="t('shell.hostEditor.proxyHostPlaceholder')"
                        @update:model-value="(value) => updateDraft({ proxyHost: String(value) })"
                      />
                    </div>
                    <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
                      <Label for="host-proxy-port">{{ t('shell.hostEditor.proxyPort') }}</Label>
                      <QtermNumberField
                        id="host-proxy-port"
                        :model-value="props.draft.proxyPort || undefined"
                        name="proxyPort"
                        :min="1"
                        :max="65535"
                        @update:model-value="(value) => value !== undefined && updateDraft({ proxyPort: value })"
                      />
                    </div>
                  </div>

                  <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
                    <Label for="host-proxy-username">
                      {{ t('shell.hostEditor.proxyUsername') }}
                      <span class="optional-label text-qt-faint font-normal">{{ t('shell.hostEditor.optional') }}</span>
                    </Label>
                    <QtermInput
                      id="host-proxy-username"
                      :model-value="props.draft.proxyUsername || ''"
                      class="h-qt-control-height min-h-qt-control-height"
                      name="proxyUsername"
                      autocomplete="off"
                      autocorrect="off"
                      autocapitalize="off"
                      spellcheck="false"
                      :placeholder="t('shell.hostEditor.proxyUsernamePlaceholder')"
                      @update:model-value="(value) => updateDraft({ proxyUsername: String(value) })"
                    />
                  </div>

                  <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
                    <Label for="host-proxy-password">
                      {{ t('shell.hostEditor.proxyPassword') }}
                      <span class="optional-label text-qt-faint font-normal">{{ t('shell.hostEditor.optional') }}</span>
                    </Label>
                    <QtermSecretInput
                      id="host-proxy-password"
                      :model-value="props.draft.proxyPassword || ''"
                      input-class="h-qt-control-height min-h-qt-control-height"
                      autocomplete="new-password"
                      :placeholder="t('shell.hostEditor.proxyPasswordPlaceholder')"
                      :show-label="t('shell.hostEditor.proxyPasswordShow')"
                      :hide-label="t('shell.hostEditor.proxyPasswordHide')"
                      @update:model-value="updateProxyPassword"
                    />
                    <QtermFormMessage v-if="props.draft.proxyPasswordConfigured" variant="hint">
                      {{ t('shell.hostEditor.proxyPasswordKeepHint') }}
                    </QtermFormMessage>
                    <label v-if="props.draft.proxyPasswordConfigured" class="settings-toggle-row">
                      <span>
                        <strong>{{ t('shell.hostEditor.proxyPasswordClear') }}</strong>
                        <small class="text-xs">{{ t('shell.hostEditor.proxyPasswordClearHint') }}</small>
                      </span>
                      <QtermSwitch
                        :model-value="props.draft.clearProxyPassword === true"
                        :aria-label="t('shell.hostEditor.proxyPasswordClear')"
                        @update:model-value="(value) => updateDraft({ clearProxyPassword: Boolean(value) })"
                      />
                    </label>
                  </div>

                  <div v-if="!libraryProxies.length" class="flex items-start gap-2">
                    <QtermFormMessage class="min-w-0 flex-1" variant="hint" data-proxy-library-empty>
                      {{ t('shell.hostEditor.proxyLibraryEmptyHint') }}
                    </QtermFormMessage>
                    <ProxyQuickCreateButton :disabled="!props.proxyGateway" @click="openProxyCreate" />
                  </div>
                </template>

                <QtermFormMessage variant="hint">{{ t('shell.hostEditor.proxyScopeHint') }}</QtermFormMessage>
              </div>
            </div>

            <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
              <Label for="host-tags">{{ t('shell.hostEditor.tags') }}</Label>
              <QtermTagsInput
                id="host-tags"
                :model-value="props.draft.tags || []"
                :max="12"
                :max-length="24"
                :placeholder="t('shell.hostEditor.tagsPlaceholder')"
                @update:model-value="(value) => updateDraft({ tags: value })"
              />
              <small class="mt-1 text-xs text-qt-muted">{{ t('shell.hostEditor.tagsHelp') }}</small>
            </div>

            <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
              <Label for="host-notes">{{ t('shell.hostEditor.notes') }}</Label>
              <QtermTextarea
                id="host-notes"
                :model-value="props.draft.notes || ''"
                name="notes"
                rows="3"
                :placeholder="t('shell.hostEditor.notesPlaceholder')"
                @update:model-value="(value) => updateDraft({ notes: String(value) })"
              />
            </div>

            <Accordion
              v-if="shows('advanced')"
              v-model="advancedSections"
              type="multiple"
              class="mt-1 rounded-lg border-d"
            >
              <AccordionItem value="advanced" class="border-0">
                <AccordionTrigger class="py-3 px-3">
                  <span class="flex flex-col gap-0.5">
                    <strong>{{ t('shell.hostEditor.advanced.title') }}</strong>
                    <small class="text-xs font-normal text-qt-muted">{{ t(advancedDescriptionKey) }}</small>
                  </span>
                </AccordionTrigger>
                <AccordionContent class="pb-3 px-3">
                  <div class="flex flex-col">
                    <div class="mb-3 border-t border-qt-border"></div>

                    <template v-if="props.draft.protocol === 'rdp'">
                      <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
                        <Label>{{ t('shell.hostEditor.display.mode') }}</Label>
                        <small class="text-xs">{{ t('shell.hostEditor.display.modeHelp') }}</small>
                        <SegmentedTabs
                          :model-value="props.draft.rdpDisplayMode || 'fit'"
                          :options="rdpDisplayModeOptions"
                          :label="t('shell.hostEditor.display.mode')"
                          @update:model-value="(value) => updateDraft({ rdpDisplayMode: String(value) })"
                        />
                      </div>

                      <div
                        v-if="(props.draft.rdpDisplayMode || 'fit') === 'fixed'"
                        class="form-field mt-3 min-w-0 grid gap-[6px] text-qt-muted"
                        role="group"
                        aria-labelledby="rdp-size-label"
                      >
                        <Label id="rdp-size-label">{{ t('shell.hostEditor.display.size') }}</Label>
                        <small class="text-xs">{{ t('shell.hostEditor.display.sizeHelp') }}</small>
                        <div class="flex items-center gap-2">
                          <QtermInput
                            id="rdp-width"
                            :model-value="String(props.draft.rdpWidth || 1280)"
                            inputmode="numeric"
                            :aria-label="t('shell.hostEditor.display.width')"
                            @update:model-value="
                              (value) => updateDraft({ rdpWidth: clampRdpSize(value, 1280, 640, 7680) })
                            "
                          />
                          <span class="text-qt-muted">×</span>
                          <QtermInput
                            id="rdp-height"
                            :model-value="String(props.draft.rdpHeight || 800)"
                            inputmode="numeric"
                            :aria-label="t('shell.hostEditor.display.height')"
                            @update:model-value="
                              (value) => updateDraft({ rdpHeight: clampRdpSize(value, 800, 480, 4320) })
                            "
                          />
                        </div>
                      </div>

                      <div class="form-field mt-3 min-w-0 grid gap-[6px] text-qt-muted">
                        <Label>{{ t('shell.hostEditor.clipboard.mode') }}</Label>
                        <small class="text-xs">{{ t('shell.hostEditor.clipboard.modeHelp') }}</small>
                        <SegmentedTabs
                          :model-value="props.draft.rdpClipboardMode || 'both'"
                          :options="rdpClipboardModeOptions"
                          :label="t('shell.hostEditor.clipboard.mode')"
                          @update:model-value="(value) => updateDraft({ rdpClipboardMode: String(value) })"
                        />
                      </div>
                      <div class="form-field mt-3 min-w-0 grid gap-[6px] text-qt-muted">
                        <Label>{{ t('shell.hostEditor.clipboard.content') }}</Label>
                        <small class="text-xs">{{ t('shell.hostEditor.clipboard.contentHelp') }}</small>
                        <SegmentedTabs
                          :model-value="props.draft.rdpClipboardContent || 'text'"
                          :options="rdpClipboardContentOptions"
                          :label="t('shell.hostEditor.clipboard.content')"
                          @update:model-value="(value) => updateDraft({ rdpClipboardContent: String(value) })"
                        />
                      </div>
                    </template>

                    <template v-if="props.draft.protocol === 'vnc'">
                      <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
                        <Label>{{ t('shell.hostEditor.vnc.viewOnly') }}</Label>
                        <small class="text-xs">{{ t('shell.hostEditor.vnc.viewOnlyHelp') }}</small>
                        <SegmentedTabs
                          :model-value="props.draft.vncViewOnly ? 'on' : 'off'"
                          :options="vncToggleOptions"
                          :label="t('shell.hostEditor.vnc.viewOnly')"
                          @update:model-value="(value) => updateDraft({ vncViewOnly: String(value) === 'on' })"
                        />
                      </div>
                      <div class="form-field mt-3 min-w-0 grid gap-[6px] text-qt-muted">
                        <Label>{{ t('shell.hostEditor.vnc.shared') }}</Label>
                        <small class="text-xs">{{ t('shell.hostEditor.vnc.sharedHelp') }}</small>
                        <SegmentedTabs
                          :model-value="props.draft.vncShared === false ? 'off' : 'on'"
                          :options="vncToggleOptions"
                          :label="t('shell.hostEditor.vnc.shared')"
                          @update:model-value="(value) => updateDraft({ vncShared: String(value) === 'on' })"
                        />
                      </div>
                      <div class="form-field mt-3 min-w-0 grid gap-[6px] text-qt-muted">
                        <Label>{{ t('shell.hostEditor.vnc.scale') }}</Label>
                        <small class="text-xs">{{ t('shell.hostEditor.vnc.scaleHelp') }}</small>
                        <SegmentedTabs
                          :model-value="props.draft.vncScale ?? 'fit'"
                          :options="vncScaleOptions"
                          :label="t('shell.hostEditor.vnc.scale')"
                          @update:model-value="(value) => updateDraft({ vncScale: String(value) })"
                        />
                      </div>

                      <label class="settings-toggle-row mt-3">
                        <span>
                          <strong>{{ t('shell.hostEditor.vnc.autoReconnect') }}</strong>
                          <small class="text-xs">{{ t('shell.hostEditor.vnc.autoReconnectHelp') }}</small>
                        </span>
                        <QtermSwitch
                          :model-value="vncAutoReconnectEnabled"
                          :aria-label="t('shell.hostEditor.vnc.autoReconnect')"
                          @update:model-value="(value: boolean) => updateDraft({ vncAutoReconnect: value })"
                        />
                      </label>
                      <div class="form-field mt-3 min-w-0 grid gap-[6px] text-qt-muted">
                        <Label for="vnc-auto-reconnect-attempts">{{
                          t('shell.hostEditor.vnc.autoReconnectAttempts')
                        }}</Label>
                        <small class="text-xs">{{ t('shell.hostEditor.vnc.autoReconnectAttemptsHelp') }}</small>
                        <QtermNumberField
                          id="vnc-auto-reconnect-attempts"
                          :model-value="props.draft.vncAutoReconnectAttempts ?? 5"
                          :min="0"
                          :max="100"
                          :step="1"
                          :disabled="!vncAutoReconnectEnabled"
                          :aria-label="t('shell.hostEditor.vnc.autoReconnectAttempts')"
                          @update:model-value="
                            (value) =>
                              updateDraft({
                                vncAutoReconnectAttempts:
                                  value === undefined ? 5 : Math.min(100, Math.max(0, Math.trunc(Number(value)))),
                              })
                          "
                        />
                      </div>

                      <div class="form-field mt-3 min-w-0 grid gap-[6px] text-qt-muted">
                        <Label>{{ t('shell.hostEditor.vnc.transport') }}</Label>
                        <small class="text-xs">{{ t('shell.hostEditor.vnc.transportHelp') }}</small>
                        <SegmentedTabs
                          :model-value="vncTransport"
                          :options="vncTransportOptions"
                          :label="t('shell.hostEditor.vnc.transport')"
                          @update:model-value="(value) => updateDraft({ vncTransport: normalizeVncTransport(value) })"
                        />
                      </div>

                      <div class="form-field mt-3 min-w-0 grid gap-[6px] text-qt-muted">
                        <Label for="vnc-ws-path">{{ t('shell.hostEditor.vnc.wsPath') }}</Label>
                        <small class="text-xs">{{ t('shell.hostEditor.vnc.wsPathHelp') }}</small>
                        <QtermInput
                          id="vnc-ws-path"
                          :model-value="props.draft.vncWsPath ?? ''"
                          :placeholder="t('shell.hostEditor.vnc.wsPathPlaceholder')"
                          :disabled="!vncWssTransport"
                          :aria-invalid="vncWsPathErrorText ? 'true' : undefined"
                          :aria-describedby="vncWsPathErrorText ? 'vnc-ws-path-error' : undefined"
                          data-vnc-ws-path
                          @update:model-value="(value) => updateDraft({ vncWsPath: String(value) })"
                        />
                        <small
                          v-if="vncWsPathErrorText"
                          id="vnc-ws-path-error"
                          class="text-xs text-red-400"
                          data-vnc-ws-path-error
                          >{{ vncWsPathErrorText }}</small
                        >
                      </div>

                      <label class="settings-toggle-row mt-3">
                        <span>
                          <strong>{{ t('shell.hostEditor.vnc.allowInsecureTls') }}</strong>
                          <small class="text-xs">{{ t('shell.hostEditor.vnc.allowInsecureTlsHelp') }}</small>
                        </span>
                        <QtermSwitch
                          :model-value="props.draft.vncAllowInsecureTls === true"
                          :disabled="!vncWssTransport"
                          :aria-label="t('shell.hostEditor.vnc.allowInsecureTls')"
                          data-vnc-allow-insecure-tls
                          @update:model-value="(value: boolean) => updateDraft({ vncAllowInsecureTls: value })"
                        />
                      </label>

                      <div class="form-field mt-3 min-w-0 grid gap-[6px] text-qt-muted">
                        <Label>{{ t('shell.hostEditor.vnc.rfbPassword') }}</Label>
                        <small class="text-xs">{{ t('shell.hostEditor.vnc.rfbPasswordHelp') }}</small>
                        <SegmentedTabs
                          :model-value="vncRfbPasswordMode"
                          :options="vncRfbPasswordModeOptions"
                          :label="t('shell.hostEditor.vnc.rfbPassword')"
                          data-vnc-rfb-password-mode
                          @update:model-value="
                            (value) => updateDraft({ vncRfbPasswordMode: value === 'same' ? 'same' : 'auto' })
                          "
                        />

                        <small
                          v-if="vncRfbPasswordMode === 'same'"
                          class="text-xs text-amber-400"
                          data-vnc-rfb-password-warning
                          >{{ t('shell.hostEditor.vnc.rfbPasswordSameWarning') }}</small
                        >
                      </div>
                    </template>

                    <template v-if="props.draft.protocol === 'serial'">
                      <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
                        <Label>{{ t('shell.hostEditor.terminal.backspace') }}</Label>
                        <small class="text-xs">{{ t('shell.hostEditor.terminal.backspaceHelp') }}</small>
                        <SegmentedTabs
                          :model-value="props.draft.backspaceMode || 'del'"
                          :options="backspaceModeOptions"
                          :label="t('shell.hostEditor.terminal.backspace')"
                          @update:model-value="
                            (value) => updateDraft({ backspaceMode: value as HostDraft['backspaceMode'] })
                          "
                        />
                      </div>
                      <div class="form-field mt-3 min-w-0 grid gap-[6px] text-qt-muted">
                        <Label>{{ t('shell.hostEditor.terminal.encoding') }}</Label>
                        <small class="text-xs">{{ t('shell.hostEditor.terminal.encodingHelp') }}</small>
                        <SegmentedTabs
                          :model-value="props.draft.encoding || 'utf-8'"
                          :options="encodingOptions"
                          :label="t('shell.hostEditor.terminal.encoding')"
                          @update:model-value="(value) => updateDraft({ encoding: value as HostDraft['encoding'] })"
                        />
                      </div>
                    </template>
                    <template v-else-if="props.draft.protocol !== 'rdp' && props.draft.protocol !== 'vnc'">
                      <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
                        <Label>{{ t('shell.hostEditor.terminal.backspace') }}</Label>
                        <small class="text-xs">{{ t('shell.hostEditor.terminal.backspaceHelp') }}</small>
                        <SegmentedTabs
                          :model-value="props.draft.backspaceMode || 'del'"
                          :options="backspaceModeOptions"
                          :label="t('shell.hostEditor.terminal.backspace')"
                          @update:model-value="
                            (value) => updateDraft({ backspaceMode: value as HostDraft['backspaceMode'] })
                          "
                        />
                      </div>
                      <div class="form-field mt-3 min-w-0 grid gap-[6px] text-qt-muted">
                        <Label>{{ t('shell.hostEditor.terminal.encoding') }}</Label>
                        <small class="text-xs">{{ t('shell.hostEditor.terminal.encodingHelp') }}</small>
                        <SegmentedTabs
                          :model-value="props.draft.encoding || 'utf-8'"
                          :options="encodingOptions"
                          :label="t('shell.hostEditor.terminal.encoding')"
                          @update:model-value="(value) => updateDraft({ encoding: value as HostDraft['encoding'] })"
                        />
                      </div>
                      <div v-if="sshHost" class="form-field mt-3 min-w-0 grid gap-[6px] text-qt-muted">
                        <Label>{{ t('shell.hostEditor.connection.type') }}</Label>
                        <small class="text-xs">{{ t('shell.hostEditor.connection.typeHelp') }}</small>
                        <SegmentedTabs
                          :model-value="props.draft.connectionType || 'standard'"
                          :options="connectionTypeOptions"
                          :label="t('shell.hostEditor.connection.type')"
                          @update:model-value="updateConnectionType"
                        />
                        <small class="mt-1 text-xs text-qt-muted">{{
                          props.draft.connectionType === 'network'
                            ? t('shell.hostEditor.connection.typeNetworkHelp')
                            : t('shell.hostEditor.connection.typeStandardHelp')
                        }}</small>
                      </div>
                      <div class="form-field mt-3 min-w-0 grid gap-[6px] text-qt-muted">
                        <Label for="host-terminal-type">{{ t('shell.hostEditor.connection.terminalType') }}</Label>
                        <small class="text-xs">{{ t('shell.hostEditor.connection.terminalTypeHelp') }}</small>
                        <Select
                          :model-value="props.draft.terminalType || defaultTerminalTypeFor(props.draft.connectionType)"
                          @update:model-value="
                            (value) => updateDraft({ terminalType: String(value) as HostDraft['terminalType'] })
                          "
                        >
                          <SelectTrigger
                            id="host-terminal-type"
                            class="w-full"
                            :aria-label="t('shell.hostEditor.connection.terminalType')"
                          >
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent position="popper" align="start">
                            <SelectItem v-for="option in terminalTypeOptions" :key="option.value" :value="option.value">
                              {{ option.label }}
                            </SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                      <div v-if="sshHost" class="mt-3 grid min-w-0 gap-3 rounded-md border border-qt-border p-3">
                        <label class="settings-toggle-row">
                          <span>
                            <strong>{{ t('shell.hostEditor.twoFactor.title') }}</strong>
                            <small class="text-xs">{{ t('shell.hostEditor.twoFactor.help') }}</small>
                          </span>
                          <QtermSwitch
                            :model-value="props.draft.sshTwoFactorEnabled ?? false"
                            :aria-label="t('shell.hostEditor.twoFactor.title')"
                            @update:model-value="updateTwoFactor"
                          />
                        </label>
                        <template v-if="props.draft.sshTwoFactorEnabled">
                          <div class="grid min-w-0 gap-1.5">
                            <Label>{{ t('shell.hostEditor.twoFactor.otp') }}</Label>
                            <QtermCombobox
                              :model-value="props.draft.sshOtpEntryId || '__manual__'"
                              :options="otpOptions"
                              :aria-label="t('shell.hostEditor.twoFactor.otp')"
                              :placeholder="t('shell.hostEditor.twoFactor.manual')"
                              @update:model-value="updateOtpSelection"
                            />
                          </div>
                          <label class="settings-toggle-row">
                            <span>
                              <strong>{{ t('shell.hostEditor.twoFactor.autoFill') }}</strong>
                              <small class="text-xs">{{ t('shell.hostEditor.twoFactor.autoFillHelp') }}</small>
                            </span>
                            <QtermSwitch
                              :model-value="props.draft.sshOtpAutoFill ?? false"
                              :disabled="
                                !(props.otpEntries ?? []).some(
                                  (entry) => entry.id === props.draft.sshOtpEntryId && entry.type === 'totp',
                                )
                              "
                              :aria-label="t('shell.hostEditor.twoFactor.autoFill')"
                              @update:model-value="(value: boolean) => updateDraft({ sshOtpAutoFill: value })"
                            />
                          </label>
                        </template>
                      </div>
                      <label v-if="sshHost" class="settings-toggle-row mt-3" :class="{ 'opacity-50': networkDevice }">
                        <span>
                          <strong>{{ t('shell.hostEditor.keepAlive') }}</strong>
                          <small class="text-xs">{{ t('shell.hostEditor.keepAliveHelp') }}</small>
                        </span>
                        <QtermSwitch
                          :model-value="props.draft.sshKeepAlive ?? true"
                          :disabled="networkDevice"
                          :aria-label="t('shell.hostEditor.keepAlive')"
                          @update:model-value="(value: boolean) => updateDraft({ sshKeepAlive: value })"
                        />
                      </label>
                      <label v-if="sshHost" class="settings-toggle-row mt-3" :class="{ 'opacity-50': networkDevice }">
                        <span>
                          <strong>{{ t('shell.hostEditor.latencyDetection') }}</strong>
                          <small class="text-xs">{{ t('shell.hostEditor.latencyDetectionHelp') }}</small>
                        </span>
                        <QtermSwitch
                          :model-value="props.draft.latencyDetection ?? true"
                          :disabled="networkDevice"
                          :aria-label="t('shell.hostEditor.latencyDetection')"
                          @update:model-value="(value: boolean) => updateDraft({ latencyDetection: value })"
                        />
                      </label>

                      <div class="form-field mt-3 min-w-0 grid gap-[6px] text-qt-muted">
                        <Label for="host-connect-timeout">{{ t('shell.hostEditor.terminal.timeout') }}</Label>
                        <small class="text-xs">{{ t('shell.hostEditor.terminal.timeoutHelp') }}</small>
                        <QtermNumberField
                          id="host-connect-timeout"
                          :model-value="props.draft.connectTimeoutSeconds ?? 15"
                          :min="1"
                          :max="300"
                          :step="1"
                          :aria-label="t('shell.hostEditor.terminal.timeout')"
                          @update:model-value="(value) => updateDraft({ connectTimeoutSeconds: Number(value) })"
                        />
                      </div>
                      <div
                        v-if="sshHost"
                        class="settings-toggle-row settings-toggle-row--stacked mt-3"
                        :class="{ 'opacity-50': networkDevice }"
                      >
                        <label class="settings-toggle-row border-0! p-0! mb-2">
                          <span>
                            <strong>{{ t('shell.hostEditor.autoDetectIcon') }}</strong>
                            <small class="text-xs">{{ t('shell.hostEditor.autoDetectIconHelp') }}</small>
                          </span>
                          <QtermSwitch
                            :model-value="props.draft.autoDetectOSIcon ?? true"
                            :disabled="networkDevice"
                            :aria-label="t('shell.hostEditor.autoDetectIcon')"
                            @update:model-value="(value: boolean) => updateDraft({ autoDetectOSIcon: value })"
                          />
                        </label>
                        <div class="border-t border-qt-border gap-2 pt-2 flex-cb">
                          <div class="flex flex-col gap-0.5 min-w-0">
                            <small class="text-xs text-qt-muted">{{
                              t('shell.hostEditor.iconDetect.resultLabel')
                            }}</small>
                            <strong class="text-xs text-qt-text font-medium">{{ currentIconLabel }}</strong>
                          </div>
                          <QtermButton
                            variant="outline"
                            type="button"
                            class="h-8! min-h-8! text-xs"
                            :disabled="props.detectingIcon || props.saving || props.testingConnection || networkDevice"
                            @click="emit('detectIcon')"
                          >
                            {{
                              props.detectingIcon
                                ? t('shell.hostEditor.iconDetect.detecting')
                                : t('shell.hostEditor.iconDetect.action')
                            }}
                          </QtermButton>
                        </div>
                      </div>
                      <label v-if="sshHost" class="settings-toggle-row mt-3" :class="{ 'opacity-50': networkDevice }">
                        <span>
                          <strong>{{ t('shell.hostEditor.x11.forwarding') }}</strong>
                          <small class="text-xs">{{ t('shell.hostEditor.x11.forwardingHelp') }}</small>
                        </span>
                        <QtermSwitch
                          :model-value="props.draft.x11Forwarding ?? false"
                          :disabled="networkDevice"
                          :aria-label="t('shell.hostEditor.x11.forwarding')"
                          @update:model-value="(value) => updateDraft({ x11Forwarding: value })"
                        />
                      </label>
                      <div
                        v-if="props.draft.x11Forwarding"
                        class="form-field mt-3 min-w-0 grid gap-[6px] text-qt-muted"
                        :class="networkDevice ? 'pointer-events-none opacity-50' : ''"
                      >
                        <Label>{{ t('shell.hostEditor.x11.mode') }}</Label>
                        <small class="text-xs">{{ t('shell.hostEditor.x11.modeHelp') }}</small>
                        <SegmentedTabs
                          :model-value="props.draft.x11Trusted === false ? 'untrusted' : 'trusted'"
                          :options="x11TrustOptions"
                          :label="t('shell.hostEditor.x11.mode')"
                          @update:model-value="(value) => updateDraft({ x11Trusted: value === 'trusted' })"
                        />
                      </div>
                      <div
                        v-if="sshHost"
                        class="form-field mt-3 min-w-0 grid gap-[6px] text-qt-muted"
                        :class="networkDevice ? 'pointer-events-none opacity-50' : ''"
                      >
                        <Label>{{ t('shell.hostEditor.directoryFollow.title') }}</Label>
                        <small class="text-xs">{{ t('shell.hostEditor.directoryFollow.help') }}</small>
                        <SegmentedTabs
                          :model-value="props.draft.directoryFollow || 'inherit'"
                          :options="directoryFollowOptions"
                          :label="t('shell.hostEditor.directoryFollow.title')"
                          @update:model-value="
                            (value) => updateDraft({ directoryFollow: value as HostDraft['directoryFollow'] })
                          "
                        />
                      </div>
                      <div
                        v-if="sshHost"
                        class="form-field mt-3 min-w-0 grid gap-[6px] text-qt-muted"
                        :class="networkDevice ? 'pointer-events-none opacity-50' : ''"
                      >
                        <Label>{{ t('shell.hostEditor.sftpHidden.title') }}</Label>
                        <small class="text-xs">{{ t('shell.hostEditor.sftpHidden.help') }}</small>
                        <SegmentedTabs
                          :model-value="props.draft.sftpShowHiddenFiles || 'inherit'"
                          :options="sftpShowHiddenFilesOptions"
                          :label="t('shell.hostEditor.sftpHidden.title')"
                          @update:model-value="
                            (value) => updateDraft({ sftpShowHiddenFiles: value as HostDraft['sftpShowHiddenFiles'] })
                          "
                        />
                      </div>
                      <div
                        v-if="sshHost"
                        class="form-field mt-3 min-w-0 grid gap-[6px] text-qt-muted"
                        :class="networkDevice ? 'pointer-events-none opacity-50' : ''"
                      >
                        <Label for="host-sftp-directory">{{ t('shell.hostEditor.sftpDirectory.title') }}</Label>
                        <small class="text-xs">{{ t('shell.hostEditor.sftpDirectory.help') }}</small>
                        <QtermInput
                          id="host-sftp-directory"
                          :model-value="props.draft.sftpDefaultDirectory || ''"
                          name="sftpDefaultDirectory"
                          autocomplete="off"
                          autocorrect="off"
                          autocapitalize="off"
                          spellcheck="false"
                          :placeholder="t('shell.hostEditor.sftpDirectory.placeholder')"
                          @update:model-value="(value) => updateDraft({ sftpDefaultDirectory: String(value) })"
                        />
                      </div>
                      <div class="form-field mt-3 min-w-0 grid gap-[6px] text-qt-muted">
                        <Label for="host-post-login">{{ t('shell.hostEditor.terminal.postLoginCommand') }}</Label>
                        <small class="text-xs">{{ t('shell.hostEditor.terminal.postLoginCommandHelp') }}</small>

                        <QtermTextarea
                          id="host-post-login"
                          :model-value="props.draft.postLoginCommand || ''"
                          name="postLoginCommand"
                          rows="3"
                          autocomplete="off"
                          autocorrect="off"
                          autocapitalize="off"
                          spellcheck="false"
                          :placeholder="t('shell.hostEditor.terminal.postLoginCommandPlaceholder')"
                          @update:model-value="(value) => updateDraft({ postLoginCommand: String(value) })"
                        />
                      </div>
                      <div class="form-field mt-3 min-w-0 grid gap-[6px] text-qt-muted">
                        <Label for="host-post-login-delay">{{ t('shell.hostEditor.terminal.postLoginDelay') }}</Label>
                        <small class="text-xs">{{ t('shell.hostEditor.terminal.postLoginDelayHelp') }}</small>
                        <QtermNumberField
                          id="host-post-login-delay"
                          :model-value="props.draft.postLoginCommandDelaySeconds ?? 0"
                          :min="0"
                          :max="300"
                          :step="1"
                          :aria-label="t('shell.hostEditor.terminal.postLoginDelay')"
                          @update:model-value="(value) => updateDraft({ postLoginCommandDelaySeconds: Number(value) })"
                        />
                      </div>
                      <div v-if="sshHost" class="form-field mt-3 min-w-0 grid gap-[6px] text-qt-muted">
                        <Label>{{ t('shell.hostEditor.terminal.environmentVariables') }}</Label>
                        <small class="text-xs">{{ t('shell.hostEditor.terminal.environmentVariablesHelp') }}</small>
                        <div class="flex flex-col gap-[8px] mt-[6px]">
                          <div
                            v-for="(item, index) in props.draft.environmentVariables || []"
                            :key="index"
                            class="host-environment-row flex flex-wrap items-center gap-[8px]"
                          >
                            <QtermInput
                              :model-value="item.name"
                              autocomplete="off"
                              autocorrect="off"
                              autocapitalize="off"
                              spellcheck="false"
                              :placeholder="t('shell.hostEditor.terminal.environmentName')"
                              :aria-label="t('shell.hostEditor.terminal.environmentName')"
                              @update:model-value="(value) => updateEnvironmentVariable(index, { name: String(value) })"
                            />
                            <QtermInput
                              :model-value="item.value"
                              autocomplete="off"
                              autocorrect="off"
                              autocapitalize="off"
                              spellcheck="false"
                              :placeholder="t('shell.hostEditor.terminal.environmentValue')"
                              :aria-label="t('shell.hostEditor.terminal.environmentValue')"
                              @update:model-value="
                                (value) => updateEnvironmentVariable(index, { value: String(value) })
                              "
                            />
                            <QtermButton
                              variant="outline"
                              type="button"
                              class="h-8! min-h-8! text-xs"
                              :aria-label="t('shell.hostEditor.terminal.environmentRemove')"
                              @click="removeEnvironmentVariable(index)"
                            >
                              {{ t('shell.hostEditor.terminal.environmentRemove') }}
                            </QtermButton>
                          </div>
                          <QtermButton
                            variant="outline"
                            type="button"
                            class="text-xs"
                            @click="addEnvironmentVariable()"
                          >
                            {{ t('shell.hostEditor.terminal.environmentAdd') }}
                          </QtermButton>
                        </div>
                      </div>
                    </template>
                  </div>
                </AccordionContent>
              </AccordionItem>
            </Accordion>

            <QtermFormMessage v-if="props.credentialsError" variant="error">{{
              props.credentialsError
            }}</QtermFormMessage>
            <QtermFormMessage v-else-if="props.credentialsLoading" variant="hint">
              {{ t('shell.hostEditor.loadingCredentials') }}
            </QtermFormMessage>
            <QtermFormMessage
              v-else-if="props.authMode === 'credential' && props.vaultStatus?.state !== 'unlocked'"
              variant="hint"
            >
              {{ t('shell.hostEditor.vaultLocked') }}
              <QtermButton variant="link" class="h-auto p-0 text-xs" type="button" @click="emit('openSecurity')">{{
                t('shell.hostEditor.openSecurity')
              }}</QtermButton>
            </QtermFormMessage>
            <QtermFormMessage
              v-else-if="props.authMode === 'credential' && props.credentials.length === 0"
              variant="hint"
            >
              {{ t('shell.hostEditor.noCredentials') }}
            </QtermFormMessage>
            <QtermFormMessage
              v-else-if="props.editing && props.authMode === 'password' && props.draft.passwordSource === 'saved'"
              variant="hint"
            >
              {{ t('shell.hostEditor.passwordStorageHint') }}
            </QtermFormMessage>

            <QtermFormMessage v-if="props.formError" variant="error">{{ props.formError }}</QtermFormMessage>
          </div>
        </ScrollArea>

        <div
          v-if="routeTestHops.length || routeTestUnavailable || routeTestError"
          class="flex max-h-40 shrink-0 flex-col gap-1.5 overflow-y-auto border-t border-qt-border px-4 py-2"
        >
          <strong class="text-xs font-medium">{{ t('shell.hostEditor.routeTestHopsTitle') }}</strong>
          <ul v-if="routeTestHops.length" class="flex flex-col gap-1" data-route-test-hops>
            <li v-for="(hop, index) in routeTestHops" :key="`${hop.kind}-${index}`" class="flex-cb gap-3 text-xs">
              <span class="min-w-0 truncate">
                {{ hop.label || hop.address }}
                <small v-if="hop.unverifiedHostKey" class="ml-1 text-qt-warning" data-route-test-unverified>
                  {{ t('shell.hostEditor.routeTestUnverified') }}
                </small>
              </span>
              <span class="shrink-0 tabular-nums" :class="hop.reachable ? 'text-qt-muted' : 'text-qt-danger'">
                {{ hop.reachable ? `${hop.latencyMs} ms` : translateErrorKey(hop.errorKey || 'error.unknown') }}
              </span>
            </li>
          </ul>

          <small v-else-if="routeTestError" class="text-xs text-qt-danger" data-route-test-error>{{
            routeTestError
          }}</small>
          <small v-else-if="routeTestUnavailable" class="text-xs text-qt-muted">{{
            t('shell.hostEditor.routeTestUnavailable')
          }}</small>
        </div>

        <QtermDialogFooter>
          <QtermButton
            variant="outline"
            type="button"
            data-connection-test
            :disabled="props.saving || props.testingConnection || routeTesting"
            @click="runConnectionTest"
          >
            {{
              props.testingConnection || routeTesting
                ? t('shell.hostEditor.testBusy')
                : t('shell.hostEditor.testConnection')
            }}
          </QtermButton>
          <div class="dialog-footer-actions flex items-center gap-[8px]">
            <QtermButton variant="outline" type="button" :disabled="props.testingConnection" @click="emit('close')">{{
              t('shell.common.cancel')
            }}</QtermButton>
            <QtermButton
              class="dialog-primary-action"
              type="submit"
              :disabled="props.saving || props.testingConnection || props.credentialsLoading"
            >
              {{ props.saving ? t('shell.common.saving') : t('shell.common.confirm') }}
            </QtermButton>
          </div>
        </QtermDialogFooter>
      </form>
    </component>
  </QtermDialog>

  <CredentialCreateDialog
    v-if="props.credentialGateway"
    :open="credentialCreateOpen"
    :kind="credentialCreateKind"
    :gateway="props.credentialGateway"
    :status="props.vaultStatus"
    :initial-key-mode="credentialCreateKeyMode"
    @created="handleCredentialCreated"
    @close="credentialCreateOpen = false"
  />

  <ProxyEditorDialog
    v-if="props.proxyGateway"
    :open="proxyCreateOpen"
    :gateway="props.proxyGateway"
    @created="handleProxyCreated"
    @close="proxyCreateOpen = false"
  />
</template>
