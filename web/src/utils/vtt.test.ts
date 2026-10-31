import { describe, expect, it } from 'vitest'
import { parseVTT } from './vtt'

describe('parseVTT', () => {
  const file = 'WEBVTT\n\n1\n00:01.000 --> 00:03.500 line:90%\nBonjour\n\n01:02:03.250 --> 01:02:05.000\nLigne 1\nLigne 2\n\n'

  it('reads short and long timestamps, cue ids and settings', () => {
    expect(parseVTT(file, true)).toEqual([
      { start: 1, end: 3.5, text: 'Bonjour' },
      { start: 3723.25, end: 3725, text: 'Ligne 1\nLigne 2' },
    ])
  })

  it('ignores a cue cut in the middle while the file is being written', () => {
    const partial = 'WEBVTT\n\n00:01.000 --> 00:02.000\nUn\n\n00:03.000 --> 00:04.000\nDeu'
    expect(parseVTT(partial, false).map((c) => c.text)).toEqual(['Un'])
    expect(parseVTT(partial, true).map((c) => c.text)).toEqual(['Un', 'Deu'])
  })

  it('keeps every cue when the live file ends on a blank line', () => {
    expect(parseVTT(file, false)).toHaveLength(2)
  })

  it('skips blocks without text or timing', () => {
    expect(parseVTT('WEBVTT\n\nNOTE commentaire\n\n00:01.000 --> 00:02.000\n\n', true)).toEqual([])
  })
})
