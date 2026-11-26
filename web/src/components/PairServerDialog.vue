<script setup lang="ts">
import { onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { peersApi, type Peer } from '@/api/peers'
import { useErrorMessage } from '@/composables/useErrorMessage'
import BaseButton from './BaseButton.vue'
import BaseDialog from './BaseDialog.vue'
import ErrorAlert from './ErrorAlert.vue'

/** Both halves of a pairing in one dialog: this server's code to hand over, and the code received from the other one. */
const emit = defineEmits<{ paired: [peer: Peer & { pairingPending: boolean }] }>()
const open = defineModel<boolean>('open', { required: true })
const { t } = useI18n()
const errorMessage = useErrorMessage()

const ownCode = ref('')
const ownCodeError = ref('')
const copied = ref(false)
let copiedTimer: ReturnType<typeof setTimeout> | undefined

const code = ref('')
const adding = ref(false)
const error = ref('')

const textareaClass =
  'mt-1.5 block w-full rounded-lg border border-white/10 bg-surface-800 px-3.5 py-2.5 font-mono text-xs break-all text-slate-100 placeholder-slate-500 focus:border-accent-500 focus:ring-2 focus:ring-accent-500/30 focus:outline-none'

watch(open, async (isOpen) => {
  if (!isOpen) return
  code.value = ''
  error.value = ''
  copied.value = false
  if (ownCode.value) return
  ownCodeError.value = ''
  try {
    ownCode.value = (await peersApi.pairingCode()).code
  } catch (err) {
    ownCodeError.value = errorMessage(err)
  }
})

async function copy() {
  try {
    await navigator.clipboard.writeText(ownCode.value)
    copied.value = true
    clearTimeout(copiedTimer)
    copiedTimer = setTimeout(() => (copied.value = false), 2000)
  } catch {
    ownCodeError.value = t('invitations.copyFailed')
  }
}

async function submit() {
  adding.value = true
  error.value = ''
  try {
    const peer = await peersApi.add(code.value.trim())
    emit('paired', peer)
    open.value = false
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    adding.value = false
  }
}

onUnmounted(() => clearTimeout(copiedTimer))
</script>

<template>
  <BaseDialog v-model:open="open" :title="t('servers.addTitle')" wide>
    <div class="space-y-6">
      <section>
        <h3 class="text-sm font-semibold text-slate-100">{{ t('servers.pairStepOwn') }}</h3>
        <p class="mt-1 text-sm leading-relaxed text-slate-400">{{ t('servers.pairingHelp') }}</p>
        <ErrorAlert v-if="ownCodeError" class="mt-3" :message="ownCodeError" />
        <div v-if="ownCode" class="mt-3">
          <label for="pairing-code" class="block text-sm font-medium text-slate-300">{{ t('servers.codeLabel') }}</label>
          <textarea
            id="pairing-code"
            :value="ownCode"
            readonly
            rows="3"
            :class="textareaClass"
            @focus="($event.target as HTMLTextAreaElement).select()"
          />
          <BaseButton class="mt-2" @click="copy">{{ copied ? t('invitations.copied') : t('invitations.copy') }}</BaseButton>
        </div>
        <div v-else-if="!ownCodeError" class="mt-3 h-20 animate-pulse rounded-lg bg-surface-800/60" />
      </section>

      <form id="pair-form" class="border-t border-white/5 pt-6" @submit.prevent="submit">
        <h3 class="text-sm font-semibold text-slate-100">{{ t('servers.pairStepTheirs') }}</h3>
        <p class="mt-1 text-sm leading-relaxed text-slate-400">{{ t('servers.addHelp') }}</p>
        <label for="peer-code" class="mt-3 block text-sm font-medium text-slate-300">{{ t('servers.addCode') }}</label>
        <textarea id="peer-code" v-model="code" rows="3" required :class="textareaClass" />
        <ErrorAlert v-if="error" class="mt-3" :message="error" />
      </form>
    </div>
    <template #footer>
      <BaseButton @click="open = false">{{ t('common.cancel') }}</BaseButton>
      <BaseButton type="submit" form="pair-form" variant="primary" :loading="adding" :disabled="!code.trim()">{{ t('servers.add') }}</BaseButton>
    </template>
  </BaseDialog>
</template>
