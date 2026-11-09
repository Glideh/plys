export interface CatalogFilters {
  genres?: number[]
  excludeGenres?: number[]
  genreMode?: 'any' | 'all'
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
}

export const selectionKeys = ['genres', 'excludeGenres', 'actors', 'creators', 'keywords', 'languages', 'countries'] as const
export type SelectionKey = typeof selectionKeys[number]
export type FilterCategory = Exclude<SelectionKey, 'excludeGenres'>
export interface FilterOption { id: string; label: string }
export interface FilterOptionsPage { options: Partial<Record<FilterCategory, FilterOption[]>>; unavailableServers: number }
export const numberKeys = ['yearMin', 'yearMax', 'ratingMin', 'votesMin', 'runtimeMin', 'runtimeMax'] as const
export type NumberKey = typeof numberKeys[number]

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
    if (source.genreMode === 'all') filters.genreMode = 'all'
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
  if (result.genres?.length && filters.genreMode === 'all') result.genreMode = 'all'
  return Object.keys(result).length ? JSON.stringify(result) : undefined
}

export function filterCount(filters: CatalogFilters): number {
  return selectionKeys.filter((key) => filters[key]?.length).length + numberKeys.filter((key) => filters[key] !== undefined).length
}