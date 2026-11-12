import { describe, expect, it } from 'vitest'
import { defaultSubtitleLanguage } from './subtitles'

describe('defaultSubtitleLanguage', () => {
  it('follows the interface language', () => {
    expect(defaultSubtitleLanguage('fr')).toBe('fr')
    expect(defaultSubtitleLanguage('pt-BR')).toBe('pt-BR')
    expect(defaultSubtitleLanguage('de-AT')).toBe('de')
    expect(defaultSubtitleLanguage('xx')).toBe('en')
  })
})
