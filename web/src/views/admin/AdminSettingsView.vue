<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { metadataLanguageNames, metadataLanguages, settingsApi, type ServerSettings } from '@/api/settings'
import BaseButton from '@/components/BaseButton.vue'
import ErrorAlert from '@/components/ErrorAlert.vue'
import { useErrorMessage } from '@/composables/useErrorMessage'

const { t } = useI18n()
const errorMessage = useErrorMessage()

const current = ref<ServerSettings | null>(null)
const newKey = ref('')
const saving = ref(false)
const error = ref('')
const notice = ref('')

async function save(body: Parameters<typeof settingsApi.update>[0]) {
  saving.value = true
  error.value = ''
  notice.value = ''
  try {
    current.value = await settingsApi.update(body)
    newKey.value = ''
    notice.value = t('settings.saved')
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  try {
    current.value = await settingsApi.get()
  } catch (err) {
    error.value = errorMessage(err)
  }
})
</script>

<template>
  <section class="max-w-2xl space-y-6">
    <ErrorAlert v-if="error" :message="error" />
    <p v-if="notice" role="status" class="rounded-lg border border-emerald-500/30 bg-emerald-500/10 px-3.5 py-2.5 text-sm text-emerald-300">
      {{ notice }}
    </p>

    <div v-if="!current" class="h-64 animate-pulse rounded-2xl bg-surface-800/60" />

    <div v-else class="space-y-6 rounded-2xl border border-white/5 bg-surface-900 p-6">
      <div>
        <h2 class="text-lg font-semibold">{{ t('settings.metadata') }}</h2>
        <p class="mt-1 text-sm leading-relaxed text-slate-400">{{ t('settings.metadataHelp') }}</p>
        <a
          href="https://www.themoviedb.org/settings/api"
          target="_blank"
          rel="noopener noreferrer"
          class="mt-2 inline-block text-sm font-medium text-accent-400 hover:text-accent-300"
        >
          {{ t('settings.getKey') }} ↗
        </a>
      </div>

      <form class="space-y-3" @submit.prevent="save({ tmdbApiKey: newKey.trim() })">
        <label for="tmdb-key" class="block text-sm font-medium text-slate-300">{{ t('settings.tmdbKey') }}</label>
        <p class="flex items-center gap-2 text-sm">
          <span :class="current.tmdbConfigured ? 'bg-emerald-400' : 'bg-amber-400'" class="size-2 rounded-full" aria-hidden="true" />
          <span :class="current.tmdbConfigured ? 'text-emerald-300' : 'text-amber-300'">
            {{ current.tmdbConfigured ? t('settings.keyConfigured') : t('settings.keyMissing') }}
          </span>
        </p>
        <div class="flex flex-col gap-2 sm:flex-row">
          <input
            id="tmdb-key"
            v-model="newKey"
            type="password"
            autocomplete="off"
            spellcheck="false"
            :placeholder="t('settings.keyPlaceholder')"
            class="block w-full rounded-lg border border-white/10 bg-surface-800 px-3.5 py-2.5 font-mono text-sm text-slate-100 placeholder-slate-500 focus:border-accent-500 focus:ring-2 focus:ring-accent-500/30 focus:outline-none"
          />
          <BaseButton type="submit" variant="primary" :loading="saving" :disabled="!newKey.trim()" class="shrink-0">
            {{ t('settings.saveKey') }}
          </BaseButton>
        </div>
        <BaseButton
          v-if="current.tmdbConfigured"
          class="hover:!bg-red-500/15 hover:!text-red-300"
          :disabled="saving"
          @click="save({ tmdbApiKey: '' })"
        >
          {{ t('settings.removeKey') }}
        </BaseButton>
      </form>

      <div class="border-t border-white/5 pt-6">
        <label for="metadata-language" class="block text-sm font-medium text-slate-300">{{ t('settings.language') }}</label>
        <select
          id="metadata-language"
          :value="current.metadataLanguage"
          :disabled="saving"
          class="mt-1.5 block w-full rounded-lg border border-white/10 bg-surface-800 px-3.5 py-2.5 text-slate-100 focus:border-accent-500 focus:ring-2 focus:ring-accent-500/30 focus:outline-none sm:w-64"
          @change="save({ metadataLanguage: ($event.target as HTMLSelectElement).value })"
        >
          <option v-for="l in metadataLanguages" :key="l" :value="l">{{ metadataLanguageNames[l] }}</option>
        </select>
        <p class="mt-1.5 text-xs text-slate-500">{{ t('settings.languageHelp') }}</p>
      </div>

      <p class="border-t border-white/5 pt-4 text-xs text-slate-500">{{ t('settings.attribution') }}</p>
    </div>
  </section>
</template>
