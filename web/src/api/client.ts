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
  role: 'admin' | 'user'
  locale: 'fr' | 'en'
  createdAt: string
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

interface RequestOptions {
  method?: 'GET' | 'POST' | 'PATCH' | 'PUT' | 'DELETE'
  body?: unknown
  /** Set to false for endpoints that must not trigger a token refresh on 401. */
  auth?: boolean
  /** Lets the request outlive the page (tab closing). */
  keepalive?: boolean
}

export async function api<T>(path: string, { method = 'GET', body, auth = true, keepalive }: RequestOptions = {}): Promise<T> {
  const send = () => {
    const headers: Record<string, string> = {}
    if (body !== undefined) headers['Content-Type'] = 'application/json'
    if (auth && accessToken) headers.Authorization = `Bearer ${accessToken}`
    return fetch(`${BASE}${path}`, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      credentials: 'same-origin',
      keepalive,
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
