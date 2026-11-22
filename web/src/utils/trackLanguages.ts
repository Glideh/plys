import { languageKey } from './format'

/**
 * Audio and subtitle languages last picked on the item page or in the player, kept in this browser only. They preset
 * the tracks of the next title played. A null subtitle language means subtitles off.
 */
export interface TrackLanguages {
  audio: string | null
  subtitle: string | null
}

export const TRACK_LANGUAGES_KEY = 'plys.trackLanguages'
const NONE: TrackLanguages = { audio: null, subtitle: null }

interface LanguageTrack {
  index: number
  language?: string
  forced?: boolean
}

/** Reads stored languages; a missing or unreadable one means no preference. */
export function parseTrackLanguages(raw: string | null): TrackLanguages {
  let value: Partial<Record<keyof TrackLanguages, unknown>> = {}
  try {
    const parsed: unknown = JSON.parse(raw ?? 'null')
    if (parsed && typeof parsed === 'object') value = parsed
  } catch {
    // Unreadable preference: none.
  }
  const read = (v: unknown) => (typeof v === 'string' ? languageKey(v) : null)
  return { audio: read(value.audio), subtitle: read(value.subtitle) }
}

export function loadTrackLanguages(): TrackLanguages {
  try {
    return parseTrackLanguages(localStorage.getItem(TRACK_LANGUAGES_KEY))
  } catch {
    return { ...NONE }
  }
}

function save(languages: TrackLanguages) {
  try {
    localStorage.setItem(TRACK_LANGUAGES_KEY, JSON.stringify(languages))
  } catch {
    // Storage unavailable: the next title opens with the server's choice.
  }
}

/**
 * The first track in the wanted language, or null. A subtitle that is not forced comes first: forced ones only cover
 * the lines spoken in another language.
 */
export function preferredTrack<T extends LanguageTrack>(tracks: readonly T[], language: string | null): T | null {
  if (!language) return null
  const matching = tracks.filter((t) => languageKey(t.language) === language)
  return matching.find((t) => !t.forced) ?? matching[0] ?? null
}

/** The languages once an audio track was picked; one without a language keeps the previous preference. */
export function withAudio(languages: TrackLanguages, language: string | undefined): TrackLanguages {
  const key = languageKey(language)
  return key ? { ...languages, audio: key } : languages
}

/** The languages once a subtitle was picked, null for off; one without a language keeps the previous preference. */
export function withSubtitle(languages: TrackLanguages, language: string | undefined | null): TrackLanguages {
  if (language === null) return { ...languages, subtitle: null }
  const key = languageKey(language)
  return key ? { ...languages, subtitle: key } : languages
}

export function rememberAudio(language: string | undefined) {
  save(withAudio(loadTrackLanguages(), language))
}

export function rememberSubtitle(language: string | undefined | null) {
  save(withSubtitle(loadTrackLanguages(), language))
}
