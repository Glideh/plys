<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { deviceLoginsApi, type DeviceLogin } from '@/api/deviceLogins'
import AuthCard from '@/components/AuthCard.vue'
import BaseButton from '@/components/BaseButton.vue'
import SubmitButton from '@/components/SubmitButton.vue'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { useAuthStore } from '@/stores/auth'

// Opened from the QR code of the TV (/link?code=…) or by hand: the user checks the code and approves the sign-in.
const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const errorMessage = useErrorMessage()

type Step = 'enter' | 'confirm' | 'approved' | 'denied'

const step = ref<Step>('enter')
const code = ref(typeof route.query.code === 'string' ? route.query.code : '')
const login = ref<DeviceLogin | null>(null)
const loading = ref(false)
const error = ref('')

async function lookup() {
  error.value = ''
  loading.value = true
  try {
    login.value = await deviceLoginsApi.lookup(code.value.trim())
    step.value = 'confirm'
  } catch (err) {
    error.value = errorMessage(err)
    step.value = 'enter'
  } finally {
    loading.value = false
  }
}

async function decide(approve: boolean) {
  if (!login.value) return
  error.value = ''
  loading.value = true
  try {
    if (approve) await deviceLoginsApi.approve(login.value.userCode)
    else await deviceLoginsApi.deny(login.value.userCode)
    step.value = approve ? 'approved' : 'denied'
  } catch (err) {
    error.value = errorMessage(err)
    step.value = 'enter'
  } finally {
    loading.value = false
  }
}

function restart() {
  code.value = ''
  login.value = null
  error.value = ''
  step.value = 'enter'
  // The code of the QR code is used: it is dropped from the address so that a reload does not show it again.
  void router.replace({ name: 'link' })
}

onMounted(() => {
  if (code.value.trim()) void lookup()
})
</script>

<template>
  <AuthCard :title="t('link.title')" :subtitle="t('link.subtitle')">
    <div class="space-y-5">
      <p class="text-sm text-slate-400">{{ t('link.account', { name: auth.user?.displayName ?? '' }) }}</p>

      <p v-if="error" role="alert" class="rounded-lg border border-red-500/30 bg-red-500/10 px-3.5 py-2.5 text-sm text-red-300">
        {{ error }}
      </p>

      <form v-if="step === 'enter'" class="space-y-5" @submit.prevent="lookup">
        <div>
          <label for="link-code" class="mb-1.5 block text-sm font-medium text-slate-300">{{ t('link.code') }}</label>
          <input
            id="link-code"
            v-model="code"
            autocomplete="off"
            autocapitalize="characters"
            spellcheck="false"
            maxlength="12"
            required
            aria-describedby="link-code-hint"
            class="block w-full rounded-lg border border-white/10 bg-surface-800 px-3.5 py-2.5 font-mono text-lg tracking-[0.3em] text-slate-100 uppercase transition focus:border-accent-500 focus:ring-2 focus:ring-accent-500/30 focus:outline-none"
          />
          <p id="link-code-hint" class="mt-1.5 text-xs text-slate-500">{{ t('link.codeHint') }}</p>
        </div>
        <SubmitButton :loading="loading">{{ t('link.continue') }}</SubmitButton>
      </form>

      <div v-else-if="step === 'confirm' && login" class="space-y-5">
        <p class="text-sm leading-relaxed text-slate-300">{{ t('link.confirm') }}</p>
        <p class="rounded-xl bg-surface-800 py-4 text-center font-mono text-3xl font-semibold tracking-[0.25em] text-slate-100">
          {{ login.userCode }}
        </p>
        <p class="text-sm text-slate-300">{{ t('link.device', { name: login.deviceName || t('link.unnamedDevice') }) }}</p>
        <p class="text-xs text-slate-500">{{ t('link.warning') }}</p>
        <div class="grid grid-cols-2 gap-3">
          <BaseButton :disabled="loading" @click="decide(false)">{{ t('link.deny') }}</BaseButton>
          <BaseButton variant="primary" :loading="loading" @click="decide(true)">{{ t('link.approve') }}</BaseButton>
        </div>
      </div>

      <div v-else class="space-y-5">
        <p role="status" class="rounded-lg border border-white/10 bg-white/5 px-3.5 py-2.5 text-sm text-slate-200">
          {{ step === 'approved' ? t('link.approved') : t('link.denied') }}
        </p>
        <div class="flex justify-between gap-3 text-sm">
          <button type="button" class="text-accent-400 hover:underline" @click="restart">{{ t('link.another') }}</button>
          <RouterLink :to="{ name: 'home' }" class="text-accent-400 hover:underline">{{ t('link.toHome') }}</RouterLink>
        </div>
      </div>
    </div>
  </AuthCard>
</template>
