<script setup lang="ts">
import { computed } from 'vue'
import { Fingerprint } from '@lucide/vue'
import { QtermButton, QtermFormMessage } from '../common'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../common/dialog'
import type { HostKeyChallenge } from '../../services/sessions'
import type { HostKeyPending } from '../../types/qterm'
import { useI18n } from '@/locales'

const props = defineProps<{ pending: HostKeyPending | null; confirming: boolean; error: string }>()
const emit = defineEmits<{ reject: []; trust: [] }>()
const { t } = useI18n()

const challenges = computed<HostKeyChallenge[]>(() => {
  const list = props.pending?.challenges
  if (list && list.length > 0) return list
  return props.pending ? [props.pending.challenge] : []
})
const hasMultiple = computed(() => challenges.value.length > 1)

function challengeRole(challenge: HostKeyChallenge): string {
  if (challenge.hopIndex > 0) {
    return t('shell.hostKey.hopValue', {
      index: challenge.hopIndex,
      label: challenge.hopLabel || challenge.address,
    })
  }
  return t('shell.hostKey.target')
}
</script>

<template>
  <QtermDialog :open="!!props.pending" @update:open="(open) => !open && !props.confirming && emit('reject')">
    <QtermDialogContent
      v-if="props.pending"
      class="host-key-dialog"
      size="xl"
      :aria-busy="props.confirming"
      @pointer-down-outside="(event) => props.confirming && event.preventDefault()"
    >
      <section class="host-key-shell">
        <QtermDialogHeader
          :title="hasMultiple ? t('shell.hostKey.multiTitle', { count: challenges.length }) : t('shell.hostKey.title')"
          :description="t('shell.hostKey.description')"
          :close-disabled="props.confirming"
          @close="emit('reject')"
        >
          <template #icon>
            <span class="host-key-icon"><Fingerprint :size="20" stroke-width="1.8" /></span>
          </template>
        </QtermDialogHeader>

        <div class="dialog-body host-key-body">
          <template v-if="hasMultiple">
            <div class="host-key-notice host-key-notice--new">
              <strong class="text-sm">{{ t('shell.hostKey.multiDescription') }}</strong>
              <p class="text-xs">{{ t('shell.hostKey.multiNotice') }}</p>
            </div>
            <ol class="host-key-list" data-host-key-challenges>
              <li
                v-for="(challenge, index) in challenges"
                :key="`${challenge.address}-${index}`"
                class="host-key-item"
                data-host-key-challenge
                :data-host-key-target="challenge.hopIndex === 0 ? 'true' : undefined"
              >
                <div class="host-key-item-head">
                  <strong class="text-sm">{{ challengeRole(challenge) }}</strong>
                </div>
                <dl class="host-key-item-fields">
                  <div>
                    <dt class="text-xs">{{ t('shell.hostKey.host') }}</dt>
                    <dd>
                      <code class="text-xs">{{ challenge.address }}</code>
                    </dd>
                  </div>
                  <div>
                    <dt class="text-xs">{{ t('shell.hostKey.algorithm') }}</dt>
                    <dd>
                      <code class="text-xs">{{ challenge.algorithm }}</code>
                    </dd>
                  </div>
                  <div>
                    <dt class="text-xs">{{ t('shell.hostKey.fingerprint') }}</dt>
                    <dd>
                      <code class="text-xs">{{ challenge.sha256Fingerprint }}</code>
                    </dd>
                  </div>
                </dl>
              </li>
            </ol>
            <p class="host-key-storage text-xs">{{ t('shell.hostKey.multiStorage') }}</p>
          </template>

          <template v-else>
            <div
              class="host-key-notice"
              :class="props.pending.challenge.issue === 'changed' ? 'host-key-notice--changed' : 'host-key-notice--new'"
              :role="props.pending.challenge.issue === 'changed' ? 'alert' : undefined"
            >
              <strong class="text-sm">{{
                props.pending.challenge.issue === 'changed'
                  ? t('shell.hostKey.changed')
                  : t('shell.hostKey.firstConnection')
              }}</strong>
              <p v-if="props.pending.challenge.issue === 'changed'" class="text-xs">
                {{ t('shell.hostKey.changedNotice') }}
              </p>
              <p v-else class="text-xs">{{ t('shell.hostKey.newNotice') }}</p>
            </div>

            <dl class="host-key-summary">
              <div v-if="props.pending.challenge.hopIndex > 0" data-host-key-hop>
                <dt class="text-xs">{{ t('shell.hostKey.hop') }}</dt>
                <dd>
                  <code class="text-xs">{{
                    t('shell.hostKey.hopValue', {
                      index: props.pending.challenge.hopIndex,
                      label: props.pending.challenge.hopLabel || props.pending.challenge.address,
                    })
                  }}</code>
                </dd>
              </div>
              <div>
                <dt class="text-xs">{{ t('shell.hostKey.host') }}</dt>
                <dd>
                  <code class="text-xs">{{ props.pending.challenge.address }}</code>
                </dd>
              </div>
              <div>
                <dt class="text-xs">{{ t('shell.hostKey.algorithm') }}</dt>
                <dd>
                  <code class="text-xs">{{ props.pending.challenge.algorithm }}</code>
                </dd>
              </div>
              <div>
                <dt class="text-xs">{{ t('shell.hostKey.fingerprint') }}</dt>
                <dd>
                  <code class="text-xs">{{ props.pending.challenge.sha256Fingerprint }}</code>
                </dd>
              </div>
            </dl>

            <div
              v-if="props.pending.challenge.issue === 'changed' && props.pending.challenge.trustedFingerprints.length"
              class="host-key-old"
            >
              <span class="text-xs">{{ t('shell.hostKey.oldFingerprint') }}</span>
              <code
                v-for="fingerprint in props.pending.challenge.trustedFingerprints"
                :key="fingerprint"
                class="text-xs"
                >{{ fingerprint }}</code
              >
            </div>

            <p class="host-key-storage text-xs">
              {{
                props.pending.challenge.issue === 'changed'
                  ? t('shell.hostKey.changedStorage')
                  : t('shell.hostKey.newStorage')
              }}
            </p>
          </template>
          <QtermFormMessage v-if="props.error" variant="error">{{ props.error }}</QtermFormMessage>
        </div>

        <QtermDialogFooter>
          <QtermButton variant="outline" :disabled="props.confirming" @click="emit('reject')">{{
            t('shell.hostKey.reject')
          }}</QtermButton>
          <QtermButton type="button" :disabled="props.confirming" @click="emit('trust')">
            {{
              props.confirming
                ? t('shell.common.saving')
                : props.pending.challenge.issue === 'changed'
                  ? t('shell.hostKey.trustChanged')
                  : t('shell.hostKey.trust')
            }}
          </QtermButton>
        </QtermDialogFooter>
      </section>
    </QtermDialogContent>
  </QtermDialog>
</template>

<style scoped>
.host-key-dialog {
  width: min(600px, calc(100vw - 32px));
  max-width: min(600px, calc(100vw - 32px)) !important;
}

.host-key-shell {
  min-width: 0;
}

.host-key-icon {
  width: 36px;
  height: 36px;
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--qt-border);
  border-radius: 7px;
  color: var(--qt-muted);
  background: color-mix(in srgb, var(--qt-subtle) 68%, transparent);
}

.host-key-body {
  gap: 12px;
}

.host-key-notice {
  display: grid;
  gap: 4px;
  padding: 10px 12px;
  border: 1px solid var(--qt-border);
  border-radius: 7px;
  background: color-mix(in srgb, var(--qt-subtle) 58%, transparent);
}

.host-key-notice--changed {
  border-color: color-mix(in srgb, var(--qt-warning) 58%, var(--qt-border));
  background: color-mix(in srgb, var(--qt-warning) 10%, var(--qt-panel));
}

.host-key-notice strong {
  font-weight: 600;
}

.host-key-notice--changed strong {
  color: color-mix(in srgb, var(--qt-warning) 78%, var(--qt-text));
}

.host-key-notice p,
.host-key-storage {
  margin: 0;
  color: var(--qt-muted);
  line-height: 1.55;
}

.host-key-notice code,
.host-key-storage,
.host-key-storage code {
  font-family: var(--qt-font-mono);
}

.host-key-summary {
  display: grid;
  margin: 0;
  overflow: hidden;
  border: 1px solid var(--qt-border);
  border-radius: 7px;
}

.host-key-list {
  display: grid;
  gap: 8px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.host-key-item {
  display: grid;
  gap: 8px;
  padding: 10px;
  border: 1px solid var(--qt-border);
  border-radius: 7px;
  background: color-mix(in srgb, var(--qt-subtle) 46%, transparent);
}

.host-key-item-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.host-key-item-head strong {
  font-weight: 600;
}

.host-key-item-fields {
  display: grid;
  gap: 6px;
  margin: 0;
}

.host-key-item-fields > div {
  display: grid;
  grid-template-columns: minmax(76px, 0.25fr) minmax(0, 1fr);
  align-items: center;
  gap: 10px;
}

.host-key-item-fields dt {
  color: var(--qt-muted);
}

.host-key-item-fields dd {
  min-width: 0;
  margin: 0;
}

.host-key-item-fields code {
  display: block;
  min-width: 0;
  overflow-wrap: anywhere;
  color: var(--qt-text);
  font-family: var(--qt-font-mono);
  line-height: 1.45;
  user-select: text;
}

.host-key-summary > div {
  min-width: 0;
  display: grid;
  grid-template-columns: minmax(76px, 0.25fr) minmax(0, 1fr);
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
}

.host-key-summary > div + div {
  border-top: 1px solid var(--qt-border);
}

.host-key-summary dt,
.host-key-old > span {
  color: var(--qt-muted);
}

.host-key-summary dd {
  min-width: 0;
  margin: 0;
}

.host-key-summary code,
.host-key-old code {
  display: block;
  min-width: 0;
  overflow-wrap: anywhere;
  color: var(--qt-text);
  font-family: var(--qt-font-mono);
  line-height: 1.45;
  user-select: text;
}

.host-key-old {
  display: grid;
  gap: 6px;
  padding: 9px 10px;
  border: 1px solid var(--qt-border);
  border-radius: 7px;
  background: color-mix(in srgb, var(--qt-subtle) 46%, transparent);
}

.host-key-storage {
  overflow-wrap: anywhere;
}

@media (max-width: 600px) {
  .host-key-dialog {
    width: calc(100vw - 16px);
    max-width: calc(100vw - 16px) !important;
  }

  .host-key-summary > div {
    grid-template-columns: minmax(68px, 0.3fr) minmax(0, 1fr);
    gap: 8px;
    padding-inline: 8px;
  }
}
</style>
