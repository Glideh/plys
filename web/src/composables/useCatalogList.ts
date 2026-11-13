import { computed, onActivated, onDeactivated, onMounted, onUnmounted, ref, useTemplateRef, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { itemSorts, type ItemCard, type ItemPage, type ItemSort } from '@/api/catalog'
import { parseFilters, serializeFilters, type CatalogFilters } from '@/api/catalogFilters'
import { useErrorMessage } from '@/composables/useErrorMessage'

const PAGE_SIZE = 60
/** Largest page the API serves (`maxPageSize`), used to reload a long list in few requests. */
const MAX_PAGE_SIZE = 200
const SEARCH_DEBOUNCE_MS = 300

export interface CatalogPageRequest {
  query: string
  sort: ItemSort
  offset: number
  limit: number
  filters: CatalogFilters
}

/**
 * Paginated item list; search and sort live in the URL so that back navigation restores the view.
 * The views using it are kept alive while the user browses items: while hidden, the list ignores the
 * route, and when it comes back it silently refreshes the pages already loaded (watched state, merges).
 */
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
  let active = true
  let refreshing = false

  function applyPage(page: ItemPage, reset: boolean) {
    items.value = reset ? page.items : [...items.value, ...page.items]
    total.value = page.total
    unavailableServers.value = page.unavailableServers ?? 0
    unavailableServerNames.value = page.unavailableServerNames ?? []
  }

  async function load(reset: boolean) {
    if ((loading.value || refreshing) && !reset) return
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
      applyPage(page, reset)
    } catch (err) {
      if (gen === generation) error.value = errorMessage(err)
    } finally {
      if (gen === generation) loading.value = false
    }
  }

  /** Reloads the pages already shown in one go, keeping them on screen (and the scroll) until the answer. */
  async function refreshLoaded() {
    if (loading.value || items.value.length === 0) return
    const gen = ++generation
    const count = items.value.length
    const req = { query: query.value.trim(), sort: sort.value, filters: filters.value }
    refreshing = true
    try {
      const offsets = Array.from({ length: Math.ceil(count / MAX_PAGE_SIZE) }, (_, i) => i * MAX_PAGE_SIZE)
      const pages = await Promise.all(offsets.map((offset) => fetchPage({ ...req, offset, limit: Math.min(MAX_PAGE_SIZE, count - offset) })))
      if (gen !== generation) return
      applyPage({ ...pages[pages.length - 1], items: pages.flatMap((p) => p.items) }, true)
    } catch {
      // The list shown is still usable; the next search, sort or scroll reports any error.
    } finally {
      refreshing = false
    }
  }

  let debounce: ReturnType<typeof setTimeout> | undefined
  let pendingSearch = false

  function applySearch() {
    pendingSearch = false
    router.replace({ query: { ...route.query, q: query.value || undefined, sort: sort.value === 'title' ? undefined : sort.value, filters: serializeFilters(filters.value) } })
    load(true)
  }

  watch([query, sort, () => serializeFilters(filters.value)], () => {
    clearTimeout(debounce)
    pendingSearch = true
    debounce = setTimeout(applySearch, SEARCH_DEBOUNCE_MS)
  })

  watch(() => route.query, (next) => {
    // A hidden (cached) list must not follow the query of the page shown in its place.
    if (!active) return
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
  onDeactivated(() => {
    active = false
    // A search typed just before leaving is applied on return, never to another page's URL.
    clearTimeout(debounce)
  })
  onActivated(() => {
    if (active) return // first mount
    active = true
    if (pendingSearch) applySearch()
    else void refreshLoaded()
  })
  onUnmounted(() => {
    observer.disconnect()
    clearTimeout(debounce)
  })

  return { items, total, loading, error, query, sort, filters, unavailableServers, unavailableServerNames, reload: () => load(true) }
}
