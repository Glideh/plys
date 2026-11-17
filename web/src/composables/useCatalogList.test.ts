import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { effectScope, nextTick, reactive } from 'vue'
import type { ItemPage } from '@/api/catalog'
import { serializeFilters } from '@/api/catalogFilters'
import { useCatalogList, type CatalogPageRequest } from './useCatalogList'

const hooks = vi.hoisted(() => ({ activated: [] as (() => void)[], deactivated: [] as (() => void)[] }))
const route = vi.hoisted(() => ({ current: undefined as unknown as { query: Record<string, string | undefined> } }))
const replace = vi.hoisted(() => vi.fn())

vi.mock('vue', async (original) => ({
  ...(await original<typeof import('vue')>()),
  onActivated: (fn: () => void) => hooks.activated.push(fn),
  onDeactivated: (fn: () => void) => hooks.deactivated.push(fn),
  onMounted: () => {},
  onUnmounted: () => {},
  useTemplateRef: () => ({ value: null }),
}))
vi.mock('vue-router', () => ({ useRoute: () => route.current, useRouter: () => ({ replace }) }))
vi.mock('@/composables/useErrorMessage', () => ({ useErrorMessage: () => () => 'failed' }))

const page: ItemPage = { items: [], total: 0, offset: 0, limit: 60 }

describe('useCatalogList', () => {
  let scope = effectScope()
  const fetchPage = vi.fn<(req: CatalogPageRequest, signal: AbortSignal) => Promise<ItemPage>>()

  beforeEach(() => {
    scope = effectScope()
    hooks.activated.length = 0
    hooks.deactivated.length = 0
    route.current = reactive({ query: {} })
    fetchPage.mockReset().mockResolvedValue(page)
    replace.mockReset()
    vi.stubGlobal('IntersectionObserver', class { observe() {} disconnect() {} })
  })

  afterEach(() => {
    scope.stop()
    vi.unstubAllGlobals()
  })

  function setup() {
    const list = scope.run(() => useCatalogList(fetchPage))!
    hooks.activated.forEach((fn) => fn()) // first mount
    return list
  }

  it('applies filters brought by the URL when a cached list comes back', async () => {
    const list = setup()
    hooks.deactivated.forEach((fn) => fn())
    // An item page links to the list filtered on an actor: the route changes while the list is hidden.
    route.current.query = { filters: serializeFilters({ actors: [31] }) }
    await nextTick()
    expect(fetchPage).not.toHaveBeenCalled()

    hooks.activated.forEach((fn) => fn())
    await nextTick()
    expect(list.filters.value).toEqual({ actors: [31] })
    expect(fetchPage).toHaveBeenCalledTimes(1)
    expect(fetchPage.mock.calls[0][0]).toMatchObject({ offset: 0, filters: { actors: [31] } })
  })

  it('refreshes the cached list when the URL is unchanged', async () => {
    const list = setup()
    list.items.value = [{ id: '1' } as never]
    hooks.deactivated.forEach((fn) => fn())
    hooks.activated.forEach((fn) => fn())
    await nextTick()
    expect(fetchPage).toHaveBeenCalledTimes(1)
    expect(fetchPage.mock.calls[0][0]).toMatchObject({ offset: 0, limit: 1, filters: {} })
  })

  it('follows the URL at once while shown (back and forward)', async () => {
    const list = setup()
    route.current.query = { q: 'alien', sort: 'year' }
    await nextTick()
    expect(list.query.value).toBe('alien')
    expect(list.sort.value).toBe('year')
    expect(fetchPage).toHaveBeenCalledTimes(1)
    expect(replace).not.toHaveBeenCalled()
  })
})
