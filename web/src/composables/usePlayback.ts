import Hls from 'hls.js'
import { onBeforeUnmount, ref, shallowRef, type Ref } from 'vue'
import { playbackApi, type PlaybackSession } from '@/api/playback'
import { audioFallbackActive, detectCaps, enableAudioFallback } from '@/utils/caps'
import { canSeekInStream } from '@/utils/player'
import { parseVTT } from '@/utils/vtt'

export type PlayerStatus = 'loading' | 'ready' | 'error'
export type SubtitleLoad = 'idle' | 'loading' | 'ready' | 'failed'

interface Options {
  versionId: () => string
  /** Selection used for the first session. */
  initial?: { start?: number; audioIndex?: number | null; subtitleIndex?: number | null }
  onEnded?: () => void
}

const VOLUME_KEY = 'plys.volume'
const MAX_FAILURES = 3
const FAILURE_WINDOW_MS = 60_000

/**
 * Drives a video element from a server playback session. The server produces a stream that starts at
 * `session.start`, so every position shown to the user is `session.start + video.currentTime`.
 */
export function usePlayback(video: Ref<HTMLVideoElement | null>, options: Options) {
  const session = shallowRef<PlaybackSession | null>(null)
  const status = ref<PlayerStatus>('loading')
  const error = ref<unknown>(null)
  const position = ref(options.initial?.start ?? 0)
  const buffered = ref(0)
  const paused = ref(true)
  const waiting = ref(true)
  const volume = ref(1)
  const muted = ref(false)
  const audioIndex = ref<number | null>(options.initial?.audioIndex ?? null)
  const subtitleIndex = ref<number | null>(options.initial?.subtitleIndex ?? null)
  const subtitleLoad = ref<SubtitleLoad>('idle')

  let hls: Hls | null = null
  let seq = 0
  let mediaRecovered = false
  // The user wants playback; applied when a new stream can play, since play() fails before.
  let wantPlay = true
  let switching = true
  let failures: number[] = []
  let track: TextTrack | null = null
  let subtitleAbort: AbortController | null = null
  let subtitleTimer: ReturnType<typeof setTimeout> | undefined

  const duration = () => session.value?.duration ?? 0

  function storedVolume() {
    try {
      const raw = JSON.parse(localStorage.getItem(VOLUME_KEY) ?? 'null') as { volume: number; muted: boolean } | null
      if (raw && raw.volume >= 0 && raw.volume <= 1) return raw
    } catch {
      // Unreadable preference: use the defaults.
    }
    return { volume: 1, muted: false }
  }

  // --- subtitles -----------------------------------------------------------------------------------

  function ensureTrack(el: HTMLVideoElement): TextTrack {
    track ??= el.addTextTrack('subtitles', 'Plys', '')
    return track
  }

  function clearCues() {
    if (track?.cues) for (const cue of Array.from(track.cues)) track.removeCue(cue)
  }

  function stopSubtitles() {
    subtitleAbort?.abort()
    subtitleAbort = null
    clearTimeout(subtitleTimer)
    clearCues()
    subtitleLoad.value = 'idle'
  }

  /** Text subtitles arrive as WebVTT; a file still being produced is fetched again until complete. */
  async function loadSubtitles(s: PlaybackSession, el: HTMLVideoElement) {
    if (!s.subtitleUrl) return
    const url = s.subtitleUrl
    const ctrl = new AbortController()
    subtitleAbort = ctrl
    subtitleLoad.value = 'loading'
    const t = ensureTrack(el)
    t.mode = 'showing'

    const run = async () => {
      try {
        const res = await fetch(url, { signal: ctrl.signal })
        if (res.status === 202) {
          subtitleTimer = setTimeout(run, 3000)
          return
        }
        if (!res.ok) throw new Error(`subtitles: ${res.status}`)
        const complete = res.headers.get('X-Subtitles-Complete') !== 'false'
        const cues = parseVTT(await res.text(), complete)
        if (ctrl.signal.aborted) return
        if (complete) clearCues()
        // While the file grows, only the new cues are added so the one on screen does not flicker.
        for (const c of cues.slice(complete ? 0 : (t.cues?.length ?? 0))) t.addCue(new VTTCue(c.start, c.end, c.text))
        subtitleLoad.value = 'ready'
        if (!complete) subtitleTimer = setTimeout(run, 10_000)
      } catch {
        if (!ctrl.signal.aborted) subtitleLoad.value = 'failed'
      }
    }
    await run()
  }

  // --- stream --------------------------------------------------------------------------------------

  function detach() {
    stopSubtitles()
    hls?.destroy()
    hls = null
    const el = video.value
    if (el) {
      el.removeAttribute('src')
      el.load()
    }
  }

  /** End of the produced stream, relative to the stream start. */
  function producedEnd(): number {
    const el = video.value
    if (hls) {
      const level = hls.levels[Math.max(hls.currentLevel, 0)]
      return level?.details?.totalduration ?? 0
    }
    return el && el.seekable.length ? el.seekable.end(el.seekable.length - 1) : 0
  }

  function fail(err: unknown) {
    seq++
    detach()
    status.value = 'error'
    error.value = err
  }

  function recover() {
    const now = Date.now()
    failures = failures.filter((t) => now - t < FAILURE_WINDOW_MS)
    failures.push(now)
    if (failures.length > MAX_FAILURES) return fail(new Error('playback keeps failing'))
    void open(position.value)
  }

  function attach(s: PlaybackSession, startAt: number) {
    const el = video.value
    if (!el) return
    const offset = Math.max(0, startAt - s.start)
    mediaRecovered = false

    if (Hls.isSupported()) {
      hls = new Hls({ startPosition: offset, autoStartLoad: true, maxBufferLength: 30, backBufferLength: 60 })
      hls.on(Hls.Events.ERROR, (_e, data) => {
        // The browser announced a codec it cannot decode: restart announcing only FLAC.
        if (/audio decoder/i.test(data.error?.message ?? '') && !audioFallbackActive()) {
          enableAudioFallback()
          recover()
          return
        }
        if (!data.fatal) return
        if (data.type === Hls.ErrorTypes.MEDIA_ERROR && !mediaRecovered) {
          mediaRecovered = true
          hls?.recoverMediaError()
          return
        }
        recover()
      })
      hls.loadSource(s.playlistUrl)
      hls.attachMedia(el)
    } else if (el.canPlayType('application/vnd.apple.mpegurl')) {
      el.src = s.playlistUrl
      el.addEventListener('loadedmetadata', () => (el.currentTime = offset), { once: true })
    } else {
      return fail(new Error('HLS is not supported by this browser'))
    }

    if (s.subtitleUrl) void loadSubtitles(s, el)
    else if (track) track.mode = 'disabled'
  }

  /** Starts a server session at a position of the file, replacing the current one. */
  async function open(startAt: number) {
    switching = true
    const my = ++seq
    status.value = 'loading'
    waiting.value = true
    error.value = null
    position.value = startAt
    detach()

    const previous = session.value
    session.value = null
    if (previous) await playbackApi.stop(previous.id).catch(() => undefined)
    if (my !== seq) return

    let s: PlaybackSession
    try {
      s = await playbackApi.start({
        versionId: options.versionId(),
        audioIndex: audioIndex.value,
        subtitleIndex: subtitleIndex.value,
        start: startAt,
        caps: detectCaps(),
      })
    } catch (err) {
      if (my === seq) fail(err)
      return
    }
    if (my !== seq) {
      void playbackApi.stop(s.id).catch(() => undefined)
      return
    }
    session.value = s
    audioIndex.value = s.audioIndex
    attach(s, startAt)
  }

  // --- controls ------------------------------------------------------------------------------------

  function seek(target: number) {
    const el = video.value
    const s = session.value
    if (!el || !s) return
    const to = Math.min(Math.max(target, 0), Math.max(duration() - 1, 0))
    position.value = to
    if (canSeekInStream(to, s.start, producedEnd())) {
      el.currentTime = to - s.start
    } else {
      void open(to)
    }
  }

  const skip = (delta: number) => seek(position.value + delta)

  function retry() {
    failures = []
    void open(position.value)
  }

  function togglePlay() {
    const el = video.value
    if (!el) return
    if (el.paused) el.play().catch(() => undefined)
    else el.pause()
  }

  function setAudio(index: number) {
    if (index === audioIndex.value) return
    audioIndex.value = index
    void open(position.value)
  }

  /** null turns subtitles off. Image subtitles are burned into the picture, so they need a new stream. */
  function setSubtitle(index: number | null) {
    if (index === subtitleIndex.value) return
    const s = session.value
    subtitleIndex.value = index
    if (index === null && s && !s.burnedSubtitle) {
      if (track) track.mode = 'disabled'
      return
    }
    if (index !== null && s && index === s.subtitleIndex && !s.burnedSubtitle && track) {
      track.mode = 'showing'
      return
    }
    void open(position.value)
  }

  function setVolume(v: number) {
    const el = video.value
    if (!el) return
    el.volume = Math.min(Math.max(v, 0), 1)
    el.muted = el.volume === 0
  }

  function toggleMute() {
    const el = video.value
    if (el) el.muted = !el.muted
  }

  // --- element events ------------------------------------------------------------------------------

  const handlers: Record<string, () => void> = {
    timeupdate() {
      const el = video.value
      const s = session.value
      if (!el || !s) return
      position.value = s.start + el.currentTime
      updateBuffered()
    },
    progress: updateBuffered,
    play() {
      paused.value = false
      wantPlay = true
    },
    pause() {
      paused.value = true
      // Reloading the element also pauses it, which is not the user's choice.
      if (!switching) wantPlay = false
    },
    waiting: () => (waiting.value = true),
    seeking: () => (waiting.value = true),
    playing() {
      waiting.value = false
      status.value = 'ready'
    },
    canplay() {
      switching = false
      if (wantPlay && video.value?.paused) video.value.play().catch(() => undefined)
      waiting.value = false
      status.value = 'ready'
    },
    volumechange() {
      const el = video.value
      if (!el) return
      volume.value = el.volume
      muted.value = el.muted
      localStorage.setItem(VOLUME_KEY, JSON.stringify({ volume: el.volume, muted: el.muted }))
    },
    ended: () => options.onEnded?.(),
    error() {
      // hls.js reports its own errors; this one comes from native HLS playback.
      if (!hls && session.value && status.value !== 'loading') recover()
    },
  }

  function updateBuffered() {
    const el = video.value
    const s = session.value
    if (!el || !s) return
    for (let i = 0; i < el.buffered.length; i++) {
      if (el.buffered.start(i) <= el.currentTime + 0.5 && el.buffered.end(i) >= el.currentTime) {
        buffered.value = s.start + el.buffered.end(i)
        return
      }
    }
    buffered.value = position.value
  }

  function onPageHide() {
    if (session.value) void playbackApi.stop(session.value.id, true).catch(() => undefined)
  }

  /** Binds the element and starts the first session. */
  function init() {
    const el = video.value
    if (!el) return
    for (const [name, fn] of Object.entries(handlers)) el.addEventListener(name, fn)
    window.addEventListener('pagehide', onPageHide)
    const saved = storedVolume()
    el.volume = saved.volume
    el.muted = saved.muted
    void open(options.initial?.start ?? 0)
  }

  function destroy() {
    seq++
    const el = video.value
    if (el) for (const [name, fn] of Object.entries(handlers)) el.removeEventListener(name, fn)
    window.removeEventListener('pagehide', onPageHide)
    detach()
    if (session.value) void playbackApi.stop(session.value.id).catch(() => undefined)
    session.value = null
  }

  onBeforeUnmount(destroy)

  return {
    session, status, error, position, buffered, paused, waiting, volume, muted, audioIndex, subtitleIndex, subtitleLoad,
    duration, init, seek, skip, retry, togglePlay, setAudio, setSubtitle, setVolume, toggleMute,
  }
}
