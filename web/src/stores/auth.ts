import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { api, onSessionExpired, refreshSession, setAccessToken, type Session, type User } from '@/api/client'
import { setLocale, type Locale } from '@/i18n'

const DEVICE_NAME = 'Web'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const needsSetup = ref(false)
  let initPromise: Promise<void> | null = null

  const isAdmin = computed(() => user.value?.role === 'admin')

  function applySession(session: Session) {
    setAccessToken(session.accessToken)
    user.value = session.user
    setLocale(session.user.locale)
  }

  function clear() {
    setAccessToken(null)
    user.value = null
  }

  onSessionExpired(clear)

  /** Resolves setup state and restores the session from the refresh cookie, once per page load. */
  function init(): Promise<void> {
    initPromise ??= (async () => {
      const status = await api<{ needsSetup: boolean }>('/setup/status', { auth: false })
      needsSetup.value = status.needsSetup
      if (!status.needsSetup) {
        const session = await refreshSession()
        if (session) applySession(session)
      }
    })().catch((err) => {
      initPromise = null
      throw err
    })
    return initPromise
  }

  async function setupAdmin(username: string, password: string, locale: Locale) {
    const session = await api<Session>('/setup/admin', {
      method: 'POST',
      body: { username, password, locale, deviceName: DEVICE_NAME, tokenDelivery: 'cookie' },
      auth: false,
    })
    needsSetup.value = false
    applySession(session)
  }

  async function login(username: string, password: string) {
    const session = await api<Session>('/auth/login', {
      method: 'POST',
      body: { username, password, deviceName: DEVICE_NAME, tokenDelivery: 'cookie' },
      auth: false,
    })
    applySession(session)
  }

  /** Creates the account of an invitation and signs it in. */
  async function acceptInvitation(body: { token: string; pin: string; username: string; password: string; locale: Locale }) {
    const session = await api<Session>('/invitations/accept', {
      method: 'POST',
      body: { ...body, deviceName: DEVICE_NAME, tokenDelivery: 'cookie' },
      auth: false,
    })
    applySession(session)
  }

  async function logout() {
    await api('/auth/logout', { method: 'POST', auth: false }).catch(() => undefined)
    clear()
  }

  /** Saves a preference of the signed-in user; the UI follows the server's answer. */
  async function updatePreferences(patch: { autoplayNext?: boolean; locale?: Locale }) {
    const updated = await api<User>('/me', { method: 'PATCH', body: patch })
    user.value = updated
    setLocale(updated.locale)
  }

  /** Uploads a JPEG or PNG avatar for the signed-in user. */
  async function setAvatar(file: Blob) {
    user.value = await api<User>('/me/avatar', { method: 'PUT', body: file })
  }

  async function removeAvatar() {
    user.value = await api<User>('/me/avatar', { method: 'DELETE' })
  }

  return { user, needsSetup, isAdmin, init, setupAdmin, login, acceptInvitation, logout, updatePreferences, setAvatar, removeAvatar }
})
