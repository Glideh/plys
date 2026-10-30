import { api } from './client'

export const libraryKinds = ['movies', 'series', 'other'] as const
export type LibraryKind = (typeof libraryKinds)[number]

export interface Library {
  id: string
  name: string
  kind: LibraryKind
  /** Only returned to admins. */
  paths?: string[]
  createdAt: string
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
  create: (body: { name: string; kind: LibraryKind; paths: string[] }) =>
    api<Library>('/libraries', { method: 'POST', body }),
  update: (id: string, body: { name?: string; paths?: string[] }) =>
    api<Library>(`/libraries/${id}`, { method: 'PATCH', body }),
  remove: (id: string) => api<void>(`/libraries/${id}`, { method: 'DELETE' }),
  browse: (path: string | null) =>
    api<BrowseResult>(`/admin/fs/browse${path ? `?path=${encodeURIComponent(path)}` : ''}`),
}
