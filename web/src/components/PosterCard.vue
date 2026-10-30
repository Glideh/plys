<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { RouteLocationRaw } from 'vue-router'

const props = defineProps<{
  to: RouteLocationRaw
  title: string
  subtitle?: string | null
  imageUrl: string | null
  /** Landscape for personal videos and episodes, portrait for posters. */
  landscape?: boolean
  badge?: string | null
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
    </div>
    <p class="mt-2 truncate text-sm font-medium text-slate-200 group-hover:text-white" :title="title">{{ title }}</p>
    <p v-if="subtitle" class="truncate text-xs text-slate-500">{{ subtitle }}</p>
  </RouterLink>
</template>
