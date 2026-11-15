import type { Locale } from '@/i18n'

export type ApiErrorCode =
  | 'validation'
  | 'invalid_body'
  | 'invalid_credentials'
  | 'invalid_refresh_token'
  | 'unauthorized'
  | 'forbidden'
  | 'not_found'
  | 'already_initialized'
  | 'username_taken'
  | 'last_admin'
  | 'cannot_delete_self'
  | 'name_taken'
  | 'path_conflict'
  | 'invalid_path'
  | 'invalid_tmdb_key'
  | 'tmdb_unreachable'
  | 'tmdb_not_configured'
  | 'transcoder_busy'
  | 'invalid_opensubtitles_credentials'
  | 'subtitles_not_configured'
  | 'subtitles_unauthorized'
  | 'subtitles_quota'
  | 'subtitles_unreachable'
  | 'subtitles_rejected'
  | 'subtitles_rate_limited'
  | 'subtitles_invalid'
  | 'rate_limited'
  | 'peer_not_paired'
  | 'public_url_required'
  | 'invitation_invalid'
  | 'invalid_pin'
  | 'internal'
  | 'network'

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: ApiErrorCode,
    message: string,
  ) {
    super(message)
  }
}

export interface User {
  id: string
  username: string
  /** Label given by an admin; shown instead of the username when set. */
  alias: string | null
  /** The alias, or the username without one: the name every view shows. */
  displayName: string
  role: 'admin' | 'user'
  locale: Locale
  autoplayNext: boolean
  createdAt: string
  /** Present only when the user has an avatar; used as a cache-busting version. */
  avatarUpdatedAt?: string
  remote: boolean
  /** Present only for remote users. */
  originServer?: string
}

export interface Session {
  accessToken: string
  tokenType: 'Bearer'
  expiresAt: string
  user: User
}

const BASE = '/api/v1'
const REFRESH_LOCK = 'toto-auth-refresh'

let accessToken: string | null = null
let refreshing: Promise<Session | null> | null = null
let sessionExpiredHandler: (() => void) | null = null

export function setAccessToken(token: string | null) {
  accessToken = token
}

export function onSessionExpired(handler: () => void) {
  sessionExpiredHandler = handler
}

// Web Locks serialize refreshes across tabs: rotation would otherwise make a sibling tab replay a revoked token.
function withRefreshLock<T>(fn: () => Promise<T>): Promise<T> {
  if (typeof navigator !== 'undefined' && navigator.locks) {
    return navigator.locks.request(REFRESH_LOCK, fn)
  }
  return fn()
}

export function refreshSession(): Promise<Session | null> {
  refreshing ??= withRefreshLock(async () => {
    const res = await fetch(`${BASE}/auth/refresh`, { method: 'POST', credentials: 'same-origin' })
    if (!res.ok) {
      accessToken = null
      return null
    }
    const session = (await res.json()) as Session
    accessToken = session.accessToken
    return session
  }).finally(() => {
    refreshing = null
  })
  return refreshing
}

/** URL of the signed-in user's avatar, or null when there is none. */
export function avatarUrl(user: Pick<User, 'id' | 'avatarUpdatedAt'> | null | undefined): string | null {
  if (!user?.avatarUpdatedAt) return null
  return `${BASE}/images/avatars/${user.id}?v=${encodeURIComponent(user.avatarUpdatedAt)}`
}

interface RequestOptions {
  method?: 'GET' | 'POST' | 'PATCH' | 'PUT' | 'DELETE'
  /** A Blob is sent as the raw request body with its own content type. */
  body?: unknown
  /** Set to false for endpoints that must not trigger a token refresh on 401. */
  auth?: boolean
  /** Lets the request outlive the page (tab closing). */
  keepalive?: boolean
  /** Aborts the request, so that the server stops working on an answer nobody waits for. */
  signal?: AbortSignal
}

export async function api<T>(path: string, { method = 'GET', body, auth = true, keepalive, signal }: RequestOptions = {}): Promise<T> {
  const send = () => {
    const headers: Record<string, string> = {}
    const raw = body instanceof Blob
    if (body !== undefined) headers['Content-Type'] = raw ? body.type : 'application/json'
    if (auth && accessToken) headers.Authorization = `Bearer ${accessToken}`
    return fetch(`${BASE}${path}`, {
      method,
      headers,
      body: body === undefined ? undefined : raw ? body : JSON.stringify(body),
      credentials: 'same-origin',
      keepalive,
      signal,
    })
  }

  let res: Response
  try {
    res = await send()
    if (res.status === 401 && auth) {
      if (await refreshSession()) {
        res = await send()
      } else {
        sessionExpiredHandler?.()
      }
    }
  } catch {
    throw new ApiError(0, 'network', 'Network error')
  }

  if (!res.ok) {
    const payload = await res.json().catch(() => null)
    throw new ApiError(res.status, payload?.error?.code ?? 'internal', payload?.error?.message ?? res.statusText)
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}
