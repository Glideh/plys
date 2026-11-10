import { describe, expect, it } from 'vitest'
import en from './en'
import fr from './fr'
import { matchLocale } from './index'
import ptBR from './pt-BR'

function keys(messages: object, prefix = ''): string[] {
  return Object.entries(messages).flatMap(([key, value]) =>
    typeof value === 'object' && value !== null ? keys(value, `${prefix}${key}.`) : [`${prefix}${key}`],
  )
}

describe('locales', () => {
  it('maps browser languages to the closest supported locale', () => {
    expect(matchLocale('pt-BR')).toBe('pt-BR')
    expect(matchLocale('pt-br')).toBe('pt-BR')
    expect(matchLocale('pt-PT')).toBe('pt-BR')
    expect(matchLocale('pt')).toBe('pt-BR')
    expect(matchLocale('fr-CA')).toBe('fr')
    expect(matchLocale('en-GB')).toBe('en')
    expect(matchLocale('de-DE')).toBeNull()
  })

  it('translates the same keys in every locale', () => {
    const reference = keys(fr).sort()
    expect(keys(en).sort()).toEqual(reference)
    expect(keys(ptBR).sort()).toEqual(reference)
  })

  it('keeps the plural forms of each message', () => {
    const plurals = (messages: object) => keys(messages).filter((key) => {
      const value = key.split('.').reduce<unknown>((node, part) => (node as Record<string, unknown>)[part], messages)
      return typeof value === 'string' && value.includes(' | ')
    })
    expect(plurals(ptBR).sort()).toEqual(plurals(fr).sort())
  })
})
