import type { ClientCaps } from '@/api/playback'

// Representative profiles: high-end levels, so a browser limited to lower ones is not wrongly trusted.
const videoProbes: [string, string][] = [
  ['h264', 'video/mp4; codecs="avc1.640033"'],
  ['hevc', 'video/mp4; codecs="hvc1.2.4.L153.B0"'],
  ['av1', 'video/mp4; codecs="av01.0.08M.08"'],
]
const audioProbes: [string, string][] = [
  ['aac', 'audio/mp4; codecs="mp4a.40.2"'],
  ['mp3', 'audio/mp4; codecs="mp3"'],
  ['opus', 'audio/mp4; codecs="opus"'],
  ['flac', 'audio/mp4; codecs="flac"'],
  ['ac3', 'audio/mp4; codecs="ac-3"'],
  ['eac3', 'audio/mp4; codecs="ec-3"'],
]

type MediaSourceLike = { isTypeSupported(type: string): boolean }

function mediaSource(): MediaSourceLike | null {
  const w = window as unknown as { ManagedMediaSource?: MediaSourceLike; MediaSource?: MediaSourceLike }
  return w.ManagedMediaSource ?? w.MediaSource ?? null
}

function supports(type: string): boolean {
  const ms = mediaSource()
  if (ms) return ms.isTypeSupported(type)
  // iPhone Safari has no MSE: native HLS decides.
  return document.createElement('video').canPlayType(type) !== ''
}

const FALLBACK_KEY = 'plys.audioFallback'

/** True once a decoder failure showed that the browser claims codecs it cannot play. */
export function audioFallbackActive(): boolean {
  return localStorage.getItem(FALLBACK_KEY) === '1'
}

export function enableAudioFallback() {
  localStorage.setItem(FALLBACK_KEY, '1')
}

export function detectCaps(): ClientCaps {
  // isTypeSupported can be wrong (embedded browsers): after a failure only FLAC is announced.
  const audio = audioFallbackActive() ? ['flac'] : audioProbes.filter(([, t]) => supports(t)).map(([c]) => c)
  return {
    videoCodecs: videoProbes.filter(([, t]) => supports(t)).map(([c]) => c),
    audioCodecs: audio,
    hdr: window.matchMedia('(dynamic-range: high)').matches,
    maxHeight: 0,
  }
}
