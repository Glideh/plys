import { computed, onActivated, onDeactivated, onMounted, onUnmounted, ref, useTemplateRef, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { itemSorts, type ItemCard, type ItemPage, type ItemSort } from '@/api/catalog'
import { parseFilters, serializeFilters, type CatalogFilters } from '@/api/catalogFilters'
import { useErrorMessage } from '@/composables/useErrorMessage'

const PAGE_SIZE = 60
/** Largest page the API serves (`maxPageSize`), used to reload the list shown in one request. */
const MAX_PAGE_SIZE = 1000
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
export function useCatalogList(fetchPage: (req: CatalogPageRequest, signal: AbortSignal) => Promise<ItemPage>) {
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
  let inFlight: AbortController | undefined

  /** Aborts the request in flight, whose answer is no longer wanted, and returns the signal of the next one. */
  function nextSignal(): AbortSignal {
    inFlight?.abort()
    inFlight = new AbortController()
    return inFlight.signal
  }

  function applyPage(page: ItemPage, reset: boolean) {
    items.value = reset ? page.items : [...items.value, ...page.items]
    total.value = page.total
    unavailableServers.value = page.unavailableServers ?? 0
    unavailableServerNames.value = page.unavailableServerNames ?? []
  }

  async function load(reset: boolean) {
    if ((loading.value || refreshing) && !reset) return
    const gen = reset ? ++generation : generation
    const signal = nextSignal()
    loading.value = true
    error.value = ''
    try {
      const page = await fetchPage({
        query: query.value.trim(),
        sort: sort.value,
        offset: reset ? 0 : items.value.length,
        limit: PAGE_SIZE,
        filters: filters.value,
      }, signal)
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
    const signal = nextSignal()
    const count = items.value.length
    const req = { query: query.value.trim(), sort: sort.value, filters: filters.value }
    refreshing = true
    try {
      // Every request aggregates the whole catalog server side: one request covers up to MAX_PAGE_SIZE items.
      const offsets = Array.from({ length: Math.ceil(count / MAX_PAGE_SIZE) }, (_, i) => i * MAX_PAGE_SIZE)
      const pages = await Promise.all(offsets.map((offset) => fetchPage({ ...req, offset, limit: Math.min(MAX_PAGE_SIZE, count - offset) }, signal)))
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

  /** Search, sort and filters as one comparable value. */
  function stateKey(q: string, s: ItemSort, f: CatalogFilters): string {
    return JSON.stringify([q, s, serializeFilters(f) ?? ''])
  }
  function routeState() {
    const next = route.query
    return {
      q: typeof next.q === 'string' ? next.q : '',
      sort: itemSorts.includes(next.sort as ItemSort) ? next.sort as ItemSort : 'title',
      filters: parseFilters(next.filters),
    }
  }
  // State the URL and the list were last brought to; edits differing from it are pending.
  let appliedKey = stateKey(query.value, sort.value, filters.value)

  function applySearch() {
    clearTimeout(debounce)
    pendingSearch = false
    appliedKey = stateKey(query.value, sort.value, filters.value)
    router.replace({ query: { ...route.query, q: query.value || undefined, sort: sort.value === 'title' ? undefined : sort.value, filters: serializeFilters(filters.value) } })
    load(true)
  }

  /** Follows the URL (back/forward, or a link to the list with its own search or filters) and reloads at once. */
  function followRoute(): boolean {
    const next = routeState()
    const key = stateKey(next.q, next.sort, next.filters)
    if (key === appliedKey) return false
    clearTimeout(debounce)
    pendingSearch = false
    appliedKey = key
    query.value = next.q
    sort.value = next.sort
    filters.value = next.filters
    load(true)
    return true
  }

  watch([query, sort, () => serializeFilters(filters.value)], () => {
    clearTimeout(debounce)
    // Already applied (state taken from the URL, or an edit undone before the debounce).
    if (stateKey(query.value, sort.value, filters.value) === appliedKey) {
      pendingSearch = false
      return
    }
    pendingSearch = true
    debounce = setTimeout(applySearch, SEARCH_DEBOUNCE_MS)
  })

  watch(() => route.query, () => {
    // A hidden (cached) list must not follow the query of the page shown in its place.
    if (active) followRoute()
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
    // The route may bring new filters (e.g. an actor opened from an item page): they win over the cached list.
    if (followRoute()) return
    if (pendingSearch) applySearch()
    else void refreshLoaded()
  })
  onUnmounted(() => {
    observer.disconnect()
    clearTimeout(debounce)
    inFlight?.abort()
  })

  return { items, total, loading, error, query, sort, filters, unavailableServers, unavailableServerNames, reload: () => load(true) }
}
