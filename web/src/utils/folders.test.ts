import { describe, expect, it } from 'vitest'
import { filterFolders } from './folders'

const entries = [{ name: 'Films' }, { name: 'Séries' }, { name: 'Documentaires' }]

describe('filterFolders', () => {
  it('keeps every entry for an empty or blank query', () => {
    expect(filterFolders(entries, '')).toEqual(entries)
    expect(filterFolders(entries, '   ')).toEqual(entries)
  })

  it('matches a substring, ignoring case and accents', () => {
    expect(filterFolders(entries, 'SERIE')).toEqual([{ name: 'Séries' }])
    expect(filterFolders(entries, 'menta')).toEqual([{ name: 'Documentaires' }])
  })

  it('returns nothing when no name matches', () => {
    expect(filterFolders(entries, 'music')).toEqual([])
  })
})
