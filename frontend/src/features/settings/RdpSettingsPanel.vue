<script setup lang="ts">
import { QtermNumberField, QtermSettingsRow, QtermSettingsSectionHeader, QtermSwitch } from '@/components/common'
import { useI18n } from '@/locales'
import {
  RDP_FIRST_FRAME_TIMEOUT_MAX,
  RDP_FIRST_FRAME_TIMEOUT_MIN,
  RDP_FIT_GRACE_MAX,
  RDP_FIT_GRACE_MIN,
  RDP_HOST_PROBE_TIMEOUT_MAX,
  RDP_HOST_PROBE_TIMEOUT_MIN,
} from './settingsStore'

const props = withDefaults(
  defineProps<{
    showResolution?: boolean

    showFps?: boolean

    firstFrameTimeoutSeconds?: number

    hostProbeTimeoutSeconds?: number

    fitGraceSeconds?: number
  }>(),
  {
    showResolution: false,
    showFps: false,
    firstFrameTimeoutSeconds: 25,
    hostProbeTimeoutSeconds: 3,
    fitGraceSeconds: 6,
  },
)

const emit = defineEmits<{
  'update:showResolution': [value: boolean]
  'update:showFps': [value: boolean]

  'update:firstFrameTimeoutSeconds': [value: number | undefined]
  'update:hostProbeTimeoutSeconds': [value: number | undefined]
  'update:fitGraceSeconds': [value: number | undefined]
}>()

const { t } = useI18n()
</script>

<template>
  <div class="flex flex-col gap-4">
    <QtermSettingsSectionHeader
      class="settings-subsection-header"
      :label="t('rdp.status.title')"
      :description="t('rdp.status.description')"
    />
    <div
      class="settings-card border border-solid border-qt-border rounded-xl [background:color-mix(in_srgb,var(--qt-panel)_32%,transparent)] [-webkit-backdrop-filter:blur(16px)_saturate(135%)] [backdrop-filter:blur(16px)_saturate(135%)]"
    >
      <QtermSettingsRow
        as="label"
        for-id="settings-rdp-show-resolution"
        class="settings-switch-row c-p"
        :label="t('rdp.status.showResolution')"
        :description="t('rdp.status.showResolutionHelp')"
      >
        <QtermSwitch
          id="settings-rdp-show-resolution"
          :model-value="props.showResolution"
          :aria-label="t('rdp.status.showResolution')"
          @update:model-value="(value) => emit('update:showResolution', value === true)"
        />
      </QtermSettingsRow>
      <QtermSettingsRow
        as="label"
        for-id="settings-rdp-show-fps"
        class="settings-switch-row c-p"
        :label="t('rdp.status.showFps')"
        :description="t('rdp.status.showFpsHelp')"
      >
        <QtermSwitch
          id="settings-rdp-show-fps"
          :model-value="props.showFps"
          :aria-label="t('rdp.status.showFps')"
          @update:model-value="(value) => emit('update:showFps', value === true)"
        />
      </QtermSettingsRow>
    </div>

    <QtermSettingsSectionHeader
      class="settings-subsection-header"
      :label="t('rdp.connection.title')"
      :description="t('rdp.connection.description')"
    />
    <div
      class="settings-card border border-solid border-qt-border rounded-xl [background:color-mix(in_srgb,var(--qt-panel)_32%,transparent)] [-webkit-backdrop-filter:blur(16px)_saturate(135%)] [backdrop-filter:blur(16px)_saturate(135%)]"
    >
      <QtermSettingsRow
        as="label"
        for-id="settings-rdp-first-frame-timeout"
        class="settings-switch-row c-p"
        :label="t('rdp.connection.firstFrameTimeout')"
        :description="t('rdp.connection.firstFrameTimeoutHelp')"
      >
        <QtermNumberField
          id="settings-rdp-first-frame-timeout"
          :model-value="props.firstFrameTimeoutSeconds"
          :min="RDP_FIRST_FRAME_TIMEOUT_MIN"
          :max="RDP_FIRST_FRAME_TIMEOUT_MAX"
          :step="5"
          :aria-label="t('rdp.connection.firstFrameTimeout')"
          class="w-24 shrink-0"
          @update:model-value="(value) => emit('update:firstFrameTimeoutSeconds', value)"
        />
      </QtermSettingsRow>
      <QtermSettingsRow
        as="label"
        for-id="settings-rdp-host-probe-timeout"
        class="settings-switch-row c-p"
        :label="t('rdp.connection.hostProbeTimeout')"
        :description="t('rdp.connection.hostProbeTimeoutHelp')"
      >
        <QtermNumberField
          id="settings-rdp-host-probe-timeout"
          :model-value="props.hostProbeTimeoutSeconds"
          :min="RDP_HOST_PROBE_TIMEOUT_MIN"
          :max="RDP_HOST_PROBE_TIMEOUT_MAX"
          :step="1"
          :aria-label="t('rdp.connection.hostProbeTimeout')"
          class="w-24 shrink-0"
          @update:model-value="(value) => emit('update:hostProbeTimeoutSeconds', value)"
        />
      </QtermSettingsRow>
      <QtermSettingsRow
        as="label"
        for-id="settings-rdp-fit-grace"
        class="settings-switch-row c-p"
        :label="t('rdp.connection.fitGrace')"
        :description="t('rdp.connection.fitGraceHelp')"
      >
        <QtermNumberField
          id="settings-rdp-fit-grace"
          :model-value="props.fitGraceSeconds"
          :min="RDP_FIT_GRACE_MIN"
          :max="RDP_FIT_GRACE_MAX"
          :step="1"
          :aria-label="t('rdp.connection.fitGrace')"
          class="w-24 shrink-0"
          @update:model-value="(value) => emit('update:fitGraceSeconds', value)"
        />
      </QtermSettingsRow>
    </div>
  </div>
</template>
