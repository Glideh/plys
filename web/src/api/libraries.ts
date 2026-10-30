import { api } from './client'

export const libraryKinds = ['movies', 'series', 'other'] as const
export type LibraryKind = (typeof libraryKinds)[number]
export type ItemKind = 'movie' | 'part' | 'series' | 'season' | 'episode' | 'video'

export interface Library {
  id: string
  name: string
  kind: LibraryKind
  /** Only returned to admins. */
  paths?: string[]
  /** Only returned by the list endpoint. */
  counts?: Partial<Record<ItemKind, number>>
  /** Admin only: movies, series and videos per metadata status. */
  matchCounts?: Partial<Record<'pending' | 'matched' | 'unmatched' | 'manual' | 'skipped', number>>
  lastScannedAt: string | null
  restricted: boolean
  /** Admin only: users allowed when the library is restricted. */
  userIds?: string[]
  createdAt: string
}

export interface ScanStatus {
  libraryId: string
  state: 'queued' | 'running' | 'done' | 'failed'
  stage?: 'files' | 'metadata'
  total: number
  processed: number
  added: number
  probed: number
  unmatched: number
  failed: number
  missing: number
  /** A folder was unreachable or empty, so missing files were not flagged. */
  incomplete: boolean
  metaTotal: number
  metaDone: number
  metaError?: string
  startedAt?: string
  finishedAt?: string
  error?: string
}

export interface FolderEntry {
  name: string
  path: string
}

export interface BrowseResult {
  /** null when listing the media roots. */
  path: string | null
  /** null when going up leads back to the media roots. */
  parent: string | null
  entries: FolderEntry[]
}

export const librariesApi = {
  list: () => api<{ items: Library[] }>('/libraries').then((r) => r.items),
  create: (body: { name: string; kind: LibraryKind; paths: string[]; restricted: boolean; userIds: string[] }) =>
    api<Library>('/libraries', { method: 'POST', body }),
  update: (id: string, body: { name?: string; paths?: string[]; restricted?: boolean; userIds?: string[] }) =>
    api<Library>(`/libraries/${id}`, { method: 'PATCH', body }),
  get: (id: string) => api<Library>(`/libraries/${id}`),
  remove: (id: string) => api<void>(`/libraries/${id}`, { method: 'DELETE' }),
  browse: (path: string | null) =>
    api<BrowseResult>(`/admin/fs/browse${path ? `?path=${encodeURIComponent(path)}` : ''}`),
  scan: (id: string) => api<ScanStatus>(`/libraries/${id}/scan`, { method: 'POST' }),
  scans: () => api<{ items: ScanStatus[] }>('/admin/scans').then((r) => r.items),
}
