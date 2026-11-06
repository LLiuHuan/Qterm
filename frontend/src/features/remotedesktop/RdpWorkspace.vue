<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { toast } from 'vue-sonner'
import type { RdpOpenInput } from '../../../bindings/changeme/internal/model/models'
import RdpCanvasPane from './RdpCanvasPane.vue'

interface RdpTab {
  key: string
  title: string

  input?: RdpOpenInput

  hostId?: string

  sessionId?: string
}

const props = withDefaults(
  defineProps<{
    initialHost?: { hostId: string; title: string } | null

    initialSessionId?: string

    initialSessionTitle?: string
  }>(),
  { initialHost: null, initialSessionId: '', initialSessionTitle: '' },
)

const tabs = ref<RdpTab[]>([])
const activeKey = ref('')
const splitKey = ref('')
const splitEnabled = ref(false)
const splitRatio = ref(50)
const formOpen = ref(true)

const engine = ref('freerdp')
const nextIndex = ref(1)

const form = reactive<RdpOpenInput>({
  host: '',
  port: 3389,
  username: '',
  password: '',
  domain: '',
  width: 1280,
  height: 800,
})

const splitTab = computed(() => tabs.value.find((tab) => tab.key === splitKey.value) ?? null)
const gridStyle = computed(() => ({
  gridTemplateColumns: splitEnabled.value && splitTab.value ? `${splitRatio.value}% 4px 1fr` : '1fr',
}))

function addInitialHostTab(): void {
  const host = props.initialHost
  if (!host || tabs.value.some((tab) => tab.hostId === host.hostId)) {
    return
  }
  const key = `rdp-tab-${nextIndex.value++}`
  tabs.value.push({ key, title: host.title || host.hostId, hostId: host.hostId })
  activeKey.value = key
  formOpen.value = false
}

function addInitialSessionTab(): void {
  const sessionId = props.initialSessionId?.trim()
  if (!sessionId || tabs.value.some((tab) => tab.sessionId === sessionId)) {
    return
  }
  const key = `rdp-tab-${nextIndex.value++}`
  tabs.value.push({ key, title: props.initialSessionTitle?.trim() || sessionId, sessionId })
  activeKey.value = key
  formOpen.value = false
}

function addTab(): void {
  if (!form.host.trim()) {
    return
  }
  const key = `rdp-tab-${nextIndex.value++}`
  tabs.value.push({
    key,
    title: `${form.host}${form.port && form.port !== 3389 ? `:${form.port}` : ''}`,
    input: { ...form, engine: engine.value },
  })
  if (!activeKey.value) {
    activeKey.value = key
  }
  if (splitEnabled.value && !splitKey.value) {
    splitKey.value = key
  }
  formOpen.value = false
}

function closeTab(key: string): void {
  const index = tabs.value.findIndex((tab) => tab.key === key)
  if (index >= 0) {
    tabs.value.splice(index, 1)
  }
  if (activeKey.value === key) {
    activeKey.value = tabs.value[Math.min(index, tabs.value.length - 1)]?.key ?? ''
  }
  if (splitKey.value === key) {
    splitKey.value = tabs.value.find((tab) => tab.key !== activeKey.value)?.key ?? ''
  }
}

function handleConnectionFailed(key: string, message: string): void {
  if (!tabs.value.some((tab) => tab.key === key)) return
  toast.error(message)
  closeTab(key)
}

onMounted(() => {
  if (props.initialSessionId?.trim()) {
    addInitialSessionTab()
    return
  }
  addInitialHostTab()
})

function toggleSplit(): void {
  splitEnabled.value = !splitEnabled.value
  if (splitEnabled.value && !splitKey.value) {
    splitKey.value = tabs.value.find((tab) => tab.key !== activeKey.value)?.key ?? ''
  }
}
</script>

<template>
  <div class="flex h-full w-full flex-col gap-2 bg-neutral-900 p-2 text-sm text-white">
    <div class="flex items-center gap-2">
      <div class="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          class="shrink-0 rounded px-2 py-1 text-xs"
          :class="tab.key === activeKey ? 'bg-sky-600 text-white' : 'bg-white/5 text-white/70 hover:bg-white/10'"
          type="button"
          @click="activeKey = tab.key"
        >
          {{ tab.title }}
          <span class="ml-1 text-white/50" role="button" @click.stop="closeTab(tab.key)">×</span>
        </button>
        <span v-if="tabs.length === 0" class="text-xs text-white/40">暂无远程桌面会话</span>
      </div>
      <button
        class="rounded px-2 py-1 text-xs"
        :class="splitEnabled ? 'bg-sky-600' : 'bg-white/5 hover:bg-white/10'"
        type="button"
        @click="toggleSplit"
      >
        分屏
      </button>
      <select v-if="splitEnabled" v-model="splitKey" class="rounded bg-white/5 px-1 py-1 text-xs">
        <option v-for="tab in tabs" :key="tab.key" :value="tab.key">{{ tab.title }}</option>
      </select>
      <button
        class="rounded bg-white/5 px-2 py-1 text-xs hover:bg-white/10"
        type="button"
        @click="formOpen = !formOpen"
      >
        新建会话
      </button>
    </div>

    <form v-if="formOpen" class="grid grid-cols-6 gap-2 rounded bg-black/30 p-2 text-xs" @submit.prevent="addTab">
      <input v-model="form.host" class="col-span-2 rounded bg-white/5 px-2 py-1" placeholder="主机" />
      <input v-model.number="form.port" class="rounded bg-white/5 px-2 py-1" placeholder="端口" type="number" />
      <input v-model="form.username" class="rounded bg-white/5 px-2 py-1" placeholder="用户名" />
      <input v-model="form.password" class="rounded bg-white/5 px-2 py-1" placeholder="密码" type="password" />
      <input v-model="form.domain" class="rounded bg-white/5 px-2 py-1" placeholder="域（可空）" />
      <input v-model.number="form.width" class="rounded bg-white/5 px-2 py-1" placeholder="宽" type="number" />
      <input v-model.number="form.height" class="rounded bg-white/5 px-2 py-1" placeholder="高" type="number" />
      <button class="col-span-2 rounded bg-sky-600 px-2 py-1 hover:bg-sky-500" type="submit">连接</button>
    </form>

    <div class="grid min-h-0 flex-1 gap-0" :style="gridStyle">
      <RdpCanvasPane
        v-for="tab in tabs"
        v-show="tab.key === activeKey || (splitEnabled && tab.key === splitKey)"
        :key="tab.key"
        :active="tab.key === activeKey"
        :host-id="tab.hostId"
        :host-title="tab.hostId ? tab.title : ''"
        :initial-session-id="tab.sessionId"
        :engine="tab.input?.engine || engine"
        :input="tab.input"
        @connection-failed="handleConnectionFailed(tab.key, $event)"
        @close="closeTab(tab.key)"
      />
      <div v-if="splitEnabled && splitTab" class="cursor-col-resize bg-white/10" @mousedown="splitRatio = splitRatio" />
      <div v-if="tabs.length === 0" class="flex-cc text-white/40">填写主机与凭据后点击"连接"建立远程桌面会话</div>
    </div>
  </div>
</template>
