<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import AuthCard from '@/components/AuthCard.vue'
import SubmitButton from '@/components/SubmitButton.vue'
import TextField from '@/components/TextField.vue'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { localeLabels, locales, setLocale, type Locale } from '@/i18n'
import { useAuthStore } from '@/stores/auth'

const { t, locale: currentLocale } = useI18n()
const auth = useAuthStore()
const router = useRouter()
const errorMessage = useErrorMessage()

const username = ref('')
const password = ref('')
const confirm = ref('')
const locale = ref(currentLocale.value as Locale)
const loading = ref(false)
const error = ref('')

async function submit() {
  error.value = ''
  if (password.value !== confirm.value) {
    error.value = t('errors.passwordMismatch')
    return
  }
  loading.value = true
  try {
    await auth.setupAdmin(username.value.trim(), password.value, locale.value)
    await router.replace({ name: 'setup-metadata' })
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <AuthCard :title="t('setup.title')" :subtitle="t('setup.subtitle')">
    <form class="space-y-5" @submit.prevent="submit">
      <TextField
        v-model="username"
        :label="t('common.username')"
        :hint="t('setup.usernameHint')"
        autocomplete="username"
        required
        :maxlength="32"
      />
      <TextField
        v-model="password"
        :label="t('common.password')"
        :hint="t('setup.passwordHint')"
        type="password"
        autocomplete="new-password"
        required
        :minlength="8"
      />
      <TextField v-model="confirm" :label="t('setup.confirmPassword')" type="password" autocomplete="new-password" required />

      <div>
        <label for="setup-locale" class="mb-1.5 block text-sm font-medium text-slate-300">{{ t('common.language') }}</label>
        <select
          id="setup-locale"
          v-model="locale"
          class="block w-full rounded-lg border border-white/10 bg-surface-800 px-3.5 py-2.5 text-slate-100 focus:border-accent-500 focus:ring-2 focus:ring-accent-500/30 focus:outline-none"
          @change="setLocale(locale)"
        >
          <option v-for="l in locales" :key="l" :value="l">{{ localeLabels[l] }}</option>
        </select>
      </div>

      <p v-if="error" role="alert" class="rounded-lg border border-red-500/30 bg-red-500/10 px-3.5 py-2.5 text-sm text-red-300">
        {{ error }}
      </p>

      <SubmitButton :loading="loading">{{ t('setup.submit') }}</SubmitButton>
    </form>
  </AuthCard>
</template>
