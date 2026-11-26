import { afterEach, expect, it, vi } from 'vitest'
import { catalogApi } from './catalog'
import { api } from './client'

vi.mock('./client', () => ({ api: vi.fn() }))

afterEach(() => vi.resetAllMocks())

it.each([
  { kind: 'season', number: 'seasonNumber', numbers: [0, 1, 2, 10] },
  { kind: 'episode', number: 'episodeNumber', numbers: [1, 2, 10] },
])('preserves the server sequence for $kind children', async ({ kind, number, numbers }) => {
  const children = numbers.map((n, i) => ({
    id: `${kind}-${n}`,
    kind,
    title: ['Zulu', 'Zebra', 'Middle', 'Alpha'][i],
    [number]: n,
  }))
  vi.mocked(api).mockResolvedValue({ children })

  const detail = await catalogApi.get('parent')

  expect(api).toHaveBeenCalledWith('/items/parent')
  expect(detail.children).toEqual(children)
})
