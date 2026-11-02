import { api } from './client'
import type { ItemKind, LibraryKind } from './libraries'

export const itemSorts = ['title', 'year', 'added', 'rating'] as const
export type ItemSort = (typeof itemSorts)[number]
export type MatchStatus = 'pending' | 'matched' | 'unmatched' | 'manual' | 'skipped'

export interface ItemCard {
  id: string
  kind: ItemKind
  title: string
  year: number | null
  posterUrl: string | null
  rating: number | null
  episodeCount?: number
  matchStatus: MatchStatus
  /** Movies and videos watched, or series whose episodes are all watched. */
  watched: boolean
  unwatchedCount?: number
}

export interface ItemPage {
  items: ItemCard[]
  total: number
  offset: number
  limit: number
}

export interface Track {
  index: number
  language?: string
  codec: string
  title?: string
  channels?: number
  default: boolean
  forced?: boolean
  imageBased?: boolean
}

export interface Version {
  id: string
  label: string
  resolution?: string
  hdr: boolean
  videoCodec?: string
  edition: string | null
  size: number
  durationSeconds: number | null
  audio: Track[]
  subtitles: Track[]
  error?: string
}

export interface ChildItem {
  id: string
  kind: ItemKind
  title: string
  overview: string | null
  year: number | null
  releaseDate: string | null
  runtimeMinutes: number | null
  rating: number | null
  posterUrl: string | null
  thumbUrl: string | null
  seasonNumber?: number
  episodeNumber?: number
  episodeNumberEnd?: number
  partNumber?: number
  childCount?: number
  watched: boolean
  /** Resume point in seconds, 0 when none. */
  positionSeconds: number
  unwatchedCount?: number
}

export interface Ancestor {
  id: string
  kind: ItemKind
  title: string
  seasonNumber?: number
}

export interface ItemDetail {
  id: string
  kind: ItemKind
  libraryId: string
  title: string
  originalTitle: string | null
  year: number | null
  overview: string | null
  tagline: string | null
  releaseDate: string | null
  runtimeMinutes: number | null
  rating: number | null
  genres: string[]
  directors: string[]
  cast: { name: string; character?: string; photoUrl: string | null }[]
  posterUrl: string | null
  backdropUrl: string | null
  thumbUrl: string | null
  tmdbId: number | null
  matchStatus: MatchStatus
  seasonNumber?: number
  episodeNumber?: number
  episodeNumberEnd?: number
  partNumber?: number
  ancestors: Ancestor[]
  versions: Version[]
  children: ChildItem[]
  watched: boolean
  positionSeconds: number
  /** What "play" starts on a series, season or multi-part movie. */
  playTarget: PlayTarget | null
  /** Follows an episode or part in viewing order. */
  next: PlayTarget | null
}

export interface PlayTarget {
  id: string
  kind: ItemKind
  title: string
  seasonNumber?: number
  episodeNumber?: number
  partNumber?: number
  positionSeconds: number
  resume: boolean
  rewatch: boolean
}

export interface MatchCandidate {
  tmdbId: number
  title: string
  originalTitle?: string
  year?: number
  overview?: string
  posterUrl: string | null
}

export const catalogApi = {
  list: (libraryId: string, params: { query?: string; sort?: ItemSort; offset?: number; limit?: number }) => {
    const qs = new URLSearchParams()
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== '') qs.set(k, String(v))
    return api<ItemPage>(`/libraries/${libraryId}/items?${qs}`)
  },
  listKind: (kind: LibraryKind, params: { library?: string; query?: string; sort?: ItemSort; offset?: number; limit?: number }) => {
    const qs = new URLSearchParams()
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== '') qs.set(k, String(v))
    return api<ItemPage>(`/kinds/${kind}/items?${qs}`)
  },
  get: (id: string) => api<ItemDetail>(`/items/${id}`),
  searchMatch: (id: string, query: string, year?: number) => {
    const qs = new URLSearchParams({ query })
    if (year) qs.set('year', String(year))
    return api<{ items: MatchCandidate[] }>(`/items/${id}/match/search?${qs}`).then((r) => r.items)
  },
  setMatch: (id: string, tmdbId: number) => api<{ merged?: boolean }>(`/items/${id}/match`, { method: 'POST', body: { tmdbId } }),
  refresh: (id: string) => api<{ merged?: boolean }>(`/items/${id}/refresh`, { method: 'POST' }),
}
