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
}

export const settingsApi = {
  get: () => api<ServerSettings>('/admin/settings'),
  /** An empty tmdbApiKey removes the key. */
  update: (body: { tmdbApiKey?: string; metadataLanguage?: string }) =>
    api<ServerSettings>('/admin/settings', { method: 'PATCH', body }),
}
