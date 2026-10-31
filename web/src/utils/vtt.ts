export interface VttCue {
  start: number
  end: number
  text: string
}

const timing = /^(?:(\d+):)?(\d{1,2}):(\d{2})[.,](\d{3})$/

function seconds(value: string): number | null {
  const m = timing.exec(value)
  if (!m) return null
  return Number(m[1] ?? 0) * 3600 + Number(m[2]) * 60 + Number(m[3]) + Number(m[4]) / 1000
}

/**
 * Parses WebVTT. A file still being written can end in the middle of a cue, so unless `complete` is set
 * the last block is ignored when the text does not end with a blank line.
 */
export function parseVTT(source: string, complete: boolean): VttCue[] {
  const text = source.replace(/\r\n?/g, '\n')
  const blocks = text.split(/\n{2,}/)
  if (!complete && !text.endsWith('\n\n')) blocks.pop()

  const cues: VttCue[] = []
  for (const block of blocks) {
    const lines = block.split('\n')
    const at = lines.findIndex((l) => l.includes('-->'))
    if (at < 0) continue
    const [from, rest] = lines[at]!.split('-->')
    const start = seconds(from!.trim())
    const end = seconds((rest ?? '').trim().split(/\s+/)[0] ?? '')
    const body = lines.slice(at + 1).join('\n').trim()
    if (start === null || end === null || !body) continue
    cues.push({ start, end, text: body })
  }
  return cues
}
