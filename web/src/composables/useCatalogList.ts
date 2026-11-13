import { computed, onMounted, onUnmounted, ref, useTemplateRef, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { itemSorts, type ItemCard, type ItemPage, type ItemSort } from '@/api/catalog'
import { parseFilters, serializeFilters, type CatalogFilters } from '@/api/catalogFilters'
import { useErrorMessage } from '@/composables/useErrorMessage'

const PAGE_SIZE = 60
const SEARCH_DEBOUNCE_MS = 300

export interface CatalogPageRequest {
  query: string
  sort: ItemSort
  offset: number
  limit: number
  filters: CatalogFilters
}

/** Paginated item list; search and sort live in the URL so that back navigation restores the view. */
export function useCatalogList(fetchPage: (req: CatalogPageRequest) => Promise<ItemPage>) {
  const route = useRoute()
  const router = useRouter()
  const errorMessage = useErrorMessage()

  const items = ref<ItemCard[]>([])
  const total = ref(0)
  const loading = ref(false)
  const error = ref('')
  const unavailableServers = ref(0)
  const unavailableServerNames = ref<string[]>([])
  const filters = ref<CatalogFilters>(parseFilters(route.query.filters))
  const query = ref(typeof route.query.q === 'string' ? route.query.q : '')
  const sort = ref<ItemSort>(itemSorts.includes(route.query.sort as ItemSort) ? (route.query.sort as ItemSort) : 'title')
  const sentinel = useTemplateRef<HTMLElement>('sentinel')

  const hasMore = computed(() => items.value.length < total.value)
  let generation = 0

  async function load(reset: boolean) {
    if (loading.value && !reset) return
    const gen = reset ? ++generation : generation
    loading.value = true
    error.value = ''
    try {
      const page = await fetchPage({
        query: query.value.trim(),
        sort: sort.value,
        offset: reset ? 0 : items.value.length,
        limit: PAGE_SIZE,
        filters: filters.value,
      })
      if (gen !== generation) return
      items.value = reset ? page.items : [...items.value, ...page.items]
      total.value = page.total
      unavailableServers.value = page.unavailableServers ?? 0
      unavailableServerNames.value = page.unavailableServerNames ?? []
    } catch (err) {
      if (gen === generation) error.value = errorMessage(err)
    } finally {
      if (gen === generation) loading.value = false
    }
  }

  let debounce: ReturnType<typeof setTimeout> | undefined
  watch([query, sort, () => serializeFilters(filters.value)], () => {
    clearTimeout(debounce)
    debounce = setTimeout(() => {
      router.replace({ query: { ...route.query, q: query.value || undefined, sort: sort.value === 'title' ? undefined : sort.value, filters: serializeFilters(filters.value) } })
      load(true)
    }, SEARCH_DEBOUNCE_MS)
  })

  watch(() => route.query, (next) => {
    const nextQuery = typeof next.q === 'string' ? next.q : ''
    const nextSort = itemSorts.includes(next.sort as ItemSort) ? next.sort as ItemSort : 'title'
    const nextFilters = parseFilters(next.filters)
    if (nextQuery === query.value && nextSort === sort.value && serializeFilters(nextFilters) === serializeFilters(filters.value)) return
    query.value = nextQuery
    sort.value = nextSort
    filters.value = nextFilters
  })

  const observer = new IntersectionObserver(
    (entries) => {
      if (entries.some((e) => e.isIntersecting) && hasMore.value) load(false)
    },
    { rootMargin: '800px' },
  )

  onMounted(() => {
    if (sentinel.value) observer.observe(sentinel.value)
  })
  onUnmounted(() => {
    observer.disconnect()
    clearTimeout(debounce)
  })

  return { items, total, loading, error, query, sort, filters, unavailableServers, unavailableServerNames, reload: () => load(true) }
}
