import { describe, expect, it } from 'vitest'
import { encodeQr, formatBits, rsDivisor, rsRemainder, versionBits } from './qr'

describe('qr', () => {
  it('computes Reed-Solomon codewords', () => {
    // "HELLO WORLD" as version 1-M, from the worked example of the specification tutorials.
    const data = [32, 91, 11, 120, 209, 114, 220, 77, 67, 64, 236, 17, 236, 17, 236, 17]
    expect(rsRemainder(data, rsDivisor(10))).toEqual([196, 35, 39, 119, 235, 215, 231, 226, 93, 23])
  })

  it('computes format and version information', () => {
    const expected = [
      '101010000010010',
      '101000100100101',
      '101111001111100',
      '101101101001011',
      '100010111111001',
      '100000011001110',
      '100111110010111',
      '100101010100000',
    ]
    expected.forEach((bits, mask) => expect(formatBits(mask)).toBe(parseInt(bits, 2)))
    expect(versionBits(7)).toBe(parseInt('000111110010010100', 2))
  })

  it('picks the smallest version that fits', () => {
    expect(encodeQr('a'.repeat(14)).size).toBe(21)
    expect(encodeQr('a'.repeat(15)).size).toBe(25)
    // An invitation link: public URL, path and a 43-character token.
    expect(encodeQr(`https://plys.example.com/invite#${'x'.repeat(43)}`).size).toBe(37)
    expect(() => encodeQr('a'.repeat(3000))).toThrow()
  })

  it('draws the finder patterns and a matching format information twice', () => {
    const { size, modules } = encodeQr('https://plys.example/invite#token')
    const finder = (x0: number, y0: number) => {
      for (let dy = 0; dy < 7; dy++) {
        for (let dx = 0; dx < 7; dx++) {
          const ring = Math.max(Math.abs(dx - 3), Math.abs(dy - 3))
          expect(modules[y0 + dy]![x0 + dx]).toBe(ring !== 2)
        }
      }
    }
    finder(0, 0)
    finder(size - 7, 0)
    finder(0, size - 7)

    const at = (x: number, y: number): number => (modules[y]![x] ? 1 : 0)
    const first = [0, 1, 2, 3, 4, 5].map((i) => at(8, i)).concat([at(8, 7), at(8, 8), at(7, 8)], [9, 10, 11, 12, 13, 14].map((i) => at(14 - i, 8)))
    const second = [0, 1, 2, 3, 4, 5, 6, 7].map((i) => at(size - 1 - i, 8)).concat([8, 9, 10, 11, 12, 13, 14].map((i) => at(8, size - 15 + i)))
    expect(second).toEqual(first)
    const value = first.reduce<number>((acc, b, i) => acc | (b << i), 0)
    expect([0, 1, 2, 3, 4, 5, 6, 7].map(formatBits)).toContain(value)
    expect(at(8, size - 8)).toBe(1)
  })
})
