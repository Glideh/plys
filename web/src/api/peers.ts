import { api } from './client'

/** A paired server; `url` is its public base URL. */
export interface Peer {
  id: string
  url: string
  /** Name the server gave itself. */
  name: string
  /** Label given by an admin of this server; shown instead of the name when set. */
  alias: string | null
  /** The alias, or the name without one. */
  displayName: string
  version: string | null
  localVersion: string
  /** `offline` after two failed calls in a row; `paused` when an admin left the server out. */
  status: 'compatible' | 'incompatible' | 'unverified' | 'offline' | 'paused'
  versionCheckedAt: string | null
  paused: boolean
  /** Last time the server answered or called this one. */
  lastSeenAt: string | null
  /** Set while the server does not answer; the API probes it again in the background. */
  offlineSince: string | null
}

export const peersApi = {
  pairingCode: () => api<{ code: string }>('/admin/federation/pairing-code'),
  list: () => api<Peer[]>('/admin/federation/peers'),
  /** `pairingPending` is true while the other server has not paired this one back. */
  add: (code: string) => api<Peer & { pairingPending: boolean }>('/admin/federation/peers', { method: 'POST', body: { code } }),
  update: (id: string, url: string) =>
    api<Peer>(`/admin/federation/peers/${id}`, { method: 'PATCH', body: { url } }),
  /** A paused server is left out of every aggregated answer, without being reported as unavailable. */
  setPaused: (id: string, paused: boolean) =>
    api<Peer>(`/admin/federation/peers/${id}/paused`, { method: 'PUT', body: { paused } }),
  setAlias: (id: string, alias: string | null) =>
    api<Peer>(`/admin/federation/peers/${id}/alias`, { method: 'PUT', body: { alias } }),
  sync: (id: string) => api<{ users: number }>(`/admin/federation/peers/${id}/sync`, { method: 'POST' }),
  remove: (id: string) => api<void>(`/admin/federation/peers/${id}`, { method: 'DELETE' }),
}
