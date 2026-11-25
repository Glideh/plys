<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { catalogApi, type MatchCandidate } from '@/api/catalog'
import type { ItemKind } from '@/api/libraries'
import { useErrorMessage } from '@/composables/useErrorMessage'
import BaseButton from './BaseButton.vue'
import BaseDialog from './BaseDialog.vue'
import ErrorAlert from './ErrorAlert.vue'
import ItemLocation from './ItemLocation.vue'

// folder and paths show where the files sit, to help find what to search for.
const props = defineProps<{ itemId: string; kind: ItemKind; title: string; year: number | null; folder?: string; paths?: string[] }>()
const emit = defineEmits<{ matched: [merged: boolean] }>()
const open = defineModel<boolean>('open', { required: true })
const { t } = useI18n()
const errorMessage = useErrorMessage()

const query = ref('')
const year = ref('')
const provider = ref<'tmdb' | 'fankai'>('tmdb')
const results = ref<MatchCandidate[] | null>(null)
const searching = ref(false)
const choosing = ref<number | null>(null)
const error = ref('')

async function search() {
  searching.value = true
  error.value = ''
  try {
    const searchYear = Number(year.value) || undefined
    results.value =
      provider.value === 'fankai'
        ? await catalogApi.searchFanKaiMatch(props.itemId, query.value.trim(), searchYear)
        : await catalogApi.searchMatch(props.itemId, query.value.trim(), searchYear)
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    searching.value = false
  }
}

async function choose(c: MatchCandidate) {
  choosing.value = 'tmdbId' in c ? c.tmdbId : c.fankaiId
  error.value = ''
  try {
    const res =
      'tmdbId' in c
        ? await catalogApi.setMatch(props.itemId, c.tmdbId)
        : await catalogApi.setFanKaiMatch(props.itemId, c.fankaiId)
    open.value = false
    emit('matched', !!res.merged)
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    choosing.value = null
  }
}

watch(open, (isOpen) => {
  if (!isOpen) return
  provider.value = 'tmdb'
  query.value = props.title
  year.value = props.year ? String(props.year) : ''
  results.value = null
  error.value = ''
  search()
})
</script>

<template>
  <BaseDialog v-model:open="open" :title="t('catalog.fixMatch')" wide>
    <p class="mb-4 text-sm text-slate-400">{{ t('catalog.matchHelp') }}</p>
    <div v-if="kind === 'series'" class="mb-4 flex gap-2">
      <button
        type="button"
        class="rounded-lg border px-3 py-1.5 text-sm"
        :class="provider === 'tmdb' ? 'border-accent-500 bg-accent-500/10 text-accent-300' : 'border-white/10 text-slate-400 hover:text-slate-200'"
        :aria-pressed="provider === 'tmdb'"
        :disabled="searching || choosing !== null"
        @click="provider = 'tmdb'; results = null; error = ''"
      >
        {{ t('catalog.matchProviderTmdb') }}
      </button>
      <button
        type="button"
        class="rounded-lg border px-3 py-1.5 text-sm"
        :class="provider === 'fankai' ? 'border-accent-500 bg-accent-500/10 text-accent-300' : 'border-white/10 text-slate-400 hover:text-slate-200'"
        :aria-pressed="provider === 'fankai'"
        :disabled="searching || choosing !== null"
        @click="provider = 'fankai'; results = null; error = ''"
      >
        {{ t('catalog.matchProviderFanKai') }}
      </button>
    </div>
    <p v-if="provider === 'fankai'" class="mb-4 text-sm text-slate-400">{{ t('catalog.matchFanKaiHelp') }}</p>
    <ItemLocation v-if="folder || paths?.length" class="mb-4 rounded-xl border border-white/5 bg-surface-800/40 p-3" :folder="folder" :paths="paths ?? []" />
    <form class="flex flex-col gap-2 sm:flex-row" @submit.prevent="search">
      <input
        v-model="query"
        :aria-label="t('catalog.matchSearch')"
        :placeholder="t('catalog.matchSearch')"
        class="flex-1 rounded-lg border border-white/10 bg-surface-800 px-3.5 py-2 text-sm focus:border-accent-500 focus:ring-2 focus:ring-accent-500/30 focus:outline-none"
      />
      <input
        v-model="year"
        inputmode="numeric"
        maxlength="4"
        :aria-label="t('catalog.matchYear')"
        :placeholder="t('catalog.matchYear')"
        class="rounded-lg border border-white/10 bg-surface-800 px-3.5 py-2 text-sm focus:border-accent-500 focus:ring-2 focus:ring-accent-500/30 focus:outline-none sm:w-24"
      />
      <BaseButton type="submit" variant="primary" :loading="searching">{{ t('catalog.matchSearchButton') }}</BaseButton>
    </form>

    <ErrorAlert v-if="error" class="mt-4" :message="error" />

    <p v-if="results && results.length === 0" class="mt-6 text-center text-sm text-slate-500">
      {{ t(provider === 'fankai' ? 'catalog.matchNoResultsFanKai' : 'catalog.matchNoResults') }}
    </p>
    <ul v-if="results?.length" class="mt-4 space-y-2">
      <li v-for="c in results" :key="'tmdbId' in c ? `tmdb-${c.tmdbId}` : `fankai-${c.fankaiId}`">
        <button
          type="button"
          :disabled="choosing !== null"
          class="flex w-full gap-4 rounded-xl border border-white/5 bg-surface-800/60 p-3 text-left transition hover:border-accent-500/60 hover:bg-surface-800 disabled:opacity-50"
          @click="choose(c)"
        >
          <img v-if="'posterUrl' in c && c.posterUrl" :src="c.posterUrl" alt="" loading="lazy" referrerpolicy="no-referrer" class="h-24 w-16 shrink-0 rounded-md object-cover" />
          <div v-else class="h-24 w-16 shrink-0 rounded-md bg-surface-700" />
          <div class="min-w-0">
            <p class="font-semibold">
              {{ c.title }} <span v-if="c.year" class="font-normal text-slate-400">({{ c.year }})</span>
            </p>
            <p v-if="c.originalTitle && c.originalTitle !== c.title" class="text-xs text-slate-500">{{ c.originalTitle }}</p>
            <p v-if="c.overview" class="mt-1 line-clamp-2 text-xs text-slate-400">{{ c.overview }}</p>
          </div>
        </button>
      </li>
    </ul>
  </BaseDialog>
</template>
