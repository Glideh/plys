<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { MIN_SEARCH_LENGTH } from '@/api/catalog'
import { loadLocalServer } from '@/api/federation'
import { librariesApi, type Library, type LibraryKind } from '@/api/libraries'
import { progressApi, type HomeItem } from '@/api/progress'
import { watchlistApi, type WatchlistItem } from '@/api/watchlist'
import CatalogGrid from '@/components/CatalogGrid.vue'
import ErrorAlert from '@/components/ErrorAlert.vue'
import HomeRail from '@/components/HomeRail.vue'
import WatchlistRail from '@/components/WatchlistRail.vue'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { useGlobalSearch } from '@/composables/useGlobalSearch'
import { useLibrarySummary } from '@/composables/useLibrarySummary'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const auth = useAuthStore()
const errorMessage = useErrorMessage()
const summary = useLibrarySummary()

// The search lives in the URL so that coming back from a result restores it.
const route = useRoute()
const router = useRouter()
const query = ref(typeof route.query.q === 'string' ? route.query.q : '')
const search = useGlobalSearch(query)
watch(search.term, (q) => router.replace({ query: { ...route.query, q: q || undefined } }))
watch(
  () => route.query.q,
  (q) => {
    const next = typeof q === 'string' ? q : ''
    if (next !== search.term.value) query.value = next
  },
)

const libraries = ref<Library[]>([])
const loading = ref(true)
const error = ref('')

const gradients: Record<LibraryKind, string> = {
  movies: 'from-accent-500/30 via-sky-700/20 to-indigo-800/30',
  series: 'from-indigo-400/30 via-violet-600/20 to-slate-700/30',
  other: 'from-rose-400/25 via-pink-700/15 to-slate-700/30',
}

const continueItems = ref<HomeItem[]>([])
const nextItems = ref<HomeItem[]>([])
const railsLoading = ref(true)
const watchlistItems = ref<WatchlistItem[]>([])

onMounted(async () => {
  // Telling local titles from those of paired servers needs this server's identity.
  Promise.all([loadLocalServer(), watchlistApi.list()])
    .then(([, list]) => (watchlistItems.value = list.items))
    .catch(() => undefined)
  // A failing rail must not hide the libraries.
  progressApi
    .continueWatching()
    .then((items) => (continueItems.value = items))
    .catch(() => undefined)
    .then(() => progressApi.nextUp())
    .then((items) => (nextItems.value = items))
    .catch(() => undefined)
    .finally(() => (railsLoading.value = false))
  try {
    libraries.value = await librariesApi.list()
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <main class="mx-auto max-w-screen-2xl space-y-12 px-4 py-10 sm:px-8">
    <div class="relative max-w-2xl">
      <svg viewBox="0 0 24 24" class="pointer-events-none absolute top-1/2 left-4 size-5 -translate-y-1/2 text-slate-500" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" />
      </svg>
      <input
        v-model="query"
        type="search"
        :placeholder="t('home.search')"
        :aria-label="t('home.search')"
        autocomplete="off"
        class="w-full rounded-xl border border-white/10 bg-surface-800 py-3 pr-4 pl-12 text-base text-slate-100 placeholder-slate-500 focus:border-accent-500 focus:ring-2 focus:ring-accent-500/30 focus:outline-none"
        @keydown.esc="query = ''"
      />
      <p v-if="search.term.value && !search.active.value" class="mt-2 text-sm text-slate-500">{{ t('home.searchHint', MIN_SEARCH_LENGTH) }}</p>
    </div>

    <section v-if="search.active.value" class="space-y-10" :aria-label="t('home.searchResults')" aria-live="polite">
      <p v-if="search.unavailableServers.value" class="text-sm text-amber-300" role="status">{{ t('filters.partial', search.unavailableServers.value) }}</p>
      <ErrorAlert v-if="search.error.value" :message="search.error.value" />
      <CatalogGrid v-else-if="search.loading.value && search.groups.value.length === 0" :items="[]" loading :kind="null" />
      <p v-else-if="!search.loading.value && search.groups.value.length === 0" class="rounded-xl border border-dashed border-white/10 px-6 py-16 text-center text-slate-500">
        {{ t('catalog.noResults') }}
      </p>
      <div v-for="group in search.groups.value" :key="group.kind" :class="{ 'opacity-60 transition': search.loading.value }">
        <div class="mb-4 flex items-baseline gap-3">
          <h2 class="text-lg font-semibold">{{ t(`libraries.kinds.${group.kind}`) }}</h2>
          <span class="text-sm text-slate-500">{{ t('catalog.count', group.total) }}</span>
          <RouterLink
            v-if="group.total > group.items.length"
            :to="{ name: 'kind', params: { kind: group.kind }, query: { q: search.term.value } }"
            class="ml-auto text-sm font-medium text-accent-300 hover:text-accent-200"
          >
            {{ t('home.seeAll') }}
          </RouterLink>
        </div>
        <CatalogGrid :items="group.items" :loading="false" :kind="group.kind" />
      </div>
    </section>

    <template v-else>
      <!-- Each rail is as wide as its posters: they sit side by side while they fit, otherwise
           the next one wraps, and a rail only scrolls once it fills the whole row. The column
           gap covers the rails' edge-to-edge scroll bleed (-mx-4 / sm:-mx-8) on both sides. -->
      <div v-if="!railsLoading && (continueItems.length > 0 || nextItems.length > 0)" class="flex flex-wrap gap-x-8 gap-y-12 sm:gap-x-16">
        <HomeRail v-if="continueItems.length > 0" :title="t('home.continueWatching')" :empty="t('home.emptyContinue')" :items="continueItems" :loading="false" resume />
        <HomeRail v-if="nextItems.length > 0" :title="t('home.nextUp')" :empty="t('home.emptyNextUp')" :items="nextItems" :loading="false" />
      </div>

      <WatchlistRail v-if="watchlistItems.length > 0" :title="t('home.watchlist')" :items="watchlistItems" />

      <section>
        <h2 class="mb-4 text-lg font-semibold">{{ t('home.libraries') }}</h2>
        <ErrorAlert v-if="error" :message="error" />
        <div v-else-if="loading" class="grid grid-cols-[repeat(auto-fill,minmax(12rem,1fr))] gap-4">
          <div v-for="i in 3" :key="i" class="aspect-video animate-pulse rounded-xl bg-surface-800/60" />
        </div>
        <div v-else-if="libraries.length === 0" class="rounded-xl border border-dashed border-white/10 px-6 py-10 text-center">
          <p class="text-sm text-slate-400">{{ t('home.emptyLibraries') }}</p>
          <template v-if="auth.isAdmin">
            <p class="mt-1 text-sm text-slate-500">{{ t('home.emptyLibrariesAdmin') }}</p>
            <RouterLink
              :to="{ name: 'admin-libraries' }"
              class="mt-4 inline-block rounded-lg bg-accent-500 px-4 py-2 text-sm font-semibold text-surface-950 transition hover:bg-accent-400"
            >
              {{ t('home.manageLibraries') }}
            </RouterLink>
          </template>
        </div>
        <ul v-else class="grid grid-cols-[repeat(auto-fill,minmax(12rem,1fr))] gap-4">
          <li v-for="lib in libraries" :key="lib.id">
            <RouterLink
              :to="{ name: 'library', params: { id: lib.id } }"
              :class="gradients[lib.kind]"
              class="flex aspect-video flex-col justify-end overflow-hidden rounded-xl border border-white/5 bg-gradient-to-br p-4 transition hover:-translate-y-0.5 hover:border-white/20 hover:shadow-xl hover:shadow-black/40"
            >
              <span class="text-xs font-medium tracking-wide text-white/60 uppercase">{{ t(`libraries.kinds.${lib.kind}`) }}</span>
              <span class="truncate text-lg font-semibold">{{ lib.name }}</span>
              <span class="truncate text-xs text-white/70">{{ summary(lib) }}</span>
            </RouterLink>
          </li>
        </ul>
      </section>
    </template>
  </main>
</template>
