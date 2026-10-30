import { describe, expect, it } from 'vitest'
import { channelLayout, formatBytes, formatRuntime, languageName } from './format'

describe('format', () => {
  it('names ISO 639-2 languages', () => {
    expect(languageName('fre', 'fr')).toBe('Français')
    expect(languageName('eng', 'en')).toBe('English')
    expect(languageName('und', 'fr')).toBeNull()
    expect(languageName(undefined, 'fr')).toBeNull()
  })

  it('formats runtimes, sizes and channels', () => {
    expect(formatRuntime(117)).toBe('1 h 57')
    expect(formatRuntime(45)).toBe('45 min')
    expect(formatRuntime(null)).toBeNull()
    expect(formatBytes(18_400_000_000, 'en')).toBe('18.4 GB')
    expect(channelLayout(6)).toBe('5.1')
  })
})
