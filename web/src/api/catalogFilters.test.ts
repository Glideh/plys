import { describe, expect, it } from 'vitest'
import { filterCount, parseFilters, serializeFilters } from './catalogFilters'

describe('catalog filters', () => {
  it('round-trips criteria through a shareable URL', () => {
    const filters = { genres: [878, 27], countries: ['GB'], ratingMin: 7.5, votesMin: 500, resolution: '2160p' as const, audio: 'atmos' as const }
    const params = new URLSearchParams({ filters: serializeFilters(filters)! })
    expect(parseFilters(params.get('filters'))).toEqual({ ...filters, genres: [27, 878] })
  })
  it('omits empty selections and drops the legacy matching mode', () => {
    expect(serializeFilters({ genres: [], actors: [] })).toBeUndefined()
    expect(parseFilters('{"genres":[27],"genreMode":"all"}')).toEqual({ genres: [27] })
    expect(filterCount({ genres: [], ratingMin: 0, countries: ['FR'], audio: 'dts' })).toBe(3)
  })
  it('rejects malformed state and strips unknown or invalid fields', () => {
    for (const raw of [undefined, '{', 'null', '[]', '1']) expect(parseFilters(raw)).toEqual({})
    expect(parseFilters('{"genres":[0,-1,"27",27,27],"countries":["FR","France"],"ratingMin":"7","other":true,"resolution":"4k","audio":"dolby"}'))
      .toEqual({ genres: [27], countries: ['FR'] })
  })
  it('uses a stable serialization and preserves zero bounds', () => {
    expect(serializeFilters({ genres: [878, 27, 878], ratingMin: 0, audio: 'ac3' })).toBe(serializeFilters({ audio: 'ac3', ratingMin: 0, genres: [27, 878] }))
  })
})
