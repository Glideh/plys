// ffprobe reports ISO 639-2 codes; Intl.DisplayNames expects ISO 639-1 for most languages.
const iso6392to1: Record<string, string> = {
  fre: 'fr', fra: 'fr', eng: 'en', ger: 'de', deu: 'de', spa: 'es', ita: 'it', jpn: 'ja', kor: 'ko',
  chi: 'zh', zho: 'zh', por: 'pt', rus: 'ru', ara: 'ar', hin: 'hi', dut: 'nl', nld: 'nl', swe: 'sv',
  dan: 'da', nor: 'no', fin: 'fi', pol: 'pl', tur: 'tr', gre: 'el', ell: 'el', heb: 'he', tha: 'th',
  cze: 'cs', ces: 'cs', hun: 'hu', rum: 'ro', ron: 'ro', ukr: 'uk', vie: 'vi', ind: 'id', per: 'fa', fas: 'fa',
}

/**
 * A language code reduced to what two tracks in the same language share: the ISO 639-1 code when known, without the
 * region ("fre", "fra", "fr" and "fr-CA" all give "fr"). Null when the language is blank or undetermined.
 */
export function languageKey(code: string | undefined | null): string | null {
  const base = code?.trim().split(/[-_]/)[0]?.toLowerCase()
  if (!base || base === 'und') return null
  return iso6392to1[base] ?? base
}

export function languageName(code: string | undefined, locale: string): string | null {
  if (!code || code === 'und') return null
  const short = iso6392to1[code.toLowerCase()] ?? code
  try {
    const name = new Intl.DisplayNames([locale], { type: 'language' }).of(short)
    return name ? name.charAt(0).toUpperCase() + name.slice(1) : code
  } catch {
    return code.toUpperCase()
  }
}

export function channelLayout(channels?: number): string | null {
  switch (channels) {
    case undefined:
    case 0:
      return null
    case 1:
      return 'Mono'
    case 2:
      return '2.0'
    case 6:
      return '5.1'
    case 8:
      return '7.1'
    default:
      return `${channels} ch`
  }
}

export function formatBytes(bytes: number, locale: string): string {
  const units = ['o', 'Ko', 'Mo', 'Go', 'To']
  const enUnits = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = bytes
  let i = 0
  while (value >= 1000 && i < units.length - 1) {
    value /= 1000
    i++
  }
  const unit = locale.startsWith('fr') ? units[i] : enUnits[i]
  return `${new Intl.NumberFormat(locale, { maximumFractionDigits: i >= 3 ? 1 : 0 }).format(value)} ${unit}`
}

export function formatRuntime(minutes: number | null | undefined): string | null {
  if (!minutes) return null
  const h = Math.floor(minutes / 60)
  const m = minutes % 60
  return h ? `${h} h ${String(m).padStart(2, '0')}` : `${m} min`
}
