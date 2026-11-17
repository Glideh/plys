<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { Check, Heart, Info, RotateCcw } from '@lucide/vue'
import { catalogApi, type CardSource, type ChildItem, type ItemDetail, type Track, type Version } from '@/api/catalog'
import { personCatalogRoute } from '@/api/catalogFilters'
import { loadLocalServer, localServerId, serverApi, serverFromQuery, serverQuery, type ServerRef } from '@/api/federation'
import { watchlistApi } from '@/api/watchlist'
import BaseButton from '@/components/BaseButton.vue'
import ErrorAlert from '@/components/ErrorAlert.vue'
import ItemInfoDialog from '@/components/ItemInfoDialog.vue'
import MatchDialog from '@/components/MatchDialog.vue'
import PosterCard from '@/components/PosterCard.vue'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { useAuthStore } from '@/stores/auth'
import { channelLayout, formatBytes, formatRuntime, languageName } from '@/utils/format'
import { formatClock } from '@/utils/player'

const props = defineProps<{ id: string }>()
const { t, d, locale } = useI18n()
const router = useRouter()
const auth = useAuthStore()
const errorMessage = useErrorMessage()

/** A version the user can read, with the server and item holding it. */
interface VersionChoice {
  server: ServerRef
  itemId: string
  positionSeconds: number
  version: Version
}

const item = ref<ItemDetail | null>(null)
const error = ref('')
const choices = ref<VersionChoice[]>([])
const versionId = ref<string>('')
const matchOpen = ref(false)
const infoOpen = ref(false)
const refreshing = ref(false)
const route = useRoute()
// Null when this server holds the item; otherwise the paired server the page is read from.
const server = serverFromQuery(route.query)

function refOf(source: CardSource): ServerRef {
  return { serverId: source.serverId, serverName: source.serverName, baseUrl: source.serverUrl ?? null }
}

/** Lists the versions of the title across every server holding it, this server's first. */
async function loadChoices(primary: ItemDetail): Promise<void> {
  let sources = primary.sources
  if (server && primary.tmdbId) {
    // A paired server only knows its own copies: this server tells whether the title is listed on any of them.
    const title = await catalogApi.titleSources(primary.kind, primary.tmdbId)
    sources = title.sources
    // Through the ref, so that the page updates.
    if (title.inWatchlist && item.value?.id === primary.id) item.value.inWatchlist = true
  } else if (server) {
    sources = []
  }

  const localId = localServerId.value ?? ''
  const primaryRef: ServerRef = server ?? {
    serverId: localId,
    serverName: sources.find((s) => s.serverId === localId)?.serverName ?? '',
    baseUrl: null,
  }

  // Series and seasons have no playable version: their other copies are not fetched.
  const seen = new Set([`${primaryRef.serverId}/${primary.id}`])
  const others = primary.children.length
    ? []
    : sources.filter((s) => {
        const key = `${s.serverId}/${s.itemId}`
        if (seen.has(key)) return false
        seen.add(key)
        return true
      })
  const fetched = await Promise.allSettled(others.map((s) => serverApi<ItemDetail>(refOf(s), `/items/${s.itemId}`)))

  const list: VersionChoice[] = []
  const add = (ref: ServerRef, detail: ItemDetail) => {
    for (const version of detail.versions) {
      list.push({ server: ref, itemId: detail.id, positionSeconds: detail.positionSeconds, version })
    }
  }
  add(primaryRef, primary)
  fetched.forEach((res, i) => {
    if (res.status === 'fulfilled') add(refOf(others[i]), res.value)
  })

  choices.value = list
  if (!list.some((c) => c.version.id === versionId.value)) versionId.value = list[0]?.version.id ?? ''
}

async function load() {
  error.value = ''
  try {
    await loadLocalServer()
    const primary = await serverApi<ItemDetail>(server, `/items/${props.id}`)
    item.value = primary
    await loadChoices(primary)
  } catch (err) {
    error.value = errorMessage(err)
  }
}
onMounted(load)

const selected = computed(() => choices.value.find((c) => c.version.id === versionId.value) ?? null)
const version = computed(() => selected.value?.version ?? null)

// Tracks the player opens with: the default audio track and no subtitles, reset when the version changes.
const audioIndex = ref<number | null>(null)
const subtitleIndex = ref<number | null>(null)
watch(
  version,
  (v) => {
    audioIndex.value = (v?.audio.find((a) => a.default) ?? v?.audio[0])?.index ?? null
    subtitleIndex.value = null
  },
  { immediate: true },
)

const selectClass =
  'w-full max-w-md truncate rounded-lg border border-white/10 bg-surface-800/80 px-3 py-1.5 text-sm text-slate-100 focus:border-accent-500 focus:outline-none'

// Once one row shows a selector, plain-text rows get the same box (transparent border,
// same padding) so every row has the same height and the values line up.
const hasSelector = computed(
  () => choices.value.length > 1 || (version.value?.audio.length ?? 0) > 1 || (version.value?.subtitles.length ?? 0) > 0,
)
const readonlyClass = computed(() =>
  hasSelector.value ? 'block max-w-md truncate border border-transparent px-3 py-1.5' : undefined,
)

function versionLabel(c: VersionChoice): string {
  return [multiServer.value ? c.server.serverName : null, c.version.label, formatBytes(c.version.size, locale.value)].filter(Boolean).join(' · ')
}

function subtitleLabel(s: Track): string {
  return trackLabel(s) + (s.forced ? ` (${t('catalog.forced')})` : '')
}
/** File locations of this server's copy, only sent to admins. */
const paths = computed(() => (server ? [] : (item.value?.versions.map((v) => v.path).filter((p): p is string => !!p) ?? [])))
const hasLocation = computed(() => !server && (!!item.value?.folder || paths.value.length > 0))
const multiServer = computed(() => new Set(choices.value.map((c) => c.server.serverId)).size > 1)
// Directors or creators, with their TMDB ID when known; older servers only send the names.
const directorPeople = computed(() => (item.value?.directorCredits?.length ? item.value.directorCredits : (item.value?.directors ?? []).map((name) => ({ name, id: undefined }))))
const hasOwnMetadata = computed(() => ['movie', 'series', 'video'].includes(item.value?.kind ?? ''))
const backdrop = computed(() => item.value?.backdropUrl ?? (item.value?.kind === 'episode' ? item.value.thumbUrl : null))
// Seasons and episodes have no backdrop of their own: the series' one is not loaded to keep a single request.
const poster = computed(() => (item.value?.kind === 'episode' || item.value?.kind === 'video' ? null : item.value?.posterUrl))

const displayTitle = computed(() => (item.value ? itemTitle(item.value) : ''))

function itemTitle(it: Pick<ItemDetail, 'kind' | 'title' | 'seasonNumber' | 'episodeNumber'>): string {
  if (it.kind === 'season') return it.seasonNumber === 0 ? t('catalog.specials') : it.title || t('catalog.season', { n: it.seasonNumber })
  if (it.kind === 'episode') return it.title || t('catalog.episode', { n: it.episodeNumber })
  return it.title
}

const metaLine = computed(() => {
  const it = item.value
  if (!it) return []
  const parts: string[] = []
  if (it.kind === 'episode' && it.seasonNumber !== undefined) {
    parts.push(`S${String(it.seasonNumber).padStart(2, '0')}E${String(it.episodeNumber).padStart(2, '0')}${it.episodeNumberEnd ? `-E${String(it.episodeNumberEnd).padStart(2, '0')}` : ''}`)
  }
  if (it.releaseDate && it.kind === 'episode') parts.push(d(new Date(it.releaseDate)))
  else if (it.year) parts.push(String(it.year))
  const runtime = formatRuntime(it.runtimeMinutes ?? (version.value?.durationSeconds ? Math.round(version.value.durationSeconds / 60) : null))
  if (runtime) parts.push(runtime)
  if (it.rating) parts.push(`★ ${it.rating.toFixed(1)}`)
  return parts
})

const childrenTitle = computed(() => {
  return item.value?.kind === 'series' ? t('catalog.seasons') : t('catalog.episodes')
})

function childTitle(c: ChildItem): string {
  if (c.kind === 'episode') {
    const n = `${c.episodeNumber}${c.episodeNumberEnd ? `-${c.episodeNumberEnd}` : ''}`
    return `${n}. ${c.title || t('catalog.episode', { n: c.episodeNumber })}`
  }
  return itemTitle(c)
}

function trackLabel(tr: { language?: string; title?: string; codec: string; channels?: number }): string {
  return [languageName(tr.language, locale.value) ?? tr.title ?? '?', tr.codec.toUpperCase(), channelLayout(tr.channels)]
    .filter(Boolean)
    .join(' · ')
}

function play(fromStart = false) {
  const c = selected.value
  if (!c) return
  const resumeAt = fromStart ? 0 : c.positionSeconds
  void router.push({
    name: 'watch',
    params: { id: c.itemId },
    query: {
      ...serverQuery(c.server),
      v: c.version.id,
      ...(resumeAt ? { t: Math.floor(resumeAt) } : {}),
      ...(audioIndex.value !== null ? { a: audioIndex.value } : {}),
      ...(subtitleIndex.value !== null ? { s: subtitleIndex.value } : {}),
    },
  })
}

function targetLabel(tg: { kind: string; seasonNumber?: number; episodeNumber?: number }): string {
  if (tg.kind === 'episode') return `S${String(tg.seasonNumber ?? 0).padStart(2, '0')}E${String(tg.episodeNumber ?? 0).padStart(2, '0')}`
  return ''
}

/** Series and seasons play the item chosen by the server. */
function playTarget() {
  const tg = item.value?.playTarget
  if (!tg) return
  void router.push({
    name: 'watch',
    params: { id: tg.id },
    query: { ...serverQuery(server), ...(tg.resume ? { t: Math.floor(tg.positionSeconds) } : {}) },
  })
}

const primaryLabel = computed(() => {
  const it = item.value
  if (!it) return ''
  if (choices.value.length) {
    if (selected.value?.positionSeconds) return t('catalog.resume', { time: formatClock(selected.value.positionSeconds) })
    return it.watched ? t('catalog.rewatch') : t('catalog.play')
  }
  const tg = it.playTarget
  if (!tg) return ''
  if (tg.rewatch) return t('catalog.rewatch')
  return t(tg.resume ? 'catalog.continueEpisode' : 'catalog.playEpisode', { label: targetLabel(tg) })
})

const togglingWatched = ref(false)
const watchedLabel = computed(() => (item.value?.watched ? t('catalog.markUnwatched') : t('catalog.markWatched')))

async function toggleWatched(id: string, watched: boolean) {
  togglingWatched.value = true
  try {
    await serverApi(server, `/items/${id}/watched`, { method: watched ? 'DELETE' : 'POST' })
    await load()
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    togglingWatched.value = false
  }
}

const togglingWatchlist = ref(false)
const watchlistLabel = computed(() => t(item.value?.inWatchlist ? 'catalog.removeFromWatchlist' : 'catalog.addToWatchlist'))

/**
 * Adds the title on the server it is read from, or removes every copy of it from the watchlist, whichever server
 * holds it, without reloading the page.
 */
async function toggleWatchlist() {
  const it = item.value
  if (!it) return
  togglingWatchlist.value = true
  try {
    if (it.inWatchlist && it.tmdbId) await watchlistApi.removeTitle(it.kind, it.tmdbId)
    else await watchlistApi.set(server, it.id, !it.inWatchlist)
    it.inWatchlist = !it.inWatchlist
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    togglingWatchlist.value = false
  }
}

/** Share of an episode already watched, for the bar on its thumbnail. */
function childProgress(c: ChildItem): number | null {
  return c.positionSeconds && c.runtimeMinutes ? c.positionSeconds / (c.runtimeMinutes * 60) : null
}

async function refresh() {
  refreshing.value = true
  try {
    const res = await catalogApi.refresh(props.id)
    if (res.merged) return router.replace({ name: 'library', params: { id: item.value!.libraryId } })
    await load()
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    refreshing.value = false
  }
}

async function onMatched(merged: boolean) {
  if (merged) return router.replace({ name: 'library', params: { id: item.value!.libraryId } })
  await load()
}
</script>

<template>
  <main>
    <ErrorAlert v-if="error" class="mx-auto mt-8 max-w-screen-xl" :message="error" />
    <div v-else-if="!item" class="mx-auto max-w-screen-xl px-4 py-10 sm:px-8">
      <div class="h-96 animate-pulse rounded-2xl bg-surface-800/60" />
    </div>

    <template v-else>
      <section class="relative isolate overflow-hidden">
        <img v-if="backdrop" :src="backdrop" alt="" class="absolute inset-0 -z-20 size-full object-cover opacity-40" />
        <div class="absolute inset-0 -z-10 bg-gradient-to-t from-surface-950 via-surface-950/80 to-surface-950/30" />
        <div class="absolute inset-0 -z-10 bg-gradient-to-r from-surface-950 via-surface-950/60 to-transparent" />

        <div class="mx-auto flex max-w-screen-xl flex-col gap-8 px-4 pt-10 pb-12 sm:px-8 md:flex-row md:items-end">
          <img
            v-if="poster"
            :src="poster"
            :alt="displayTitle"
            class="w-40 shrink-0 rounded-xl shadow-2xl shadow-black/60 ring-1 ring-white/10 md:w-56"
          />
          <div class="min-w-0 flex-1">
            <nav v-if="item.ancestors.length" class="mb-2 flex flex-wrap gap-1 text-sm text-slate-400" aria-label="breadcrumb">
              <template v-for="(a, i) in item.ancestors" :key="a.id">
                <RouterLink :to="{ name: 'item', params: { id: a.id }, query: serverQuery(server) }" class="hover:text-white">
                  {{ a.kind === 'season' ? itemTitle({ ...a, episodeNumber: undefined }) : a.title }}
                </RouterLink>
                <span v-if="i < item.ancestors.length - 1" aria-hidden="true">›</span>
              </template>
            </nav>
            <h1 class="text-3xl font-bold tracking-tight md:text-5xl">{{ displayTitle }}</h1>
            <p v-if="item.originalTitle && item.originalTitle !== item.title" class="mt-1 text-slate-400">{{ item.originalTitle }}</p>
            <p class="mt-3 flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-slate-300">
              <span v-for="(m, i) in metaLine" :key="i">{{ m }}</span>
              <span v-for="g in item.genres" :key="g" class="rounded-full border border-white/15 px-2 py-0.5 text-xs">{{ g }}</span>
              <span v-if="auth.isAdmin && item.matchStatus === 'unmatched'" class="rounded-full bg-amber-500/20 px-2 py-0.5 text-xs text-amber-300">
                {{ t('catalog.unidentified') }}
              </span>
            </p>
            <p v-if="item.tagline" class="mt-4 text-lg text-slate-300 italic">{{ item.tagline }}</p>
            <p v-if="item.overview" class="mt-3 max-w-3xl leading-relaxed text-slate-300">{{ item.overview }}</p>

            <div class="mt-6 flex flex-wrap items-center gap-3">
              <!-- When resuming, the restart choice is attached to the play button so both clearly open the player. -->
              <div v-if="choices.length" class="inline-flex">
                <BaseButton
                  variant="primary"
                  class="px-6 py-2.5 text-base"
                  :class="selected?.positionSeconds ? '!rounded-r-none' : ''"
                  @click="play()"
                >
                  ▶ {{ primaryLabel }}
                </BaseButton>
                <BaseButton
                  v-if="selected?.positionSeconds"
                  variant="primary"
                  class="!rounded-l-none border-l border-surface-950/25 !px-3"
                  :aria-label="t('catalog.fromStart')"
                  :title="t('catalog.fromStart')"
                  @click="play(true)"
                >
                  <RotateCcw class="size-5" aria-hidden="true" />
                </BaseButton>
              </div>
              <BaseButton v-else-if="item.playTarget" variant="primary" class="px-6 py-2.5 text-base" @click="playTarget">
                ▶ {{ primaryLabel }}
              </BaseButton>
              <BaseButton
                v-if="choices.length || item.playTarget"
                class="!px-2.5"
                :class="item.watched ? '!text-accent-300 hover:!text-accent-400' : '!text-slate-200/35 hover:!text-slate-200/70'"
                :loading="togglingWatched"
                :aria-pressed="item.watched"
                :aria-label="watchedLabel"
                :title="watchedLabel"
                @click="toggleWatched(item.id, item.watched)"
              >
                <Check v-if="!togglingWatched" class="size-5" :stroke-width="2.5" aria-hidden="true" />
              </BaseButton>
              <BaseButton
                v-if="item.ancestors.length === 0"
                class="!px-2.5"
                :class="item.inWatchlist ? '!text-rose-400 hover:!text-rose-300' : ''"
                :loading="togglingWatchlist"
                :aria-pressed="item.inWatchlist"
                :aria-label="watchlistLabel"
                :title="watchlistLabel"
                @click="toggleWatchlist"
              >
                <Heart v-if="!togglingWatchlist" class="size-5" :fill="item.inWatchlist ? 'currentColor' : 'none'" aria-hidden="true" />
              </BaseButton>
              <BaseButton v-if="hasLocation" class="!px-2.5" :aria-label="t('catalog.details')" :title="t('catalog.details')" @click="infoOpen = true">
                <Info class="size-5" aria-hidden="true" />
              </BaseButton>
              <template v-if="auth.isAdmin && !server && hasOwnMetadata">
                <BaseButton v-if="item.kind !== 'video'" @click="matchOpen = true">{{ t('catalog.fixMatch') }}</BaseButton>
                <BaseButton :loading="refreshing" @click="refresh">{{ t('catalog.refresh') }}</BaseButton>
              </template>
            </div>

            <!-- One row per choice: a selector when there are several options, plain text otherwise. -->
            <dl v-if="version" class="mt-6 grid max-w-3xl gap-x-6 gap-y-2 text-sm sm:grid-cols-[auto_1fr] sm:items-center">
              <dt id="item-version" class="text-slate-500">{{ t('catalog.version') }}</dt>
              <dd class="min-w-0 text-slate-300">
                <select v-if="choices.length > 1" v-model="versionId" aria-labelledby="item-version" :class="selectClass">
                  <option v-for="c in choices" :key="c.version.id" :value="c.version.id">{{ versionLabel(c) }}</option>
                </select>
                <span v-else-if="selected" :class="readonlyClass">{{ versionLabel(selected) }}</span>
              </dd>
              <dt id="item-audio" class="text-slate-500">{{ t('catalog.audio') }}</dt>
              <dd class="min-w-0 text-slate-300">
                <select v-if="version.audio.length > 1" v-model="audioIndex" aria-labelledby="item-audio" :class="selectClass">
                  <option v-for="a in version.audio" :key="a.index" :value="a.index">{{ trackLabel(a) }}</option>
                </select>
                <span v-else :class="readonlyClass">{{ version.audio[0] ? trackLabel(version.audio[0]) : t('catalog.none') }}</span>
              </dd>
              <dt id="item-subtitles" class="text-slate-500">{{ t('catalog.subtitles') }}</dt>
              <dd class="min-w-0 text-slate-300">
                <select v-if="version.subtitles.length" v-model="subtitleIndex" aria-labelledby="item-subtitles" :class="selectClass">
                  <option :value="null">{{ t('catalog.none') }}</option>
                  <option v-for="s in version.subtitles" :key="s.index" :value="s.index">{{ subtitleLabel(s) }}</option>
                </select>
                <span v-else :class="readonlyClass">{{ t('catalog.none') }}</span>
              </dd>
              <template v-if="version.error">
                <dt class="text-amber-400">{{ t('catalog.unreadable') }}</dt>
                <dd class="truncate font-mono text-xs text-amber-300/80" :title="version.error">{{ version.error }}</dd>
              </template>
            </dl>
          </div>
        </div>
      </section>

      <div class="mx-auto max-w-screen-xl space-y-12 px-4 pb-16 sm:px-8">
        <section v-if="item.children.length">
          <h2 class="mb-4 text-xl font-semibold">{{ childrenTitle }}</h2>

          <ul v-if="item.kind === 'series'" class="grid grid-cols-2 gap-x-4 gap-y-6 sm:grid-cols-4 lg:grid-cols-6">
            <li v-for="c in item.children" :key="c.id">
              <PosterCard
                :to="{ name: 'item', params: { id: c.id }, query: serverQuery(server) }"
                :title="childTitle(c)"
                :subtitle="c.childCount ? t('catalog.episodeCount', c.childCount) : null"
                :image-url="c.posterUrl ?? item.posterUrl"
                :watched="c.watched"
                :unwatched="c.unwatchedCount"
              />
            </li>
          </ul>

          <ul v-else class="divide-y divide-white/5 overflow-hidden rounded-2xl border border-white/5 bg-surface-900/60">
            <li v-for="c in item.children" :key="c.id" class="relative">
              <RouterLink :to="{ name: 'item', params: { id: c.id }, query: serverQuery(server) }" class="group flex gap-4 p-4 pr-14 transition hover:bg-white/[0.03]">
                <div class="relative aspect-video w-40 shrink-0 overflow-hidden rounded-lg bg-surface-800 sm:w-56">
                  <img v-if="c.thumbUrl" :src="c.thumbUrl" alt="" loading="lazy" class="size-full object-cover" />
                  <div v-if="childProgress(c)" class="absolute inset-x-0 bottom-0 h-1.5 bg-black/60">
                    <div class="h-full bg-accent-400" :style="{ width: Math.min(childProgress(c)!, 1) * 100 + '%' }" />
                  </div>
                </div>
                <div class="min-w-0 py-1">
                  <p class="font-semibold group-hover:text-white">{{ childTitle(c) }}</p>
                  <p class="mt-0.5 text-xs text-slate-500">
                    {{ [c.releaseDate ? d(new Date(c.releaseDate)) : null, formatRuntime(c.runtimeMinutes)].filter(Boolean).join(' · ') }}
                  </p>
                  <p v-if="c.overview" class="mt-2 line-clamp-2 text-sm text-slate-400">{{ c.overview }}</p>
                </div>
              </RouterLink>
              <button
                type="button"
                class="absolute top-4 right-4 grid size-8 place-items-center rounded-full transition hover:bg-white/10"
                :class="c.watched ? 'text-accent-300' : 'text-slate-600 hover:text-slate-300'"
                :title="c.watched ? t('catalog.markUnwatched') : t('catalog.markWatched')"
                :aria-label="c.watched ? t('catalog.markUnwatched') : t('catalog.markWatched')"
                :aria-pressed="c.watched"
                :disabled="togglingWatched"
                @click="toggleWatched(c.id, c.watched)"
              >
                <svg viewBox="0 0 24 24" class="size-5" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M5 12l5 5L20 7" /></svg>
              </button>
            </li>
          </ul>
        </section>

        <section v-if="item.directors.length || item.cast.length">
          <h2 class="mb-4 text-xl font-semibold">{{ t('catalog.cast') }}</h2>
          <p v-if="item.directors.length" class="mb-4 text-sm text-slate-400">
            {{ item.kind === 'series' ? t('catalog.createdBy') : t('catalog.directedBy') }} :
            <template v-for="(p, i) in directorPeople" :key="p.name">
              <template v-if="i">, </template>
              <RouterLink
                v-if="p.id"
                :to="personCatalogRoute(item.kind, 'creators', p.id)"
                :title="t('catalog.personTitles', { name: p.name })"
                class="text-slate-200 underline-offset-4 hover:text-white hover:underline"
              >{{ p.name }}</RouterLink>
              <span v-else class="text-slate-200">{{ p.name }}</span>
            </template>
          </p>
          <ul class="-mx-1 -mt-1 flex gap-4 overflow-x-auto px-1 pt-1 pb-2">
            <li v-for="c in item.cast" :key="c.name + c.character" class="w-28 shrink-0 text-center">
              <component
                :is="c.id ? RouterLink : 'div'"
                v-bind="c.id ? { to: personCatalogRoute(item.kind, 'actors', c.id), title: t('catalog.personTitles', { name: c.name }) } : {}"
                class="group block rounded-lg outline-none"
              >
                <div class="mx-auto size-24 overflow-hidden rounded-full bg-surface-800 ring-1 ring-white/10 transition group-hover:ring-2 group-hover:ring-accent-400 group-focus-visible:ring-2 group-focus-visible:ring-accent-400">
                  <img v-if="c.photoUrl" :src="c.photoUrl" :alt="c.name" loading="lazy" class="size-full object-cover" />
                </div>
                <p class="mt-2 truncate text-sm font-medium" :class="{ 'group-hover:text-white': c.id }">{{ c.name }}</p>
                <p v-if="c.character" class="truncate text-xs text-slate-500">{{ c.character }}</p>
              </component>
            </li>
          </ul>
        </section>
      </div>

      <MatchDialog
        v-if="auth.isAdmin && !server"
        v-model:open="matchOpen"
        :item-id="item.id"
        :title="item.title"
        :year="item.year"
        :folder="item.folder"
        :paths="paths"
        @matched="onMatched"
      />
      <ItemInfoDialog v-if="hasLocation" v-model:open="infoOpen" :folder="item.folder" :paths="paths" />
    </template>
  </main>
</template>
