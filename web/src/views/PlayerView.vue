<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, useTemplateRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { catalogApi, type ItemDetail, type Track } from '@/api/catalog'
import BaseButton from '@/components/BaseButton.vue'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { usePlayback } from '@/composables/usePlayback'
import { channelLayout, languageName } from '@/utils/format'
import { formatClock } from '@/utils/player'

const props = defineProps<{ id: string }>()
const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const errorMessage = useErrorMessage()

const item = ref<ItemDetail | null>(null)
const loadError = ref('')

onMounted(async () => {
  try {
    item.value = await catalogApi.get(props.id)
  } catch (err) {
    loadError.value = errorMessage(err)
    return
  }
  if (!version.value) loadError.value = t('player.unavailable')
  else player.init()
})

const version = computed(() => {
  const wanted = typeof route.query.v === 'string' ? route.query.v : ''
  return item.value?.versions.find((v) => v.id === wanted) ?? item.value?.versions[0] ?? null
})

const title = computed(() => {
  const it = item.value
  if (!it) return ''
  if (it.kind === 'episode') {
    const code = `S${String(it.seasonNumber ?? 0).padStart(2, '0')}E${String(it.episodeNumber ?? 0).padStart(2, '0')}`
    return [it.ancestors[0]?.title, code, it.title].filter(Boolean).join(' · ')
  }
  if (it.kind === 'part') return [it.ancestors[0]?.title, t('catalog.part', { n: it.partNumber })].filter(Boolean).join(' · ')
  return it.title
})

const startAt = Number(route.query.t) > 0 ? Number(route.query.t) : 0

const videoEl = useTemplateRef<HTMLVideoElement>('video')
const stage = useTemplateRef<HTMLElement>('stage')

const player = usePlayback(videoEl, {
  versionId: () => version.value?.id ?? '',
  initial: { start: startAt },
})
const { session, status, position, buffered, paused, waiting, volume, muted, audioIndex, subtitleIndex, subtitleLoad } = player

const total = computed(() => session.value?.duration || version.value?.durationSeconds || 0)

// --- seek bar -------------------------------------------------------------------------------------

const dragging = ref<number | null>(null)
const shown = computed(() => dragging.value ?? position.value)
const pct = (v: number) => (total.value ? Math.min(100, Math.max(0, (v / total.value) * 100)) : 0)

function onSeekInput(e: Event) {
  dragging.value = Number((e.target as HTMLInputElement).value)
}
function onSeekChange(e: Event) {
  dragging.value = null
  player.seek(Number((e.target as HTMLInputElement).value))
}

// --- controls visibility --------------------------------------------------------------------------

const menu = ref<'audio' | 'subtitles' | null>(null)
const awake = ref(true)
let hideTimer: ReturnType<typeof setTimeout> | undefined

const controlsVisible = computed(() => awake.value || paused.value || menu.value !== null || status.value !== 'ready')

function wake() {
  awake.value = true
  clearTimeout(hideTimer)
  hideTimer = setTimeout(() => (awake.value = false), 3000)
}
onMounted(wake)
onBeforeUnmount(() => clearTimeout(hideTimer))

function toggleMenu(name: 'audio' | 'subtitles') {
  menu.value = menu.value === name ? null : name
}

// --- fullscreen -----------------------------------------------------------------------------------

const fullscreen = ref(false)
const onFullscreenChange = () => (fullscreen.value = document.fullscreenElement !== null)
onMounted(() => document.addEventListener('fullscreenchange', onFullscreenChange))
onBeforeUnmount(() => document.removeEventListener('fullscreenchange', onFullscreenChange))

function toggleFullscreen() {
  if (document.fullscreenElement) void document.exitFullscreen()
  else void stage.value?.requestFullscreen?.().catch(() => undefined)
}

// --- keyboard -------------------------------------------------------------------------------------

function onKeydown(e: KeyboardEvent) {
  if (e.ctrlKey || e.metaKey || e.altKey) return
  const onButton = (e.target as HTMLElement | null)?.tagName === 'BUTTON'
  wake()
  switch (e.key) {
    case ' ':
      if (onButton) return
      player.togglePlay()
      break
    case 'k':
      player.togglePlay()
      break
    case 'ArrowLeft':
    case 'j':
      player.skip(e.shiftKey ? -30 : -10)
      break
    case 'ArrowRight':
    case 'l':
      player.skip(e.shiftKey ? 30 : 10)
      break
    case 'ArrowUp':
      player.setVolume(volume.value + 0.05)
      break
    case 'ArrowDown':
      player.setVolume(volume.value - 0.05)
      break
    case 'm':
      player.toggleMute()
      break
    case 'f':
      toggleFullscreen()
      break
    case 'Escape':
      if (menu.value) menu.value = null
      else if (!document.fullscreenElement) leave()
      break
    default:
      return
  }
  e.preventDefault()
}
onMounted(() => window.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))

function leave() {
  if (window.history.state?.back) router.back()
  else void router.replace({ name: 'item', params: { id: props.id } })
}

// --- labels ---------------------------------------------------------------------------------------

function trackLabel(tr: Track): string {
  return [languageName(tr.language, locale.value) ?? tr.title ?? '?', tr.codec.toUpperCase(), channelLayout(tr.channels), tr.forced ? t('catalog.forced') : null]
    .filter(Boolean)
    .join(' · ')
}

const mode = computed(() => {
  const s = session.value
  if (!s) return ''
  if (s.copyVideo && s.copyAudio) return t('player.direct')
  if (s.copyVideo) return t('player.audioConverted')
  const accel = s.accel === 'none' ? t('player.cpu') : s.accel.toUpperCase()
  return t('player.transcoding', { accel }) + (s.burnedSubtitle ? ` · ${t('player.burned')}` : '')
})

const subtitleNote = computed(() => {
  if (subtitleLoad.value === 'loading') return t('player.subtitlesLoading')
  if (subtitleLoad.value === 'failed') return t('player.subtitlesFailed')
  return ''
})

const failure = computed(() => (loadError.value || (status.value === 'error' ? errorMessage(player.error.value) : '')))

function pick(kind: 'audio' | 'subtitles', index: number | null) {
  menu.value = null
  if (kind === 'audio') player.setAudio(index as number)
  else player.setSubtitle(index)
}
</script>

<template>
  <div
    ref="stage"
    class="fixed inset-0 overflow-hidden bg-black text-white select-none"
    :class="{ 'cursor-none': !controlsVisible }"
    @mousemove="wake"
    @touchstart.passive="wake"
  >
    <video ref="video" class="size-full object-contain" playsinline @click="player.togglePlay" @dblclick="toggleFullscreen" />

    <div v-if="!failure && (status === 'loading' || waiting)" class="pointer-events-none absolute inset-0 grid place-items-center">
      <span class="size-14 animate-spin rounded-full border-4 border-white/20 border-t-accent-400" role="status" />
    </div>

    <div v-if="failure" class="absolute inset-0 grid place-items-center bg-black/80 p-6">
      <div class="max-w-md text-center">
        <p class="text-lg">{{ failure }}</p>
        <div class="mt-5 flex justify-center gap-3">
          <BaseButton @click="leave">{{ t('player.back') }}</BaseButton>
          <BaseButton v-if="!loadError" variant="primary" @click="player.retry">{{ t('player.retry') }}</BaseButton>
        </div>
      </div>
    </div>

    <div
      class="pointer-events-none absolute inset-0 flex flex-col justify-between transition-opacity duration-300"
      :class="controlsVisible ? 'opacity-100' : 'opacity-0'"
    >
      <header class="pointer-events-auto flex items-center gap-3 bg-gradient-to-b from-black/80 to-transparent p-4 pb-10">
        <button type="button" class="rounded-full p-2 hover:bg-white/15" :aria-label="t('player.back')" @click="leave">
          <svg viewBox="0 0 24 24" class="size-6" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M15 18l-6-6 6-6" /></svg>
        </button>
        <h1 class="min-w-0 flex-1 truncate text-lg font-semibold">{{ title }}</h1>
        <span v-if="mode" class="rounded-full bg-white/10 px-3 py-1 text-xs text-slate-200">{{ mode }}</span>
      </header>

      <footer class="pointer-events-auto bg-gradient-to-t from-black/85 to-transparent px-4 pt-12 pb-4 sm:px-6">
        <p v-if="subtitleNote" class="mb-2 text-sm text-slate-300">{{ subtitleNote }}</p>

        <div class="relative h-5">
          <div class="absolute inset-x-0 top-1/2 h-1.5 -translate-y-1/2 overflow-hidden rounded-full bg-white/20">
            <div class="absolute inset-y-0 left-0 bg-white/25" :style="{ width: pct(buffered) + '%' }" />
            <div class="absolute inset-y-0 left-0 bg-accent-400" :style="{ width: pct(shown) + '%' }" />
          </div>
          <input
            type="range"
            class="seek absolute inset-0 w-full"
            min="0"
            :max="total"
            step="1"
            :value="shown"
            :disabled="!total"
            :aria-label="t('player.position')"
            @input="onSeekInput"
            @change="onSeekChange"
          />
        </div>

        <div class="mt-1 flex items-center gap-1 sm:gap-2">
          <button type="button" class="rounded-full p-2 hover:bg-white/15" :aria-label="paused ? t('player.play') : t('player.pause')" @click="player.togglePlay">
            <svg v-if="paused" viewBox="0 0 24 24" class="size-7" fill="currentColor"><path d="M8 5v14l11-7z" /></svg>
            <svg v-else viewBox="0 0 24 24" class="size-7" fill="currentColor"><path d="M6 5h4v14H6zM14 5h4v14h-4z" /></svg>
          </button>
          <button type="button" class="rounded-full p-2 text-sm font-semibold hover:bg-white/15" :aria-label="t('player.back10')" @click="player.skip(-10)">
            <svg viewBox="0 0 24 24" class="size-6" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 12a9 9 0 1 0 3-6.7L3 8" /><path d="M3 3v5h5" /><text x="8.2" y="15.5" font-size="7" fill="currentColor" stroke="none" font-weight="700">10</text></svg>
          </button>
          <button type="button" class="rounded-full p-2 hover:bg-white/15" :aria-label="t('player.forward10')" @click="player.skip(10)">
            <svg viewBox="0 0 24 24" class="size-6" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 1 1-3-6.7L21 8" /><path d="M21 3v5h-5" /><text x="8.2" y="15.5" font-size="7" fill="currentColor" stroke="none" font-weight="700">10</text></svg>
          </button>

          <div class="group/vol flex items-center">
            <button type="button" class="rounded-full p-2 hover:bg-white/15" :aria-label="muted ? t('player.unmute') : t('player.mute')" @click="player.toggleMute">
              <svg viewBox="0 0 24 24" class="size-6" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M11 5 6 9H3v6h3l5 4z" fill="currentColor" />
                <template v-if="muted || volume === 0"><path d="M16 9l5 6M21 9l-5 6" /></template>
                <template v-else><path d="M15.5 8.5a5 5 0 0 1 0 7" /><path v-if="volume > 0.5" d="M18.5 5.5a9 9 0 0 1 0 13" /></template>
              </svg>
            </button>
            <input
              type="range"
              class="seek hidden w-20 sm:block"
              min="0"
              max="1"
              step="0.05"
              :value="muted ? 0 : volume"
              :aria-label="t('player.volume')"
              @input="player.setVolume(Number(($event.target as HTMLInputElement).value))"
            />
          </div>

          <span class="ml-1 text-sm tabular-nums text-slate-200">{{ formatClock(shown) }} / {{ formatClock(total) }}</span>
          <span class="flex-1" />

          <div v-if="version && version.audio.length > 1" class="relative">
            <button type="button" class="rounded-full px-3 py-2 text-sm hover:bg-white/15" :aria-expanded="menu === 'audio'" @click="toggleMenu('audio')">
              {{ t('player.audio') }}
            </button>
            <ul v-if="menu === 'audio'" class="absolute right-0 bottom-full mb-2 max-h-80 w-72 overflow-y-auto rounded-xl bg-surface-900/95 p-1 shadow-2xl ring-1 ring-white/10 backdrop-blur" role="menu">
              <li v-for="a in version.audio" :key="a.index" role="none">
                <button type="button" role="menuitemradio" :aria-checked="a.index === audioIndex" class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-sm hover:bg-white/10" @click="pick('audio', a.index)">
                  <span class="w-4 text-accent-400">{{ a.index === audioIndex ? '✓' : '' }}</span>{{ trackLabel(a) }}
                </button>
              </li>
            </ul>
          </div>

          <div v-if="version && version.subtitles.length" class="relative">
            <button type="button" class="rounded-full px-3 py-2 text-sm hover:bg-white/15" :aria-expanded="menu === 'subtitles'" @click="toggleMenu('subtitles')">
              {{ t('player.subtitles') }}
            </button>
            <ul v-if="menu === 'subtitles'" class="absolute right-0 bottom-full mb-2 max-h-80 w-72 overflow-y-auto rounded-xl bg-surface-900/95 p-1 shadow-2xl ring-1 ring-white/10 backdrop-blur" role="menu">
              <li role="none">
                <button type="button" role="menuitemradio" :aria-checked="subtitleIndex === null" class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-sm hover:bg-white/10" @click="pick('subtitles', null)">
                  <span class="w-4 text-accent-400">{{ subtitleIndex === null ? '✓' : '' }}</span>{{ t('player.subtitlesOff') }}
                </button>
              </li>
              <li v-for="s in version.subtitles" :key="s.index" role="none">
                <button type="button" role="menuitemradio" :aria-checked="s.index === subtitleIndex" class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-sm hover:bg-white/10" @click="pick('subtitles', s.index)">
                  <span class="w-4 text-accent-400">{{ s.index === subtitleIndex ? '✓' : '' }}</span>{{ trackLabel(s) }}
                </button>
              </li>
            </ul>
          </div>

          <button type="button" class="rounded-full p-2 hover:bg-white/15" :aria-label="fullscreen ? t('player.exitFullscreen') : t('player.fullscreen')" @click="toggleFullscreen">
            <svg viewBox="0 0 24 24" class="size-6" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path v-if="!fullscreen" d="M8 3H5a2 2 0 0 0-2 2v3M21 8V5a2 2 0 0 0-2-2h-3M3 16v3a2 2 0 0 0 2 2h3M16 21h3a2 2 0 0 0 2-2v-3" />
              <path v-else d="M8 3v3a2 2 0 0 1-2 2H3M21 8h-3a2 2 0 0 1-2-2V3M3 16h3a2 2 0 0 1 2 2v3M16 21v-3a2 2 0 0 1 2-2h3" />
            </svg>
          </button>
        </div>
      </footer>
    </div>
  </div>
</template>

<style scoped>
.seek {
  appearance: none;
  background: transparent;
  height: 1.25rem;
  cursor: pointer;
}
.seek::-webkit-slider-runnable-track {
  background: transparent;
}
.seek::-webkit-slider-thumb {
  appearance: none;
  width: 0.9rem;
  height: 0.9rem;
  border-radius: 9999px;
  background: var(--color-accent-300);
  margin-top: 0.15rem;
}
.seek::-moz-range-track {
  background: transparent;
}
.seek::-moz-range-thumb {
  width: 0.9rem;
  height: 0.9rem;
  border: 0;
  border-radius: 9999px;
  background: var(--color-accent-300);
}
.seek:disabled {
  cursor: default;
}
</style>
