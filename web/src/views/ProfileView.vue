<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseButton from '@/components/BaseButton.vue'
import ErrorAlert from '@/components/ErrorAlert.vue'
import UserAvatar from '@/components/UserAvatar.vue'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { localeLabels, locales, type Locale } from '@/i18n'
import { useAuthStore } from '@/stores/auth'

const MAX_AVATAR_BYTES = 2 * 1024 * 1024

const { t } = useI18n()
const auth = useAuthStore()
const errorMessage = useErrorMessage()

const saving = ref(false)
const error = ref('')
const avatarInput = ref<HTMLInputElement | null>(null)
const avatarBusy = ref(false)

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

async function onAvatarPicked(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  error.value = ''
  if (file.size > MAX_AVATAR_BYTES) {
    error.value = t('profile.avatarTooLarge')
    return
  }
  avatarBusy.value = true
  try {
    await auth.setAvatar(file)
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    avatarBusy.value = false
  }
}

async function removeAvatar() {
  error.value = ''
  avatarBusy.value = true
  try {
    await auth.removeAvatar()
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    avatarBusy.value = false
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

    <div class="flex items-center gap-5 rounded-2xl border border-white/5 bg-surface-900 p-6">
      <UserAvatar :user="auth.user" class="size-20 text-2xl" />
      <div class="min-w-0 flex-1 space-y-3">
        <p class="text-sm font-medium text-slate-300">{{ t('profile.avatar') }}</p>
        <p class="text-xs text-slate-500">{{ t('profile.avatarHelp') }}</p>
        <div class="flex flex-wrap gap-2">
          <BaseButton :loading="avatarBusy" @click="avatarInput?.click()">{{ t('profile.avatarUpload') }}</BaseButton>
          <BaseButton v-if="auth.user?.avatarUpdatedAt" :disabled="avatarBusy" variant="danger" @click="removeAvatar">
            {{ t('profile.avatarRemove') }}
          </BaseButton>
        </div>
        <input
          ref="avatarInput"
          type="file"
          accept="image/jpeg,image/png"
          class="hidden"
          @change="onAvatarPicked"
        />
      </div>
    </div>

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
