import { api } from './client'

export const metadataLanguages = ['fr-FR', 'en-US', 'de-DE', 'es-ES', 'it-IT', 'pt-BR'] as const

export const metadataLanguageNames: Record<string, string> = {
  'fr-FR': 'Français',
  'en-US': 'English (US)',
  'de-DE': 'Deutsch',
  'es-ES': 'Español',
  'it-IT': 'Italiano',
  'pt-BR': 'Português (Brasil)',
}

export interface ServerSettings {
  tmdbConfigured: boolean
  metadataLanguage: string
  serverName: string
  publicUrl: string
  openSubtitlesConfigured: boolean
  openSubtitlesUsername: string
}

export const settingsApi = {
  get: () => api<ServerSettings>('/admin/settings'),
  /**
   * An empty tmdbApiKey removes the key. An empty publicUrl clears it. An empty openSubtitlesApiKey removes the
   * OpenSubtitles configuration, an empty openSubtitlesUsername its login.
   */
  update: (body: {
    tmdbApiKey?: string
    metadataLanguage?: string
    serverName?: string
    publicUrl?: string
    openSubtitlesApiKey?: string
    openSubtitlesUsername?: string
    openSubtitlesPassword?: string
  }) =>
    api<ServerSettings>('/admin/settings', { method: 'PATCH', body }),
}
