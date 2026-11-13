import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { effectScope, nextTick, ref } from 'vue'
import { GLOBAL_SEARCH_DEBOUNCE_MS, useGlobalSearch } from './useGlobalSearch'

vi.mock('@/composables/useErrorMessage', () => ({ useErrorMessage: () => () => 'failed' }))

function json(status: number, body: unknown) {
  return new Response(JSON.stringify(body), { status })
}

const movies = { kind: 'movies', items: [{ id: '1', kind: 'movie', title: 'Alien' }], total: 1 }

describe('useGlobalSearch', () => {
  const fetchMock = vi.fn<typeof fetch>()
  let scope = effectScope()

  beforeEach(() => {
    scope = effectScope()
    vi.useFakeTimers()
    vi.stubGlobal('fetch', fetchMock)
  })

  afterEach(() => {
    scope.stop()
    fetchMock.mockReset()
    vi.unstubAllGlobals()
    vi.useRealTimers()
  })

  function setup(initial = '') {
    const query = ref(initial)
    const search = scope.run(() => useGlobalSearch(query))!
    return { query, search }
  }

  async function settle() {
    await vi.advanceTimersByTimeAsync(GLOBAL_SEARCH_DEBOUNCE_MS)
    await vi.runAllTimersAsync()
  }

  it('waits for 3 characters before searching', async () => {
    const { query, search } = setup()
    query.value = '  al '
    await nextTick()
    await settle()
    expect(fetchMock).not.toHaveBeenCalled()
    expect(search.active.value).toBe(false)
    expect(search.loading.value).toBe(false)
  })

  it('debounces typing and searches the trimmed query once', async () => {
    fetchMock.mockResolvedValue(json(200, { groups: [movies], unavailableServers: 1, unavailableServerNames: ['Peer'] }))
    const { query, search } = setup()
    query.value = 'ali'
    await nextTick()
    query.value = 'alie'
    await nextTick()
    query.value = ' alien '
    await nextTick()
    expect(search.loading.value).toBe(true)
    await settle()
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(fetchMock.mock.calls[0]![0]).toBe('/api/v1/search?query=alien')
    expect(search.groups.value).toEqual([movies])
    expect(search.unavailableServers.value).toBe(1)
    expect(search.unavailableServerNames.value).toEqual(['Peer'])
    expect(search.loading.value).toBe(false)
  })

  it('clears the results when the query becomes too short', async () => {
    fetchMock.mockResolvedValue(json(200, { groups: [movies], unavailableServers: 0 }))
    const { query, search } = setup('alien')
    await settle()
    expect(search.groups.value).toHaveLength(1)
    query.value = 'al'
    await nextTick()
    expect(search.groups.value).toEqual([])
  })

  it('ignores a response that arrives after the query changed', async () => {
    let resolveFirst: (r: Response) => void = () => undefined
    fetchMock
      .mockReturnValueOnce(new Promise((resolve) => (resolveFirst = resolve)))
      .mockResolvedValueOnce(json(200, { groups: [], unavailableServers: 0 }))
    const { query, search } = setup('alien')
    await vi.advanceTimersByTimeAsync(GLOBAL_SEARCH_DEBOUNCE_MS)
    query.value = 'blade'
    await nextTick()
    await settle()
    resolveFirst(json(200, { groups: [movies], unavailableServers: 0 }))
    await vi.runAllTimersAsync()
    expect(search.groups.value).toEqual([])
  })

  it('reports errors', async () => {
    fetchMock.mockResolvedValue(json(500, { error: { code: 'internal', message: 'boom' } }))
    const { search } = setup('alien')
    await settle()
    expect(search.error.value).toBe('failed')
    expect(search.loading.value).toBe(false)
  })
})
