<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { type HomeItem } from '@/api/progress'
import { formatClock } from '@/utils/player'

defineProps<{ title: string; empty: string; items: HomeItem[]; loading: boolean; resume?: boolean }>()
const { t } = useI18n()

function label(it: HomeItem): string {
  if (it.kind === 'episode') {
    const code = `S${String(it.seasonNumber ?? 0).padStart(2, '0')}E${String(it.episodeNumber ?? 0).padStart(2, '0')}`
    return [code, it.title].filter(Boolean).join(' · ')
  }
  if (it.kind === 'part') return t('catalog.part', { n: it.partNumber })
  return ''
}

function subtitle(it: HomeItem, resume: boolean): string | null {
  const parts = [label(it)]
  if (resume && it.durationSeconds) parts.push(t('home.remaining', { time: formatClock(it.durationSeconds - it.positionSeconds) }))
  return parts.filter(Boolean).join(' · ') || null
}
</script>

<template>
  <section class="min-w-0">
    <h2 class="mb-4 text-lg font-semibold">{{ title }}</h2>
    <div v-if="loading" class="flex gap-4 overflow-hidden">
      <div v-for="i in 6" :key="i" class="aspect-[2/3] w-36 shrink-0 animate-pulse rounded-xl bg-surface-800/60" />
    </div>
    <p v-else-if="items.length === 0" class="rounded-xl border border-dashed border-white/10 px-6 py-10 text-center text-sm text-slate-500">
      {{ empty }}
    </p>
    <ul v-else class="-mx-4 flex gap-4 overflow-x-auto px-4 pb-3 sm:-mx-8 sm:px-8">
      <li v-for="it in items" :key="it.id" class="w-36 shrink-0 sm:w-40">
        <div class="block">
          <RouterLink
            :to="{ name: 'watch', params: { id: it.id }, query: resume && it.positionSeconds ? { t: Math.floor(it.positionSeconds) } : {} }"
            :aria-label="t('catalog.play')"
            class="group relative block aspect-[2/3] overflow-hidden rounded-xl bg-surface-800 shadow-lg shadow-black/30 ring-1 ring-white/5 focus-visible:ring-2 focus-visible:ring-accent-400 focus-visible:outline-none"
          >
            <img
              v-if="it.posterUrl"
              :src="it.posterUrl"
              :alt="it.showTitle || it.title"
              loading="lazy"
              decoding="async"
              class="size-full object-cover"
            />
            <div
              v-else
              class="grid size-full place-items-center bg-gradient-to-br from-surface-700 to-surface-900 text-4xl font-black text-white/20"
              aria-hidden="true"
            >
              {{ (it.showTitle || it.title).trim().charAt(0).toUpperCase() || '?' }}
            </div>
            <span class="absolute inset-0 grid place-items-center bg-black/25 transition group-hover:bg-black/45" aria-hidden="true">
              <span class="grid size-12 place-items-center rounded-full bg-black/60 text-white shadow-lg transition group-hover:scale-110 group-hover:bg-accent-500 group-hover:text-surface-950">
                <svg viewBox="0 0 24 24" class="size-6" fill="currentColor"><path d="M8 5v14l11-7z" /></svg>
              </span>
            </span>
            <div v-if="resume && it.durationSeconds" class="absolute inset-x-0 bottom-0 h-1.5 bg-black/60" role="progressbar" :aria-valuenow="Math.round((it.positionSeconds / it.durationSeconds) * 100)" aria-valuemin="0" aria-valuemax="100">
              <div class="h-full bg-accent-400" :style="{ width: Math.min(it.positionSeconds / it.durationSeconds, 1) * 100 + '%' }" />
            </div>
          </RouterLink>
          <RouterLink :to="{ name: 'item', params: { id: it.id } }" class="group mt-2 block focus-visible:outline-none">
            <p class="truncate text-sm font-medium text-slate-200 group-hover:text-white" :title="it.showTitle || it.title">{{ it.showTitle || it.title }}</p>
            <p v-if="subtitle(it, !!resume)" class="truncate text-xs text-slate-500">{{ subtitle(it, !!resume) }}</p>
          </RouterLink>
        </div>
      </li>
    </ul>
  </section>
</template>
