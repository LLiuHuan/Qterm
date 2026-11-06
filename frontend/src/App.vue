<script setup lang="ts">
import { defineAsyncComponent } from 'vue'
import { TooltipProvider } from './components/common/control-primitives'

const HostEditorWindow = defineAsyncComponent(() => import('./app/HostEditorWindow.vue'))
const QtermShell = defineAsyncComponent(() => import('./app/QtermShell.vue'))
const RdpWindow = defineAsyncComponent(() => import('./app/RdpWindow.vue'))
const SftpWindow = defineAsyncComponent(() => import('./app/SftpWindow.vue'))
const VncWindow = defineAsyncComponent(() => import('./app/VncWindow.vue'))

const params = typeof window !== 'undefined' ? new URLSearchParams(window.location.search) : new URLSearchParams()
const isSftpWindow = params.has('sftpSession')

const isRdpWindow = params.has('rdpWorkspace')

const isVncWindow = params.has('vncWorkspace')

const isHostEditorWindow = params.has('hostEditorWindow')
</script>

<template>
  <TooltipProvider :delay-duration="350">
    <SftpWindow v-if="isSftpWindow" />
    <RdpWindow v-else-if="isRdpWindow" />
    <VncWindow v-else-if="isVncWindow" />
    <HostEditorWindow v-else-if="isHostEditorWindow" />
    <QtermShell v-else />
  </TooltipProvider>
</template>
