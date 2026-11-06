<script setup lang="ts">
import { QtermInfoList, type QtermInfoItem } from '../../components/common'
import type { DockerGateway, DockerNetwork, DockerNetworkDetailView, DockerNetworkMember } from '../../services/docker'
import { formatDateTime } from '@/lib/datetime'
import { useI18n } from '@/locales'
import DockerResourceDetailDialog from './DockerResourceDetailDialog.vue'

const props = withDefaults(
  defineProps<{
    open: boolean

    network?: DockerNetwork | null
    sessionId: string
    gateway: DockerGateway
  }>(),
  { network: null },
)

const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()

function networkID(): string {
  return props.network?.id ?? ''
}

async function load(): Promise<DockerNetworkDetailView> {
  return props.gateway.networkDetail(props.sessionId, networkID())
}

function yesNo(value: boolean): string {
  return t(value ? 'shell.docker.detailYes' : 'shell.docker.detailNo')
}

function basicItems(detail: DockerNetworkDetailView): QtermInfoItem[] {
  return [
    { label: t('shell.docker.networkId'), value: detail.id, mono: true },
    { label: t('shell.docker.detailName'), value: detail.name, mono: true },
    { label: t('shell.docker.networkDriver'), value: detail.driver },
    { label: t('shell.docker.networkScope'), value: detail.scope },
    { label: t('shell.docker.detailCreated'), value: formatDateTime(detail.created) },
    { label: t('shell.docker.networkInternal'), value: yesNo(detail.internal) },
    { label: t('shell.docker.networkAttachable'), value: yesNo(detail.attachable) },
    { label: t('shell.docker.networkIngress'), value: yesNo(detail.ingress) },
    { label: t('shell.docker.networkEnableIpv6'), value: yesNo(detail.enableIpv6) },
  ]
}

function subnetItems(detail: DockerNetworkDetailView): QtermInfoItem[] {
  return detail.subnets.map((subnet) => ({ label: subnet.subnet, value: subnet.gateway, mono: true }))
}

function memberItems(detail: DockerNetworkDetailView): QtermInfoItem[] {
  return detail.containers.map((member: DockerNetworkMember) => ({
    label: member.name,
    value: [member.ipv4, member.ipv6, member.mac].filter((part) => part.trim() !== '').join(' · '),
    mono: true,
  }))
}

function optionItems(detail: DockerNetworkDetailView): QtermInfoItem[] {
  return detail.options.map((option) => ({ label: option.key, value: option.value, mono: true }))
}

function labelItems(detail: DockerNetworkDetailView): QtermInfoItem[] {
  return detail.labels.map((label) => ({ label: label.key, value: label.value, mono: true }))
}
</script>

<template>
  <DockerResourceDetailDialog
    :open="props.open"
    :title="t('shell.docker.networkDetailTitle')"
    :description="props.network?.name ?? ''"
    :target-key="networkID()"
    :load="load"
    @close="emit('close')"
  >
    <template #default="{ detail }">
      <section class="grid gap-2">
        <h3 class="m-0 text-sm font-medium text-qt-text">{{ t('shell.docker.detailBasic') }}</h3>
        <QtermInfoList :items="basicItems(detail)" />
      </section>

      <section class="grid gap-2">
        <h3 class="m-0 text-sm font-medium text-qt-text">{{ t('shell.docker.networkSubnets') }}</h3>
        <QtermInfoList v-if="detail.subnets.length" :items="subnetItems(detail)" />
        <span v-else class="block text-xs text-qt-muted">{{ t('shell.docker.networkSubnetsEmpty') }}</span>
      </section>

      <section class="grid gap-2">
        <h3 class="m-0 text-sm font-medium text-qt-text">{{ t('shell.docker.networkMembers') }}</h3>
        <QtermInfoList v-if="detail.containers.length" :items="memberItems(detail)" />
        <span v-else class="block text-xs text-qt-muted">{{ t('shell.docker.networkMembersEmpty') }}</span>
      </section>

      <section class="grid gap-2">
        <h3 class="m-0 text-sm font-medium text-qt-text">{{ t('shell.docker.networkOptions') }}</h3>
        <QtermInfoList v-if="detail.options.length" :items="optionItems(detail)" />
        <span v-else class="block text-xs text-qt-muted">{{ t('shell.docker.networkOptionsEmpty') }}</span>
      </section>

      <section class="grid gap-2">
        <h3 class="m-0 text-sm font-medium text-qt-text">{{ t('shell.docker.detailLabels') }}</h3>
        <QtermInfoList v-if="detail.labels.length" :items="labelItems(detail)" />
        <span v-else class="block text-xs text-qt-muted">{{ t('shell.docker.networkLabelsEmpty') }}</span>
      </section>
    </template>
  </DockerResourceDetailDialog>
</template>
