import { describe, expect, it } from 'vitest'
import { filterCount, parseFilters, personCatalogRoute, serializeFilters } from './catalogFilters'

describe('catalog filters', () => {
  it('round-trips criteria through a shareable URL', () => {
    const filters = { genres: [878, 27], countries: ['GB'], votesMin: 500, resolution: '2160p' as const, audio: 'atmos' as const }
    const params = new URLSearchParams({ filters: serializeFilters(filters)! })
    expect(parseFilters(params.get('filters'))).toEqual({ ...filters, genres: [27, 878] })
  })
  it('omits empty selections and drops the legacy matching mode', () => {
    expect(serializeFilters({ genres: [], actors: [] })).toBeUndefined()
    expect(parseFilters('{"genres":[27],"genreMode":"all"}')).toEqual({ genres: [27] })
    expect(filterCount({ genres: [], votesMin: 0, countries: ['FR'], audio: 'dts' })).toBe(3)
  })
  it('rejects malformed state and strips unknown or invalid fields', () => {
    for (const raw of [undefined, '{', 'null', '[]', '1']) expect(parseFilters(raw)).toEqual({})
    expect(parseFilters('{"genres":[0,-1,"27",27,27],"countries":["FR","France"],"ratingMin":"7","other":true,"resolution":"4k","audio":"dolby"}'))
      .toEqual({ genres: [27], countries: ['FR'] })
  })
  it('uses a stable serialization and preserves zero bounds', () => {
    expect(serializeFilters({ genres: [878, 27, 878], votesMin: 0, audio: 'ac3' })).toBe(serializeFilters({ audio: 'ac3', votesMin: 0, genres: [27, 878] }))
  })
})

describe('person catalog route', () => {
  it('opens the catalog of the title kind filtered on the person', () => {
    expect(personCatalogRoute('movie', 'actors', 10205)).toEqual({ name: 'kind', params: { kind: 'movies' }, query: { filters: '{"actors":[10205]}' } })
    expect(personCatalogRoute('episode', 'creators', 66633).params.kind).toBe('series')
    expect(personCatalogRoute('video', 'actors', 1).params.kind).toBe('other')
    expect(parseFilters(personCatalogRoute('series', 'creators', 66633).query.filters)).toEqual({ creators: [66633] })
  })
})
