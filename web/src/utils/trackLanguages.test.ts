import { describe, expect, it } from 'vitest'
import { languageKey } from './format'
import { loadTrackLanguages, parseTrackLanguages, preferredTrack, rememberAudio, withAudio, withSubtitle } from './trackLanguages'

const track = (index: number, language?: string, forced = false) => ({ index, language, forced })

describe('languageKey', () => {
  it('gives the same key to every spelling of a language', () => {
    expect(languageKey('fre')).toBe('fr')
    expect(languageKey('fra')).toBe('fr')
    expect(languageKey('fr-CA')).toBe('fr')
    expect(languageKey('pt-BR')).toBe('pt')
    expect(languageKey('POR')).toBe('pt')
    expect(languageKey('xyz')).toBe('xyz')
  })
  it('has no key for a blank or undetermined language', () => {
    expect(languageKey('und')).toBeNull()
    expect(languageKey('')).toBeNull()
    expect(languageKey(undefined)).toBeNull()
  })
})

describe('parseTrackLanguages', () => {
  it('reads stored languages', () => {
    expect(parseTrackLanguages('{"audio":"eng","subtitle":"fr"}')).toEqual({ audio: 'en', subtitle: 'fr' })
  })
  it('means no preference for missing or unreadable values', () => {
    expect(parseTrackLanguages(null)).toEqual({ audio: null, subtitle: null })
    expect(parseTrackLanguages('not json')).toEqual({ audio: null, subtitle: null })
    expect(parseTrackLanguages('{"audio":3,"subtitle":null}')).toEqual({ audio: null, subtitle: null })
  })
})

describe('preferredTrack', () => {
  const tracks = [track(1, 'eng'), track(2, 'fre', true), track(3, 'fre'), track(4, 'fra')]
  it('picks the first track in the language, a full subtitle before a forced one', () => {
    expect(preferredTrack(tracks, 'en')?.index).toBe(1)
    expect(preferredTrack(tracks, 'fr')?.index).toBe(3)
    expect(preferredTrack([track(2, 'fre', true)], 'fr')?.index).toBe(2)
  })
  it('finds nothing without a preference or a matching track', () => {
    expect(preferredTrack(tracks, null)).toBeNull()
    expect(preferredTrack(tracks, 'de')).toBeNull()
  })
})

describe('withAudio and withSubtitle', () => {
  const start = { audio: 'en', subtitle: 'fr' }
  it('keep audio and subtitles apart', () => {
    expect(withAudio(start, 'jpn')).toEqual({ audio: 'ja', subtitle: 'fr' })
    expect(withSubtitle(start, 'pt-BR')).toEqual({ audio: 'en', subtitle: 'pt' })
  })
  it('remember subtitles off, and ignore a track without a language', () => {
    expect(withSubtitle(start, null)).toEqual({ audio: 'en', subtitle: null })
    expect(withSubtitle(start, 'und')).toEqual(start)
    expect(withAudio(start, undefined)).toEqual(start)
  })
  it('save nothing when storage is unavailable', () => {
    // The tests run without a localStorage, like a browser that blocks it.
    expect(() => rememberAudio('fre')).not.toThrow()
    expect(loadTrackLanguages()).toEqual({ audio: null, subtitle: null })
  })
})
