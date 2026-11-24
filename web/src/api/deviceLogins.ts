import { api } from './client'

/** A TV waiting for a signed-in phone to approve its sign-in. */
export interface DeviceLogin {
  userCode: string
  deviceName: string
  expiresAt: string
}

export const deviceLoginsApi = {
  lookup: (userCode: string) => api<DeviceLogin>('/device-logins/lookup', { method: 'POST', body: { userCode } }),
  approve: (userCode: string) => api<void>('/device-logins/approve', { method: 'POST', body: { userCode } }),
  deny: (userCode: string) => api<void>('/device-logins/deny', { method: 'POST', body: { userCode } }),
}
