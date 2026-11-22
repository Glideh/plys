/** Subtitle rendering chosen in the player, kept in this browser only. The keys match the Android TV client. */
export const SUBTITLE_SIZES = ['small', 'medium', 'large', 'xlarge'] as const
export const SUBTITLE_BACKGROUNDS = ['outline', 'box'] as const

export type SubtitleSize = (typeof SUBTITLE_SIZES)[number]
export type SubtitleBackground = (typeof SUBTITLE_BACKGROUNDS)[number]
export interface SubtitleStyle {
  size: SubtitleSize
  background: SubtitleBackground
}

export const SUBTITLE_STYLE_KEY = 'plys.subtitleStyle'
export const DEFAULT_SUBTITLE_STYLE: SubtitleStyle = { size: 'medium', background: 'outline' }

/** Reads a stored style; an unknown or missing field falls back to the default. */
export function parseSubtitleStyle(raw: string | null): SubtitleStyle {
  let value: Partial<Record<keyof SubtitleStyle, unknown>> = {}
  try {
    const parsed: unknown = JSON.parse(raw ?? 'null')
    if (parsed && typeof parsed === 'object') value = parsed
  } catch {
    // Unreadable preference: use the defaults.
  }
  return {
    size: SUBTITLE_SIZES.find((s) => s === value.size) ?? DEFAULT_SUBTITLE_STYLE.size,
    background: SUBTITLE_BACKGROUNDS.find((b) => b === value.background) ?? DEFAULT_SUBTITLE_STYLE.background,
  }
}

export function loadSubtitleStyle(): SubtitleStyle {
  try {
    return parseSubtitleStyle(localStorage.getItem(SUBTITLE_STYLE_KEY))
  } catch {
    return { ...DEFAULT_SUBTITLE_STYLE }
  }
}

export function saveSubtitleStyle(style: SubtitleStyle) {
  try {
    localStorage.setItem(SUBTITLE_STYLE_KEY, JSON.stringify(style))
  } catch {
    // Storage unavailable: the choice lasts until the page is left.
  }
}

/** Classes on the video element that the `::cue` rules of style.css read. */
export function subtitleClasses(style: SubtitleStyle): string[] {
  return [`subs-${style.size}`, `subs-${style.background}`]
}
