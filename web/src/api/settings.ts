import { api } from './client'

export const metadataLanguages = ['fr-FR', 'en-US', 'de-DE', 'es-ES', 'it-IT'] as const

export const metadataLanguageNames: Record<string, string> = {
  'fr-FR': 'Français',
  'en-US': 'English (US)',
  'de-DE': 'Deutsch',
  'es-ES': 'Español',
  'it-IT': 'Italiano',
}

export interface ServerSettings {
  tmdbConfigured: boolean
  metadataLanguage: string
  serverName: string
  publicUrl: string
}

export const settingsApi = {
  get: () => api<ServerSettings>('/admin/settings'),
  /** An empty tmdbApiKey removes the key. An empty publicUrl clears it. */
  update: (body: { tmdbApiKey?: string; metadataLanguage?: string; serverName?: string; publicUrl?: string }) =>
    api<ServerSettings>('/admin/settings', { method: 'PATCH', body }),
}
