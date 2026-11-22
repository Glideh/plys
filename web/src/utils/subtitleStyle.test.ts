import { describe, expect, it } from 'vitest'
import { DEFAULT_SUBTITLE_STYLE, loadSubtitleStyle, parseSubtitleStyle, saveSubtitleStyle, subtitleClasses } from './subtitleStyle'

describe('parseSubtitleStyle', () => {
  it('reads a stored style', () => {
    expect(parseSubtitleStyle('{"size":"xlarge","background":"box"}')).toEqual({ size: 'xlarge', background: 'box' })
  })
  it('falls back to the default for missing, unknown or unreadable values', () => {
    expect(parseSubtitleStyle(null)).toEqual(DEFAULT_SUBTITLE_STYLE)
    expect(parseSubtitleStyle('not json')).toEqual(DEFAULT_SUBTITLE_STYLE)
    expect(parseSubtitleStyle('{"size":"huge","background":"box"}')).toEqual({ size: 'medium', background: 'box' })
    expect(parseSubtitleStyle('{"size":"small"}')).toEqual({ size: 'small', background: 'outline' })
  })
})

describe('subtitle style storage', () => {
  it('uses the default and saves nothing when storage is unavailable', () => {
    // The tests run without a localStorage, like a browser that blocks it.
    expect(() => saveSubtitleStyle({ size: 'large', background: 'box' })).not.toThrow()
    expect(loadSubtitleStyle()).toEqual(DEFAULT_SUBTITLE_STYLE)
  })
})

describe('subtitleClasses', () => {
  it('names the size and the background', () => {
    expect(subtitleClasses({ size: 'small', background: 'outline' })).toEqual(['subs-small', 'subs-outline'])
  })
})
