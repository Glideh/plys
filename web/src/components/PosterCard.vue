<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { RouteLocationRaw } from 'vue-router'

const props = defineProps<{
  to: RouteLocationRaw
  title: string
  subtitle?: string | null
  /** Name of the paired server holding the item, shown dimmed. */
  source?: string | null
  imageUrl: string | null
  /** Landscape for personal videos and episodes, portrait for posters. */
  landscape?: boolean
  badge?: string | null
  /** Fraction already watched, shown as a bar. */
  progress?: number | null
  watched?: boolean
  /** Number of items left to watch, shown instead of the check. */
  unwatched?: number | null
}>()

const { t } = useI18n()
const initial = computed(() => props.title.trim().charAt(0).toUpperCase() || '?')
</script>

<template>
  <RouterLink :to="to" class="group block focus-visible:outline-none">
    <div
      :class="landscape ? 'aspect-video' : 'aspect-[2/3]'"
      class="relative overflow-hidden rounded-xl bg-surface-800 shadow-lg shadow-black/30 ring-1 ring-white/5 transition duration-300 group-hover:-translate-y-1 group-hover:shadow-xl group-hover:shadow-black/50 group-hover:ring-accent-500/60 group-focus-visible:ring-2 group-focus-visible:ring-accent-400"
    >
      <img
        v-if="imageUrl"
        :src="imageUrl"
        :alt="title"
        loading="lazy"
        decoding="async"
        class="size-full object-cover transition duration-500 group-hover:scale-105"
      />
      <div
        v-else
        class="grid size-full place-items-center bg-gradient-to-br from-surface-700 to-surface-900 text-4xl font-black text-white/20"
        aria-hidden="true"
      >
        {{ initial }}
      </div>
      <span
        v-if="badge"
        class="absolute top-2 left-2 rounded-md bg-amber-500/90 px-1.5 py-0.5 text-[10px] font-bold tracking-wide text-surface-950 uppercase"
        :title="t('catalog.unidentified')"
      >
        {{ badge }}
      </span>
      <span
        v-if="unwatched"
        class="absolute top-2 right-2 rounded-md bg-accent-500 px-1.5 py-0.5 text-xs font-bold text-surface-950 shadow"
        :title="t('catalog.unwatchedCount', unwatched)"
      >
        {{ unwatched }}
      </span>
      <span
        v-else-if="watched"
        class="absolute top-2 right-2 grid size-6 place-items-center rounded-full bg-black/60 text-accent-300 shadow"
        :title="t('catalog.watched')"
      >
        <svg viewBox="0 0 24 24" class="size-4" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M5 12l5 5L20 7" /></svg>
      </span>
      <div v-if="progress" class="absolute inset-x-0 bottom-0 h-1.5 bg-black/60" role="progressbar" :aria-valuenow="Math.round(progress * 100)" aria-valuemin="0" aria-valuemax="100">
        <div class="h-full bg-accent-400" :style="{ width: Math.min(progress, 1) * 100 + '%' }" />
      </div>
    </div>
    <p class="mt-2 truncate text-sm font-medium text-slate-200 group-hover:text-white" :title="title">{{ title }}</p>
    <p v-if="subtitle" class="truncate text-xs text-slate-500">{{ subtitle }}</p>
    <p v-if="source" class="truncate text-xs text-slate-600">{{ source }}</p>
  </RouterLink>
</template>
