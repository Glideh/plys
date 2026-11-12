import type { ExternalSubtitle } from './catalog'
import { serverApi, type ServerRef } from './federation'

/** A subtitle found on OpenSubtitles; `hashMatch` means it was made for this exact file. */
export interface SubtitleResult {
  fileId: number
  language: string
  release: string
  downloadCount: number
  hearingImpaired: boolean
  machineTranslated: boolean
  trusted: boolean
  hashMatch: boolean
  fps?: number
  uploader?: string
}

/** Languages offered in the search, as OpenSubtitles codes. */
export const subtitleLanguages = [
  'en', 'fr', 'es', 'de', 'it', 'pt-PT', 'pt-BR', 'nl', 'pl', 'ru', 'ar', 'zh-CN', 'zh-TW', 'ja', 'ko', 'sv', 'da',
  'no', 'fi', 'tr', 'el', 'he', 'cs', 'hu', 'ro', 'uk',
] as const

/** The OpenSubtitles language matching the interface language. */
export function defaultSubtitleLanguage(locale: string): string {
  if (subtitleLanguages.includes(locale as (typeof subtitleLanguages)[number])) return locale
  const short = locale.split('-')[0] ?? 'en'
  return subtitleLanguages.includes(short as (typeof subtitleLanguages)[number]) ? short : 'en'
}

/** Calls go to the server holding the file, like playback. */
export const subtitlesApi = {
  search: (server: ServerRef | null, itemId: string, versionId: string, language: string) =>
    serverApi<{ results: SubtitleResult[] }>(
      server,
      `/items/${itemId}/subtitles/search?${new URLSearchParams({ versionId, language })}`,
    ),
  download: (server: ServerRef | null, itemId: string, versionId: string, r: SubtitleResult) =>
    serverApi<ExternalSubtitle>(server, `/items/${itemId}/subtitles`, {
      method: 'POST',
      body: { versionId, fileId: r.fileId, language: r.language, release: r.release, hearingImpaired: r.hearingImpaired },
    }),
  /** Admins of this server only. */
  remove: (itemId: string, versionId: string, subtitleId: string) =>
    serverApi<void>(null, `/items/${itemId}/subtitles/${subtitleId}?${new URLSearchParams({ versionId })}`, { method: 'DELETE' }),
}
