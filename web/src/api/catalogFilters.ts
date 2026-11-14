import type { ItemKind, LibraryKind } from './libraries'

export const resolutions = ['2160p', '1080p', '720p', 'SD'] as const
export type Resolution = typeof resolutions[number]
export const audioFormats = ['atmos', 'truehd', 'eac3', 'ac3', 'dtsHdMa', 'dts', 'flac', 'pcm', 'aac', 'opus', 'mp3'] as const
export type AudioFormat = typeof audioFormats[number]

export interface CatalogFilters {
  genres?: number[]
  excludeGenres?: number[]
  actors?: number[]
  creators?: number[]
  keywords?: number[]
  languages?: string[]
  countries?: string[]
  yearMin?: number
  yearMax?: number
  ratingMin?: number
  votesMin?: number
  runtimeMin?: number
  runtimeMax?: number
  resolution?: Resolution
  audio?: AudioFormat
}

export const selectionKeys = ['genres', 'excludeGenres', 'actors', 'creators', 'keywords', 'languages', 'countries'] as const
export type SelectionKey = typeof selectionKeys[number]
export type FilterCategory = Exclude<SelectionKey, 'excludeGenres'>
export interface FilterOption { id: string; label: string }
export interface FilterOptionsPage { options: Partial<Record<FilterCategory, FilterOption[]>>; unavailableServers: number; unavailableServerNames?: string[] }
export const numberKeys = ['yearMin', 'yearMax', 'ratingMin', 'votesMin', 'runtimeMin', 'runtimeMax'] as const
export type NumberKey = typeof numberKeys[number]
/** Technical criteria picked from a fixed list of values. */
export const choiceKeys = ['resolution', 'audio'] as const
export type ChoiceKey = typeof choiceKeys[number]
export const choiceValues: { resolution: readonly Resolution[]; audio: readonly AudioFormat[] } = { resolution: resolutions, audio: audioFormats }

export function parseFilters(raw: unknown): CatalogFilters {
  if (typeof raw !== 'string') return {}
  try {
    const parsed: unknown = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return {}
    const source = parsed as Record<string, unknown>
    const filters: CatalogFilters = {}
    for (const key of selectionKeys) {
      const values = source[key]
      if (!Array.isArray(values)) continue
      if (key === 'languages' || key === 'countries') {
        filters[key] = [...new Set(values.filter((value): value is string => typeof value === 'string' && /^[a-z]{2}$/i.test(value)).map((value) => key === 'countries' ? value.toUpperCase() : value.toLowerCase()))].slice(0, 20)
      } else {
        filters[key] = [...new Set(values.filter((value): value is number => Number.isInteger(value) && value > 0 && value <= 2147483647))].slice(0, 20)
      }
    }
    for (const key of numberKeys) {
      const value = source[key]
      if (typeof value === 'number' && Number.isFinite(value)) filters[key] = value
    }
    if (resolutions.includes(source.resolution as Resolution)) filters.resolution = source.resolution as Resolution
    if (audioFormats.includes(source.audio as AudioFormat)) filters.audio = source.audio as AudioFormat
    return filters
  } catch { return {} }
}

export function serializeFilters(filters: CatalogFilters): string | undefined {
  const result: CatalogFilters = {}
  for (const key of selectionKeys) {
    const values = filters[key]
    if (!values?.length) continue
    if (key === 'countries' || key === 'languages') result[key] = [...new Set(filters[key])].sort()
    else result[key] = [...new Set(filters[key])].sort((left, right) => left - right)
  }
  for (const key of numberKeys) if (filters[key] !== undefined) result[key] = filters[key]
  if (filters.resolution) result.resolution = filters.resolution
  if (filters.audio) result.audio = filters.audio
  return Object.keys(result).length ? JSON.stringify(result) : undefined
}

export function filterCount(filters: CatalogFilters): number {
  return selectionKeys.filter((key) => filters[key]?.length).length + numberKeys.filter((key) => filters[key] !== undefined).length
    + choiceKeys.filter((key) => filters[key]).length
}

/** Catalog of a title's kind filtered on one person (TMDB ID), opened from the cast and crew of an item page. */
export function personCatalogRoute(kind: ItemKind, category: 'actors' | 'creators', id: number) {
  const catalogKind: LibraryKind = kind === 'series' || kind === 'season' || kind === 'episode' ? 'series' : kind === 'video' ? 'other' : 'movies'
  return { name: 'kind', params: { kind: catalogKind }, query: { filters: serializeFilters({ [category]: [id] }) } }
}
