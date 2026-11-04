import { api } from './client'

/** A paired server; `url` is its public base URL. */
export interface Peer {
  id: string
  url: string
  name: string
  version: string | null
  localVersion: string
  status: 'compatible' | 'incompatible' | 'unverified' | 'offline'
  versionCheckedAt: string | null
}

export const peersApi = {
  pairingCode: () => api<{ code: string }>('/admin/federation/pairing-code'),
  list: () => api<Peer[]>('/admin/federation/peers'),
  add: (code: string) => api<Peer>('/admin/federation/peers', { method: 'POST', body: { code } }),
  update: (id: string, url: string) =>
    api<Peer>(`/admin/federation/peers/${id}`, { method: 'PATCH', body: { url } }),
  sync: (id: string) => api<{ users: number }>(`/admin/federation/peers/${id}/sync`, { method: 'POST' }),
  remove: (id: string) => api<void>(`/admin/federation/peers/${id}`, { method: 'DELETE' }),
}
