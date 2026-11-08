import { api } from './client'
import type { CardSource } from './catalog'
import { serverApi, type ServerRef } from './federation'
import type { ItemKind } from './libraries'

/** A title of the watchlist, kept by the server holding it. */
export interface WatchlistItem {
  id: string
  kind: ItemKind
  title: string
  year: number | null
  posterUrl: string | null
  addedAt: string
  source: CardSource
}

export const watchlistApi = {
  /** The watchlist across this server and its paired servers, most recently added first. */
  list: () => api<{ items: WatchlistItem[]; unavailableServers: number }>('/me/watchlist'),
  /** Adds or removes a title on the server holding it (this server when `server` is null). */
  set: (server: ServerRef | null, itemId: string, listed: boolean) =>
    serverApi<void>(server, `/items/${itemId}/watchlist`, { method: listed ? 'POST' : 'DELETE' }),
}
