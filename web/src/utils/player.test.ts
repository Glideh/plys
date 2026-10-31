import { describe, expect, it } from 'vitest'
import { canSeekInStream, formatClock } from './player'

describe('formatClock', () => {
  it('formats minutes and hours', () => {
    expect(formatClock(0)).toBe('0:00')
    expect(formatClock(65.9)).toBe('1:05')
    expect(formatClock(3600)).toBe('1:00:00')
    expect(formatClock(7384)).toBe('2:03:04')
    expect(formatClock(-5)).toBe('0:00')
  })
})

describe('canSeekInStream', () => {
  it('accepts targets inside the produced range', () => {
    expect(canSeekInStream(700, 600, 300)).toBe(true)
  })
  it('rejects targets before the stream start', () => {
    expect(canSeekInStream(500, 600, 300)).toBe(false)
  })
  it('rejects targets beyond what was produced', () => {
    expect(canSeekInStream(900, 600, 300)).toBe(false)
    expect(canSeekInStream(899, 600, 300)).toBe(false)
  })
})
