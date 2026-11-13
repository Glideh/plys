import { describe, expect, it } from 'vitest'
import { scrollTarget } from './scroll'

describe('scrollTarget', () => {
  it('restores the saved position on back/forward', () => {
    expect(scrollTarget({ path: '/kinds/movie' }, { path: '/items/1' }, { left: 0, top: 1200 })).toEqual({ left: 0, top: 1200 })
  })

  it('keeps the scroll when only the query changes', () => {
    expect(scrollTarget({ path: '/kinds/movie' }, { path: '/kinds/movie' }, null)).toBe(false)
  })

  it('goes to the top of a new page', () => {
    expect(scrollTarget({ path: '/items/2' }, { path: '/items/1' }, null)).toEqual({ left: 0, top: 0 })
  })
})
