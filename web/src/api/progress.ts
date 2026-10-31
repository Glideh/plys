import { api } from './client'
import type { ItemKind } from './libraries'

/** Card of the home rails ("continue watching", "next up"). */
export interface HomeItem {
  id: string
  kind: ItemKind
  title: string
  showTitle?: string
  seasonNumber?: number
  episodeNumber?: number
  partNumber?: number
  posterUrl: string | null
  positionSeconds: number
  durationSeconds: number
  updatedAt: string
}

export const progressApi = {
  report: (itemId: string, positionSeconds: number, durationSeconds: number, keepalive = false) =>
    api<{ watched: boolean }>(`/items/${itemId}/progress`, {
      method: 'PUT',
      body: { positionSeconds, durationSeconds },
      keepalive,
    }),
  setWatched: (itemId: string, watched: boolean) =>
    api<void>(`/items/${itemId}/watched`, { method: watched ? 'POST' : 'DELETE' }),
  continueWatching: () => api<{ items: HomeItem[] }>('/me/continue-watching').then((r) => r.items),
  nextUp: () => api<{ items: HomeItem[] }>('/me/next-up').then((r) => r.items),
}
