import { api } from './client'

/** What this browser can decode; the server copies the streams it can play and transcodes the others. */
export interface ClientCaps {
  videoCodecs: string[]
  audioCodecs: string[]
  hdr: boolean
  maxHeight: number
}

export interface PlaybackSession {
  id: string
  playlistUrl: string
  /** Position of the stream start in the file, in seconds. */
  start: number
  duration: number
  copyVideo: boolean
  copyAudio: boolean
  accel: string
  audioIndex: number | null
  subtitleIndex: number | null
  subtitleUrl: string | null
  burnedSubtitle: boolean
}

export interface StartPlaybackBody {
  versionId: string
  audioIndex?: number | null
  subtitleIndex?: number | null
  start: number
  caps: ClientCaps
}

export const playbackApi = {
  start: (body: StartPlaybackBody) => api<PlaybackSession>('/playback/sessions', { method: 'POST', body }),
  stop: (id: string, keepalive = false) => api<void>(`/playback/sessions/${id}`, { method: 'DELETE', keepalive }),
}
