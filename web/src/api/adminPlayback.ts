import { api, type User } from './client'
import type { ClientCaps } from './playback'

interface SourceTrack {
  codec: string
  profile?: string
  language?: string
  channels?: number
  width?: number
  height?: number
  pixFmt?: string
  colorTransfer?: string
}

export type PlaybackReason =
  | 'image_subtitle'
  | 'video_container_codec'
  | 'video_client_codec'
  | 'h264_high_bit_depth'
  | 'hdr_unsupported'
  | 'height_limit'
  | 'audio_container_codec'
  | 'audio_client_codec'
  | 'preferred_surround'

type ProcessingDevice = 'cpu' | 'qsv' | 'vaapi' | 'vulkan'

export interface AdminPlaybackSession {
  id: string
  user: Pick<User, 'id' | 'displayName' | 'avatarUpdatedAt'> | null
  source: {
    title: string
    container: string
    video?: SourceTrack
    audio?: SourceTrack
    subtitle?: SourceTrack
    externalSubtitle: boolean
  }
  caps: ClientCaps
  plan: {
    copyVideo: boolean
    copyAudio: boolean
    tonemap: boolean
    burnSubtitle: boolean
    width?: number
    height?: number
    audioChannels?: number
  }
  reasons: PlaybackReason[]
  videoAccel?: 'qsv' | 'vaapi' | 'none'
  videoPipeline?: {
    decode: ProcessingDevice
    toneMap?: ProcessingDevice
    burnSubtitle?: ProcessingDevice
    scale: ProcessingDevice
    encode: ProcessingDevice
  }
  outputAudioCodec?: string
  process: 'running' | 'buffered' | 'finished' | 'failed'
  /** Media time FFmpeg produces per second; absent while unmeasured or not running. */
  speed?: number
  startedAt: string
  lastAccess: string
}

export const adminPlaybackApi = {
  list: (signal: AbortSignal) =>
    api<{ items: AdminPlaybackSession[] }>('/admin/playback/sessions', { signal }).then((result) => result.items),
}
