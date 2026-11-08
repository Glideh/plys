<script setup lang="ts">
import { Heart } from '@lucide/vue'
import { isRemote, serverQuery, type ServerRef } from '@/api/federation'
import type { WatchlistItem } from '@/api/watchlist'
import PosterCard from '@/components/PosterCard.vue'

defineProps<{ title: string; items: WatchlistItem[] }>()

/** The paired server holding the title; null when it is this server. */
function remote(it: WatchlistItem): ServerRef | null {
  if (!isRemote(it.source.serverId)) return null
  return { serverId: it.source.serverId, serverName: it.source.serverName, baseUrl: it.source.serverUrl ?? null }
}
</script>

<template>
  <section class="min-w-0">
    <h2 class="mb-4 flex items-center gap-2 text-lg font-semibold">
      <Heart class="size-5 text-rose-400" fill="currentColor" aria-hidden="true" />{{ title }}
    </h2>
    <ul class="-mx-4 flex gap-4 overflow-x-auto px-4 pb-3 sm:-mx-8 sm:px-8">
      <li v-for="it in items" :key="`${it.source.serverId}/${it.id}`" class="w-36 shrink-0 sm:w-40">
        <PosterCard
          :to="{ name: 'item', params: { id: it.id }, query: serverQuery(remote(it)) }"
          :title="it.title"
          :subtitle="it.year ? String(it.year) : null"
          :source="remote(it)?.serverName ?? null"
          :image-url="it.posterUrl"
        />
      </li>
    </ul>
  </section>
</template>
