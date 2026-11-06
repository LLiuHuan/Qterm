<script setup lang="ts">
import { onMounted } from 'vue'
import SftpPanel from '../features/sftp/SftpPanel.vue'
import { createSftpGateway } from '../services/sftp'
import { setSessionWindowTitle } from '../services/sessionWindows'

const params = new URLSearchParams(window.location.search)
const sessionId = params.get('sftpSession')?.trim() || ''
const title = params.get('sftpTitle')?.trim() || 'SFTP'
const gateway = createSftpGateway()

onMounted(() => {
  void setSessionWindowTitle(title)
})
</script>

<template>
  <main class="w-full h-full min-h-0 flex flex-col text-qt-text bg-qt-panel" :aria-label="title">
    <SftpPanel
      class="flex-1 min-h-0 p-[12px]"
      :gateway="gateway"
      :session-id="sessionId"
      :available="true"
      :preview="false"
    />
  </main>
</template>
