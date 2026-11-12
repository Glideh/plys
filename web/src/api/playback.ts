import { api } from './client'
import { absoluteUrl, peerApi, type ServerRef } from './federation'

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
  externalSubtitleId: string | null
  subtitleUrl: string | null
  burnedSubtitle: boolean
}

export interface StartPlaybackBody {
  versionId: string
  audioIndex?: number | null
  subtitleIndex?: number | null
  /** A downloaded subtitle, instead of a track of the file. */
  externalSubtitleId?: string | null
  start: number
  caps: ClientCaps
}

export const playbackApi = {
  start: (body: StartPlaybackBody) => api<PlaybackSession>('/playback/sessions', { method: 'POST', body }),
  stop: (id: string, keepalive = false) => api<void>(`/playback/sessions/${id}`, { method: 'DELETE', keepalive }),
}

export interface PlaybackTransport {
  start: (body: StartPlaybackBody) => Promise<PlaybackSession>
  stop: (id: string, keepalive?: boolean) => Promise<void>
}

/** Sessions run on the server holding the file: this one, or the paired server named by `server`. */
export function playbackTransport(server: ServerRef | null): PlaybackTransport {
  if (!server?.baseUrl) return playbackApi
  return {
    start: async (body) => {
      const s = await peerApi<PlaybackSession>(server, '/playback/sessions', { method: 'POST', body })
      return { ...s, playlistUrl: absoluteUrl(server, s.playlistUrl) ?? s.playlistUrl, subtitleUrl: absoluteUrl(server, s.subtitleUrl) }
    },
    stop: (id, keepalive) => peerApi<void>(server, `/playback/sessions/${id}`, { method: 'DELETE', keepalive }),
  }
}
