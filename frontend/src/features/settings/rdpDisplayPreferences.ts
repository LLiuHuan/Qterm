import { reactive } from 'vue'
import { loadSettings, type QtermSettings } from './settingsStore'

export const rdpDisplayPreferences = reactive({
  showResolution: loadSettings().rdpShowResolution ?? false,
  showFps: loadSettings().rdpShowFps ?? false,

  hostProbeTimeoutSeconds: loadSettings().rdpHostProbeTimeoutSeconds ?? 3,

  fitGraceSeconds: loadSettings().rdpFitGraceSeconds ?? 6,
})

export function applyRdpDisplayPreferences(settings: QtermSettings): void {
  rdpDisplayPreferences.showResolution = settings.rdpShowResolution ?? false
  rdpDisplayPreferences.showFps = settings.rdpShowFps ?? false
  rdpDisplayPreferences.hostProbeTimeoutSeconds = settings.rdpHostProbeTimeoutSeconds ?? 3
  rdpDisplayPreferences.fitGraceSeconds = settings.rdpFitGraceSeconds ?? 6
}
