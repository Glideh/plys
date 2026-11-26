<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminPlaybackApi, type AdminPlaybackSession } from '@/api/adminPlayback'
import ErrorAlert from '@/components/ErrorAlert.vue'
import UserAvatar from '@/components/UserAvatar.vue'
import { useErrorMessage } from '@/composables/useErrorMessage'

const POLL_MS = 5000
const { t, locale } = useI18n()
const errorMessage = useErrorMessage()
const sessions = ref<AdminPlaybackSession[]>([])
const loading = ref(true)
const error = ref('')
const controller = new AbortController()
let timer: ReturnType<typeof setTimeout> | undefined

async function refresh() {
  try {
    sessions.value = await adminPlaybackApi.list(controller.signal)
    error.value = ''
  } catch (err) {
    if (!controller.signal.aborted) error.value = errorMessage(err)
  } finally {
    loading.value = false
    if (!controller.signal.aborted) timer = setTimeout(refresh, POLL_MS)
  }
}

function describe(...parts: (string | false | undefined)[]) {
  return parts.filter(Boolean).join(' · ')
}

function dimensions(width?: number, height?: number) {
  return width && height ? `${width}×${height}` : undefined
}

function channels(count?: number) {
  return count ? t('adminPlayback.channels', count) : undefined
}

function codecs(list: string[]) {
  return list.join(', ').toUpperCase() || t('adminPlayback.none')
}

const SUBTITLE_CODECS: Record<string, string> = { hdmv_pgs_subtitle: 'PGS', dvd_subtitle: 'VobSub', dvb_subtitle: 'DVB' }

function bitDepth(pixFmt?: string) {
  const bits = pixFmt?.match(/p(1[02])/)?.[1]
  return bits ? t('adminPlayback.bits', { bits }) : undefined
}

function dynamicRange(transfer?: string) {
  if (transfer === 'smpte2084') return 'HDR PQ'
  if (transfer === 'arib-std-b67') return 'HDR HLG'
  return undefined
}

function speed(value: number) {
  return t('adminPlayback.speed', {
    speed: new Intl.NumberFormat(locale.value, { minimumFractionDigits: 1, maximumFractionDigits: 1 }).format(value),
  })
}

function mode({ plan, source }: AdminPlaybackSession) {
  if (source.video && !plan.copyVideo) return 'video'
  if (source.audio && !plan.copyAudio) return 'audio'
  return 'direct'
}

function columns({ source }: AdminPlaybackSession) {
  return Number(Boolean(source.video)) + Number(Boolean(source.audio)) || 1
}

function pipelineSteps(pipeline: NonNullable<AdminPlaybackSession['videoPipeline']>) {
  return [
    { name: 'decode', device: pipeline.decode },
    ...(pipeline.toneMap ? [{ name: 'toneMap', device: pipeline.toneMap }] : []),
    ...(pipeline.burnSubtitle ? [{ name: 'burnSubtitle', device: pipeline.burnSubtitle }] : []),
    { name: 'scale', device: pipeline.scale },
    { name: 'encode', device: pipeline.encode },
  ]
}

onMounted(refresh)
onUnmounted(() => {
  controller.abort()
  clearTimeout(timer)
})
</script>

<template>
  <section :aria-busy="loading">
    <h2 class="text-lg font-semibold">{{ t('admin.playback') }}</h2>
    <p class="mt-1 mb-6 text-sm text-slate-400">{{ t('adminPlayback.help') }}</p>

    <ErrorAlert v-if="error" :message="error" class="mb-4" />
    <div v-if="loading" class="h-40 animate-pulse rounded-2xl bg-surface-800/60" />
    <p v-else-if="!error && sessions.length === 0" class="rounded-2xl border border-dashed border-white/10 px-6 py-14 text-center text-sm text-slate-400">
      {{ t('adminPlayback.empty') }}
    </p>
    <p v-if="error && sessions.length" class="mb-4 text-sm text-slate-400">{{ t('adminPlayback.stale') }}</p>

    <ul class="space-y-4">
      <li v-for="session in sessions" :key="session.id" class="rounded-2xl border border-white/5 bg-surface-900 p-4 sm:p-5">
        <div class="flex min-w-0 items-center gap-3">
          <UserAvatar :user="session.user" class="size-9 shrink-0" />
          <div class="min-w-0">
            <h3 class="font-semibold wrap-break-word">{{ session.source.title || t('adminPlayback.untitled') }}</h3>
            <p class="text-sm text-slate-400 wrap-break-word">{{ session.user?.displayName || t('adminPlayback.unknownUser') }}</p>
          </div>
        </div>
        <div class="mt-3 flex min-w-0 items-center gap-3 text-sm">
          <span class="shrink-0 rounded-full px-2.5 py-0.5 text-xs font-medium"
            :class="{ direct: 'bg-emerald-500/15 text-emerald-300', audio: 'bg-sky-500/15 text-sky-300', video: 'bg-amber-500/15 text-amber-300' }[mode(session)]">
            {{ t(`adminPlayback.mode.${mode(session)}`) }}
          </span>
          <span v-if="session.process === 'running'" class="min-w-0 truncate tabular-nums"
            :class="session.speed !== undefined && session.speed < 1 ? 'font-semibold text-red-300' : 'text-slate-200'"
            :title="session.speed !== undefined && session.speed < 1 ? t('adminPlayback.slow') : undefined">
            {{ session.speed === undefined ? t('adminPlayback.measuring') : speed(session.speed) }}
          </span>
          <span v-else class="min-w-0 truncate" :class="session.process === 'failed' ? 'text-red-300' : 'text-slate-400'">
            {{ t(`adminPlayback.process.${session.process}`) }}
          </span>
        </div>

        <table class="mt-4 w-full table-fixed text-left text-sm text-slate-300">
          <thead class="text-xs text-slate-400">
            <tr>
              <th v-if="session.source.video" scope="col" class="pb-1 pe-3 font-medium text-slate-100" :class="{ 'w-3/5': session.source.audio }">{{ t('adminPlayback.video') }}</th>
              <th v-if="session.source.audio" scope="col" class="pb-1 font-medium text-slate-100">{{ t('adminPlayback.audio') }}</th>
            </tr>
          </thead>
          <tbody class="border-t border-white/5 align-top wrap-break-word">
            <tr>
              <th scope="rowgroup" :colspan="columns(session)" class="pt-2 text-xs font-normal text-slate-400">{{ t('adminPlayback.source') }}</th>
            </tr>
            <tr>
              <td v-if="session.source.video" class="pt-0.5 pb-2 pe-3">
                {{ describe(session.source.video.codec.toUpperCase(), dimensions(session.source.video.width, session.source.video.height), dynamicRange(session.source.video.colorTransfer), bitDepth(session.source.video.pixFmt)) }}
              </td>
              <td v-if="session.source.audio" class="pt-0.5 pb-2">{{ describe(session.source.audio.codec.toUpperCase(), channels(session.source.audio.channels)) }}</td>
            </tr>
          </tbody>
          <tbody class="border-t border-white/5 align-top wrap-break-word">
            <tr>
              <th scope="rowgroup" :colspan="columns(session)" class="pt-2 text-xs font-normal text-slate-400">{{ t('adminPlayback.client') }}</th>
            </tr>
            <tr>
              <td v-if="session.source.video" class="pt-0.5 pb-2 pe-3">
                {{ describe(codecs(session.caps.videoCodecs), session.caps.maxHeight ? `≤ ${session.caps.maxHeight}p` : t('adminPlayback.noLimit'), session.caps.hdr ? 'HDR' : t('adminPlayback.sdrOnly')) }}
              </td>
              <td v-if="session.source.audio" class="pt-0.5 pb-2">
                {{ describe(codecs(session.caps.audioCodecs), session.caps.preferredAudioCodec && t('adminPlayback.preferred', { codec: session.caps.preferredAudioCodec.toUpperCase() })) }}
              </td>
            </tr>
          </tbody>
          <tbody class="border-t border-white/5 align-top wrap-break-word">
            <tr>
              <th scope="rowgroup" :colspan="columns(session)" class="pt-2 text-xs font-normal text-slate-400">{{ t('adminPlayback.output') }}</th>
            </tr>
            <tr>
              <td v-if="session.source.video" class="pt-0.5 pb-2 pe-3">
                <p :class="session.plan.copyVideo ? 'text-emerald-300' : 'text-amber-300'">
                  {{ session.plan.copyVideo ? t('adminPlayback.copy') : describe('H.264', dimensions(session.plan.width, session.plan.height), session.plan.tonemap && t('adminPlayback.tonemap')) }}
                </p>
                <p v-if="session.plan.burnSubtitle && session.source.subtitle" class="text-amber-300">
                  {{ t('adminPlayback.burnedSubtitle', { codec: SUBTITLE_CODECS[session.source.subtitle.codec] ?? session.source.subtitle.codec.toUpperCase() }) }}
                </p>
                <ul v-if="session.videoPipeline" class="mt-1 text-xs text-slate-400">
                  <li v-for="step in pipelineSteps(session.videoPipeline)" :key="step.name">
                    {{ t('adminPlayback.step', { step: t(`adminPlayback.steps.${step.name}`), device: t(`adminPlayback.devices.${step.device}`) }) }}
                  </li>
                </ul>
              </td>
              <td v-if="session.source.audio" class="pt-0.5 pb-2" :class="session.plan.copyAudio ? 'text-emerald-300' : 'text-sky-300'">
                {{ session.plan.copyAudio ? t('adminPlayback.copy') : describe(session.outputAudioCodec?.toUpperCase(), channels(session.plan.audioChannels)) }}
              </td>
            </tr>
          </tbody>
        </table>

        <p v-if="session.reasons.length" class="mt-3 text-sm text-slate-400">
          <span class="text-slate-300">{{ t('adminPlayback.reason') }}</span>
          {{ session.reasons.map((reason) => t(`adminPlayback.reasons.${reason}`)).join(' · ') }}
        </p>
      </li>
    </ul>
  </section>
</template>
