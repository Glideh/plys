import { computed, onScopeDispose, ref, watch, type Ref } from 'vue'
import { catalogApi, MIN_SEARCH_LENGTH, type SearchGroup } from '@/api/catalog'
import { useErrorMessage } from '@/composables/useErrorMessage'

export const GLOBAL_SEARCH_DEBOUNCE_MS = 300

/** Searches every kind on every server while the user types, once the query is long enough. */
export function useGlobalSearch(query: Ref<string>, limit?: number) {
  const errorMessage = useErrorMessage()
  const groups = ref<SearchGroup[]>([])
  const unavailableServers = ref(0)
  const loading = ref(false)
  const error = ref('')
  const term = computed(() => query.value.trim())
  /** True once the query is long enough to be searched. */
  const active = computed(() => [...term.value].length >= MIN_SEARCH_LENGTH)
  let generation = 0
  let debounce: ReturnType<typeof setTimeout> | undefined

  async function run(value: string, gen: number) {
    try {
      const result = await catalogApi.search(value, limit)
      if (gen !== generation) return
      groups.value = result.groups
      unavailableServers.value = result.unavailableServers
    } catch (err) {
      if (gen === generation) error.value = errorMessage(err)
    } finally {
      if (gen === generation) loading.value = false
    }
  }

  watch(
    term,
    (value) => {
      clearTimeout(debounce)
      const gen = ++generation
      error.value = ''
      if (!active.value) {
        groups.value = []
        unavailableServers.value = 0
        loading.value = false
        return
      }
      loading.value = true
      debounce = setTimeout(() => run(value, gen), GLOBAL_SEARCH_DEBOUNCE_MS)
    },
    { immediate: true },
  )

  onScopeDispose(() => clearTimeout(debounce))

  return { groups, unavailableServers, loading, error, active, term }
}
