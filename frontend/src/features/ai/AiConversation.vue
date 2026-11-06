<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { ArrowDown, Check, ChevronDown, Copy, History, RefreshCw, Sparkles, SquarePen, Terminal } from '@lucide/vue'
import { IconButton, QtermButton } from '../../components/common'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../../components/common/dialog'
import { ScrollArea } from '../../components/common/control-primitives'
import {
  CodeBlock,
  CodeBlockContent,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
  Message,
  MessageContent,
  MessageGroup,
  MessageFooter,
  Trace,
  TraceCollapsible,
  TraceCollapsibleContent,
  TraceCollapsibleTrigger,
  TraceTitle,
} from '../../components/common/ai'
import { useI18n } from '../../locales'
import { toast } from 'vue-sonner'
import {
  aiErrorMessage,
  normalizeContextBreakdown,
  runAIBackgroundCommand,
  startChatStream,
  type AIContextBreakdown,
  type AIMessage,
  type AIUsage,
} from '../../services/ai'
import {
  destructiveCommandReason,
  isClearlyReadOnlyCommand,
  reviewCommandWithAI,
  readApprovalMode,
  writeApprovalMode,
  type AIApprovalMode,
  type AIExecutionChannel,
} from '../../services/aiApproval'
import { translateErrorKey } from '../../services/errors'
import { recordAudit } from '../../services/aiAudit'
import { clearAILaunch, useAILaunch } from '../../services/aiAgentLaunch'
import { runCommandInSession } from '../sessions/terminalInput'
import { readTerminalTail } from '../sessions/terminalTail'
import { captureCommandOutput } from '../../services/aiCommandCapture'
import { createSessionGateway, type SessionState } from '../../services/sessions'
import AiCommandCard from './AiCommandCard.vue'
import AiConfirmation from './AiConfirmation.vue'
import AiContextDialog from './AiContextDialog.vue'
import AiContextSourcesDialog from './AiContextSourcesDialog.vue'
import {
  addContextSource,
  buildContext,
  byteLength,
  readContextFile,
  truncateUtf8Tail,
  AI_CONTEXT_BUDGET_BYTES,
  type AiContextSource,
  type ContextSourceInput,
  type ContextSourceErrorCode,
} from '../../services/aiContextSources'
import AiMessageText from './AiMessageText.vue'
import AiComposer from './AiComposer.vue'
import { Reasoning, ReasoningContent, ReasoningTrigger } from '../../components/ai-elements/reasoning'
import { Tool } from '../../components/ai-elements/tool'
import { Response } from '../../components/ai-elements/response'
import { Task } from '../../components/ai-elements/task'
import AiHistoryDialog from '../../components/dialogs/AiHistoryDialog.vue'
import {
  findProvider,
  readActiveProviderID,
  readArchivedChats,
  readHistory,
  readProviders,
  writeActiveProviderID,
  writeProviders,
  writeHistory,
  type AIProvider,
  type ArchivedChat,
  type StoredMessage,
} from '../../services/aiProviders'
import {
  activateConversation,
  createConversation,
  listConversations,
  listLegacyConversations,
  migrateSessionConversations,
  makeMessage,
  readActiveConversation,
  readConversation,
  readLegacyAgentChats,
  removeConversation,
  saveConversation,
  type AiConversationRecord,
  type ConversationMessage,
} from '../../services/aiConversationStore'
import { useAiAgentTurn, type AiAgentTurnIdentity } from '../../composables/useAiAgentTurn'
import { requestSnippetCreator } from '../../services/snippetCreator'
import {
  extractThinkContent,
  looksLikeStructuredReply,
  parseStructuredReply,
  type StructuredReply,
} from '../../services/aiStructuredOutput'
import {
  readAgentSettings,
  writeAgentSettings,
  RunState,
  StepKind,
  type AgentRun,
  type AgentStep,
} from '../../services/aiAgent'

const props = defineProps<{
  active: boolean

  terminalContext?: string

  hostId?: string
  sessionId?: string
  sessionTitle?: string
  sessionState?: SessionState

  chatMode?: 'ask' | 'agent'
}>()

const emit = defineEmits<{ 'update:mode': [value: 'ask' | 'agent'] }>()

const { t } = useI18n()

const connectionAvailable = computed(() => props.sessionState === 'connected')

interface ChatMessage extends AIMessage {
  id?: string
  at?: number
  mode?: 'ask' | 'agent'
  providerId?: string
  model?: string
  agentRun?: AgentRun
  manualStepIds?: string[]
  targetTitle?: string

  legacy?: boolean
  reasoning?: string
  output?: string
  executions?: StoredMessage['executions']
  usage?: AIUsage

  feedback?: 'up' | 'down'

  context?: AIContextBreakdown
}

const messages = ref<ChatMessage[]>([])
const conversation = ref<AiConversationRecord | null>(null)
const conversationId = ref('')
const activeAgentMessageId = ref('')
const draft = ref('')
const busy = ref(false)
const error = ref('')
const contextSources = ref<AiContextSource[]>([])
const contextSourcesOpen = ref(false)
const contextSourcesError = ref('')
const contextReading = ref(false)
const requestContextText = ref('')
let contextGeneration = 0
const approvalMode = ref<AIApprovalMode>(readApprovalMode())
const executionChannel = ref<AIExecutionChannel>(readAgentSettings().executionChannel)
const backgroundAvailable = ref(true)
const sessionGateway = createSessionGateway()
const automaticWarningOpen = ref(false)
const executing = ref(false)
const reviewBusy = ref(false)

const requestUsage = ref<AIUsage | null>(null)
const requestStartedAt = ref<number | null>(null)
const requestDurationMs = ref<number | null>(null)
const firstTextDurationMs = ref<number | null>(null)

const contextDetailsOpen = ref(false)
const backgroundCancel = ref<(() => void) | null>(null)
const captureCancel = ref<(() => void) | null>(null)
const scroll = ref<InstanceType<typeof ScrollArea> | null>(null)
const scrollViewport = ref<HTMLElement | null>(null)
const showScrollToLatest = ref(false)
const isNearBottom = ref(true)

const provider = ref<AIProvider | null>(null)
const archivedChats = ref<ArchivedChat[]>([])
const historyOpen = ref(false)
const viewingLegacyHistory = ref(false)

const activeArchiveId = ref('')

const streamSession = ref<{ stop: () => void } | null>(null)
let streamGeneration = 0
let lastScrollTop = 0
let scrollResizeObserver: ResizeObserver | null = null
let disposed = false
const launch = useAILaunch()

function conversationScope() {
  return { hostId: props.hostId, sessionId: props.sessionId, sessionTitle: props.sessionTitle }
}

function applyConversationProvider(record: AiConversationRecord): void {
  const preferred = readProviders().find((item) => item.enabled && item.id === record.providerId)
  if (preferred)
    provider.value = {
      ...preferred,
      model:
        record.model && (preferred.models.includes(record.model) || preferred.model === record.model)
          ? record.model
          : preferred.model,
    }
}

function currentConversationIdentity(): AiAgentTurnIdentity {
  return {
    conversationId: conversationId.value || `legacy:${props.sessionId ?? 'unattached'}`,
    sessionId: props.sessionId ?? '',
    providerId: provider.value?.id ?? '',
  }
}

function agentRunMessage(run: AgentRun): ChatMessage | undefined {
  return messages.value.find((message) => message.agentRun?.id === run.id || message.id === activeAgentMessageId.value)
}

function agentReplyContent(run: AgentRun): string {
  const answer = [...(run.steps ?? [])].reverse().find((step) => step.kind === StepKind.StepKindAnswer)
  if (answer?.observation) return answer.observation
  if (run.state === RunState.RunFailed || run.state === RunState.RunBlocked || run.state === RunState.RunTimedOut) {
    return (
      run.errorDetail ||
      (run.errorKey ? translateErrorKey(run.errorKey, t('shell.aiChat.agent.failed')) : (run.notes ?? []).join('\n'))
    )
  }
  return run.draft || (run.notes ?? []).join('\n')
}

function persistUnifiedConversation(): boolean {
  const record = conversation.value
  if (!record) return false
  if (viewingLegacyHistory.value) return true

  if (messages.value.length && messages.value.every((message) => message.legacy)) return true
  const next: AiConversationRecord = {
    ...record,
    providerId: provider.value?.id,
    model: provider.value?.model,
    updatedAt: Date.now(),
    title:
      messages.value
        .find((message) => message.role === 'user' && !message.legacy)
        ?.content.trim()
        .slice(0, 60) || record.title,
    messages: messages.value
      .filter((message): message is ChatMessage & { id: string } => Boolean(message.id))
      .map((message) => ({
        ...message,
        id: message.id,
        role: message.role as 'user' | 'assistant',
        mode: message.mode ?? 'ask',
      })) as ConversationMessage[],
  }
  if (!saveConversation(next)) {
    error.value = t('shell.aiChat.historySaveFailed')
    return false
  }
  conversation.value = next
  return true
}

function legacyHistoryChats(): ArchivedChat[] {
  const target = provider.value
  if (!target) return []
  const active = readHistory(target.id)
  return [
    ...readArchivedChats(target.id),
    ...readLegacyAgentChats(target.id),
    ...(active.length
      ? [
          {
            id: `legacy-active:${target.id}`,
            title:
              active.find((message) => message.role === 'user')?.content.slice(0, 60) ||
              t('shell.aiChat.historyUntitled'),
            updatedAt: 0,
            messages: active,
          },
        ]
      : []),
  ]
}

function refreshArchivedChats(): void {
  const legacy = legacyHistoryChats()
  const unified = [
    ...listConversations(conversationScope()),
    ...(props.hostId ? listLegacyConversations() : []),
  ].map<ArchivedChat>((record) => ({
    id: record.id,
    title: record.title,
    updatedAt: record.updatedAt,
    messages: record.messages,
    legacy: Boolean(props.hostId && !record.hostId),
    modelLabel: [
      record.providerId ? readProviders().find((item) => item.id === record.providerId)?.name : '',
      record.model,
    ]
      .filter(Boolean)
      .join(' · '),
  }))
  const signature = (chat: ArchivedChat) =>
    JSON.stringify(chat.messages.map(({ role, content }) => ({ role, content })))
  const unifiedSignatures = new Set(unified.map(signature))
  const seen = new Set<string>()
  archivedChats.value = [
    ...unified,
    ...legacy.map((chat) => ({ ...chat, legacy: true })).filter((chat) => !unifiedSignatures.has(signature(chat))),
  ].filter((chat) => {
    if (seen.has(chat.id)) return false
    seen.add(chat.id)
    return true
  })
}

const agentTurn = useAiAgentTurn({
  getIdentity: currentConversationIdentity,
  getProvider: () => provider.value,
  getApprovalMode: () => approvalMode.value,
  getContextText: () => requestContextText.value,
  onRunUpdated: (run, identity) => {
    if (identityKeyForAgent(identity) !== identityKeyForAgent(currentConversationIdentity())) return
    const target = agentRunMessage(run)
    if (!target) return
    messages.value = messages.value.map((message) =>
      message.id === target.id
        ? {
            ...message,
            mode: 'agent' as const,
            agentRun: run,
            manualStepIds: Array.from(
              new Set([
                ...(message.manualStepIds ?? []),
                ...(run.steps ?? []).filter((step) => Boolean(step.pendingId)).map((step) => step.id),
              ]),
            ),
            reasoning: run.notes?.join('\n') || undefined,
            content: agentReplyContent(run),
            usage: agentUsage(run) ?? message.usage,
          }
        : message,
    )
    persistUnifiedConversation()
  },
})

function identityKeyForAgent(identity: AiAgentTurnIdentity): string {
  return `${identity.conversationId}\u0000${identity.sessionId}\u0000${identity.providerId}`
}

function agentRisk(step: AgentStep): 'readonly' | 'confirm' | 'danger' {
  if (step.risk === 'danger') return 'danger'
  if (step.risk === 'readonly') return 'readonly'
  return 'confirm'
}

function shouldRenderAgentCommand(step: AgentStep): boolean {
  return step.kind === StepKind.StepKindCommand && Boolean(step.pendingId)
}

function structuredReply(message: ChatMessage): StructuredReply | null {
  return message.role === 'assistant' && !message.agentRun ? parseStructuredReply(message.content) : null
}

function thinkText(message: ChatMessage): string {
  if (message.role !== 'assistant' || message.agentRun) return ''
  return extractThinkContent(message.content).reasoning
}

function displayText(message: ChatMessage): string {
  if (message.role !== 'assistant' || message.agentRun) return message.content
  const base = structuredReply(message)?.text ?? message.content
  return extractThinkContent(base).visible
}

function insertStructuredCommand(command: string): void {
  if (!connectionAvailable.value || viewingLegacyHistory.value) return
  if (!props.sessionId) return
  if (runCommandInSession(props.sessionId, command)) {
    toast.success(t('shell.aiChat.commandSent'))
  } else {
    toast.error(t('shell.aiChat.runUnavailable'))
  }
}

function messageInFlight(message: ChatMessage, index: number): boolean {
  if (composerBusy.value && index === messages.value.length - 1) return true
  const run = message.agentRun
  if (!run) return false
  return activeAgentRunId.value === run.id && agentTurn.live.value
}

function agentUsage(run: AgentRun): AIUsage | undefined {
  const usage = run.usage
  if (!usage) return undefined
  const next: AIUsage = {}
  if (usage.inputTokens !== null && usage.inputTokens !== undefined) next.inputTokens = usage.inputTokens
  if (usage.outputTokens !== null && usage.outputTokens !== undefined) next.outputTokens = usage.outputTokens
  if (usage.totalTokens !== null && usage.totalTokens !== undefined) next.totalTokens = usage.totalTokens
  if (usage.cacheReadTokens !== null && usage.cacheReadTokens !== undefined)
    next.cacheReadTokens = usage.cacheReadTokens
  if (usage.cacheWriteTokens !== null && usage.cacheWriteTokens !== undefined)
    next.cacheWriteTokens = usage.cacheWriteTokens
  return Object.keys(next).length ? next : undefined
}

function agentRunLive(message: ChatMessage): boolean {
  const state = message.agentRun?.state
  return (
    (state === RunState.RunRunning || state === RunState.RunWaitingApproval) &&
    activeAgentRunId.value === message.agentRun?.id &&
    agentTurn.live.value
  )
}

function interruptedAgentRun(message: ChatMessage): boolean {
  const run = message.agentRun
  return Boolean(
    run &&
    !agentRunLive(message) &&
    (run.state === RunState.RunRunning ||
      run.state === RunState.RunWaitingApproval ||
      run.errorKey === 'ai.agent.run_not_found'),
  )
}

function continueInterrupted(message: ChatMessage): void {
  if (!connectionAvailable.value || composerBusy.value || viewingLegacyHistory.value || !message.agentRun) return
  draft.value = t('shell.aiChat.resumePrompt', { prompt: message.agentRun.prompt })
  emit('update:mode', 'agent')
}

function agentRunStatus(message: ChatMessage): string {
  if (interruptedAgentRun(message)) return t('shell.aiChat.interruptedRun')
  const state = message.agentRun?.state
  if (state === RunState.RunCompleted) return t('shell.aiChat.agent.completed')
  if (state === RunState.RunFailed) return t('shell.aiChat.agent.failed')
  if (state === RunState.RunCancelled) return t('shell.aiChat.agent.cancelled')
  if (state === RunState.RunTimedOut) return t('shell.aiChat.agent.timedOut')
  if (state === RunState.RunBlocked) return t('shell.aiChat.agent.blocked')
  if (state === RunState.RunRejected) return t('shell.aiChat.agent.rejected')
  if (state === RunState.RunWaitingApproval) return t('shell.aiChat.agent.waiting')
  return t('shell.aiChat.agent.running')
}

function agentRunSeconds(message: ChatMessage): number | null {
  const run = message.agentRun
  if (!run || agentRunLive(message)) return null
  const start = Date.parse(run.createdAt)
  const end = Date.parse(run.updatedAt)
  if (Number.isNaN(start) || Number.isNaN(end) || end < start) return null
  return Math.max(1, Math.round((end - start) / 1000))
}

function agentAutomaticSteps(message: ChatMessage): AgentStep[] {
  return (message.agentRun?.steps ?? []).filter(
    (step) => step.kind !== StepKind.StepKindAnswer && !shouldRenderAgentCommand(step),
  )
}

function decideAgentStep(step: AgentStep, approve: boolean): void {
  if (!connectionAvailable.value || viewingLegacyHistory.value) return
  if (!step.pendingId || !props.sessionId) return
  void agentTurn.decide(step.id, step.pendingId, approve).catch((cause) => {
    error.value = streamFailureMessage(cause)
  })
}

const SCROLL_BOTTOM_THRESHOLD = 48

function selectChannel(value: string): void {
  if (composerBusy.value || executing.value || reviewBusy.value) return
  executionChannel.value = value === 'terminal' ? 'terminal' : 'background'
  writeAgentSettings({ ...readAgentSettings(), executionChannel: executionChannel.value })
}

const channelOptions = computed(() => [
  {
    value: 'background',
    label:
      props.sessionId && !backgroundAvailable.value
        ? `${t('shell.aiChat.background')} · ${t('shell.aiChat.backgroundSSHOnly')}`
        : t('shell.aiChat.background'),
  },
  { value: 'terminal', label: t('shell.aiChat.terminalExecution') },
])

const agentBusy = computed(() => agentTurn.live.value || agentTurn.starting.value)
const composerBusy = computed(() => busy.value || agentBusy.value)
const agentDeciding = computed(() => agentTurn.deciding.value)
const activeAgentRunId = computed(() => agentTurn.run.value?.id)
const agentError = computed(() => (agentTurn.error.value ? streamFailureMessage(new Error(agentTurn.error.value)) : ''))

const latestRequestContext = computed<{ breakdown: AIContextBreakdown; mode: 'ask' | 'agent' } | null>(() => {
  for (let index = messages.value.length - 1; index >= 0; index -= 1) {
    const message = messages.value[index]
    const agentContext = normalizeContextBreakdown(message.agentRun?.context)
    if (agentContext) return { breakdown: agentContext, mode: 'agent' }
    const askContext = normalizeContextBreakdown(message.context)
    if (askContext) return { breakdown: askContext, mode: message.mode === 'agent' ? 'agent' : 'ask' }
  }
  return null
})

function updateUsage(value: unknown): void {
  if (!value || typeof value !== 'object') return
  const candidate = value as Partial<AIUsage>
  const usage: AIUsage = {}
  for (const field of ['inputTokens', 'outputTokens', 'totalTokens', 'cacheReadTokens', 'cacheWriteTokens'] as const) {
    const count = candidate[field]
    if (typeof count === 'number' && Number.isSafeInteger(count) && count >= 0) usage[field] = count
  }
  if (Object.keys(usage).length) requestUsage.value = usage
}

function markRequestFinished(): void {
  if (requestStartedAt.value === null || requestDurationMs.value !== null) return
  requestDurationMs.value = performance.now() - requestStartedAt.value
}

function resetRequestMetrics(): void {
  requestUsage.value = null
  requestStartedAt.value = null
  requestDurationMs.value = null
  firstTextDurationMs.value = null
}

function clearContextSources(): void {
  contextGeneration += 1
  contextSources.value = []
  contextSourcesError.value = ''
  contextSourcesOpen.value = false
  contextReading.value = false
  requestContextText.value = ''
}

function sourceError(code: ContextSourceErrorCode): void {
  contextSourcesError.value = t(`shell.aiChat.contextSources.error.${code}`)
  contextSourcesOpen.value = true
}

function addSource(input: ContextSourceInput): void {
  if (composerBusy.value || contextReading.value || viewingLegacyHistory.value) return
  const result = addContextSource(contextSources.value, input)
  if (result.error) {
    sourceError(result.error.code)
    return
  }
  const built = buildContext(result.sources)
  if (built.error) {
    sourceError(built.error.code)
    return
  }
  contextSources.value = result.sources
  contextSourcesError.value = ''
}

function addTerminalSource(): void {
  if (composerBusy.value || contextReading.value || !connectionAvailable.value || viewingLegacyHistory.value) return
  const existing = contextSources.value.filter((item) => item.kind !== 'terminal')
  const label = t('shell.aiChat.contextSources.kind.terminal')
  const raw = props.terminalContext ?? readTerminalTail(props.sessionId ?? '')
  if (!raw.trim()) {
    sourceError('source_empty')
    return
  }
  const remaining = AI_CONTEXT_BUDGET_BYTES - buildContext(existing).bytes - byteLength(`[${label}]\n\n`)
  const marker = '[terminal context truncated]\n'
  if (remaining <= byteLength(marker)) {
    sourceError('context_budget')
    return
  }
  const tail = truncateUtf8Tail(raw, byteLength(raw) > remaining ? remaining - byteLength(marker) : remaining)
  const result = addContextSource(existing, {
    kind: 'terminal',
    label,
    text: (tail.truncated ? marker : '') + tail.text,
    truncated: tail.truncated,
  })
  if (result.error) {
    sourceError(result.error.code)
    return
  }
  contextSources.value = result.sources
  contextSourcesError.value = ''
}

async function addFiles(files: File[]): Promise<void> {
  if (composerBusy.value || contextReading.value || viewingLegacyHistory.value) return
  const generation = contextGeneration
  contextReading.value = true
  contextSourcesError.value = ''
  try {
    for (const file of files) {
      const result = await readContextFile(file)
      if (generation !== contextGeneration || disposed) return
      if (result.error) {
        sourceError(result.error.code)
        break
      }
      if (!result.source) continue
      const added = addContextSource(contextSources.value, result.source)
      const built = buildContext(added.sources)
      if (added.error || built.error) {
        sourceError((added.error ?? built.error)!.code)
        break
      }
      contextSources.value = added.sources
    }
  } catch {
    if (generation === contextGeneration) sourceError('file_unreadable')
  } finally {
    if (generation === contextGeneration) contextReading.value = false
  }
}

function removeSource(id: string): void {
  if (composerBusy.value || contextReading.value) return
  contextSources.value = contextSources.value.filter((item) => item.id !== id)
  contextSourcesError.value = ''
}
function clearSourceItems(): void {
  if (composerBusy.value || contextReading.value) return
  contextSources.value = []
  contextSourcesError.value = ''
}
function openContextDetails(): void {
  contextSourcesOpen.value = false
  contextDetailsOpen.value = true
}

function captureContext(): string | null {
  const built = buildContext(contextSources.value)
  if (built.error) {
    sourceError(built.error.code)
    contextSourcesOpen.value = true
    return null
  }
  return built.text
}

function setApprovalMode(mode: AIApprovalMode): void {
  if (busy.value || executing.value) return
  if (mode === 'automatic' && approvalMode.value !== 'automatic') {
    automaticWarningOpen.value = true
    return
  }
  approvalMode.value = mode
  writeApprovalMode(mode)
}

function enableAutomaticMode(): void {
  approvalMode.value = 'automatic'
  writeApprovalMode('automatic')
  automaticWarningOpen.value = false
}

watch(
  () => props.sessionId,
  async (sessionId, _previous, onCleanup) => {
    let current = true
    onCleanup(() => {
      current = false
    })

    backgroundAvailable.value = true
    executionChannel.value = readAgentSettings().executionChannel
    if (!sessionId) return
    try {
      const session = await sessionGateway.get(sessionId)
      if (!current) return
      backgroundAvailable.value = session.kind === 'ssh'
      if (!backgroundAvailable.value && executionChannel.value === 'background') executionChannel.value = 'terminal'
    } catch {}
  },
  { immediate: true },
)

function readScrollViewport(): HTMLElement | null {
  const root = scroll.value?.$el as HTMLElement | undefined
  return root?.querySelector<HTMLElement>('[data-slot="scroll-area-viewport"]') ?? null
}

function updateScrollState(): void {
  const viewport = scrollViewport.value
  if (!viewport) return
  const distanceFromBottom = viewport.scrollHeight - viewport.clientHeight - viewport.scrollTop
  isNearBottom.value = distanceFromBottom <= SCROLL_BOTTOM_THRESHOLD
  showScrollToLatest.value = !isNearBottom.value && viewport.scrollHeight > viewport.clientHeight
}

function onScroll(): void {
  const viewport = scrollViewport.value
  if (!viewport || Math.abs(viewport.scrollTop - lastScrollTop) < 0.5) return
  lastScrollTop = viewport.scrollTop
  updateScrollState()
}

function unbindScrollViewport(): void {
  scrollResizeObserver?.disconnect()
  scrollResizeObserver = null
  scrollViewport.value?.removeEventListener('scroll', onScroll)
  scrollViewport.value = null
}

function bindScrollViewport(): void {
  const viewport = readScrollViewport()
  if (!viewport || viewport === scrollViewport.value) return
  unbindScrollViewport()
  scrollViewport.value = viewport
  viewport.addEventListener('scroll', onScroll, { passive: true })

  if (typeof ResizeObserver !== 'undefined') {
    scrollResizeObserver = new ResizeObserver(() => {
      if (scrollViewport.value !== viewport) return
      if (isNearBottom.value) scrollToLatest('auto')
      else updateScrollState()
    })
    scrollResizeObserver.observe(viewport)
    const content = viewport.querySelector('[data-ai-messages]')
    if (content) scrollResizeObserver.observe(content)
  }
  updateScrollState()
}

function scrollToLatest(behavior: ScrollBehavior = 'auto'): void {
  const viewport = scrollViewport.value ?? readScrollViewport()
  if (!viewport) return
  viewport.scrollTo({ top: viewport.scrollHeight, behavior })

  lastScrollTop = viewport.scrollTop
  isNearBottom.value = true
  showScrollToLatest.value = false
}

function loadConversation(): void {
  error.value = ''
  busy.value = false
  resetRequestMetrics()

  contextDetailsOpen.value = false
  clearContextSources()
  executionChannel.value = readAgentSettings().executionChannel
  approvalMode.value = readApprovalMode()
  const providers = readProviders().filter((item) => item.enabled)
  const current = findProvider(providers, readActiveProviderID())
  provider.value = current ?? providers[0] ?? null
  const scope = conversationScope()
  viewingLegacyHistory.value = false
  if (props.hostId && props.sessionId && !migrateSessionConversations(scope))
    error.value = t('shell.aiChat.historySaveFailed')
  const stored = readActiveConversation(scope)
  conversation.value = stored
    ? { ...stored, sessionId: props.sessionId ?? '', sessionTitle: props.sessionTitle ?? '' }
    : createConversation(scope)
  conversationId.value = conversation.value.id
  applyConversationProvider(conversation.value)

  messages.value = stored
    ? (stored.messages as ChatMessage[])
    : provider.value && !props.hostId
      ? readHistory(provider.value.id).map((message) => ({
          ...message,
          id: undefined,
          mode: 'ask' as const,
          legacy: true,
        }))
      : []
  activeAgentMessageId.value = ''
  const activeRun = connectionAvailable.value ? agentTurn.restore(currentConversationIdentity()) : null
  if (activeRun) {
    const existing = messages.value.find((message) => message.agentRun?.id === activeRun.id)
    if (!existing && (activeRun.state === RunState.RunRunning || activeRun.state === RunState.RunWaitingApproval)) {
      messages.value = [
        ...messages.value,
        {
          id: makeMessage({ role: 'assistant', content: '' }).id,
          role: 'assistant',
          content: agentReplyContent(activeRun),
          mode: 'agent',
          agentRun: activeRun,
        },
      ]
    }
  }
  refreshArchivedChats()
  void nextTick(() => {
    bindScrollViewport()
    scrollToLatest('auto')
  })
}

watch(
  () => props.active,
  (active) => {
    if (!active) {
      stopStream()
      agentTurn.stopPolling()
      unbindScrollViewport()
      showScrollToLatest.value = false
      automaticWarningOpen.value = false
      resetRequestMetrics()
      return
    }
    loadConversation()
  },
  { immediate: true },
)

watch(
  () => [props.hostId, props.sessionId],
  () => {
    if (!props.active) return
    stopStream()
    backgroundCancel.value?.()
    captureCancel.value?.()
    executing.value = false
    reviewBusy.value = false
    void agentTurn.cancelAndClear()
    loadConversation()
  },
)

onMounted(() => {
  void nextTick(bindScrollViewport)
})

watch(
  launch,
  (value) => {
    if (!value || value.mode !== 'agent' || agentBusy.value) return
    if (value.kind === 'selection') {
      addSource({ kind: 'selection', label: t('shell.aiChat.contextSources.kind.selection'), text: value.text })
      draft.value = t('shell.aiChat.contextSources.selectionPrompt')
    } else if (value.kind === 'file') {
      draft.value = t('shell.aiChat.agent.promptFile', { path: value.text })
    } else {
      addTerminalSource()
      draft.value = t('shell.aiChat.agent.promptRecent')
    }
    clearAILaunch(value.id)
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  disposed = true
  clearContextSources()
  stopStream()
  unbindScrollViewport()
  backgroundCancel.value?.()
  captureCancel.value?.()
})

watch(
  messages,
  () => {
    void nextTick(() => {
      if (isNearBottom.value) scrollToLatest('auto')
      else updateScrollState()
    })
  },
  { deep: true },
)

function streamFailureMessage(cause: unknown): string {
  const raw = cause instanceof Error ? cause.message : ''
  if (raw.startsWith('ai.')) {
    const [, , ...rest] = raw.split(' ')
    const base = translateErrorKey(raw.split(' ')[0] ?? raw, t('shell.aiChat.failed'))
    return rest.length ? `${base}（${rest.join(' ')}）` : base
  }
  return aiErrorMessage(cause, t('shell.aiChat.failed'))
}

function messageFor(outcome: { errorKey: string; errorDetail: string }): string {
  const base = translateErrorKey(outcome.errorKey, t('shell.aiChat.failed'))
  return outcome.errorDetail ? `${base}（${outcome.errorDetail}）` : base
}

function storedMessages(): StoredMessage[] {
  return messages.value
    .filter((message): message is StoredMessage => message.role !== 'system')
    .map((message) => ({
      role: message.role,
      content: message.content,
      ...(message.reasoning ? { reasoning: message.reasoning } : {}),
      ...(message.output ? { output: message.output } : {}),
      ...(message.executions?.length ? { executions: message.executions } : {}),
      ...(message.usage ? { usage: message.usage } : {}),
    }))
}

function persistHistory(_target: AIProvider): boolean {
  if (messages.value.length && messages.value.every((message) => message.legacy)) {
    if (!writeHistory(_target.id, storedMessages())) {
      error.value = t('shell.aiChat.historySaveFailed')
      return false
    }
  }
  return persistUnifiedConversation()
}

interface ReplySegment {
  kind: 'text' | 'command'
  content: string
}

function splitReply(content: string): ReplySegment[] {
  const segments: ReplySegment[] = []
  const pattern = /```(?:sh|bash)\n([\s\S]*?)```/g
  let cursor = 0
  for (const match of content.matchAll(pattern)) {
    const index = match.index ?? 0
    if (index > cursor) segments.push({ kind: 'text', content: content.slice(cursor, index) })
    segments.push({ kind: 'command', content: (match[1] ?? '').trim() })
    cursor = index + match[0].length
  }
  if (cursor < content.length) segments.push({ kind: 'text', content: content.slice(cursor) })
  return segments.filter((segment) => segment.content.trim().length > 0)
}

const pendingCommand = ref('')

const destructiveReason = ref('')
const confirmOpen = ref(false)

function askRunCommand(command: string, messageIndex: number, segmentIndex: number): void {
  if (!connectionAvailable.value || viewingLegacyHistory.value) return
  pendingCommand.value = command
  pendingOutputIndex.value = messageIndex
  void routeSuggestedCommand(command, messageIndex, segmentIndex)
}

const pendingOutputIndex = ref(-1)
const pendingSegmentIndex = ref(-1)
const pendingSessionId = ref('')
let approvalVersion = 0

watch(
  () => [props.sessionId, provider.value?.id, approvalMode.value],
  () => {
    approvalVersion += 1
    confirmOpen.value = false
    pendingOutputIndex.value = -1
    pendingSegmentIndex.value = -1
  },
)

watch(
  connectionAvailable,
  (available) => {
    if (available) {
      if (props.active) loadConversation()
      return
    }
    stopStream()
    approvalVersion += 1
    confirmOpen.value = false
    automaticWarningOpen.value = false
    pendingOutputIndex.value = -1
    pendingSegmentIndex.value = -1
    backgroundCancel.value?.()
    captureCancel.value?.()
    executing.value = false
    reviewBusy.value = false
    void agentTurn.cancelAndClear()
  },
  { flush: 'sync' },
)

function saveCommandOutput(
  messageIndex: number,
  segmentIndex: number,
  command: string,
  target: string,
  output: string,
  exitCode?: number,
  outcome?: 'unknown' | 'not_sent',
): boolean {
  messages.value = messages.value.map((message, index) =>
    index === messageIndex
      ? {
          ...message,
          executions: [
            ...(message.executions ?? []).filter((entry) => entry.segmentIndex !== segmentIndex),
            {
              segmentIndex,
              command,
              target,
              output,
              ...(exitCode === undefined ? {} : { exitCode }),
              ...(outcome ? { outcome } : {}),
            },
          ],
        }
      : message,
  )
  return provider.value ? persistHistory(provider.value) : persistUnifiedConversation()
}

function isRecordedCommand(message: ChatMessage, execution: NonNullable<StoredMessage['executions']>[number]): boolean {
  const segment = splitReply(message.content)[execution.segmentIndex]
  return segment?.kind === 'command' && segment.content === execution.command
}

function commandRisk(command: string): 'readonly' | 'confirm' | 'danger' {
  if (destructiveCommandReason(command)) return 'danger'
  return isClearlyReadOnlyCommand(command) ? 'readonly' : 'confirm'
}

interface CommandAction {
  segmentIndex: number
  command: string
  target: string
  output?: string
  exitCode?: number
  awaiting: boolean
  running: boolean
  reviewing: boolean
  canRun: boolean
  outcome?: 'unknown' | 'not_sent'
}

function commandActions(message: ChatMessage, messageIndex: number): CommandAction[] {
  const actions: CommandAction[] = (message.executions ?? []).map((execution) => ({
    segmentIndex: execution.segmentIndex,
    command: execution.command,
    target: execution.target,
    output: execution.output,
    ...(execution.exitCode === undefined ? {} : { exitCode: execution.exitCode }),
    awaiting: false,
    running: false,
    reviewing: false,
    canRun: isRecordedCommand(message, execution),
    outcome: execution.outcome,
  }))
  if (pendingOutputIndex.value !== messageIndex || pendingSegmentIndex.value < 0) return actions
  const segmentIndex = pendingSegmentIndex.value
  const pending: CommandAction = {
    segmentIndex,
    command: pendingCommand.value,
    target: props.sessionTitle || t('shell.aiChat.notAvailable'),
    awaiting: confirmOpen.value && !executing.value,
    running: executing.value,
    reviewing: reviewBusy.value,
    canRun: false,
  }
  const position = actions.findIndex((action) => action.segmentIndex === segmentIndex)
  if (position >= 0) actions.splice(position, 1, pending)
  else actions.push(pending)
  return actions.sort((left, right) => left.segmentIndex - right.segmentIndex)
}

function actionOutputTarget(action: CommandAction): string | undefined {
  return action.output === undefined ? undefined : action.target
}

async function executeApprovedCommand(command: string, messageIndex: number, segmentIndex: number): Promise<void> {
  const sessionId = props.sessionId ?? ''
  if (!connectionAvailable.value || !sessionId || executing.value) return
  executing.value = true
  const target = props.sessionTitle || sessionId
  const originConversationId = conversationId.value
  const originMessage = messages.value[messageIndex]
  const version = approvalVersion
  const valid = () =>
    !disposed &&
    connectionAvailable.value &&
    props.active &&
    props.sessionId === sessionId &&
    conversationId.value === originConversationId &&
    approvalVersion === version &&
    (originMessage?.id ? messages.value[messageIndex]?.id === originMessage.id : true)
  confirmOpen.value = false
  error.value = ''

  if (!saveCommandOutput(messageIndex, segmentIndex, command, target, '', undefined, 'unknown')) {
    executing.value = false
    confirmOpen.value = true
    toast.error(t('shell.aiChat.historySaveFailed'))
    return
  }
  try {
    if (executionChannel.value === 'background') {
      await executeInBackgroundChannel(sessionId, target, command, messageIndex, segmentIndex, valid)
      return
    }
    await executeInTerminalChannel(sessionId, target, command, messageIndex, segmentIndex, valid)
  } finally {
    if (valid()) {
      backgroundCancel.value = null
      captureCancel.value = null
      executing.value = false

      pendingOutputIndex.value = -1
      pendingSegmentIndex.value = -1
    }
  }
}

async function executeInBackgroundChannel(
  sessionId: string,
  target: string,
  command: string,
  messageIndex: number,
  segmentIndex: number,
  valid: () => boolean,
): Promise<void> {
  const request = runAIBackgroundCommand(sessionId, command)
  let cancelled = false
  backgroundCancel.value = () => {
    cancelled = true
    request.cancel()
  }
  try {
    const result = await request.promise
    if (!valid() || cancelled) return
    saveCommandOutput(messageIndex, segmentIndex, command, target, result.output.trimEnd(), result.exitCode)
    if (props.sessionTitle) {
      recordAudit({ at: Date.now(), sessionId, sessionTitle: target, command })
    }
    toast.success(t('shell.aiChat.commandFinished'))
  } catch {
    if (cancelled || !valid()) return

    toast.error(t('shell.aiChat.commandOutcomeUnknown'))
  }
}

async function executeInTerminalChannel(
  sessionId: string,
  target: string,
  command: string,
  messageIndex: number,
  segmentIndex: number,
  valid: () => boolean,
): Promise<void> {
  const capture = captureCommandOutput(sessionId)
  let cancelled = false
  captureCancel.value = () => {
    cancelled = true
    capture.cancel()
  }
  if (!runCommandInSession(sessionId, command)) {
    capture.cancel()
    captureCancel.value = null
    saveCommandOutput(messageIndex, segmentIndex, command, target, '', undefined, 'not_sent')
    const failure = t('shell.aiChat.commandNotSent')
    error.value = failure
    toast.error(failure)
    return
  }
  if (props.sessionTitle) {
    recordAudit({ at: Date.now(), sessionId, sessionTitle: target, command })
  }
  toast.success(t('shell.aiChat.commandSent'))
  await captureTerminalOutput(capture, command, target, messageIndex, segmentIndex, () => cancelled || !valid())
}

async function captureTerminalOutput(
  capture: ReturnType<typeof captureCommandOutput>,
  command: string,
  target: string,
  messageIndex: number,
  segmentIndex: number,
  isCancelled: () => boolean,
): Promise<void> {
  try {
    const output = (await capture.promise).trimEnd()
    if (!isCancelled()) {
      saveCommandOutput(messageIndex, segmentIndex, command, target, output, undefined, 'unknown')
    }
  } finally {
    if (!isCancelled()) captureCancel.value = null
  }
}

async function routeSuggestedCommand(command: string, messageIndex: number, segmentIndex: number): Promise<void> {
  if (!connectionAvailable.value || !props.sessionId || executing.value || reviewBusy.value) return
  const targetSessionId = props.sessionId
  const version = ++approvalVersion
  pendingCommand.value = command
  pendingOutputIndex.value = messageIndex
  pendingSegmentIndex.value = segmentIndex
  pendingSessionId.value = props.sessionId

  const destructive = destructiveCommandReason(command)
  if (destructive) {
    destructiveReason.value = destructive
    confirmOpen.value = true
    return
  }
  destructiveReason.value = ''
  if (messages.value[messageIndex]?.executions?.some((entry) => entry.segmentIndex === segmentIndex && entry.outcome)) {
    confirmOpen.value = true
    return
  }
  if (approvalMode.value === 'strict') {
    confirmOpen.value = true
    return
  }
  if (approvalMode.value === 'balanced' && !isClearlyReadOnlyCommand(command)) {
    confirmOpen.value = true
    return
  }
  if (approvalMode.value === 'review') {
    if (!provider.value) return
    const target = provider.value
    reviewBusy.value = true
    const approved = await reviewCommandWithAI(target, command)
    reviewBusy.value = false
    if (
      !props.active ||
      provider.value?.id !== target.id ||
      props.sessionId !== targetSessionId ||
      version !== approvalVersion
    )
      return
    if (!connectionAvailable.value || viewingLegacyHistory.value) return
    if (!approved) {
      confirmOpen.value = true
      return
    }
  }
  await executeApprovedCommand(command, messageIndex, segmentIndex)
}

function cancelPendingCommand(): void {
  confirmOpen.value = false
  destructiveReason.value = ''
  pendingCommand.value = ''
  pendingOutputIndex.value = -1
  pendingSegmentIndex.value = -1
  pendingSessionId.value = ''
}

function confirmRunCommand(): void {
  if (!connectionAvailable.value || viewingLegacyHistory.value) return
  if (!confirmOpen.value || pendingOutputIndex.value < 0 || pendingSessionId.value !== props.sessionId) return

  void executeApprovedCommand(pendingCommand.value, pendingOutputIndex.value, pendingSegmentIndex.value)
}

const availableProviders = computed(() => readProviders().filter((item) => item.enabled))

function switchProvider(providerId: string, model?: string): void {
  const next = availableProviders.value.find((item) => item.id === providerId)
  if (!next) return
  const providerChanged = provider.value?.id !== next.id
  writeActiveProviderID(next.id)
  if (model && model !== next.model) {
    writeProviders(readProviders().map((item) => (item.id === next.id ? { ...item, model } : item)))
  }
  provider.value = model ? { ...next, model } : next
  if (conversation.value && !viewingLegacyHistory.value) persistUnifiedConversation()
  if (providerChanged) {
    resetRequestMetrics()

    error.value = ''
  }
}

async function copyMessage(content: string): Promise<void> {
  try {
    await navigator.clipboard.writeText(content)
    toast.success(t('shell.aiChat.copied'))
  } catch {
    toast.error(t('shell.aiChat.copyFailed'))
  }
}

function regenerate(index: number): void {
  if (!connectionAvailable.value || viewingLegacyHistory.value) return
  if (composerBusy.value || executing.value || reviewBusy.value) return
  const userIndex = messages.value.slice(0, index).findLastIndex((item) => item.role === 'user')
  const previousUser = messages.value[userIndex]
  if (!previousUser) return

  messages.value = messages.value.slice(0, index)
  void send(previousUser.content, true)
}

function analyzeOutput(output: string): void {
  draft.value = t('shell.aiChat.analyzePrompt', { output: output.slice(0, 4000) })
}

function stopStream(): void {
  if (busy.value) persistUnifiedConversation()
  streamGeneration += 1
  streamSession.value?.stop()
  streamSession.value = null
  markRequestFinished()
  busy.value = false
}

function stopCurrentOperation(): void {
  if (agentBusy.value) {
    void agentTurn.cancel().catch((cause) => {
      error.value = streamFailureMessage(cause)
    })
    return
  }
  if (busy.value) {
    stopStream()
    return
  }
  backgroundCancel.value?.()
  captureCancel.value?.()
}

function newConversation(): void {
  if (composerBusy.value || executing.value || reviewBusy.value || !provider.value || !messages.value.length) return
  if (!persistUnifiedConversation()) {
    toast.error(t('shell.aiChat.historySaveFailed'))
    return
  }
  const nextConversation = createConversation(conversationScope())
  if (!saveConversation(nextConversation)) {
    toast.error(t('shell.aiChat.historySaveFailed'))
    return
  }
  void agentTurn.cancelAndClear()
  clearContextSources()
  viewingLegacyHistory.value = false
  conversation.value = nextConversation
  conversationId.value = nextConversation.id
  messages.value = []
  activeAgentMessageId.value = ''
  activeArchiveId.value = ''
  resetRequestMetrics()
  refreshArchivedChats()
  draft.value = ''
  error.value = ''
  void nextTick(() => scrollToLatest('auto'))
}

function restoreConversation(id: string): void {
  if (composerBusy.value || executing.value || reviewBusy.value || !provider.value) return
  const unified = readConversation(id, conversationScope())
  if (unified) {
    if (!persistUnifiedConversation()) return
    const selected = activateConversation(conversationScope(), id)
    if (!selected) {
      toast.error(t('shell.aiChat.historySaveFailed'))
      return
    }
    void agentTurn.cancelAndClear()
    clearContextSources()
    viewingLegacyHistory.value = false
    conversation.value = selected
    applyConversationProvider(selected)
    conversationId.value = selected.id
    messages.value = selected.messages as ChatMessage[]
    activeAgentMessageId.value = ''
    if (connectionAvailable.value) agentTurn.restore(currentConversationIdentity())
    activeArchiveId.value = id
    refreshArchivedChats()
    resetRequestMetrics()
    draft.value = ''
    error.value = ''
    void nextTick(() => scrollToLatest('auto'))
    return
  }
  const restored = [...listLegacyConversations(), ...legacyHistoryChats()].find((chat) => chat.id === id)
  if (!restored || !persistUnifiedConversation()) return
  void agentTurn.cancelAndClear()
  clearContextSources()
  viewingLegacyHistory.value = true
  messages.value = restored.messages.map((message) => ({ ...message, id: undefined, legacy: true }))
  activeArchiveId.value = id
  draft.value = ''
  resetRequestMetrics()
  error.value = ''
  void nextTick(() => scrollToLatest('auto'))
}

function clearHistory(): void {
  if (!provider.value) return
  for (const record of listConversations(conversationScope())) {
    if (record.id !== conversationId.value && !removeConversation(record.id)) {
      toast.error(t('shell.aiChat.historySaveFailed'))
      refreshArchivedChats()
      return
    }
  }
  refreshArchivedChats()
  activeArchiveId.value = ''
  historyOpen.value = false
  toast.success(t('shell.aiChat.historyCleared'))
}

function agentHistory(): Array<{ role: 'user' | 'assistant'; content: string }> {
  const history: Array<{ role: 'user' | 'assistant'; content: string }> = []

  for (let index = 0; index < messages.value.length - 1; index += 1) {
    const user = messages.value[index]!
    const answer = messages.value[index + 1]!
    if (user.role !== 'user' || user.legacy || answer.role !== 'assistant' || answer.legacy || !answer.content.trim())
      continue
    if (answer.agentRun && answer.agentRun.state !== RunState.RunCompleted) continue
    history.push({ role: 'user', content: user.content }, { role: 'assistant', content: answer.content })
    index += 1
  }
  return history
}

async function sendAgent(preset?: unknown, reuseUser = false): Promise<void> {
  if (!connectionAvailable.value || viewingLegacyHistory.value) return
  const presetText = typeof preset === 'string' ? preset : ''
  const text = (presetText || draft.value).trim()
  if (!text) return
  if (composerBusy.value || executing.value || reviewBusy.value) {
    toast.info(t('shell.aiChat.busyHint'))
    return
  }
  if (!provider.value) {
    error.value = t('shell.aiChat.noProvider')
    return
  }
  if (!props.sessionId) {
    error.value = t('shell.aiChat.agent.sessionRequired')
    toast.error(error.value)
    return
  }
  if (contextReading.value) return
  const attachedContext = captureContext()
  if (attachedContext === null) return
  requestContextText.value = attachedContext
  if (!presetText) draft.value = ''
  const origin = identityKeyForAgent(currentConversationIdentity())

  const history = agentHistory()
  const user = reuseUser
    ? undefined
    : makeMessage({
        role: 'user',
        content: text,
        mode: 'agent',
        targetTitle: props.sessionTitle || props.sessionId,
        providerId: provider.value.id,
        model: provider.value.model,
      })
  const assistant = makeMessage({
    role: 'assistant',
    content: '',
    mode: 'agent',
    targetTitle: props.sessionTitle || props.sessionId,
    providerId: provider.value.id,
    model: provider.value.model,
  })
  activeAgentMessageId.value = assistant.id
  messages.value = reuseUser ? [...messages.value, assistant] : [...messages.value, user!, assistant]
  persistUnifiedConversation()
  error.value = ''
  await nextTick()
  scrollToLatest('auto')
  try {
    if (
      disposed ||
      !connectionAvailable.value ||
      !props.active ||
      identityKeyForAgent(currentConversationIdentity()) !== origin ||
      activeAgentMessageId.value !== assistant.id
    )
      return
    await agentTurn.start(text, history)
  } catch (cause) {
    if (
      disposed ||
      !connectionAvailable.value ||
      !props.active ||
      identityKeyForAgent(currentConversationIdentity()) !== origin ||
      activeAgentMessageId.value !== assistant.id
    )
      return
    const failure = streamFailureMessage(cause)
    error.value = failure
    toast.error(failure)
    messages.value = messages.value.filter((message) => message.id !== assistant.id)
    persistUnifiedConversation()
  }
}

async function send(preset?: unknown, reuseUser = false): Promise<void> {
  if (!connectionAvailable.value || viewingLegacyHistory.value) return
  if (props.chatMode === 'agent') {
    await sendAgent(preset, reuseUser)
    return
  }

  const presetText = typeof preset === 'string' ? preset : ''
  const text = (presetText || draft.value).trim()
  if (!text) return

  if (composerBusy.value || executing.value || reviewBusy.value) {
    toast.info(t('shell.aiChat.busyHint'))
    return
  }
  const target = provider.value
  if (!target) {
    error.value = t('shell.aiChat.noProvider')
    return
  }
  if (contextReading.value) return
  const attachedContext = captureContext()
  if (attachedContext === null) return
  requestContextText.value = attachedContext
  if (!presetText) draft.value = ''
  const history = agentHistory()
  const userMessage = reuseUser
    ? undefined
    : makeMessage({
        role: 'user',
        content: text,
        mode: 'ask',
        providerId: target.id,
        model: target.model,
      })
  if (!reuseUser) messages.value = [...messages.value, userMessage!]
  const requestHistory = [...history, { role: 'user' as const, content: text }]
  persistHistory(target)
  persistUnifiedConversation()
  requestUsage.value = null
  requestStartedAt.value = performance.now()
  requestDurationMs.value = null
  firstTextDurationMs.value = null
  busy.value = true
  error.value = ''

  const replyIndex = messages.value.length
  const assistantMessage = makeMessage({
    role: 'assistant',
    content: '',
    mode: 'ask',
    providerId: target.id,
    model: target.model,
  })
  messages.value = [...messages.value, assistantMessage]
  const assistantMessageId = assistantMessage.id
  const generation = ++streamGeneration
  const requestConversationId = conversationId.value
  const requestSessionId = props.sessionId
  const currentRequest = () =>
    !disposed &&
    connectionAvailable.value &&
    props.active &&
    streamGeneration === generation &&
    conversationId.value === requestConversationId &&
    props.sessionId === requestSessionId
  await nextTick()
  scrollToLatest('auto')
  try {
    if (!currentRequest()) return
    const session = await startChatStream(
      {
        providerId: target.id,
        type: target.type,
        baseUrl: target.baseUrl,
        model: target.model,

        messages: [
          {
            role: 'system',
            content: `${t('shell.aiChat.systemPrompt')}\n\n${t('shell.aiChat.structuredFormat')}`,
          },
          ...requestHistory,
        ],

        contextText: attachedContext,
      },
      (poll) => {
        if (!currentRequest()) return
        updateUsage(poll.usage)
        if (firstTextDurationMs.value === null && poll.content.trim() && requestStartedAt.value !== null) {
          firstTextDurationMs.value = performance.now() - requestStartedAt.value
        }
        if (poll.done) markRequestFinished()
        messages.value = messages.value.map((message, index) =>
          index === replyIndex && message.id === assistantMessageId
            ? {
                ...message,
                role: 'assistant',
                content: poll.content,
                reasoning: poll.reasoning,
                ...(poll.done && requestUsage.value ? { usage: requestUsage.value } : {}),
              }
            : message,
        )
        if (poll.done) {
          if (poll.errorKey) {
            const failure = messageFor(poll)
            error.value = failure
            toast.error(failure)
          }
          void finalizeStream(target, replyIndex, assistantMessageId)
        }
      },
    )
    if (!currentRequest()) {
      session.stop()
      return
    }

    if (session.context) {
      const breakdown = session.context
      messages.value = messages.value.map((message) =>
        message.id === assistantMessageId ? { ...message, context: breakdown } : message,
      )
    }
    streamSession.value = busy.value ? session : null
  } catch (cause) {
    if (!currentRequest()) return

    messages.value = messages.value.filter(
      (message, index) => index !== replyIndex || message.id !== assistantMessageId,
    )
    markRequestFinished()
    const failure = streamFailureMessage(cause)
    error.value = failure
    toast.error(failure)
    busy.value = false
  }
}

async function finalizeStream(target: AIProvider, replyIndex: number, assistantMessageId: string): Promise<void> {
  markRequestFinished()
  const reply = messages.value[replyIndex]?.content ?? ''
  if (!reply.trim()) {
    messages.value = messages.value.filter(
      (message, index) => index !== replyIndex || message.id !== assistantMessageId,
    )
  } else {
    persistHistory(target)
    persistUnifiedConversation()
  }
  streamSession.value = null
  busy.value = false
  await nextTick()
  if (isNearBottom.value) scrollToLatest('auto')
}
</script>

<template>
  <div class="@container flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden" data-ai-conversation>
    <div class="ai-tabs flex min-w-0 shrink-0 items-center gap-1 border-b border-qt-border px-2 py-1.5" data-ai-toolbar>
      <Terminal class="size-3.5 shrink-0 text-qt-muted" aria-hidden="true" />
      <span class="min-w-0 flex-1 truncate text-xs text-qt-muted" :title="props.sessionTitle || props.sessionId">
        {{ props.sessionTitle || props.sessionId || t('shell.aiChat.agent.sessionRequired') }}
      </span>
      <DropdownMenu>
        <DropdownMenuTrigger as-child>
          <QtermButton
            variant="ghost"
            size="xs"
            class="h-7! min-h-7! max-w-[45%] shrink-0 gap-1 px-1.5 text-xs text-qt-muted"
            :disabled="composerBusy || executing || reviewBusy"
            :aria-label="`${t('shell.aiChat.executionChannel')}: ${channelOptions.find((option) => option.value === executionChannel)?.label}`"
            data-ai-channel-switch
          >
            <span class="truncate">{{
              channelOptions.find((option) => option.value === executionChannel)?.label
            }}</span>
            <ChevronDown class="size-3 shrink-0" aria-hidden="true" />
          </QtermButton>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" :collision-padding="8" class="max-w-[calc(100vw-1rem)]">
          <DropdownMenuLabel>{{ t('shell.aiChat.executionChannel') }}</DropdownMenuLabel>
          <DropdownMenuItem v-for="option in channelOptions" :key="option.value" @select="selectChannel(option.value)">
            {{ option.label }}
            <Check v-if="option.value === executionChannel" class="ms-auto size-3.5" aria-hidden="true" />
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <div class="ms-auto flex shrink-0 items-center gap-1">
        <IconButton
          class="size-7! min-h-7!"
          :label="t('shell.aiChat.history')"
          size="icon-sm"
          :disabled="composerBusy || executing || reviewBusy || !provider"
          data-ai-chat-history
          @click="historyOpen = true"
        >
          <History class="size-3.5" aria-hidden="true" />
        </IconButton>
        <IconButton
          class="size-7! min-h-7!"
          :label="t('shell.aiChat.newConversation')"
          size="icon-sm"
          :disabled="composerBusy || executing || reviewBusy || !provider || !messages.length"
          data-ai-chat-new
          @click="newConversation"
        >
          <SquarePen class="size-3.5" aria-hidden="true" />
        </IconButton>
      </div>
    </div>
    <div class="relative min-h-0 flex-1 overflow-hidden">
      <ScrollArea ref="scroll" type="hover" :scroll-hide-delay="600" class="ai-message-scroll h-full min-h-0 min-w-0">
        <div class="mx-auto flex w-full min-w-0 max-w-[52rem] flex-col gap-4 px-2 pt-3 pb-4" data-ai-messages>
          <div v-if="!messages.length" class="flex flex-col items-center gap-2 px-3 py-16 text-center sm:py-20">
            <span
              class="flex size-10 items-center justify-center rounded-2xl border border-qt-border bg-qt-subtle text-qt-muted"
            >
              <Sparkles :size="18" aria-hidden="true" />
            </span>
            <p class="m-0 text-sm font-medium text-qt-text">{{ t('shell.aiChat.empty') }}</p>
            <p class="m-0 max-w-[28rem] text-xs leading-5 text-qt-muted">{{ t('shell.aiChat.emptyHint') }}</p>
          </div>
          <MessageGroup v-else class="gap-5">
            <Message
              v-for="(message, index) in messages"
              :key="message.id || index"
              :align="message.role === 'user' ? 'end' : 'start'"
              class="flex-col gap-1 text-sm data-[align=end]:flex-col"
            >
              <MessageContent
                :class="
                  message.role === 'user'
                    ? 'w-fit max-w-[88%] self-end rounded-xl border border-qt-border/70 bg-qt-subtle px-3 py-2'
                    : 'max-w-full px-1 py-1'
                "
              >
                <Trace v-if="!message.agentRun && (message.reasoning || thinkText(message))" class="mb-1">
                  <TraceCollapsible>
                    <TraceCollapsibleTrigger>
                      <TraceTitle>{{ t('shell.aiChat.reasoning') }}</TraceTitle>
                    </TraceCollapsibleTrigger>
                    <TraceCollapsibleContent>
                      <p class="m-0 whitespace-pre-wrap break-words text-xs text-qt-muted">
                        {{ message.reasoning || thinkText(message) }}
                      </p>
                    </TraceCollapsibleContent>
                  </TraceCollapsible>
                </Trace>

                <template v-if="message.role === 'assistant'">
                  <template v-if="!message.agentRun">
                    <AiMessageText
                      v-if="structuredReply(message)"
                      :text="displayText(message)"
                      :streaming="composerBusy && index === messages.length - 1"
                    />
                    <div
                      v-for="(card, cardIndex) in structuredReply(message)?.commandCards ?? []"
                      :key="cardIndex"
                      class="mt-2 flex min-w-0 flex-col gap-1"
                    >
                      <AiCommandCard
                        :command="card.command"
                        :target="card.target || props.sessionTitle || t('shell.aiChat.notAvailable')"
                        :policy="t(`shell.aiChat.modeHint.${approvalMode}`)"
                        :risk="commandRisk(card.command)"
                        :awaiting-confirmation="false"
                        :running="false"
                        :reviewing="false"
                        :disabled="viewingLegacyHistory || !connectionAvailable || !props.sessionId || composerBusy"
                        :can-run="false"
                      />
                      <p v-if="card.title || card.description" class="m-0 px-1 text-xs text-qt-muted">
                        {{ card.title }}
                        <span v-if="card.description"> · {{ card.description }}</span>
                      </p>
                      <div class="flex gap-2 px-1">
                        <QtermButton
                          size="sm"
                          class="h-7! min-h-7! text-xs"
                          :disabled="viewingLegacyHistory || !connectionAvailable || !props.sessionId || composerBusy"
                          data-ai-insert-command
                          @click="insertStructuredCommand(card.command)"
                        >
                          {{ t('shell.aiChat.insertCommand') }}
                        </QtermButton>
                        <QtermButton
                          size="sm"
                          variant="outline"
                          class="h-7! min-h-7! text-xs"
                          data-ai-copy-command
                          @click="copyMessage(card.command)"
                        >
                          {{ t('shell.aiChat.copyCommand') }}
                        </QtermButton>
                        <QtermButton
                          size="sm"
                          variant="outline"
                          class="h-7! min-h-7! text-xs"
                          data-ai-save-snippet
                          @click="requestSnippetCreator(card.command)"
                        >
                          {{ t('shell.aiChat.saveCommand') }}
                        </QtermButton>
                      </div>
                    </div>
                    <Trace v-if="!structuredReply(message) && looksLikeStructuredReply(message.content)" class="mt-1">
                      <TraceCollapsible>
                        <TraceCollapsibleTrigger>
                          <TraceTitle>{{
                            composerBusy && index === messages.length - 1
                              ? t('shell.aiChat.formattingReply')
                              : t('shell.aiChat.rawReply')
                          }}</TraceTitle>
                        </TraceCollapsibleTrigger>
                        <TraceCollapsibleContent>
                          <pre
                            class="m-0 mt-1 max-h-48 overflow-auto whitespace-pre-wrap break-words text-xs text-qt-muted"
                            >{{ message.content }}</pre>
                        </TraceCollapsibleContent>
                      </TraceCollapsible>
                    </Trace>
                    <AiMessageText
                      v-else-if="!structuredReply(message)"
                      :text="displayText(message)"
                      :streaming="composerBusy && index === messages.length - 1"
                    />
                  </template>
                  <div
                    v-if="interruptedAgentRun(message)"
                    class="mt-2 rounded-lg border border-qt-border p-2 text-xs text-qt-muted"
                    data-ai-interrupted-run
                  >
                    <p class="m-0 leading-5">{{ t('shell.aiChat.interruptedRunHint') }}</p>
                    <QtermButton
                      class="mt-2"
                      size="sm"
                      variant="outline"
                      :disabled="!connectionAvailable || composerBusy || viewingLegacyHistory"
                      data-ai-continue-run
                      @click="continueInterrupted(message)"
                      >{{ t('shell.aiChat.continueRun') }}</QtermButton
                    >
                  </div>
                  <Task v-if="message.agentRun" :default-open="agentRunLive(message)" class="mt-2">
                    <template #title>
                      {{ agentRunStatus(message) }}
                      <span v-if="agentRunSeconds(message) !== null"
                        >，{{ t('shell.aiChat.agent.elapsed', { seconds: agentRunSeconds(message) ?? 0 }) }}</span
                      >
                      <span v-if="message.agentRun?.steps?.length">
                        {{ t('shell.aiChat.agent.stepsSuffix', { count: message.agentRun.steps.length }) }}
                      </span>
                    </template>
                    <div class="flex flex-col gap-1 px-1 pb-1">
                      <Reasoning
                        v-for="(note, noteIndex) in message.agentRun.notes ?? []"
                        :key="`agent-note-${noteIndex}`"
                        :default-open="false"
                      >
                        <ReasoningTrigger
                          class="max-w-full"
                          :label="`${t('shell.aiChat.agent.thinkingLabel')} · ${note.slice(0, 80)}`"
                        />
                        <ReasoningContent>
                          <p class="m-0 whitespace-pre-wrap break-words">{{ note }}</p>
                        </ReasoningContent>
                      </Reasoning>
                      <Tool
                        v-for="step in agentAutomaticSteps(message)"
                        :key="`agent-auto-${step.id}`"
                        :title="step.title || step.command || step.kind"
                        :command="step.command || step.title || ''"
                        :state="step.state"
                        :output="step.observation ?? undefined"
                        :exit-code="step.exitCode ?? undefined"
                      />
                    </div>
                  </Task>
                  <template v-for="step in message.agentRun?.steps ?? []" :key="`agent-step-${step.id}`">
                    <AiConfirmation
                      v-if="shouldRenderAgentCommand(step)"
                      class="mt-2"
                      :command="step.command || step.title || ''"
                      :risk="agentRisk(step)"
                      :disabled="
                        viewingLegacyHistory ||
                        !connectionAvailable ||
                        agentDeciding ||
                        !props.sessionId ||
                        activeAgentRunId !== message.agentRun?.id
                      "
                      @confirm="decideAgentStep(step, true)"
                      @cancel="decideAgentStep(step, false)"
                    />
                  </template>

                  <template v-for="action in commandActions(message, index)" :key="action.segmentIndex">
                    <AiCommandCard
                      class="mt-2"
                      :command="action.command"
                      :target="action.target"
                      :policy="t(`shell.aiChat.modeHint.${approvalMode}`)"
                      :risk="commandRisk(action.command)"
                      :warning="
                        action.awaiting && destructiveReason
                          ? t('shell.aiChat.destructiveWarning', { reason: destructiveReason })
                          : undefined
                      "
                      :awaiting-confirmation="action.awaiting"
                      :running="action.running"
                      :reviewing="action.reviewing"
                      :disabled="
                        viewingLegacyHistory ||
                        !connectionAvailable ||
                        !props.sessionId ||
                        composerBusy ||
                        executing ||
                        reviewBusy
                      "
                      :can-run="action.canRun"
                      :result-state="action.outcome"
                      :output="action.output"
                      :output-target="actionOutputTarget(action)"
                      :exit-code="action.exitCode"
                      @confirm="confirmRunCommand"
                      @cancel="cancelPendingCommand"
                      @stop="stopCurrentOperation"
                      @run="askRunCommand(action.command, index, action.segmentIndex)"
                      @analyze="analyzeOutput(action.output ?? '')"
                    />
                  </template>
                  <Response
                    v-if="message.agentRun && message.content"
                    class="mt-2"
                    :text="message.content"
                    :streaming="composerBusy && index === messages.length - 1"
                  />
                </template>
                <p v-else class="m-0 whitespace-pre-wrap break-words leading-6">{{ message.content }}</p>

                <Trace v-if="message.output" class="mt-1">
                  <TraceCollapsible>
                    <TraceCollapsibleTrigger>
                      <TraceTitle>{{ t('shell.aiChat.commandOutput') }}</TraceTitle>
                    </TraceCollapsibleTrigger>
                    <TraceCollapsibleContent>
                      <CodeBlock size="sm" class="mt-1">
                        <CodeBlockContent language="text" wrap>{{ message.output }}</CodeBlockContent>
                      </CodeBlock>
                      <QtermButton
                        variant="outline"
                        class="mt-1 h-7! min-h-7! text-xs"
                        data-ai-analyze-output
                        @click="analyzeOutput(message.output)"
                      >
                        {{ t('shell.aiChat.analyzeOutput') }}
                      </QtermButton>
                    </TraceCollapsibleContent>
                  </TraceCollapsible>
                </Trace>
              </MessageContent>

              <MessageFooter
                v-if="!messageInFlight(message, index)"
                class="group/footer w-full px-1"
                :class="message.role === 'user' ? 'justify-end' : 'justify-start'"
              >
                <div class="invisible flex flex-nowrap items-center gap-1 group-hover/footer:visible">
                  <IconButton
                    class="size-7! min-h-7!"
                    :label="t('shell.aiChat.copyMessage')"
                    size="icon-sm"
                    data-ai-message-copy
                    @click="copyMessage(message.content)"
                  >
                    <Copy class="size-3.5" aria-hidden="true" />
                  </IconButton>
                  <template v-if="message.role === 'assistant'">
                    <IconButton
                      class="size-7! min-h-7!"
                      :label="t('shell.aiChat.regenerate')"
                      size="icon-sm"
                      data-ai-message-regenerate
                      :disabled="
                        viewingLegacyHistory || !connectionAvailable || composerBusy || executing || reviewBusy
                      "
                      @click="regenerate(index)"
                    >
                      <RefreshCw class="size-3.5" aria-hidden="true" />
                    </IconButton>
                  </template>
                  <span
                    class="ms-1 w-10 shrink-0 text-right text-xs whitespace-nowrap text-qt-muted"
                    data-ai-message-time
                  >
                    {{
                      new Date(message.at ?? conversation?.updatedAt ?? Date.now()).toLocaleTimeString(undefined, {
                        hour: '2-digit',
                        minute: '2-digit',
                      })
                    }}
                  </span>
                </div>
              </MessageFooter>
            </Message>
          </MessageGroup>

          <article
            v-if="error || agentError"
            class="rounded-xl border border-qt-danger/35 bg-qt-danger/5 px-3.5 py-3 text-sm"
            data-ai-chat-error
          >
            <div class="flex items-center gap-2 text-xs font-medium text-qt-danger">
              <span>{{ t('shell.aiChat.errorLabel') }}</span>
              <span v-if="provider" class="truncate font-normal text-qt-muted">
                {{
                  t('shell.aiChat.currentProvider', { name: provider.name || provider.baseUrl, model: provider.model })
                }}
              </span>
            </div>
            <p class="m-0 mt-1 whitespace-pre-wrap break-words">{{ error || agentError }}</p>
            <p v-if="provider" class="m-0 mt-2 text-xs leading-5 text-qt-muted" data-ai-chat-context>
              {{ t('shell.aiChat.diagnoseHint') }}
            </p>
          </article>
          <div v-if="composerBusy" class="flex items-center gap-2 px-1 text-xs text-qt-muted" role="status">
            <span class="size-1.5 animate-pulse rounded-full bg-qt-accent" aria-hidden="true" />
            <span>{{ t('shell.aiChat.thinking') }}</span>
          </div>
          <div v-if="reviewBusy || executing" class="flex items-center gap-2 px-1 text-xs text-qt-muted" role="status">
            <span class="size-1.5 animate-pulse rounded-full bg-qt-accent" aria-hidden="true" />
            <span>{{ t(reviewBusy ? 'shell.aiChat.reviewing' : 'shell.aiChat.executing') }}</span>
          </div>
        </div>
      </ScrollArea>
      <button
        v-if="showScrollToLatest"
        type="button"
        class="absolute bottom-3 left-1/2 z-10 flex size-8 -translate-x-1/2 items-center justify-center rounded-full border border-qt-border bg-qt-panel text-qt-text shadow-md transition-colors hover:bg-qt-subtle focus-visible:ring-2 focus-visible:ring-qt-accent/50 focus-visible:outline-none"
        :aria-label="t('shell.aiChat.scrollLatest')"
        data-ai-scroll-latest
        @click="scrollToLatest()"
      >
        <ArrowDown :size="15" aria-hidden="true" />
      </button>
    </div>
    <p v-if="viewingLegacyHistory" class="mx-2 my-1 text-xs leading-5 text-qt-muted" data-ai-legacy-history>
      {{ t('shell.aiChat.legacyHistoryHint') }}
    </p>
    <div class="mx-auto w-full min-w-0 max-w-[52rem] shrink-0 px-2 pt-1 pb-2" data-ai-composer>
      <p
        v-if="!connectionAvailable"
        class="m-0 px-1 pb-2 text-xs leading-5 text-qt-muted"
        role="status"
        data-ai-disconnected
      >
        {{ t('shell.aiChat.sessionUnavailable') }}
      </p>
      <AiComposer
        v-model="draft"
        :mode="props.chatMode === 'agent' ? 'agent' : 'ask'"
        :approval-mode="approvalMode"
        :provider="provider"
        :providers="availableProviders"
        :busy="composerBusy || executing"
        :locked="reviewBusy || contextReading || !connectionAvailable"
        :context-count="contextSources.length"
        :disabled="!provider || !connectionAvailable || viewingLegacyHistory"
        :placeholder="t(props.chatMode === 'agent' ? 'shell.aiChat.agent.placeholder' : 'shell.aiChat.placeholder')"
        @update:mode="emit('update:mode', $event)"
        @context="contextSourcesOpen = true"
        @approval="setApprovalMode"
        @model="switchProvider"
        @submit="send()"
        @stop="stopCurrentOperation"
      />
    </div>
  </div>
  <AiContextSourcesDialog
    :open="contextSourcesOpen"
    :items="contextSources"
    :busy="composerBusy || contextReading"
    :error="contextSourcesError"
    @close="contextSourcesOpen = false"
    @add-terminal="addTerminalSource"
    @selection="addSource({ kind: 'selection', label: t('shell.aiChat.contextSources.kind.selection'), text: $event })"
    @files="addFiles"
    @remove="removeSource"
    @clear="clearSourceItems"
    @details="openContextDetails"
  />
  <AiHistoryDialog
    :open="historyOpen"
    :chats="archivedChats"
    :active-id="viewingLegacyHistory ? activeArchiveId : conversationId"
    :provider-label="provider ? `${provider.name || provider.baseUrl} · ${provider.model}` : ''"
    @close="historyOpen = false"
    @restore="
      (id) => {
        restoreConversation(id)
        historyOpen = false
      }
    "
    @clear="clearHistory"
  />
  <AiContextDialog
    :open="contextDetailsOpen"
    :breakdown="latestRequestContext?.breakdown ?? null"
    :mode="latestRequestContext?.mode ?? 'ask'"
    @close="contextDetailsOpen = false"
  />
  <QtermDialog :open="automaticWarningOpen" @update:open="(open) => !open && (automaticWarningOpen = false)">
    <QtermDialogContent v-if="automaticWarningOpen" class="max-w-[460px]" data-ai-automatic-warning>
      <QtermDialogHeader
        :title="t('shell.aiChat.automaticWarningTitle')"
        :description="t('shell.aiChat.automaticWarningDescription')"
      />
      <div
        class="dialog-body rounded-lg border border-qt-warning/40 bg-qt-warning/5 px-3 py-3 text-sm leading-6 text-qt-text"
      >
        {{ t('shell.aiChat.automaticWarningBody') }}
      </div>
      <QtermDialogFooter>
        <QtermButton variant="outline" @click="automaticWarningOpen = false">{{
          t('shell.aiChat.cancel')
        }}</QtermButton>
        <QtermButton variant="destructive" data-ai-automatic-confirm @click="enableAutomaticMode">
          {{ t('shell.aiChat.enableAutomatic') }}
        </QtermButton>
      </QtermDialogFooter>
    </QtermDialogContent>
  </QtermDialog>
</template>

<style scoped>
.ai-message-scroll :deep([data-slot='scroll-area-viewport']) {
  overflow-x: hidden !important;
  overflow-anchor: none;
  overscroll-behavior-y: contain;
}
.ai-message-scroll :deep([data-slot='scroll-area-viewport'] > div) {
  display: block !important;
  width: 100%;
  min-width: 0;
}
</style>
