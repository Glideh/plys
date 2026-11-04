import { ref } from 'vue'
import type { LocationQuery } from 'vue-router'
import { api, ApiError, type ApiErrorCode } from './client'

/** A server holding a copy of a title; `baseUrl` is null for the server serving this page. */
export interface ServerRef {
  serverId: string
  serverName: string
  baseUrl: string | null
}

/** Identity of the server serving this page; null until `loadLocalServer` resolves. */
export const localServerId = ref<string | null>(null)

export async function loadLocalServer(): Promise<void> {
  if (localServerId.value) return
  const info = await api<{ id: string }>('/server', { auth: false })
  localServerId.value = info.id
}

export function isRemote(serverId: string): boolean {
  return localServerId.value !== null && serverId !== localServerId.value
}

/** Resolves a URL returned by a peer (relative to its origin) against the peer's base URL. */
export function absoluteUrl(server: ServerRef | null, url: string | null): string | null {
  if (!url || !server?.baseUrl || /^https?:\/\//.test(url)) return url
  return `${server.baseUrl}${url}`
}

interface CachedToken {
  token: string
  expiresAt: number
}

const tokens = new Map<string, CachedToken>()
const TOKEN_MARGIN_MS = 30_000

async function peerToken(serverId: string, fresh = false): Promise<string> {
  const cached = tokens.get(serverId)
  if (!fresh && cached && cached.expiresAt - Date.now() > TOKEN_MARGIN_MS) return cached.token
  const res = await api<{ token: string; expiresAt: string }>('/me/federation-token', {
    method: 'POST',
    body: { audience: serverId },
  })
  tokens.set(serverId, { token: res.token, expiresAt: Date.parse(res.expiresAt) })
  return res.token
}

export interface PeerRequest {
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE'
  body?: unknown
  keepalive?: boolean
}

/**
 * Calls the API of a paired server with a token it accepts for this user.
 * The peer is a different origin: its CORS allow-list must contain this page's origin.
 */
export async function peerApi<T>(server: ServerRef, path: string, { method = 'GET', body, keepalive }: PeerRequest = {}): Promise<T> {
  if (!server.baseUrl) throw new Error('peerApi called for the local server')
  const send = async (fresh: boolean) => {
    const token = await peerToken(server.serverId, fresh)
    return fetch(`${server.baseUrl}/api/v1${path}`, {
      method,
      headers: {
        Authorization: `Bearer ${token}`,
        ...(body === undefined ? {} : { 'Content-Type': 'application/json' }),
      },
      body: body === undefined ? undefined : JSON.stringify(body),
      keepalive,
    })
  }

  let res: Response
  try {
    res = await send(false)
    if (res.status === 401) {
      tokens.delete(server.serverId)
      res = await send(true)
    }
  } catch {
    throw new ApiError(0, 'network', 'Network error')
  }

  if (!res.ok) {
    const payload = await res.json().catch(() => null)
    throw new ApiError(res.status, (payload?.error?.code ?? 'internal') as ApiErrorCode, payload?.error?.message ?? res.statusText)
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

/** Calls the API of the server holding an item: this server when `server` is null. */
export function serverApi<T>(server: ServerRef | null, path: string, opts: PeerRequest = {}): Promise<T> {
  return server?.baseUrl ? peerApi<T>(server, path, opts) : api<T>(path, opts)
}

/** Query parameters naming a paired server so that another page can reach it; empty for this server. */
export function serverQuery(server: ServerRef | null): Record<string, string> {
  if (!server?.baseUrl) return {}
  return { server: server.serverId, serverName: server.serverName, serverUrl: server.baseUrl }
}

export function serverFromQuery(query: LocationQuery): ServerRef | null {
  const serverId = typeof query.server === 'string' ? query.server : ''
  const baseUrl = typeof query.serverUrl === 'string' ? query.serverUrl : ''
  if (!serverId || !baseUrl) return null
  return { serverId, serverName: typeof query.serverName === 'string' ? query.serverName : '', baseUrl }
}
