<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Copy, FileKey, FileText, RotateCcw, Upload, WandSparkles } from '@lucide/vue'
import { toast } from 'vue-sonner'
import { QtermButton, QtermFormMessage, QtermInput, QtermSecretInput, QtermTextarea } from '../common'
import {
  Label,
  RadioGroup,
  RadioGroupItem,
  ScrollArea,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from '../common/control-primitives'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../common/dialog'
import {
  credentialErrorMessage,
  type CredentialGateway,
  type CredentialRecord,
  type CredentialVaultStatus,
} from '../../services/credentials'
import { writeClipboardText } from '../../services/clipboard'
import { useI18n } from '@/locales'

const props = withDefaults(
  defineProps<{
    open: boolean
    kind: 'password' | 'privateKey' | 'sshAgent'

    gateway: CredentialGateway

    status: CredentialVaultStatus | null

    editing?: CredentialRecord | null

    initialKeyMode?: 'generate' | 'upload' | 'manual'
  }>(),
  { editing: null, initialKeyMode: undefined },
)

const emit = defineEmits<{
  created: [credentialId: string]

  updated: [credentialId: string]
  close: []
}>()

const { t } = useI18n()

const name = ref('')
const secret = ref('')
const passphrase = ref('')
const username = ref('')
const publicKey = ref('')
const installCommand = ref('')
const keyMode = ref<'generate' | 'upload' | 'manual'>('manual')
const keyAlgorithm = ref<'ed25519' | 'ecdsa' | 'rsa'>('ed25519')
const keyCurve = ref<'p-256' | 'p-384' | 'p-521'>('p-256')
const keyBits = ref<2048 | 4096>(4096)
const generatedKeyLocked = ref(false)
const generating = ref(false)
const keyOptionsDisabled = computed(() => generatedKeyLocked.value || generating.value)
const selectedFileName = ref('')
const localKeyToken = ref('')
const saving = ref(false)
const formError = ref('')
const fileInput = ref<HTMLInputElement | null>(null)
const vaultUnlocked = computed(() => props.status?.state === 'unlocked')

function kindLabel(kind: 'password' | 'privateKey' | 'sshAgent'): string {
  return t('settings.credentials.kind.' + kind)
}

function resetForm(): void {
  name.value = ''
  secret.value = ''
  passphrase.value = ''
  username.value = ''
  publicKey.value = ''
  installCommand.value = ''

  keyMode.value = props.initialKeyMode ?? 'generate'
  keyAlgorithm.value = 'ed25519'
  keyCurve.value = 'p-256'
  keyBits.value = 4096
  generatedKeyLocked.value = false
  selectedFileName.value = ''
  localKeyToken.value = ''
  formError.value = ''
}

function initializeForm(): void {
  resetForm()
  const item = props.editing
  if (!item) return

  keyMode.value = item.kind === 'privateKey' ? 'manual' : keyMode.value
  name.value = item.name
  username.value = item.username ?? ''
  publicKey.value = item.publicKey ?? ''
  void loadEditableSecret(item.id)
}

watch(
  () => props.open,
  (open) => {
    if (open) initializeForm()
  },
  { immediate: true },
)

async function generateKey(): Promise<void> {
  if (!props.gateway.generatePrivateKey || generating.value) return
  generatedKeyLocked.value = false
  generating.value = true
  try {
    const generated = await props.gateway.generatePrivateKey({
      algorithm: keyAlgorithm.value,
      curve: keyCurve.value,
      bits: keyBits.value,
      passphrase: passphrase.value,
    })
    secret.value = generated.privateKey
    publicKey.value = generated.publicKey
    installCommand.value = generated.installCommand
    selectedFileName.value = t('settings.credentials.privateKey.generated')
    generatedKeyLocked.value = true
    if (!name.value.trim())
      name.value = t('settings.credentials.privateKey.generatedName', { algorithm: keyAlgorithm.value.toUpperCase() })
    toast.success(t('settings.credentials.toast.generated'))
  } catch (error) {
    toast.error(credentialErrorMessage(error, t('settings.credentials.error.generate')))
  } finally {
    generating.value = false
  }
}

async function loadEditableSecret(id: string): Promise<void> {
  const gateway = props.gateway
  if (!gateway.revealCredential) return
  try {
    const revealed = await gateway.revealCredential(id)

    if (props.editing?.id !== id) return
    secret.value = revealed.secret
    passphrase.value = revealed.passphrase ?? ''
    username.value = revealed.username ?? ''
  } catch (error) {
    toast.error(credentialErrorMessage(error, t('settings.credentials.error.reveal')))
  }
}

function prepareKeyRegeneration(): void {
  generatedKeyLocked.value = false
  secret.value = ''
  publicKey.value = ''
  installCommand.value = ''
  selectedFileName.value = ''
}

function switchKeyMode(value: string | number): void {
  if (value !== 'generate' && value !== 'upload' && value !== 'manual') return
  if (value !== keyMode.value) prepareKeyRegeneration()
  keyMode.value = value
}

async function copyValue(value: string): Promise<void> {
  try {
    await writeClipboardText(value)
    toast.success(t('settings.credentials.toast.copied'))
  } catch (error) {
    toast.error(credentialErrorMessage(error, t('settings.credentials.toast.copyFailed')))
  }
}

function closeForm(): void {
  if (saving.value) return
  emit('close')
}

function openPrivateKeyFilePicker(): void {
  fileInput.value?.click()
}

async function readPrivateKey(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  if (file.size > 16 * 1024 * 1024) {
    toast.error(t('settings.credentials.file.tooLarge'))
    return
  }
  try {
    const content = await file.text()
    if (!content.trim()) throw new Error(t('settings.credentials.file.empty'))
    keyMode.value = 'upload'
    generatedKeyLocked.value = false
    secret.value = content
    publicKey.value = ''
    installCommand.value = ''
    selectedFileName.value = file.name
    localKeyToken.value = ''
    if (!name.value.trim()) name.value = file.name.replace(/\.(pem|key|txt|ppk)$/i, '')
  } catch (error) {
    toast.error(credentialErrorMessage(error, t('settings.credentials.file.readFailed')))
  }
}

async function save(): Promise<void> {
  const normalizedName = name.value.trim()
  if (!normalizedName) {
    formError.value = t('settings.credentials.validation.nameRequired')
    return
  }
  if (!props.editing && props.kind !== 'sshAgent' && !secret.value && !localKeyToken.value) {
    formError.value = t(
      props.kind === 'password'
        ? 'settings.credentials.validation.passwordRequired'
        : 'settings.credentials.validation.privateKeyRequired',
    )
    return
  }
  if (props.kind === 'password' && !props.editing && !secret.value.trim()) {
    formError.value = t('settings.credentials.validation.passwordRequired')
    return
  }
  if (props.kind === 'sshAgent' && !publicKey.value.trim()) {
    formError.value = t('settings.credentials.validation.agentPublicKeyRequired')
    return
  }
  saving.value = true
  formError.value = ''
  try {
    let result: CredentialRecord
    const editing = props.editing
    if (editing) {
      result = editing
      if (normalizedName !== result.name) result = await props.gateway.rename(result.id, normalizedName)
      if (secret.value || props.kind === 'sshAgent') {
        result = await props.gateway.replace(
          result.id,
          result.kind,
          props.kind === 'sshAgent' ? publicKey.value : secret.value,
          passphrase.value,
          {
            username: username.value,
            publicKey: publicKey.value,
          },
        )
      }
      toast.success(t('settings.credentials.toast.updated'))
      emit('updated', result.id)
    } else {
      result =
        localKeyToken.value && props.gateway.importLocalPrivateKey
          ? await props.gateway.importLocalPrivateKey({
              token: localKeyToken.value,
              name: normalizedName,
              username: username.value,
              passphrase: passphrase.value,
            })
          : await props.gateway.create(
              normalizedName,
              props.kind,
              props.kind === 'sshAgent' ? publicKey.value : secret.value,
              passphrase.value,
              { username: username.value, publicKey: publicKey.value },
            )
      toast.success(t('settings.credentials.toast.created'))
      emit('created', result.id)
    }
    emit('close')
  } catch (error) {
    formError.value = credentialErrorMessage(
      error,
      t(props.editing ? 'settings.credentials.error.update' : 'settings.credentials.error.create'),
    )
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <QtermDialog :open="props.open" @update:open="(open) => !open && closeForm()">
    <QtermDialogContent
      v-if="props.open"
      class="credential-editor-dialog text-sm z-60 w-[calc(100vw-32px)] !max-w-[530px]"
      overlay-class="z-60"
      size="lg"
    >
      <input
        ref="fileInput"
        class="sr-only"
        type="file"
        accept=".pem,.key,.txt,.ppk,application/x-pem-file,text/plain"
        @change="readPrivateKey"
      />
      <form @submit.prevent="save">
        <QtermDialogHeader
          :title="
            t(props.editing ? 'settings.credentials.dialog.editTitle' : 'settings.credentials.dialog.createTitle', {
              kind: kindLabel(props.kind),
            })
          "
          @close="closeForm"
        />
        <ScrollArea type="hover" :scroll-hide-delay="600" class="credential-editor-scroll">
          <div class="dialog-body">
            <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
              <Label for="credential-name"
                >{{ t('settings.credentials.field.name') }} <span class="required-mark text-qt-danger">*</span></Label
              >
              <QtermInput
                id="credential-name"
                v-model="name"
                autofocus
                :placeholder="t('settings.credentials.field.namePlaceholder')"
              />
            </div>
            <div v-if="props.kind === 'password'" class="form-field min-w-0 grid gap-[6px] text-qt-muted">
              <Label for="credential-username"
                >{{ t('settings.credentials.field.username') }}
                <span class="optional-label text-qt-faint font-normal">{{
                  t('settings.credentials.field.optional')
                }}</span></Label
              >
              <QtermInput
                id="credential-username"
                v-model="username"
                autocomplete="off"
                autocorrect="off"
                autocapitalize="off"
                spellcheck="false"
                :placeholder="t('settings.credentials.field.usernamePlaceholder')"
              />
            </div>
            <div v-if="props.kind === 'password'" class="form-field min-w-0 grid gap-[6px] text-qt-muted">
              <Label for="credential-password"
                >{{ t('settings.credentials.field.password') }}
                <span class="required-mark text-qt-danger">*</span></Label
              >

              <QtermSecretInput
                id="credential-password"
                autocomplete="new-password"
                :key="props.open ? 'open' : 'closed'"
                v-model="secret"
                :placeholder="t('settings.credentials.field.passwordPlaceholder')"
                :show-label="t('settings.credentials.secret.show')"
                :hide-label="t('settings.credentials.secret.hide')"
              />
            </div>
            <template v-else-if="props.kind === 'sshAgent'">
              <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
                <Label for="credential-agent-public-key"
                  >{{ t('settings.credentials.field.agentPublicKey') }}
                  <span class="required-mark text-qt-danger">*</span></Label
                >
                <QtermTextarea
                  id="credential-agent-public-key"
                  v-model="publicKey"
                  rows="4"
                  :placeholder="t('settings.credentials.field.agentPublicKeyPlaceholder')"
                />
                <small class="field-help text-xs">{{ t('settings.credentials.field.agentPublicKeyHelp') }}</small>
              </div>
            </template>
            <template v-else>
              <div class="grid gap-2">
                <template v-if="!props.editing">
                  <Label>{{ t('settings.credentials.privateKey.modeLabel') }}</Label>
                </template>
                <Tabs
                  :model-value="keyMode"
                  default-value="generate"
                  class="qterm-segmented-tabs grid gap-3"
                  @update:model-value="switchKeyMode"
                >
                  <TabsList v-if="!props.editing" class="qterm-segmented-tabs__list" variant="default">
                    <TabsTrigger value="generate" class="qterm-segmented-tabs__trigger"
                      ><WandSparkles :size="15" />{{ t('settings.credentials.privateKey.generateMode') }}</TabsTrigger
                    >
                    <TabsTrigger value="upload" class="qterm-segmented-tabs__trigger"
                      ><Upload :size="15" />{{ t('settings.credentials.privateKey.uploadMode') }}</TabsTrigger
                    >
                    <TabsTrigger value="manual" class="qterm-segmented-tabs__trigger"
                      ><FileText :size="15" />{{ t('settings.credentials.privateKey.manualMode') }}</TabsTrigger
                    >
                  </TabsList>

                  <TabsContent value="generate">
                    <div
                      class="grid gap-4 rounded-lg border border-border bg-muted/20 p-4 text-sm"
                      :class="{ 'credential-key-options-locked': keyOptionsDisabled }"
                    >
                      <div class="grid gap-2">
                        <Label>{{ t('settings.credentials.privateKey.algorithm') }}</Label>
                        <RadioGroup
                          v-model="keyAlgorithm"
                          class="grid gap-2"
                          aria-label="Key algorithm"
                          :disabled="keyOptionsDisabled"
                        >
                          <label
                            class="flex min-h-8 c-p items-start gap-2 rounded-md border border-border px-2 py-1.5 text-sm leading-5 text-foreground transition-colors hover:border-ring hover:bg-muted/50 has-data-[state=checked]:border-primary has-data-[state=checked]:bg-muted"
                          >
                            <RadioGroupItem value="ed25519" class="mt-0.5" />
                            <span
                              >ED25519
                              <small class="text-xs text-muted-foreground">{{
                                t('settings.credentials.privateKey.ed25519Hint')
                              }}</small></span
                            >
                          </label>
                          <label
                            class="flex min-h-8 c-p items-start gap-2 rounded-md border border-border px-2 py-1.5 text-sm leading-5 text-foreground transition-colors hover:border-ring hover:bg-muted/50 has-data-[state=checked]:border-primary has-data-[state=checked]:bg-muted"
                          >
                            <RadioGroupItem value="ecdsa" class="mt-0.5" />
                            <span
                              >ECDSA
                              <small class="text-xs text-muted-foreground">{{
                                t('settings.credentials.privateKey.ecdsaHint')
                              }}</small></span
                            >
                          </label>
                          <label
                            class="flex min-h-8 c-p items-start gap-2 rounded-md border border-border px-2 py-1.5 text-sm leading-5 text-foreground transition-colors hover:border-ring hover:bg-muted/50 has-data-[state=checked]:border-primary has-data-[state=checked]:bg-muted"
                          >
                            <RadioGroupItem value="rsa" class="mt-0.5" />
                            <span
                              >RSA
                              <small class="text-xs text-muted-foreground">{{
                                t('settings.credentials.privateKey.rsaHint')
                              }}</small></span
                            >
                          </label>
                        </RadioGroup>
                      </div>

                      <div v-if="keyAlgorithm === 'ecdsa'" class="grid gap-2">
                        <Label>{{ t('settings.credentials.privateKey.curveSize') }}</Label>
                        <RadioGroup
                          v-model="keyCurve"
                          class="grid gap-2"
                          aria-label="ECDSA curve size"
                          :disabled="keyOptionsDisabled"
                        >
                          <label
                            class="flex min-h-8 c-p items-center gap-2 rounded-md border border-border px-2 py-1.5 text-sm leading-5 text-foreground transition-colors hover:border-ring hover:bg-muted/50 has-data-[state=checked]:border-primary has-data-[state=checked]:bg-muted"
                            ><RadioGroupItem value="p-256" /><span>{{
                              t('settings.credentials.privateKey.curveP256')
                            }}</span></label
                          >
                          <label
                            class="flex min-h-8 c-p items-center gap-2 rounded-md border border-border px-2 py-1.5 text-sm leading-5 text-foreground transition-colors hover:border-ring hover:bg-muted/50 has-data-[state=checked]:border-primary has-data-[state=checked]:bg-muted"
                            ><RadioGroupItem value="p-384" /><span>{{
                              t('settings.credentials.privateKey.curveP384')
                            }}</span></label
                          >
                          <label
                            class="flex min-h-8 c-p items-center gap-2 rounded-md border border-border px-2 py-1.5 text-sm leading-5 text-foreground transition-colors hover:border-ring hover:bg-muted/50 has-data-[state=checked]:border-primary has-data-[state=checked]:bg-muted"
                            ><RadioGroupItem value="p-521" /><span>{{
                              t('settings.credentials.privateKey.curveP521')
                            }}</span></label
                          >
                        </RadioGroup>
                      </div>

                      <div v-if="keyAlgorithm === 'rsa'" class="grid gap-2">
                        <Label>{{ t('settings.credentials.privateKey.keyLength') }}</Label>
                        <RadioGroup
                          :model-value="String(keyBits)"
                          class="grid gap-2"
                          aria-label="RSA key length"
                          :disabled="keyOptionsDisabled"
                          @update:model-value="(value) => (keyBits = Number(value) as 2048 | 4096)"
                        >
                          <label
                            class="flex min-h-8 c-p items-start gap-2 rounded-md border border-border px-2 py-1.5 text-sm leading-5 text-foreground transition-colors hover:border-ring hover:bg-muted/50 has-data-[state=checked]:border-primary has-data-[state=checked]:bg-muted"
                            ><RadioGroupItem value="2048" class="mt-0.5" /><span
                              >{{ t('settings.credentials.privateKey.rsa2048Length') }}
                              <small class="text-xs text-muted-foreground">{{
                                t('settings.credentials.privateKey.rsa2048Hint')
                              }}</small></span
                            ></label
                          >
                          <label
                            class="flex min-h-8 c-p items-start gap-2 rounded-md border border-border px-2 py-1.5 text-sm leading-5 text-foreground transition-colors hover:border-ring hover:bg-muted/50 has-data-[state=checked]:border-primary has-data-[state=checked]:bg-muted"
                            ><RadioGroupItem value="4096" class="mt-0.5" /><span
                              >{{ t('settings.credentials.privateKey.rsa4096Length') }}
                              <small class="text-xs text-muted-foreground">{{
                                t('settings.credentials.privateKey.rsa4096Hint')
                              }}</small></span
                            ></label
                          >
                        </RadioGroup>
                      </div>

                      <div class="grid gap-2">
                        <Label for="credential-passphrase-generate"
                          >{{ t('settings.credentials.field.passphrase') }}
                          <span class="optional-label text-qt-faint font-normal">{{
                            t('settings.credentials.field.optional')
                          }}</span></Label
                        >
                        <QtermSecretInput
                          id="credential-passphrase-generate"
                          autocomplete="new-password"
                          :key="props.open ? 'open' : 'closed'"
                          v-model="passphrase"
                          :disabled="keyOptionsDisabled"
                          :placeholder="t('settings.credentials.field.passphrasePlaceholder')"
                          :show-label="t('settings.credentials.passphrase.show')"
                          :hide-label="t('settings.credentials.passphrase.hide')"
                        />
                      </div>

                      <QtermButton
                        :variant="generatedKeyLocked ? 'outline' : 'default'"
                        type="button"
                        :disabled="generating"
                        @click="generatedKeyLocked ? prepareKeyRegeneration() : generateKey()"
                      >
                        <RotateCcw v-if="generatedKeyLocked" :size="15" /><WandSparkles v-else :size="15" />{{
                          t(
                            generatedKeyLocked
                              ? 'settings.credentials.privateKey.regenerate'
                              : 'settings.credentials.privateKey.generate',
                          )
                        }}
                      </QtermButton>
                    </div>
                  </TabsContent>

                  <TabsContent value="upload">
                    <div class="grid gap-4 rounded-lg border border-border bg-muted/20 p-4 text-sm">
                      <div class="grid gap-2">
                        <Label>{{ t('settings.credentials.privateKey.chooseFile') }}</Label>
                        <QtermButton
                          variant="outline"
                          class="w-full justify-center"
                          type="button"
                          @click="openPrivateKeyFilePicker"
                        >
                          <Upload :size="15" />{{ t('settings.credentials.privateKey.chooseFile') }}
                        </QtermButton>
                        <p
                          v-if="selectedFileName"
                          class="m-0 flex items-center gap-1.5 break-all text-sm text-foreground"
                          role="status"
                        >
                          <FileKey :size="14" aria-hidden="true" />{{ selectedFileName }}
                        </p>
                        <small class="text-xs leading-5 text-muted-foreground">{{
                          t('settings.credentials.privateKey.supportedFormats')
                        }}</small>
                      </div>
                    </div>
                  </TabsContent>

                  <TabsContent value="manual">
                    <div class="grid gap-2">
                      <Label for="credential-private-key">{{ t('settings.credentials.field.privateKey') }}</Label>
                      <QtermTextarea
                        id="credential-private-key"
                        v-model="secret"
                        rows="8"
                        class="min-h-37.5 font-mono text-xs"
                        :placeholder="t('settings.credentials.field.privateKeyPlaceholder')"
                      />
                    </div>
                  </TabsContent>
                </Tabs>
              </div>

              <div v-if="keyMode === 'upload' || keyMode === 'manual'" class="grid gap-2">
                <Label for="credential-passphrase"
                  >{{ t('settings.credentials.field.passphrase') }}
                  <span class="optional-label text-qt-faint font-normal">{{
                    t('settings.credentials.field.optional')
                  }}</span></Label
                >
                <QtermSecretInput
                  id="credential-passphrase"
                  autocomplete="new-password"
                  :key="props.open ? 'open' : 'closed'"
                  v-model="passphrase"
                  :placeholder="t('settings.credentials.field.passphrasePlaceholder')"
                  :show-label="t('settings.credentials.passphrase.show')"
                  :hide-label="t('settings.credentials.passphrase.hide')"
                />
              </div>

              <div v-if="publicKey && !props.editing" class="credential-public-key grid gap-[8px]">
                <Label>{{ t('settings.credentials.field.publicKey') }}</Label>
                <div
                  class="credential-public-key-value min-h-10 select-text whitespace-pre-wrap break-all rounded-md border border-border bg-transparent px-2.5 py-2 font-mono text-xs leading-5 text-foreground min-w-0 wrap-anywhere"
                  role="textbox"
                  aria-readonly="true"
                  tabindex="0"
                  :aria-label="t('settings.credentials.field.publicKey')"
                >
                  {{ publicKey }}
                </div>
                <div class="dialog-footer-actions flex items-center gap-[8px]">
                  <QtermButton variant="outline" type="button" @click="copyValue(publicKey)"
                    ><Copy :size="15" />{{ t('settings.credentials.action.copyPublicKey') }}</QtermButton
                  ><QtermButton variant="outline" type="button" @click="copyValue(installCommand)"
                    ><Copy :size="15" />{{ t('settings.credentials.action.copyInstallCommand') }}</QtermButton
                  >
                </div>
              </div>
            </template>
            <QtermFormMessage v-if="formError" variant="error">{{ formError }}</QtermFormMessage>
          </div>
        </ScrollArea>
        <QtermDialogFooter>
          <div></div>
          <div class="dialog-footer-actions flex items-center gap-[8px]">
            <QtermButton variant="outline" type="button" :disabled="saving" @click="closeForm">{{
              t('settings.credentials.action.cancel')
            }}</QtermButton>
            <QtermButton class="dialog-primary-action" type="submit" :disabled="saving || !vaultUnlocked">{{
              t(saving ? 'settings.credentials.action.saving' : 'settings.credentials.action.save')
            }}</QtermButton>
          </div>
        </QtermDialogFooter>
      </form>
    </QtermDialogContent>
  </QtermDialog>
</template>
