import { createI18n } from 'vue-i18n'
import en from './en'
import fr from './fr'

export const locales = ['fr', 'en'] as const
export type Locale = (typeof locales)[number]

function browserLocale(): Locale {
  const lang = navigator.language.slice(0, 2)
  return (locales as readonly string[]).includes(lang) ? (lang as Locale) : 'fr'
}

export const i18n = createI18n({
  legacy: false,
  locale: browserLocale(),
  fallbackLocale: 'en',
  messages: { fr, en },
})

export function setLocale(locale: Locale) {
  i18n.global.locale.value = locale
  document.documentElement.lang = locale
}
