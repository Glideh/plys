<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ItemCard } from '@/api/catalog'
import { absoluteUrl, isRemote, serverQuery, type ServerRef } from '@/api/federation'
import type { LibraryKind } from '@/api/libraries'
import PosterCard from '@/components/PosterCard.vue'
import { useAuthStore } from '@/stores/auth'

const props = defineProps<{ items: ItemCard[]; loading: boolean; kind: LibraryKind | null }>()
const { t } = useI18n()
const auth = useAuthStore()

/** The server of the primary source when it is a paired peer; null when the card is local. */
function remotePrimary(it: ItemCard): ServerRef | null {
  const s = it.sources?.[0]
  if (!s || !isRemote(s.serverId)) return null
  return { serverId: s.serverId, serverName: s.serverName, baseUrl: s.serverUrl ?? null }
}

function target(it: ItemCard) {
  const server = remotePrimary(it)
  const s = it.sources?.[0]
  if (!server || !s) return { name: 'item', params: { id: it.id } }
  return { name: 'item', params: { id: s.itemId }, query: serverQuery(server) }
}

const landscape = computed(() => props.kind === 'other')
const gridClass = computed(() =>
  landscape.value
    ? 'grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5'
    : 'grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 2xl:grid-cols-8',
)

function subtitle(it: ItemCard): string | null {
  if (it.kind === 'series' && it.episodeCount) return t('catalog.episodeCount', it.episodeCount)
  return it.year ? String(it.year) : null
}
</script>

<template>
  <ul :class="gridClass" class="grid gap-x-4 gap-y-6">
    <li v-for="it in items" :key="it.id">
      <PosterCard
        :to="target(it)"
        :title="it.title"
        :subtitle="subtitle(it)"
        :source="remotePrimary(it)?.serverName ?? null"
        :image-url="absoluteUrl(remotePrimary(it), it.posterUrl)"
        :landscape="it.kind === 'video'"
        :badge="auth.isAdmin && it.matchStatus === 'unmatched' ? '?' : null"
        :watched="it.watched"
        :unwatched="it.kind === 'series' ? it.unwatchedCount : null"
      />
    </li>
    <template v-if="loading">
      <li v-for="i in 12" :key="`s${i}`" :class="landscape ? 'aspect-video' : 'aspect-[2/3]'" class="animate-pulse rounded-xl bg-surface-800/60" />
    </template>
  </ul>
</template>
