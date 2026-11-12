<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ExternalSubtitle } from '@/api/catalog'
import type { ServerRef } from '@/api/federation'
import { defaultSubtitleLanguage, subtitleLanguages, subtitlesApi, type SubtitleResult } from '@/api/subtitles'
import BaseDialog from '@/components/BaseDialog.vue'
import ErrorAlert from '@/components/ErrorAlert.vue'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { languageName } from '@/utils/format'

const props = defineProps<{ server: ServerRef | null; itemId: string; versionId: string }>()
const emit = defineEmits<{ downloaded: [subtitle: ExternalSubtitle] }>()
const open = defineModel<boolean>('open', { required: true })

const { t, locale } = useI18n()
const errorMessage = useErrorMessage()

const language = ref(defaultSubtitleLanguage(locale.value))
const results = ref<SubtitleResult[] | null>(null)
const searching = ref(false)
const downloading = ref<number | null>(null)
const error = ref('')
let seq = 0

const languages = computed(() =>
  subtitleLanguages
    .map((code) => ({ code, name: languageName(code, locale.value) ?? code }))
    .sort((a, b) => a.name.localeCompare(b.name, locale.value)),
)

async function search() {
  const my = ++seq
  searching.value = true
  error.value = ''
  results.value = null
  try {
    const res = await subtitlesApi.search(props.server, props.itemId, props.versionId, language.value)
    if (my === seq) results.value = res.results
  } catch (err) {
    if (my === seq) error.value = errorMessage(err)
  } finally {
    if (my === seq) searching.value = false
  }
}

watch(
  [open, language],
  ([isOpen]) => {
    if (isOpen) void search()
  },
  { immediate: true },
)

async function pick(r: SubtitleResult) {
  downloading.value = r.fileId
  error.value = ''
  try {
    const sub = await subtitlesApi.download(props.server, props.itemId, props.versionId, r)
    emit('downloaded', sub)
    open.value = false
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    downloading.value = null
  }
}
</script>

<template>
  <BaseDialog v-model:open="open" :title="t('subtitleSearch.title')" wide>
    <div class="space-y-4">
      <div>
        <label for="subtitle-language" class="block text-sm font-medium text-slate-300">{{ t('common.language') }}</label>
        <select
          id="subtitle-language"
          v-model="language"
          class="mt-1.5 block w-full rounded-lg border border-white/10 bg-surface-800 px-3.5 py-2.5 text-slate-100 focus:border-accent-500 focus:ring-2 focus:ring-accent-500/30 focus:outline-none sm:w-64"
        >
          <option v-for="l in languages" :key="l.code" :value="l.code">{{ l.name }}</option>
        </select>
      </div>

      <ErrorAlert v-if="error" :message="error" />

      <div v-if="searching" class="space-y-2" role="status" :aria-label="t('common.loading')">
        <div v-for="n in 4" :key="n" class="h-14 animate-pulse rounded-xl bg-surface-800/60" />
      </div>
      <p v-else-if="results && !results.length" class="py-6 text-center text-sm text-slate-400">{{ t('subtitleSearch.empty') }}</p>
      <ul v-else-if="results" class="space-y-1.5">
        <li v-for="r in results" :key="r.fileId">
          <button
            type="button"
            class="flex w-full items-center gap-3 rounded-xl border border-white/5 bg-surface-800/60 px-3.5 py-2.5 text-left transition hover:border-accent-500/40 hover:bg-surface-800 disabled:opacity-60"
            :disabled="downloading !== null"
            @click="pick(r)"
          >
            <span class="min-w-0 flex-1">
              <span class="block truncate text-sm text-slate-100" :title="r.release">{{ r.release || t('subtitleSearch.untitled') }}</span>
              <span class="mt-1 flex flex-wrap items-center gap-1.5 text-xs text-slate-400">
                <span v-if="r.hashMatch" class="rounded-full bg-emerald-500/15 px-2 py-0.5 text-emerald-300">{{ t('subtitleSearch.hashMatch') }}</span>
                <span v-if="r.hearingImpaired" class="rounded-full bg-white/10 px-2 py-0.5">{{ t('subtitleSearch.hearingImpaired') }}</span>
                <span v-if="r.machineTranslated" class="rounded-full bg-amber-500/15 px-2 py-0.5 text-amber-300">{{ t('subtitleSearch.machineTranslated') }}</span>
                <span>{{ t('subtitleSearch.downloads', { n: r.downloadCount }) }}</span>
              </span>
            </span>
            <span v-if="downloading === r.fileId" class="size-5 shrink-0 animate-spin rounded-full border-2 border-white/20 border-t-accent-400" role="status" />
          </button>
        </li>
      </ul>
      <p class="text-xs text-slate-500">{{ t('subtitleSearch.attribution') }}</p>
    </div>
  </BaseDialog>
</template>
