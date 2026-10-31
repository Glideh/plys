<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { type HomeItem } from '@/api/progress'
import PosterCard from '@/components/PosterCard.vue'
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
  <section>
    <h2 class="mb-4 text-lg font-semibold">{{ title }}</h2>
    <div v-if="loading" class="flex gap-4 overflow-hidden">
      <div v-for="i in 6" :key="i" class="aspect-[2/3] w-36 shrink-0 animate-pulse rounded-xl bg-surface-800/60" />
    </div>
    <p v-else-if="items.length === 0" class="rounded-xl border border-dashed border-white/10 px-6 py-10 text-center text-sm text-slate-500">
      {{ empty }}
    </p>
    <ul v-else class="-mx-4 flex gap-4 overflow-x-auto px-4 pb-3 sm:-mx-8 sm:px-8">
      <li v-for="it in items" :key="it.id" class="w-36 shrink-0 sm:w-40">
        <PosterCard
          :to="{ name: 'watch', params: { id: it.id }, query: resume && it.positionSeconds ? { t: Math.floor(it.positionSeconds) } : {} }"
          :title="it.showTitle || it.title"
          :subtitle="subtitle(it, !!resume)"
          :image-url="it.posterUrl"
          :progress="resume && it.durationSeconds ? it.positionSeconds / it.durationSeconds : null"
        />
      </li>
    </ul>
  </section>
</template>
