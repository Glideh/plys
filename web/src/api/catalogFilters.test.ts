import { describe, expect, it } from 'vitest'
import { filterCount, parseFilters, serializeFilters } from './catalogFilters'

describe('catalog filters', () => {
  it('round-trips criteria through a shareable URL', () => {
    const filters = { genres: [878, 27], genreMode: 'all' as const, countries: ['GB'], ratingMin: 7.5, votesMin: 500 }
    const params = new URLSearchParams({ filters: serializeFilters(filters)! })
    expect(parseFilters(params.get('filters'))).toEqual({ ...filters, genres: [27, 878] })
  })
  it('omits empty selections and an unused matching mode', () => {
    expect(serializeFilters({ genres: [], actors: [], genreMode: 'all' })).toBeUndefined()
    expect(filterCount({ genres: [], ratingMin: 0, countries: ['FR'] })).toBe(2)
  })
  it('rejects malformed state and strips unknown or invalid fields', () => {
    for (const raw of [undefined, '{', 'null', '[]', '1']) expect(parseFilters(raw)).toEqual({})
    expect(parseFilters('{"genres":[0,-1,"27",27,27],"countries":["FR","France"],"ratingMin":"7","other":true}'))
      .toEqual({ genres: [27], countries: ['FR'] })
  })
  it('uses a stable serialization and preserves zero bounds', () => {
    expect(serializeFilters({ genres: [878, 27, 878], ratingMin: 0 })).toBe(serializeFilters({ ratingMin: 0, genres: [27, 878] }))
  })
})