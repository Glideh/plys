import { api } from './client'

/** Public identity of the server serving this page. */
export interface ServerInfo {
  id: string
  name: string
  version: string
  publicKey: string
}

export const serverApi = {
  info: () => api<ServerInfo>('/server', { auth: false }),
}
