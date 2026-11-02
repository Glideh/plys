<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { metadataLanguageNames, metadataLanguages, settingsApi } from '@/api/settings'
import AuthCard from '@/components/AuthCard.vue'
import BaseButton from '@/components/BaseButton.vue'
import ErrorAlert from '@/components/ErrorAlert.vue'
import SubmitButton from '@/components/SubmitButton.vue'
import TextField from '@/components/TextField.vue'
import { useErrorMessage } from '@/composables/useErrorMessage'

const { t } = useI18n()
const router = useRouter()
const errorMessage = useErrorMessage()

const tmdbKey = ref('')
const metadataLanguage = ref<string>(metadataLanguages[0])
const loading = ref(false)
const error = ref('')

onMounted(async () => {
  try {
    metadataLanguage.value = (await settingsApi.get()).metadataLanguage
  } catch (err) {
    error.value = errorMessage(err)
  }
})

async function submit() {
  error.value = ''
  loading.value = true
  try {
    const key = tmdbKey.value.trim()
    await settingsApi.update({ metadataLanguage: metadataLanguage.value, ...(key ? { tmdbApiKey: key } : {}) })
    await router.replace({ name: 'home' })
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <AuthCard :title="t('setupMetadata.title')" :subtitle="t('setupMetadata.subtitle')">
    <form class="space-y-5" @submit.prevent="submit">
      <TextField v-model="tmdbKey" :label="t('settings.tmdbKey')" type="password" autocomplete="off" />

      <div>
        <label for="setup-metadata-language" class="mb-1.5 block text-sm font-medium text-slate-300">{{ t('settings.language') }}</label>
        <select
          id="setup-metadata-language"
          v-model="metadataLanguage"
          :disabled="loading"
          class="block w-full rounded-lg border border-white/10 bg-surface-800 px-3.5 py-2.5 text-slate-100 focus:border-accent-500 focus:ring-2 focus:ring-accent-500/30 focus:outline-none"
        >
          <option v-for="l in metadataLanguages" :key="l" :value="l">{{ metadataLanguageNames[l] }}</option>
        </select>
      </div>

      <ErrorAlert v-if="error" :message="error" />

      <SubmitButton :loading="loading">{{ t('common.save') }}</SubmitButton>
    </form>

    <BaseButton type="button" class="mt-3 w-full" :disabled="loading" @click="router.replace({ name: 'home' })">
      {{ t('setupMetadata.skip') }}
    </BaseButton>
  </AuthCard>
</template>
