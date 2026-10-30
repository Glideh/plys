<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, useTemplateRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { catalogApi, itemSorts, type ItemCard, type ItemSort } from '@/api/catalog'
import { librariesApi, type Library } from '@/api/libraries'
import ErrorAlert from '@/components/ErrorAlert.vue'
import PosterCard from '@/components/PosterCard.vue'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { useAuthStore } from '@/stores/auth'

const PAGE_SIZE = 60
const SEARCH_DEBOUNCE_MS = 300

const props = defineProps<{ id: string }>()
const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const errorMessage = useErrorMessage()

const library = ref<Library | null>(null)
const items = ref<ItemCard[]>([])
const total = ref(0)
const loading = ref(false)
const error = ref('')
// Search and sort live in the URL so that back navigation restores the view.
const query = ref(typeof route.query.q === 'string' ? route.query.q : '')
const sort = ref<ItemSort>(itemSorts.includes(route.query.sort as ItemSort) ? (route.query.sort as ItemSort) : 'title')
const sentinel = useTemplateRef<HTMLElement>('sentinel')

const hasMore = computed(() => items.value.length < total.value)
let generation = 0

async function loadPage(reset: boolean) {
  if (loading.value && !reset) return
  const gen = reset ? ++generation : generation
  loading.value = true
  error.value = ''
  try {
    const page = await catalogApi.list(props.id, {
      query: query.value.trim(),
      sort: sort.value,
      offset: reset ? 0 : items.value.length,
      limit: PAGE_SIZE,
    })
    if (gen !== generation) return
    items.value = reset ? page.items : [...items.value, ...page.items]
    total.value = page.total
  } catch (err) {
    if (gen === generation) error.value = errorMessage(err)
  } finally {
    if (gen === generation) loading.value = false
  }
}

let debounce: ReturnType<typeof setTimeout> | undefined
watch([query, sort], () => {
  clearTimeout(debounce)
  debounce = setTimeout(() => {
    router.replace({ query: { ...route.query, q: query.value || undefined, sort: sort.value === 'title' ? undefined : sort.value } })
    loadPage(true)
  }, SEARCH_DEBOUNCE_MS)
})

const observer = new IntersectionObserver(
  (entries) => {
    if (entries.some((e) => e.isIntersecting) && hasMore.value) loadPage(false)
  },
  { rootMargin: '800px' },
)

onMounted(async () => {
  if (sentinel.value) observer.observe(sentinel.value)
  try {
    library.value = await librariesApi.get(props.id)
  } catch (err) {
    error.value = errorMessage(err)
    return
  }
  await loadPage(true)
})
onUnmounted(() => {
  observer.disconnect()
  clearTimeout(debounce)
})

function subtitle(it: ItemCard): string | null {
  if (it.kind === 'series' && it.episodeCount) return t('catalog.episodeCount', it.episodeCount)
  return it.year ? String(it.year) : null
}
</script>

<template>
  <main class="mx-auto max-w-screen-2xl px-4 py-8 sm:px-8">
    <header class="mb-8 flex flex-wrap items-end gap-4">
      <div class="mr-auto">
        <p v-if="library" class="text-xs font-medium tracking-widest text-accent-400 uppercase">
          {{ t(`libraries.kinds.${library.kind}`) }}
        </p>
        <h1 class="text-3xl font-bold tracking-tight">{{ library?.name ?? '' }}</h1>
        <p v-if="library" class="mt-1 text-sm text-slate-500">{{ t('catalog.count', total) }}</p>
      </div>
      <input
        v-model="query"
        type="search"
        :placeholder="t('catalog.search')"
        :aria-label="t('catalog.search')"
        class="w-full rounded-lg border border-white/10 bg-surface-800 px-3.5 py-2 text-sm text-slate-100 placeholder-slate-500 focus:border-accent-500 focus:ring-2 focus:ring-accent-500/30 focus:outline-none sm:w-64"
      />
      <label class="flex items-center gap-2 text-sm text-slate-400">
        {{ t('catalog.sort') }}
        <select
          v-model="sort"
          class="rounded-lg border border-white/10 bg-surface-800 px-3 py-2 text-sm text-slate-100 focus:border-accent-500 focus:outline-none"
        >
          <option v-for="s in itemSorts" :key="s" :value="s">{{ t(`catalog.sorts.${s}`) }}</option>
        </select>
      </label>
    </header>

    <ErrorAlert v-if="error" class="mb-6" :message="error" />

    <p v-if="!loading && !error && items.length === 0" class="rounded-xl border border-dashed border-white/10 px-6 py-16 text-center text-slate-500">
      {{ query ? t('catalog.noResults') : t('catalog.empty') }}
    </p>

    <ul
      :class="library?.kind === 'other' ? 'grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5' : 'grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 2xl:grid-cols-8'"
      class="grid gap-x-4 gap-y-6"
    >
      <li v-for="it in items" :key="it.id">
        <PosterCard
          :to="{ name: 'item', params: { id: it.id } }"
          :title="it.title"
          :subtitle="subtitle(it)"
          :image-url="it.posterUrl"
          :landscape="it.kind === 'video'"
          :badge="auth.isAdmin && it.matchStatus === 'unmatched' ? '?' : null"
        />
      </li>
      <template v-if="loading">
        <li v-for="i in 12" :key="`s${i}`" :class="library?.kind === 'other' ? 'aspect-video' : 'aspect-[2/3]'" class="animate-pulse rounded-xl bg-surface-800/60" />
      </template>
    </ul>
    <div ref="sentinel" class="h-px" aria-hidden="true" />
  </main>
</template>
