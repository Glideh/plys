export function formatClock(totalSeconds: number): string {
  const s = Math.max(0, Math.floor(totalSeconds))
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = String(s % 60).padStart(2, '0')
  return h ? `${h}:${String(m).padStart(2, '0')}:${sec}` : `${m}:${sec}`
}

/**
 * A seek stays inside the running stream when the target was already produced; otherwise the server must
 * start a new stream at the target. `producedEnd` is relative to the stream start.
 */
export function canSeekInStream(target: number, streamStart: number, producedEnd: number, margin = 2): boolean {
  const local = target - streamStart
  return local >= 0 && local <= producedEnd - margin
}
