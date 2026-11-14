import { api } from './client'
import type { ItemKind, LibraryKind } from './libraries'
import { serializeFilters, type CatalogFilters, type FilterCategory, type FilterOptionsPage } from './catalogFilters'

export const itemSorts = ['title', 'year', 'added', 'rating'] as const
export type ItemSort = (typeof itemSorts)[number]
export type MatchStatus = 'pending' | 'matched' | 'unmatched' | 'manual' | 'skipped'

/** A server holding a version of a grouped title; the first source of a card is its primary one. */
export interface CardSource {
  serverId: string
  serverName: string
  /** Absolute URL of a paired server; absent for the local one. */
  serverUrl?: string
  itemId: string
}

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
  /** Only set on the kind listings, which merge the servers of the federation. */
  sources?: CardSource[]
}

export interface ItemPage {
  items: ItemCard[]
  total: number
  offset: number
  limit: number
  unavailableServers?: number
  /** Names of the excluded servers; paused servers are neither counted nor named. */
  unavailableServerNames?: string[]
}

/** Searches shorter than this, once trimmed, are rejected by the API. */
export const MIN_SEARCH_LENGTH = 3

export interface SearchGroup {
  kind: LibraryKind
  /** The best matches of the kind, at most the requested limit. */
  items: ItemCard[]
  /** Every matching title of the kind. */
  total: number
}

export interface SearchResult {
  /** Only the kinds with matches, in the order movies, series, other. */
  groups: SearchGroup[]
  unavailableServers: number
  /** Names of the excluded servers; paused servers are neither counted nor named. */
  unavailableServerNames?: string[]
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

export interface ExternalSubtitle {
  id: string
  /** OpenSubtitles code: ISO 639-1 (`fr`), or with a region (`pt-BR`). */
  language: string
  release: string
  hearingImpaired: boolean
  source: 'opensubtitles'
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
  /** Subtitles downloaded online for this file, shared by every user. */
  externalSubtitles: ExternalSubtitle[]
  error?: string
  /** Location of the file on the server, only sent to admins. */
  path?: string
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

/** A credited person; id is the TMDB person, absent when unknown (older servers or metadata). */
export interface Person {
  id?: number
  name: string
  character?: string
  photoUrl: string | null
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
  /** Directors or creators with their picture; absent from older servers, empty until the metadata is refreshed. */
  directorCredits?: Person[]
  cast: Person[]
  posterUrl: string | null
  backdropUrl: string | null
  /** Title logo shown by the player while paused; seasons, episodes and parts carry their series' or movie's. Absent from older servers. */
  logoUrl?: string | null
  thumbUrl: string | null
  tmdbId: number | null
  matchStatus: MatchStatus
  /** Subtitles can be searched online for the versions of this item. */
  subtitleSearch: boolean
  seasonNumber?: number
  episodeNumber?: number
  episodeNumberEnd?: number
  partNumber?: number
  ancestors: Ancestor[]
  versions: Version[]
  children: ChildItem[]
  watched: boolean
  positionSeconds: number
  /** Set on top-level titles (movies, series, videos) the user added to their watchlist. */
  inWatchlist: boolean
  /** What "play" starts on a series, season or multi-part movie. */
  playTarget: PlayTarget | null
  /** Follows an episode or part in viewing order. */
  next: PlayTarget | null
  /** Other servers holding a version of this title; only set on local top-level titles. */
  sources: CardSource[]
  /** Deepest folder holding every file of the item and its children, only sent to admins. */
  folder?: string
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
  list: (libraryId: string, params: { query?: string; sort?: ItemSort; offset?: number; limit?: number; filters?: CatalogFilters }, signal?: AbortSignal) => {
    const qs = new URLSearchParams()
    for (const [key, value] of Object.entries({ ...params, filters: params.filters && serializeFilters(params.filters) })) if (value !== undefined && value !== '') qs.set(key, String(value))
    return api<ItemPage>(`/libraries/${libraryId}/items?${qs}`, { signal })
  },
  listKind: (kind: LibraryKind, params: { library?: string; query?: string; sort?: ItemSort; offset?: number; limit?: number; filters?: CatalogFilters }, signal?: AbortSignal) => {
    const qs = new URLSearchParams()
    for (const [key, value] of Object.entries({ ...params, filters: params.filters && serializeFilters(params.filters) })) if (value !== undefined && value !== '') qs.set(key, String(value))
    return api<ItemPage>(`/kinds/${kind}/items?${qs}`, { signal })
  },
  search: (query: string, limit?: number) => {
    const qs = new URLSearchParams({ query })
    if (limit) qs.set('limit', String(limit))
    return api<SearchResult>(`/search?${qs}`)
  },
  get: (id: string) => api<ItemDetail>(`/items/${id}`),
  filterOptions: (kind: LibraryKind, library?: string, category?: FilterCategory, query = '', selected: string[] = [], locale = 'fr') => {
    const params = new URLSearchParams({ query, locale })
    if (library) params.set('library', library)
    if (category) params.set('category', category)
    if (selected.length) params.set('selected', selected.join(','))
    return api<FilterOptionsPage>(`/kinds/${kind}/filters?${params}`)
  },
  titleSources: (kind: ItemKind, tmdbId: number) =>
    api<{ sources: CardSource[] }>(`/kinds/${kind}/titles/${tmdbId}/sources`).then((r) => r.sources),
  searchMatch: (id: string, query: string, year?: number) => {
    const qs = new URLSearchParams({ query })
    if (year) qs.set('year', String(year))
    return api<{ items: MatchCandidate[] }>(`/items/${id}/match/search?${qs}`).then((r) => r.items)
  },
  setMatch: (id: string, tmdbId: number) => api<{ merged?: boolean }>(`/items/${id}/match`, { method: 'POST', body: { tmdbId } }),
  refresh: (id: string) => api<{ merged?: boolean }>(`/items/${id}/refresh`, { method: 'POST' }),
}
