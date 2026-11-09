import { createI18n } from 'vue-i18n'
import en from './en'
import fr from './fr'
import ptBR from './pt-BR'

export const locales = ['fr', 'en', 'pt-BR'] as const
export type Locale = (typeof locales)[number]
export const localeLabels: Record<Locale, string> = { fr: 'Français', en: 'English', 'pt-BR': 'Português (Brasil)' }

/** Maps a browser language such as pt-PT or en-GB to the closest supported locale. */
export function matchLocale(tag: string): Locale | null {
  const lower = tag.toLowerCase()
  const exact = locales.find((l) => l.toLowerCase() === lower)
  if (exact) return exact
  const language = lower.split('-')[0]
  return locales.find((l) => l.toLowerCase().split('-')[0] === language) ?? null
}

function browserLocale(): Locale {
  return matchLocale(navigator.language) ?? 'fr'
}

const datetimeFormats = {
  long: { year: 'numeric', month: 'long', day: 'numeric', hour: '2-digit', minute: '2-digit' },
} as const

export const i18n = createI18n({
  legacy: false,
  locale: browserLocale(),
  fallbackLocale: 'en',
  messages: { fr, en, 'pt-BR': ptBR },
  datetimeFormats: { fr: datetimeFormats, en: datetimeFormats, 'pt-BR': datetimeFormats },
})

export function setLocale(locale: Locale) {
  i18n.global.locale.value = locale
  document.documentElement.lang = locale
}
