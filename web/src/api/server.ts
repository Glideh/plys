import { api } from './client'

/** Public identity of the server serving this page. */
export interface ServerInfo {
  id: string
  name: string
  version: string
  publicKey: string
  /** A newer published release of Plys, when the server knows one. */
  update?: ServerUpdate
}

export interface ServerUpdate {
  version: string
  /** Release page with the notes. */
  url: string
}

export const serverApi = {
  info: () => api<ServerInfo>('/server', { auth: false }),
}
