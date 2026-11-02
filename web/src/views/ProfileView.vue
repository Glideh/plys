<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import ErrorAlert from '@/components/ErrorAlert.vue'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { localeLabels, locales, type Locale } from '@/i18n'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const auth = useAuthStore()
const errorMessage = useErrorMessage()

const saving = ref(false)
const error = ref('')

async function changeLocale(event: Event) {
  error.value = ''
  saving.value = true
  try {
    await auth.updatePreferences({ locale: (event.target as HTMLSelectElement).value as Locale })
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <section class="max-w-2xl space-y-6 px-4 py-8 sm:px-8">
    <div>
      <h1 class="text-2xl font-semibold">{{ t('profile.title') }}</h1>
      <p class="mt-1 text-sm text-slate-400">{{ t('profile.subtitle') }}</p>
    </div>

    <ErrorAlert v-if="error" :message="error" />

    <div class="space-y-3 rounded-2xl border border-white/5 bg-surface-900 p-6">
      <label for="profile-locale" class="block text-sm font-medium text-slate-300">{{ t('profile.language') }}</label>
      <select
        id="profile-locale"
        :value="auth.user?.locale"
        :disabled="saving"
        class="block w-full rounded-lg border border-white/10 bg-surface-800 px-3.5 py-2.5 text-slate-100 focus:border-accent-500 focus:ring-2 focus:ring-accent-500/30 focus:outline-none sm:w-64"
        @change="changeLocale"
      >
        <option v-for="l in locales" :key="l" :value="l">{{ localeLabels[l] }}</option>
      </select>
      <p class="text-xs text-slate-500">{{ t('profile.languageHelp') }}</p>
    </div>
  </section>
</template>
