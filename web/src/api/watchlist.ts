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
  /** Groups the copies of a title; absent from older peers. */
  tmdbId?: number | null
  posterUrl: string | null
  addedAt: string
  source: CardSource
}

export const watchlistApi = {
  /** The watchlist across this server and its paired servers, most recently added first. */
  list: () => api<{ items: WatchlistItem[]; unavailableServers: number; unavailableServerNames?: string[] }>('/me/watchlist'),
  /** Adds or removes a title on the server holding it (this server when `server` is null). */
  set: (server: ServerRef | null, itemId: string, listed: boolean) =>
    serverApi<void>(server, `/items/${itemId}/watchlist`, { method: listed ? 'POST' : 'DELETE' }),
  /** Removes a title from the watchlist on every server, whichever copy was added. */
  removeTitle: (kind: ItemKind, tmdbId: number) => api<void>(`/kinds/${kind}/titles/${tmdbId}/watchlist`, { method: 'DELETE' }),
}
