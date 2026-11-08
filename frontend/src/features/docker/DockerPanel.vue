<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, shallowReactive, watch } from 'vue'
import {
  ChevronDown,
  ChevronRight,
  CircleAlert,
  Container,
  Database,
  Loader2,
  Network,
  Package,
  RefreshCw,
} from '@lucide/vue'
import { IconButton, QtermButton, QtermEmptyState, QtermFormMessage, SegmentedTabs } from '../../components/common'
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
  ScrollArea,
} from '../../components/common/control-primitives'
import { ContextMenu, ContextMenuTrigger } from '../../components/common/overlay-controls'
import QtermLoadMore from '../../components/common/layout/QtermLoadMore.vue'
import DeleteConfirmDialog from '../../components/dialogs/DeleteConfirmDialog.vue'
import DockerContainerMenu from './DockerContainerMenu.vue'
import DockerGroupMenu from './DockerGroupMenu.vue'
import DockerContainerDetailDialog from './DockerContainerDetailDialog.vue'
import DockerImageDetailDialog from './DockerImageDetailDialog.vue'
import DockerNetworkDetailDialog from './DockerNetworkDetailDialog.vue'
import DockerVolumeDetailDialog from './DockerVolumeDetailDialog.vue'
import DockerImageRow from './DockerImageRow.vue'
import DockerNetworkRow from './DockerNetworkRow.vue'
import DockerVolumeRow from './DockerVolumeRow.vue'
import { useSnippetGroupPagination } from '../snippets/useSnippetGroupPagination'
import {
  dockerImageName,
  dockerRemoveTarget,
  dockerUnavailableDescriptionKey,
  isDanglingImage,
  type DockerContainer,
  type DockerComposeAction,
  type DockerContainerAction,
  type DockerEnvironment,
  type DockerGateway,
  type DockerImage,
  type DockerNetwork,
  type DockerVolume,
} from '../../services/docker'
import { decodeAppError, errorMessage } from '../../services/errors'
import { requestRemoteTextEditor } from '../remoteedit/useRemoteEdits'
import { useI18n } from '@/locales'

const props = defineProps<{
  gateway: DockerGateway

  sessionId: string

  hostId: string
}>()

const { t } = useI18n()

const COMPACT_WIDTH = 480

const root = ref<HTMLElement | null>(null)

const panelWidth = ref(Number.POSITIVE_INFINITY)
const compact = computed(() => panelWidth.value < COMPACT_WIDTH)

type DockerTab = 'containers' | 'images' | 'volumes' | 'networks'

interface DockerTabState<T> {
  items: T[]
  loading: boolean
  error: string
  loaded: boolean
}

function createTabState<T>(): DockerTabState<T> {
  return shallowReactive<DockerTabState<T>>({ items: [] as T[], loading: false, error: '', loaded: false })
}

const activeTab = ref<DockerTab>('containers')
const containerTab = createTabState<DockerContainer>()
const imageTab = createTabState<DockerImage>()
const volumeTab = createTabState<DockerVolume>()
const networkTab = createTabState<DockerNetwork>()

const containers = computed(() => containerTab.items)

const environment = ref<DockerEnvironment | null>(null)

const loading = ref(false)

const error = ref('')

const actionError = ref('')

const busyId = ref('')

const busyAction = ref<'' | DockerContainerAction | DockerComposeAction>('')
const expandedGroupIds = ref<string[]>([])
const detailTarget = ref<DockerContainer | null>(null)

const imageDetailTarget = ref<DockerImage | null>(null)
const volumeDetailTarget = ref<DockerVolume | null>(null)
const networkDetailTarget = ref<DockerNetwork | null>(null)
const deleteTarget = ref<DockerContainer | null>(null)
const deleteError = ref('')
const deleting = ref(false)

const groupDeleteTarget = ref<DockerContainerGroup | null>(null)
const groupDeleteError = ref('')
const deletingGroup = ref(false)

const imageTarget = ref<DockerImage | null>(null)
const volumeTarget = ref<DockerVolume | null>(null)
const removeError = ref('')
const removing = ref(false)

type DockerPruneMode = 'dangling' | 'unused'
const pruneMode = ref<DockerPruneMode | ''>('')
const pruneError = ref('')
const pruning = ref(false)

const networkTarget = ref<DockerNetwork | null>(null)
const networkDeleteError = ref('')
const removingNetwork = ref(false)
const pruneVolumesOpen = ref(false)
const pruneVolumesError = ref('')
const pruningVolumes = ref(false)
const pruneNetworksOpen = ref(false)
const pruneNetworksError = ref('')
const pruningNetworks = ref(false)

const enabled = ref<boolean | null>(null)
const enabling = ref(false)
const enableError = ref('')

interface DockerContainerGroup {
  id: string
  name: string

  directory: string
  compose: boolean
  running: number
  total: number
  containers: DockerContainer[]
}

function byName(left: DockerContainer, right: DockerContainer): number {
  return left.name < right.name ? -1 : left.name > right.name ? 1 : 0
}

const groups = computed<DockerContainerGroup[]>(() => {
  const byProject = new Map<string, DockerContainer[]>()
  const standalone: DockerContainer[] = []
  for (const container of containers.value) {
    if (!container.project) {
      standalone.push(container)
      continue
    }
    const list = byProject.get(container.project) ?? []
    list.push(container)
    byProject.set(container.project, list)
  }
  const result: DockerContainerGroup[] = [...byProject.entries()]
    .sort(([left], [right]) => (left < right ? -1 : left > right ? 1 : 0))
    .map(([project, list]) => ({
      id: `project:${project}`,
      name: project,
      directory: list.find((item) => item.projectDir)?.projectDir ?? '',
      compose: true,
      running: list.filter((item) => item.state === 'running').length,
      total: list.length,
      containers: [...list].sort(byName),
    }))
  if (standalone.length) {
    result.push({
      id: 'standalone',
      name: t('shell.docker.standalone'),
      directory: '',
      compose: false,
      running: standalone.filter((item) => item.state === 'running').length,
      total: standalone.length,
      containers: [...standalone].sort(byName),
    })
  }
  return result
})

const composeGroups = computed(() => groups.value.filter((group) => group.compose))
const standaloneGroup = computed(() => groups.value.find((group) => !group.compose) ?? null)
const unavailable = computed(() => environment.value !== null && !environment.value.available)
const unavailableDescription = computed(() => {
  const message = environment.value?.message ?? ''
  return message ? t(dockerUnavailableDescriptionKey(message)) : t('shell.docker.unavailableHint')
})

const ready = computed(() => enabled.value === true && environment.value?.available === true)

const subtitle = computed(() => {
  const env = environment.value

  if (!env?.dockerVersion) return ''
  return t('shell.docker.subtitle', {
    docker: env.dockerVersion,
    compose: env.composeVersion || '—',
  })
})

const tabLoading = computed(() => {
  if (activeTab.value === 'images') return imageTab.loading
  if (activeTab.value === 'volumes') return volumeTab.loading
  if (activeTab.value === 'networks') return networkTab.loading
  return loading.value || containerTab.loading
})

const refreshing = computed(
  () => loading.value || containerTab.loading || imageTab.loading || volumeTab.loading || networkTab.loading,
)

const currentError = computed(() => {
  if (activeTab.value === 'images') return imageTab.error
  if (activeTab.value === 'volumes') return volumeTab.error
  if (activeTab.value === 'networks') return networkTab.error
  return containerTab.error
})

function tabCount<T>(state: DockerTabState<T>): number | undefined {
  return state.loaded ? state.items.length : undefined
}

const tabOptions = computed(() => [
  {
    value: 'containers',
    label: t('shell.docker.tabContainers'),
    count: compact.value ? undefined : tabCount(containerTab),
  },
  { value: 'images', label: t('shell.docker.tabImages'), count: compact.value ? undefined : tabCount(imageTab) },
  { value: 'volumes', label: t('shell.docker.tabVolumes'), count: compact.value ? undefined : tabCount(volumeTab) },
  {
    value: 'networks',
    label: t('shell.docker.tabNetworks'),
    count: compact.value ? undefined : tabCount(networkTab),
  },
])

const imageGroups = computed(() =>
  [
    { id: 'in-use', title: t('shell.docker.inUse'), items: imageTab.items.filter((image) => image.inUse) },
    { id: 'unused', title: t('shell.docker.unused'), items: imageTab.items.filter((image) => !image.inUse) },
  ].filter((group) => group.items.length > 0),
)

const volumeGroups = computed(() =>
  [
    { id: 'in-use', title: t('shell.docker.inUse'), items: volumeTab.items.filter((volume) => volume.inUse) },
    { id: 'unused', title: t('shell.docker.unused'), items: volumeTab.items.filter((volume) => !volume.inUse) },
  ].filter((group) => group.items.length > 0),
)

const networkGroups = computed(() =>
  [
    { id: 'in-use', title: t('shell.docker.inUse'), items: networkTab.items.filter((network) => network.inUse) },
    { id: 'unused', title: t('shell.docker.unused'), items: networkTab.items.filter((network) => !network.inUse) },
  ].filter((group) => group.items.length > 0),
)

const removableUnusedNetworks = computed(() => networkTab.items.filter((network) => !network.inUse && !network.builtIn))

const unusedImages = computed(() => imageTab.items.filter((image) => !image.inUse))
const danglingImages = computed(() => imageTab.items.filter((image) => isDanglingImage(image)))

function imageLabel(image: DockerImage): string {
  return dockerImageName(image) || t('shell.docker.danglingImage')
}

const pruneItems = computed(() => {
  if (pruneMode.value !== 'unused') return []
  return [
    t('shell.docker.confirmPruneItems', { count: unusedImages.value.length }),
    ...unusedImages.value.slice(0, 5).map(imageLabel),
  ]
})

const pruneMessage = computed(() => {
  if (pruneMode.value === 'unused') {
    return t('shell.docker.confirmPruneUnusedMessage', { count: unusedImages.value.length })
  }
  if (pruneMode.value === 'dangling') {
    return t('shell.docker.confirmPruneMessage', { count: danglingImages.value.length })
  }
  return ''
})

const pruneNetworkItems = computed(() => [
  t('shell.docker.confirmPruneNetworkItems', { count: removableUnusedNetworks.value.length }),
  ...removableUnusedNetworks.value.slice(0, 5).map((network) => network.name),
])

const unusedVolumes = computed(() => volumeTab.items.filter((volume) => !volume.inUse))

const pruneVolumeItems = computed(() => [
  t('shell.docker.confirmPruneVolumeItems', { count: unusedVolumes.value.length }),
  ...unusedVolumes.value.slice(0, 5).map((volume) => volume.name),
])

const openGroupIds = computed<string[]>({
  get: () => composeGroups.value.filter((group) => !expandedGroupIds.value.includes(group.id)).map((group) => group.id),
  set: (value) => {
    expandedGroupIds.value = composeGroups.value
      .filter((group) => !(value ?? []).includes(group.id))
      .map((group) => group.id)
  },
})

const { loadMore, reset, visibleCount, visibleItems } = useSnippetGroupPagination(20)

function imageGroupKey(groupId: string): string {
  return `images:${groupId}`
}

function volumeGroupKey(groupId: string): string {
  return `volumes:${groupId}`
}

function networkGroupKey(groupId: string): string {
  return `networks:${groupId}`
}

function isComposeGroupExpanded(group: DockerContainerGroup): boolean {
  return !expandedGroupIds.value.includes(group.id)
}

const groupKeyOfStandalone = 'standalone'

function composeBusyAction(group: DockerContainerGroup): DockerComposeAction | '' {
  if (busyId.value !== group.id) return ''
  const action = busyAction.value
  return action === 'up' || action === 'stop' || action === 'restart' || action === 'down' ? action : ''
}

const NOT_ENABLED_KEY = 'docker.not_enabled'

function isNotEnabled(cause: unknown): boolean {
  return decodeAppError(cause)?.messageKey === NOT_ENABLED_KEY
}

function resetTab<T>(state: DockerTabState<T>): void {
  state.items = []
  state.loading = false
  state.error = ''
  state.loaded = false
}

function resetTabs(): void {
  resetTab(containerTab)
  resetTab(imageTab)
  resetTab(volumeTab)
  resetTab(networkTab)
  actionError.value = ''

  reset()
}

function showEnableCard(): void {
  enabled.value = false
  environment.value = null
  error.value = ''
  resetTabs()
}

async function loadTab<T>(state: DockerTabState<T>, fetch: () => Promise<T[]>, quiet = false): Promise<void> {
  if (!quiet) {
    state.loading = true
    state.error = ''
    actionError.value = ''
    reset()
  }
  try {
    state.items = await fetch()
    state.loaded = true
  } catch (cause) {
    if (isNotEnabled(cause)) {
      showEnableCard()
      return
    }
    state.items = []
    state.error = errorMessage(cause, t('shell.docker.actionFailed'))
  } finally {
    state.loading = false
  }
}

function loadContainers(quiet = false): Promise<void> {
  return loadTab(containerTab, () => props.gateway.containers(props.sessionId), quiet)
}

function loadImages(quiet = false): Promise<void> {
  return loadTab(imageTab, () => props.gateway.images(props.sessionId), quiet)
}

function loadVolumes(quiet = false): Promise<void> {
  return loadTab(volumeTab, () => props.gateway.volumes(props.sessionId), quiet)
}

function loadNetworks(quiet = false): Promise<void> {
  return loadTab(networkTab, () => props.gateway.networks(props.sessionId), quiet)
}

function isLoaded(tab: DockerTab): boolean {
  if (tab === 'images') return imageTab.loaded
  if (tab === 'volumes') return volumeTab.loaded
  if (tab === 'networks') return networkTab.loaded
  return containerTab.loaded
}

function isTabLoading(tab: DockerTab): boolean {
  if (tab === 'images') return imageTab.loading
  if (tab === 'volumes') return volumeTab.loading
  if (tab === 'networks') return networkTab.loading
  return containerTab.loading
}

async function ensureLoaded(tab: DockerTab): Promise<void> {
  if (!ready.value || isLoaded(tab) || isTabLoading(tab)) return
  if (tab === 'images') await loadImages()
  else if (tab === 'volumes') await loadVolumes()
  else if (tab === 'networks') await loadNetworks()
  else await loadContainers()
}

async function reload(tab: DockerTab): Promise<void> {
  if (!props.sessionId || !ready.value) return
  if (tab === 'images') await loadImages()
  else if (tab === 'volumes') await loadVolumes()
  else if (tab === 'networks') await loadNetworks()
  else await loadContainers()
}

async function initialize(): Promise<void> {
  if (!props.sessionId) {
    enabled.value = null
    environment.value = null
    loading.value = false
    resetTabs()
    return
  }
  loading.value = true
  error.value = ''
  enableError.value = ''
  try {
    const allowed = await props.gateway.panelEnabled(props.sessionId)
    enabled.value = allowed
    if (!allowed) {
      environment.value = null
      resetTabs()
      return
    }
    const env = await props.gateway.environment(props.sessionId)
    environment.value = env
    if (!env.available) {
      resetTabs()
      return
    }

    await loadContainers()
    if (activeTab.value !== 'containers') await ensureLoaded(activeTab.value)
  } catch (cause) {
    if (isNotEnabled(cause)) {
      showEnableCard()
      return
    }
    environment.value = null
    resetTabs()
    error.value = errorMessage(cause, t('shell.docker.actionFailed'))
  } finally {
    loading.value = false
  }
}

async function enable(): Promise<void> {
  if (enabling.value || !props.sessionId || !props.hostId) return
  enabling.value = true
  enableError.value = ''
  try {
    await props.gateway.setPanelEnabled(props.hostId, true)
    enabled.value = true
    await initialize()
  } catch (cause) {
    enableError.value = errorMessage(cause, t('shell.docker.actionFailed'))
  } finally {
    enabling.value = false
  }
}

async function refresh(): Promise<void> {
  if (!props.sessionId || refreshing.value) return
  if (!ready.value) {
    await initialize()
    return
  }
  await reload(activeTab.value)
}

const AUTO_REFRESH_MS = 5000
let autoRefreshTimer: number | undefined

let autoRefreshInFlight = false

async function autoRefresh(): Promise<void> {
  if (autoRefreshInFlight) return
  if (
    !ready.value ||
    busyId.value ||
    deleting.value ||
    deletingGroup.value ||
    removing.value ||
    removingNetwork.value ||
    pruning.value ||
    pruningNetworks.value
  ) {
    return
  }
  if (
    detailTarget.value ||
    deleteTarget.value ||
    imageTarget.value ||
    volumeTarget.value ||
    networkTarget.value ||
    groupDeleteTarget.value ||
    pruneMode.value ||
    pruneNetworksOpen.value
  ) {
    return
  }
  if (typeof document !== 'undefined' && document.hidden) return
  const tab = activeTab.value
  if (isTabLoading(tab)) return
  autoRefreshInFlight = true
  try {
    if (tab === 'images') await loadImages(true)
    else if (tab === 'volumes') await loadVolumes(true)
    else if (tab === 'networks') await loadNetworks(true)
    else await loadContainers(true)
  } finally {
    autoRefreshInFlight = false
  }
}

function startAutoRefresh(): void {
  stopAutoRefresh()
  autoRefreshTimer = window.setInterval(() => void autoRefresh(), AUTO_REFRESH_MS)
}

function stopAutoRefresh(): void {
  if (autoRefreshTimer !== undefined) {
    window.clearInterval(autoRefreshTimer)
    autoRefreshTimer = undefined
  }
}

function selectTab(value: string): void {
  if (value === 'containers' || value === 'images' || value === 'volumes' || value === 'networks')
    activeTab.value = value
}

watch(
  () => props.sessionId,
  () => {
    activeTab.value = 'containers'
    actionError.value = ''
    detailTarget.value = null
    deleteTarget.value = null
    imageTarget.value = null
    volumeTarget.value = null
    networkTarget.value = null

    imageDetailTarget.value = null
    volumeDetailTarget.value = null
    networkDetailTarget.value = null
    pruneMode.value = ''
    pruneNetworksOpen.value = false
    expandedGroupIds.value = []

    reset()
    void initialize()
  },
)

watch(activeTab, (tab) => {
  actionError.value = ''

  reset()
  void ensureLoaded(tab)
})

async function runAction(container: DockerContainer, action: DockerContainerAction): Promise<void> {
  if (busyId.value) return
  busyId.value = container.id
  busyAction.value = action
  actionError.value = ''
  try {
    await props.gateway.containerAction(props.sessionId, container.id, action, false)

    await loadContainers()
  } catch (cause) {
    if (isNotEnabled(cause)) {
      showEnableCard()
      return
    }
    actionError.value = errorMessage(cause, t('shell.docker.actionFailed'))
  } finally {
    busyId.value = ''
    busyAction.value = ''
  }
}

async function runComposeAction(group: DockerContainerGroup, action: DockerComposeAction): Promise<void> {
  if (busyId.value) return
  const anchor = group.containers[0]
  if (!anchor) {
    actionError.value = t('shell.docker.actionFailed')
    return
  }
  busyId.value = group.id
  busyAction.value = action
  actionError.value = ''
  try {
    await props.gateway.composeAction(props.sessionId, anchor.id, action)
    await loadContainers()
  } catch (cause) {
    if (isNotEnabled(cause)) {
      showEnableCard()
      return
    }
    actionError.value = errorMessage(cause, t('shell.docker.actionFailed'))
  } finally {
    busyId.value = ''
    busyAction.value = ''
  }
}

async function openTerminalCommand(
  container: DockerContainer,
  run: (sessionId: string, containerId: string) => Promise<void>,
  failureKey: string,
): Promise<void> {
  if (busyId.value) return
  busyId.value = container.id
  actionError.value = ''
  try {
    await run(props.sessionId, container.id)
  } catch (cause) {
    if (isNotEnabled(cause)) {
      showEnableCard()
      return
    }
    actionError.value = errorMessage(cause, t(failureKey))
  } finally {
    busyId.value = ''
    busyAction.value = ''
  }
}

function openTerminal(container: DockerContainer): Promise<void> {
  return openTerminalCommand(
    container,
    (sessionId, containerId) => props.gateway.openContainerTerminal(sessionId, containerId),
    'shell.docker.terminalFailed',
  )
}

function openLogs(container: DockerContainer): Promise<void> {
  return openTerminalCommand(
    container,
    (sessionId, containerId) => props.gateway.openContainerLogs(sessionId, containerId),
    'shell.docker.logsFailed',
  )
}

function openStats(container: DockerContainer): Promise<void> {
  return openTerminalCommand(
    container,
    (sessionId, containerId) => props.gateway.openContainerStats(sessionId, containerId),
    'shell.docker.statsFailed',
  )
}

function openGroupLogs(group: DockerContainerGroup): Promise<void> {
  return openGroupTerminalCommand(
    group,
    (sessionId, containerId) => props.gateway.openComposeLogs(sessionId, containerId),
    'shell.docker.logsFailed',
  )
}

function openGroupStats(group: DockerContainerGroup): Promise<void> {
  return openGroupTerminalCommand(
    group,
    (sessionId, containerId) => props.gateway.openComposeStats(sessionId, containerId),
    'shell.docker.statsFailed',
  )
}

async function openGroupTerminalCommand(
  group: DockerContainerGroup,
  run: (sessionId: string, containerId: string) => Promise<void>,
  failureKey: string,
): Promise<void> {
  const anchor = group.containers[0]
  if (!anchor || busyId.value) return
  busyId.value = group.id
  actionError.value = ''
  try {
    await run(props.sessionId, anchor.id)
  } catch (cause) {
    if (isNotEnabled(cause)) {
      showEnableCard()
      return
    }
    actionError.value = errorMessage(cause, t(failureKey))
  } finally {
    busyId.value = ''
  }
}

async function editComposeFile(group: DockerContainerGroup): Promise<void> {
  const anchor = group.containers[0]
  if (!anchor || busyId.value) return
  busyId.value = group.id
  actionError.value = ''
  try {
    const path = await props.gateway.composeFilePath(props.sessionId, anchor.id)
    if (!path) {
      actionError.value = t('shell.docker.composeFileMissing')
      return
    }
    requestRemoteTextEditor(props.sessionId, path)
  } catch (cause) {
    if (isNotEnabled(cause)) {
      showEnableCard()
      return
    }
    actionError.value = errorMessage(cause, t('shell.docker.composeFileMissing'))
  } finally {
    busyId.value = ''
  }
}

function requestDeleteGroup(group: DockerContainerGroup): void {
  groupDeleteError.value = ''
  groupDeleteTarget.value = group
}

async function confirmDeleteGroup(): Promise<void> {
  const group = groupDeleteTarget.value
  if (!group || deletingGroup.value) return
  const anchor = group.containers[0]
  if (!anchor) {
    groupDeleteTarget.value = null
    groupDeleteError.value = ''
    actionError.value = t('shell.docker.actionFailed')
    return
  }
  deletingGroup.value = true
  groupDeleteError.value = ''
  try {
    await props.gateway.composeAction(props.sessionId, anchor.id, 'down')
    groupDeleteTarget.value = null
    await loadContainers()
  } catch (cause) {
    if (isNotEnabled(cause)) {
      groupDeleteTarget.value = null
      showEnableCard()
      return
    }
    groupDeleteError.value = errorMessage(cause, t('shell.docker.actionFailed'))
  } finally {
    deletingGroup.value = false
  }
}

function requestDelete(container: DockerContainer): void {
  deleteError.value = ''
  deleteTarget.value = container
}

async function confirmDelete(): Promise<void> {
  const container = deleteTarget.value
  if (!container || deleting.value) return
  deleting.value = true
  deleteError.value = ''
  try {
    await props.gateway.containerAction(props.sessionId, container.id, 'remove', container.state === 'running')
    deleteTarget.value = null
    await loadContainers()
  } catch (cause) {
    if (isNotEnabled(cause)) {
      deleteTarget.value = null
      showEnableCard()
      return
    }
    deleteError.value = errorMessage(cause, t('shell.docker.actionFailed'))
  } finally {
    deleting.value = false
  }
}

function requestRemoveImage(image: DockerImage): void {
  if (removing.value) return
  removeError.value = ''
  imageTarget.value = image
}

async function confirmRemoveImage(): Promise<void> {
  const image = imageTarget.value
  if (!image || removing.value) return
  removing.value = true
  busyId.value = image.id
  removeError.value = ''
  try {
    await props.gateway.removeImage(props.sessionId, dockerRemoveTarget(image))
    imageTarget.value = null

    await loadImages()
  } catch (cause) {
    if (isNotEnabled(cause)) {
      imageTarget.value = null
      showEnableCard()
      return
    }
    removeError.value = errorMessage(cause, t('shell.docker.actionFailed'))
  } finally {
    removing.value = false
    busyId.value = ''
  }
}

function requestRemoveVolume(volume: DockerVolume): void {
  if (removing.value) return
  removeError.value = ''
  volumeTarget.value = volume
}

async function confirmRemoveVolume(): Promise<void> {
  const volume = volumeTarget.value
  if (!volume || removing.value) return
  removing.value = true
  busyId.value = volume.name
  removeError.value = ''
  try {
    await props.gateway.removeVolume(props.sessionId, volume.name)
    volumeTarget.value = null
    await loadVolumes()
  } catch (cause) {
    if (isNotEnabled(cause)) {
      volumeTarget.value = null
      showEnableCard()
      return
    }
    removeError.value = errorMessage(cause, t('shell.docker.actionFailed'))
  } finally {
    removing.value = false
    busyId.value = ''
  }
}

function requestPrune(mode: DockerPruneMode): void {
  if (pruning.value) return
  pruneError.value = ''
  pruneMode.value = mode
}

async function confirmPrune(): Promise<void> {
  const mode = pruneMode.value
  if (!mode || pruning.value) return
  pruning.value = true
  pruneError.value = ''
  try {
    await props.gateway.pruneImages(props.sessionId, mode === 'unused')
    pruneMode.value = ''
    await loadImages()
  } catch (cause) {
    if (isNotEnabled(cause)) {
      pruneMode.value = ''
      showEnableCard()
      return
    }
    pruneError.value = errorMessage(cause, t('shell.docker.actionFailed'))
  } finally {
    pruning.value = false
  }
}

function requestRemoveNetwork(network: DockerNetwork): void {
  if (removingNetwork.value) return
  networkDeleteError.value = ''
  networkTarget.value = network
}

async function confirmRemoveNetwork(): Promise<void> {
  const network = networkTarget.value
  if (!network || removingNetwork.value) return
  removingNetwork.value = true
  busyId.value = network.id
  networkDeleteError.value = ''
  try {
    await props.gateway.removeNetwork(props.sessionId, network.id)
    networkTarget.value = null

    await loadNetworks()
  } catch (cause) {
    if (isNotEnabled(cause)) {
      networkTarget.value = null
      showEnableCard()
      return
    }
    networkDeleteError.value = errorMessage(cause, t('shell.docker.actionFailed'))
  } finally {
    removingNetwork.value = false
    busyId.value = ''
  }
}

function requestPruneVolumes(): void {
  if (pruningVolumes.value) return
  pruneVolumesError.value = ''
  pruneVolumesOpen.value = true
}

async function confirmPruneVolumes(): Promise<void> {
  if (!pruneVolumesOpen.value || pruningVolumes.value) return
  pruningVolumes.value = true
  pruneVolumesError.value = ''
  try {
    await props.gateway.pruneVolumes?.(props.sessionId)
    pruneVolumesOpen.value = false
    await loadVolumes()
  } catch (cause) {
    if (isNotEnabled(cause)) {
      pruneVolumesOpen.value = false
      showEnableCard()
      return
    }
    pruneVolumesError.value = errorMessage(cause, t('shell.docker.actionFailed'))
  } finally {
    pruningVolumes.value = false
  }
}

function requestPruneNetworks(): void {
  if (pruningNetworks.value) return
  pruneNetworksError.value = ''
  pruneNetworksOpen.value = true
}

async function confirmPruneNetworks(): Promise<void> {
  if (!pruneNetworksOpen.value || pruningNetworks.value) return
  pruningNetworks.value = true
  pruneNetworksError.value = ''
  try {
    await props.gateway.pruneNetworks(props.sessionId)
    pruneNetworksOpen.value = false
    await loadNetworks()
  } catch (cause) {
    if (isNotEnabled(cause)) {
      pruneNetworksOpen.value = false
      showEnableCard()
      return
    }
    pruneNetworksError.value = errorMessage(cause, t('shell.docker.actionFailed'))
  } finally {
    pruningNetworks.value = false
  }
}

let observer: ResizeObserver | undefined

onMounted(() => {
  if (typeof ResizeObserver !== 'undefined' && root.value) {
    observer = new ResizeObserver((entries) => {
      const width = entries[0]?.contentRect?.width
      if (typeof width === 'number' && width > 0) panelWidth.value = width
    })
    observer.observe(root.value)
  }
  void initialize()

  startAutoRefresh()
})

onBeforeUnmount(() => {
  stopAutoRefresh()
  observer?.disconnect()
  observer = undefined
})
</script>

<template>
  <div
    ref="root"
    class="flex h-full w-full min-h-0 min-w-0 max-w-full flex-col overflow-hidden"
    data-testid="docker-panel"
  >
    <header class="flex h-9 shrink-0 items-center justify-between gap-2 border-b border-qt-border px-2">
      <p class="m-0 min-w-0 truncate text-xs text-qt-muted" data-testid="docker-subtitle">{{ subtitle }}</p>

      <IconButton
        size="icon"
        :label="t('shell.docker.refresh')"
        :disabled="refreshing || !props.sessionId"
        :aria-busy="refreshing"
        @click="refresh"
      >
        <RefreshCw :size="16" class="size-4" aria-hidden="true" />
      </IconButton>
    </header>

    <div
      v-if="ready"
      class="docker-tabs w-full shrink-0 border-b border-qt-border px-2 py-1.5"
      data-testid="docker-tabs"
    >
      <SegmentedTabs
        :model-value="activeTab"
        :options="tabOptions"
        :label="t('shell.docker.tabs')"
        @update:model-value="selectTab"
      />
    </div>

    <ScrollArea class="min-h-0 min-w-0 w-full max-w-full flex-1 overflow-hidden" type="hover" :scroll-hide-delay="600">
      <div v-if="enabled === false" class="flex flex-col gap-2 px-2 py-2" data-testid="docker-enable">
        <QtermEmptyState
          variant="outlined"
          class="px-3 py-3"
          :icon="Container"
          :title="t('shell.docker.enableTitle')"
          :description="t('shell.docker.enableHint')"
        >
          <QtermButton
            size="sm"
            class="mt-2"
            :disabled="enabling || !props.hostId"
            :aria-busy="enabling"
            @click="enable"
          >
            {{ t('shell.docker.enableAction') }}
          </QtermButton>
        </QtermEmptyState>
        <QtermFormMessage v-if="enableError" variant="error">{{ enableError }}</QtermFormMessage>
      </div>

      <QtermEmptyState
        v-else-if="unavailable"
        :icon="Container"
        :title="t('shell.docker.unavailable')"
        :description="unavailableDescription"
        data-testid="docker-unavailable"
      >
        <QtermButton variant="outline" size="sm" class="mt-2" @click="initialize">{{
          t('shell.docker.retry')
        }}</QtermButton>
      </QtermEmptyState>

      <template v-else>
        <QtermFormMessage v-if="actionError" variant="error" class="mx-3 mt-2">{{ actionError }}</QtermFormMessage>

        <template v-if="activeTab === 'containers'">
          <QtermEmptyState
            v-if="tabLoading && !containers.length"
            variant="outlined"
            role="status"
            busy
            :icon="Loader2"
            icon-class="spin"
            class="mx-1 my-2 px-3"
            :description="t('shell.docker.loading')"
            data-testid="docker-loading"
          />

          <QtermEmptyState
            v-else-if="currentError || error"
            variant="outlined"
            role="alert"
            :icon="CircleAlert"
            class="mx-1 my-2 px-3"
            :description="currentError || error"
            data-testid="docker-error"
          />

          <QtermEmptyState
            v-else-if="!groups.length"
            variant="outlined"
            :icon="Container"
            :title="t('shell.docker.empty')"
            :description="t('shell.docker.emptyHint')"
            class="mx-1 my-2 px-3"
            data-testid="docker-empty"
          >
            <QtermButton
              size="sm"
              variant="outline"
              class="mt-2"
              :disabled="refreshing || !props.sessionId"
              @click="refresh"
            >
              {{ t('shell.docker.refresh') }}
            </QtermButton>
          </QtermEmptyState>

          <div v-else>
            <Accordion
              v-if="composeGroups.length"
              v-model="openGroupIds"
              type="multiple"
              class="flex flex-col gap-2 px-2 pt-2"
            >
              <AccordionItem
                v-for="group in composeGroups"
                :key="group.id"
                :value="group.id"
                class="overflow-hidden rounded-[8px] border border-qt-border"
              >
                <ContextMenu>
                  <ContextMenuTrigger as-child>
                    <div class="flex items-center">
                      <AccordionTrigger class="min-w-0 flex-1 items-center justify-start gap-2 px-2.5 py-2">
                        <template #icon><span class="hidden" /></template>
                        <ChevronRight
                          :size="14"
                          class="shrink-0 text-qt-muted group-aria-expanded/accordion-trigger:hidden"
                          aria-hidden="true"
                        />
                        <ChevronDown
                          :size="14"
                          class="hidden shrink-0 text-qt-muted group-aria-expanded/accordion-trigger:inline"
                          aria-hidden="true"
                        />
                        <span class="flex min-w-0 flex-1 flex-col gap-0.5 text-left">
                          <span class="flex min-w-0 items-center gap-2">
                            <span class="truncate text-sm font-medium text-qt-text">{{ group.name }}</span>
                            <small class="shrink-0 rounded-full bg-qt-hover px-1.5 text-xs leading-4 text-qt-muted"
                              >{{ group.running }}/{{ group.total }}</small
                            >
                          </span>
                          <span v-if="group.directory" class="truncate text-xs text-qt-muted">{{
                            group.directory
                          }}</span>
                        </span>
                      </AccordionTrigger>
                    </div>
                  </ContextMenuTrigger>

                  <DockerGroupMenu
                    :running="group.running > 0"
                    :busy="!!busyId"
                    :busy-action="composeBusyAction(group)"
                    @start="runComposeAction(group, 'up')"
                    @stop="runComposeAction(group, 'stop')"
                    @restart="runComposeAction(group, 'restart')"
                    @logs="openGroupLogs(group)"
                    @stats="openGroupStats(group)"
                    @compose="editComposeFile(group)"
                    @remove="requestDeleteGroup(group)"
                  />
                </ContextMenu>
                <AccordionContent class="pt-1 pb-1">
                  <DockerContainerMenu
                    v-for="container in visibleItems(group.id, group.containers, isComposeGroupExpanded(group))"
                    :key="container.id"
                    :container="container"
                    :busy="busyId === container.id || busyId === group.id"
                    @action="(action) => runAction(container, action)"
                    @detail="detailTarget = container"
                    @logs="openLogs(container)"
                    @stats="openStats(container)"
                    @terminal="openTerminal(container)"
                    @remove="requestDelete(container)"
                  />
                  <QtermLoadMore
                    class="px-2.5 py-1.5"
                    :visible="visibleCount(group.id, group.containers.length, isComposeGroupExpanded(group))"
                    :total="group.containers.length"
                    :load-more-label="t('shell.common.loadMore')"
                    @load-more="loadMore(group.id, group.containers.length)"
                  />
                </AccordionContent>
              </AccordionItem>
            </Accordion>

            <section v-if="standaloneGroup" class="mx-2 mt-2 overflow-hidden rounded-[8px] border border-qt-border">
              <header class="flex min-w-0 items-center gap-2 px-2.5 py-2">
                <span class="truncate text-sm font-medium text-qt-text">{{ t('shell.docker.standalone') }}</span>
                <small class="shrink-0 rounded-full bg-qt-hover px-1.5 text-xs leading-4 text-qt-muted"
                  >{{ standaloneGroup.running }}/{{ standaloneGroup.total }}</small
                >
              </header>

              <div>
                <DockerContainerMenu
                  v-for="container in visibleItems(groupKeyOfStandalone, standaloneGroup.containers, true)"
                  :key="container.id"
                  :container="container"
                  :busy="busyId === container.id"
                  @action="(action) => runAction(container, action)"
                  @detail="detailTarget = container"
                  @logs="openLogs(container)"
                  @stats="openStats(container)"
                  @terminal="openTerminal(container)"
                  @remove="requestDelete(container)"
                />
                <QtermLoadMore
                  class="px-2.5 py-1.5"
                  :visible="visibleCount(groupKeyOfStandalone, standaloneGroup.containers.length, true)"
                  :total="standaloneGroup.containers.length"
                  :load-more-label="t('shell.common.loadMore')"
                  @load-more="loadMore(groupKeyOfStandalone, standaloneGroup.containers.length)"
                />
              </div>
            </section>
          </div>
        </template>

        <template v-else-if="activeTab === 'images'">
          <div
            class="flex items-center justify-end gap-1 border-b border-qt-border px-2 py-2"
            data-testid="docker-image-tools"
          >
            <QtermButton
              variant="outline"
              size="xs"
              :disabled="pruning || !!busyId || imageTab.loading"
              @click="requestPrune('dangling')"
            >
              {{ t('shell.docker.pruneDangling') }}
            </QtermButton>
            <QtermButton
              variant="destructive"
              size="xs"
              :disabled="!unusedImages.length || pruning || !!busyId"
              @click="requestPrune('unused')"
            >
              {{ t('shell.docker.pruneUnused') }}
            </QtermButton>
          </div>

          <QtermEmptyState
            v-if="imageTab.loading && !imageTab.items.length"
            variant="outlined"
            role="status"
            busy
            :icon="Loader2"
            icon-class="spin"
            class="mx-1 my-2 px-3"
            :description="t('shell.docker.loadingImages')"
            data-testid="docker-images-loading"
          />

          <QtermEmptyState
            v-else-if="imageTab.error"
            variant="outlined"
            role="alert"
            :icon="CircleAlert"
            class="mx-1 my-2 px-3"
            :description="imageTab.error"
            data-testid="docker-images-error"
          />

          <QtermEmptyState
            v-else-if="!imageTab.items.length"
            variant="outlined"
            :icon="Package"
            :title="t('shell.docker.imagesEmpty')"
            :description="t('shell.docker.imagesEmptyHint')"
            class="mx-1 my-2 px-3"
            data-testid="docker-images-empty"
          >
            <QtermButton
              size="sm"
              variant="outline"
              class="mt-2"
              :disabled="refreshing || !props.sessionId"
              @click="refresh"
            >
              {{ t('shell.docker.refresh') }}
            </QtermButton>
          </QtermEmptyState>

          <div v-else data-testid="docker-images">
            <section
              v-for="group in imageGroups"
              :key="group.id"
              :data-image-group="group.id"
              class="mx-2 mt-2 overflow-hidden rounded-[8px] border border-qt-border"
            >
              <header class="flex min-w-0 items-center gap-2 px-2.5 py-2">
                <span class="truncate text-sm font-medium text-qt-text">{{ group.title }}</span>
                <small class="shrink-0 rounded-full bg-qt-hover px-1.5 text-xs leading-4 text-qt-muted">{{
                  group.items.length
                }}</small>
              </header>

              <DockerImageRow
                v-for="image in visibleItems(imageGroupKey(group.id), group.items, true)"
                :key="`${image.id}:${image.repository}:${image.tag}`"
                :image="image"
                :busy="busyId === image.id"
                @detail="imageDetailTarget = image"
                @remove="requestRemoveImage(image)"
              />
              <QtermLoadMore
                class="px-2.5 py-1.5"
                :visible="visibleCount(imageGroupKey(group.id), group.items.length, true)"
                :total="group.items.length"
                :load-more-label="t('shell.common.loadMore')"
                @load-more="loadMore(imageGroupKey(group.id), group.items.length)"
              />
            </section>
          </div>
        </template>

        <template v-else-if="activeTab === 'volumes'">
          <QtermEmptyState
            v-if="volumeTab.loading && !volumeTab.items.length"
            variant="outlined"
            role="status"
            busy
            :icon="Loader2"
            icon-class="spin"
            class="mx-1 my-2 px-3"
            :description="t('shell.docker.loadingVolumes')"
            data-testid="docker-volumes-loading"
          />

          <QtermEmptyState
            v-else-if="volumeTab.error"
            variant="outlined"
            role="alert"
            :icon="CircleAlert"
            class="mx-1 my-2 px-3"
            :description="volumeTab.error"
            data-testid="docker-volumes-error"
          />

          <QtermEmptyState
            v-else-if="!volumeTab.items.length"
            variant="outlined"
            :icon="Database"
            :title="t('shell.docker.volumesEmpty')"
            :description="t('shell.docker.volumesEmptyHint')"
            class="mx-1 my-2 px-3"
            data-testid="docker-volumes-empty"
          >
            <QtermButton
              size="sm"
              variant="outline"
              class="mt-2"
              :disabled="refreshing || !props.sessionId"
              @click="refresh"
            >
              {{ t('shell.docker.refresh') }}
            </QtermButton>
          </QtermEmptyState>

          <template v-else>
            <div
              class="flex items-center justify-end gap-1 border-b border-qt-border px-2 py-2"
              data-testid="docker-volume-tools"
            >
              <QtermButton
                variant="destructive"
                size="xs"
                :disabled="!unusedVolumes.length || pruningVolumes || !!busyId"
                @click="requestPruneVolumes"
              >
                {{ t('shell.docker.pruneVolumes') }}
              </QtermButton>
            </div>

            <div data-testid="docker-volumes">
              <section
                v-for="group in volumeGroups"
                :key="group.id"
                :data-volume-group="group.id"
                class="mx-2 mt-2 overflow-hidden rounded-[8px] border border-qt-border"
              >
                <header class="flex min-w-0 items-center gap-2 px-2.5 py-2">
                  <span class="truncate text-sm font-medium text-qt-text">{{ group.title }}</span>
                  <small class="shrink-0 rounded-full bg-qt-hover px-1.5 text-xs leading-4 text-qt-muted">{{
                    group.items.length
                  }}</small>
                </header>
                <DockerVolumeRow
                  v-for="volume in visibleItems(volumeGroupKey(group.id), group.items, true)"
                  :key="volume.name"
                  :volume="volume"
                  :busy="busyId === volume.name"
                  @detail="volumeDetailTarget = volume"
                  @remove="requestRemoveVolume(volume)"
                />
                <QtermLoadMore
                  class="px-2.5 py-1.5"
                  :visible="visibleCount(volumeGroupKey(group.id), group.items.length, true)"
                  :total="group.items.length"
                  :load-more-label="t('shell.common.loadMore')"
                  @load-more="loadMore(volumeGroupKey(group.id), group.items.length)"
                />
              </section>
            </div>
          </template>
        </template>

        <template v-else>
          <QtermEmptyState
            v-if="networkTab.loading && !networkTab.items.length"
            variant="outlined"
            role="status"
            busy
            :icon="Loader2"
            icon-class="spin"
            class="mx-1 my-2 px-3"
            :description="t('shell.docker.loadingNetworks')"
            data-testid="docker-networks-loading"
          />

          <QtermEmptyState
            v-else-if="networkTab.error"
            variant="outlined"
            role="alert"
            :icon="CircleAlert"
            class="mx-1 my-2 px-3"
            :description="networkTab.error"
            data-testid="docker-networks-error"
          />

          <QtermEmptyState
            v-else-if="!networkTab.items.length"
            variant="outlined"
            :icon="Network"
            :title="t('shell.docker.networksEmpty')"
            :description="t('shell.docker.networksEmptyHint')"
            class="mx-1 my-2 px-3"
            data-testid="docker-networks-empty"
          >
            <QtermButton
              size="sm"
              variant="outline"
              class="mt-2"
              :disabled="refreshing || !props.sessionId"
              @click="refresh"
            >
              {{ t('shell.docker.refresh') }}
            </QtermButton>
          </QtermEmptyState>

          <template v-else>
            <div
              class="flex items-center justify-end gap-1 border-b border-qt-border px-2 py-2"
              data-testid="docker-network-tools"
            >
              <QtermButton
                variant="destructive"
                size="xs"
                :disabled="!removableUnusedNetworks.length || pruningNetworks || !!busyId"
                @click="requestPruneNetworks"
              >
                {{ t('shell.docker.pruneNetworks') }}
              </QtermButton>
            </div>

            <div data-testid="docker-networks">
              <section
                v-for="group in networkGroups"
                :key="group.id"
                :data-network-group="group.id"
                class="mx-2 mt-2 overflow-hidden rounded-[8px] border border-qt-border"
              >
                <header class="flex min-w-0 items-center gap-2 px-2.5 py-2">
                  <span class="truncate text-sm font-medium text-qt-text">{{ group.title }}</span>
                  <small class="shrink-0 rounded-full bg-qt-hover px-1.5 text-xs leading-4 text-qt-muted">{{
                    group.items.length
                  }}</small>
                </header>
                <DockerNetworkRow
                  v-for="network in visibleItems(networkGroupKey(group.id), group.items, true)"
                  :key="network.id || network.name"
                  :network="network"
                  :busy="busyId === network.id"
                  @detail="networkDetailTarget = network"
                  @remove="requestRemoveNetwork(network)"
                />
                <QtermLoadMore
                  class="px-2.5 py-1.5"
                  :visible="visibleCount(networkGroupKey(group.id), group.items.length, true)"
                  :total="group.items.length"
                  :load-more-label="t('shell.common.loadMore')"
                  @load-more="loadMore(networkGroupKey(group.id), group.items.length)"
                />
              </section>
            </div>
          </template>
        </template>
      </template>
    </ScrollArea>

    <DockerContainerDetailDialog
      :open="!!detailTarget"
      :gateway="props.gateway"
      :session-id="props.sessionId"
      :container-id="detailTarget?.id ?? ''"
      :container-name="detailTarget?.name ?? ''"
      @close="detailTarget = null"
    />

    <DockerImageDetailDialog
      :open="!!imageDetailTarget"
      :image="imageDetailTarget"
      :session-id="props.sessionId"
      :gateway="props.gateway"
      @close="imageDetailTarget = null"
    />
    <DockerVolumeDetailDialog
      :open="!!volumeDetailTarget"
      :volume="volumeDetailTarget"
      :session-id="props.sessionId"
      :gateway="props.gateway"
      @close="volumeDetailTarget = null"
    />
    <DockerNetworkDetailDialog
      :open="!!networkDetailTarget"
      :network="networkDetailTarget"
      :session-id="props.sessionId"
      :gateway="props.gateway"
      @close="networkDetailTarget = null"
    />

    <DeleteConfirmDialog
      :open="!!deleteTarget"
      :title="t('shell.docker.confirmDeleteTitle')"
      :message="deleteTarget ? t('shell.docker.confirmDeleteMessage', { name: deleteTarget.name }) : ''"
      :hint="deleteTarget?.state === 'running' ? t('shell.docker.confirmDeleteRunning') : ''"
      :error="deleteError"
      :confirm-text="t('shell.docker.delete')"
      :loading="deleting"
      @close="deleteTarget = null"
      @confirm="confirmDelete"
    />

    <DeleteConfirmDialog
      :open="!!imageTarget"
      :title="t('shell.docker.confirmRemoveImageTitle')"
      :message="imageTarget ? t('shell.docker.confirmRemoveImageMessage', { name: imageLabel(imageTarget) }) : ''"
      :hint="imageTarget?.inUse ? t('shell.docker.inUseImageHint') : ''"
      :error="removeError"
      :confirm-text="t('shell.docker.removeImage')"
      :loading="removing"
      @close="imageTarget = null"
      @confirm="confirmRemoveImage"
    />

    <DeleteConfirmDialog
      :open="!!volumeTarget"
      :title="t('shell.docker.confirmRemoveVolumeTitle')"
      :message="volumeTarget ? t('shell.docker.confirmRemoveVolumeMessage', { name: volumeTarget.name }) : ''"
      :hint="volumeTarget?.inUse ? t('shell.docker.inUseVolumeHint') : ''"
      :error="removeError"
      :confirm-text="t('shell.docker.removeVolume')"
      :loading="removing"
      @close="volumeTarget = null"
      @confirm="confirmRemoveVolume"
    />

    <DeleteConfirmDialog
      :open="!!pruneMode"
      :title="t('shell.docker.confirmPruneTitle')"
      :message="pruneMessage"
      :hint="pruneMode === 'unused' ? t('shell.docker.pruneUnusedWarning') : ''"
      :items="pruneItems"
      :error="pruneError"
      :confirm-text="pruneMode === 'unused' ? t('shell.docker.pruneUnused') : t('shell.docker.pruneDangling')"
      :loading="pruning"
      @close="pruneMode = ''"
      @confirm="confirmPrune"
    />

    <DeleteConfirmDialog
      :open="!!networkTarget"
      :title="t('shell.docker.confirmRemoveNetworkTitle')"
      :message="networkTarget ? t('shell.docker.confirmRemoveNetworkMessage', { name: networkTarget.name }) : ''"
      :hint="networkTarget?.inUse ? t('shell.docker.inUseNetworkHint') : ''"
      :error="networkDeleteError"
      :confirm-text="t('shell.docker.removeNetwork')"
      :loading="removingNetwork"
      @close="networkTarget = null"
      @confirm="confirmRemoveNetwork"
    />

    <DeleteConfirmDialog
      :open="pruneVolumesOpen"
      :title="t('shell.docker.confirmPruneVolumesTitle')"
      :message="t('shell.docker.confirmPruneVolumesMessage', { count: unusedVolumes.length })"
      :hint="t('shell.docker.pruneVolumesWarning')"
      :items="pruneVolumeItems"
      :error="pruneVolumesError"
      :confirm-text="t('shell.docker.pruneVolumes')"
      :loading="pruningVolumes"
      @close="pruneVolumesOpen = false"
      @confirm="confirmPruneVolumes"
    />

    <DeleteConfirmDialog
      :open="pruneNetworksOpen"
      :title="t('shell.docker.confirmPruneNetworksTitle')"
      :message="t('shell.docker.confirmPruneNetworksMessage', { count: removableUnusedNetworks.length })"
      :hint="t('shell.docker.pruneNetworksWarning')"
      :items="pruneNetworkItems"
      :error="pruneNetworksError"
      :confirm-text="t('shell.docker.pruneNetworks')"
      :loading="pruningNetworks"
      @close="pruneNetworksOpen = false"
      @confirm="confirmPruneNetworks"
    />

    <DeleteConfirmDialog
      :open="!!groupDeleteTarget"
      :title="t('shell.docker.deleteGroupTitle')"
      :message="t('shell.docker.deleteGroupMessage')"
      :hint="t('shell.docker.deleteGroupHint')"
      :items="groupDeleteTarget ? [groupDeleteTarget.name] : []"
      :error="groupDeleteError"
      :confirm-text="t('shell.docker.deleteGroup')"
      :loading="deletingGroup"
      @close="groupDeleteTarget = null"
      @confirm="confirmDeleteGroup"
    />
  </div>
</template>

<style scoped>
.docker-tabs :deep(.qterm-segmented-tabs__list) {
  background: color-mix(in srgb, var(--qt-panel) 16%, transparent);
}

.docker-tabs :deep(.qterm-segmented-tabs__trigger)[data-state='active'] {
  background: color-mix(in srgb, var(--qt-panel) 46%, transparent);
  border: 1px solid color-mix(in srgb, var(--qt-text) 18%, transparent);
}

[data-testid='docker-panel'] :deep([data-slot='scroll-area-viewport'] > div) {
  display: block !important;
  width: 100%;
  min-width: 0;
  max-width: 100%;
}

.docker-tabs :deep(.qterm-segmented-tabs),
.docker-tabs :deep(.qterm-segmented-tabs__list) {
  width: 100%;
  min-width: 0;
}

.docker-tabs :deep(.qterm-segmented-tabs__list) {
  justify-content: flex-start;
  overflow-x: auto;
  scrollbar-width: none;
}

.docker-tabs :deep(.qterm-segmented-tabs__list::-webkit-scrollbar) {
  display: none;
}

.docker-tabs :deep(.qterm-segmented-tabs__trigger) {
  flex: 1 0 auto;
  min-width: max-content;
}
</style>
