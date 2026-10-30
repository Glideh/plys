import { describe, expect, it } from 'vitest'
import { safeRedirect } from './redirect'

describe('safeRedirect', () => {
  it.each([
    ['/libraries/1', '/libraries/1'],
    ['//evil.example', '/'],
    ['/\\evil.example', '/'],
    ['https://evil.example', '/'],
    [undefined, '/'],
    [['/a'], '/'],
  ])('%j -> %s', (input, expected) => {
    expect(safeRedirect(input)).toBe(expected)
  })
})
