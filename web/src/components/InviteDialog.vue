<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { invitationsApi, type CreatedInvitation } from '@/api/invitations'
import BaseButton from '@/components/BaseButton.vue'
import BaseDialog from '@/components/BaseDialog.vue'
import ErrorAlert from '@/components/ErrorAlert.vue'
import QrCode from '@/components/QrCode.vue'
import { useErrorMessage } from '@/composables/useErrorMessage'
import type { Locale } from '@/i18n'

const open = defineModel<boolean>('open', { required: true })
const emit = defineEmits<{ created: [] }>()

const { t, d, locale } = useI18n()
const errorMessage = useErrorMessage()

const invitation = ref<CreatedInvitation | null>(null)
const creating = ref(false)
const error = ref('')
const copied = ref<'url' | 'pin' | null>(null)
let copiedTimer: ReturnType<typeof setTimeout> | undefined

// Each opening creates a new invitation: its link and PIN are only known now.
watch(open, async (isOpen) => {
  if (!isOpen) return
  invitation.value = null
  error.value = ''
  copied.value = null
  creating.value = true
  try {
    invitation.value = await invitationsApi.create(locale.value as Locale)
    emit('created')
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    creating.value = false
  }
})

async function copy(what: 'url' | 'pin') {
  if (!invitation.value) return
  try {
    await navigator.clipboard.writeText(what === 'url' ? invitation.value.url : invitation.value.pin)
    copied.value = what
    clearTimeout(copiedTimer)
    copiedTimer = setTimeout(() => (copied.value = null), 2000)
  } catch {
    error.value = t('invitations.copyFailed')
  }
}

function selectAll(event: FocusEvent) {
  ;(event.target as HTMLInputElement).select()
}
</script>

<template>
  <BaseDialog v-model:open="open" :title="t('invitations.title')">
    <div v-if="creating" class="h-72 animate-pulse rounded-xl bg-surface-800/60" />
    <div v-else-if="invitation" class="space-y-5">
      <QrCode :value="invitation.url" :label="t('invitations.qrLabel')" class="mx-auto size-48" />

      <div>
        <label for="invite-url" class="mb-1.5 block text-sm font-medium text-slate-300">{{ t('invitations.link') }}</label>
        <div class="flex gap-2">
          <input
            id="invite-url"
            :value="invitation.url"
            readonly
            class="block min-w-0 flex-1 rounded-lg border border-white/10 bg-surface-800 px-3.5 py-2.5 text-sm text-slate-100 focus:border-accent-500 focus:outline-none"
            @focus="selectAll"
          />
          <BaseButton @click="copy('url')">{{ copied === 'url' ? t('invitations.copied') : t('invitations.copy') }}</BaseButton>
        </div>
      </div>

      <div>
        <span class="mb-1.5 block text-sm font-medium text-slate-300">{{ t('invitations.pin') }}</span>
        <div class="flex items-center gap-3">
          <span class="font-mono text-3xl font-semibold tracking-[0.4em] text-slate-100" data-testid="invite-pin">{{ invitation.pin }}</span>
          <BaseButton @click="copy('pin')">{{ copied === 'pin' ? t('invitations.copied') : t('invitations.copy') }}</BaseButton>
        </div>
      </div>

      <p class="rounded-lg border border-accent-500/20 bg-accent-500/10 px-3.5 py-2.5 text-sm leading-relaxed text-slate-300">
        {{ t('invitations.channels') }}
      </p>
      <p class="text-xs text-slate-500">
        {{ t('invitations.expires', { date: d(new Date(invitation.expiresAt), 'long') }) }}
      </p>
    </div>
    <ErrorAlert v-if="error" class="mt-4" :message="error" />
    <template #footer>
      <BaseButton variant="primary" @click="open = false">{{ t('common.close') }}</BaseButton>
    </template>
  </BaseDialog>
</template>
