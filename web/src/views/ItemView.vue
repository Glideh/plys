<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { catalogApi, type ChildItem, type ItemDetail } from '@/api/catalog'
import BaseButton from '@/components/BaseButton.vue'
import ErrorAlert from '@/components/ErrorAlert.vue'
import MatchDialog from '@/components/MatchDialog.vue'
import PosterCard from '@/components/PosterCard.vue'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { useAuthStore } from '@/stores/auth'
import { channelLayout, formatBytes, formatRuntime, languageName } from '@/utils/format'

const props = defineProps<{ id: string }>()
const { t, d, locale } = useI18n()
const router = useRouter()
const auth = useAuthStore()
const errorMessage = useErrorMessage()

const item = ref<ItemDetail | null>(null)
const error = ref('')
const versionId = ref<string>('')
const matchOpen = ref(false)
const refreshing = ref(false)

async function load() {
  error.value = ''
  try {
    item.value = await catalogApi.get(props.id)
    versionId.value = item.value.versions[0]?.id ?? ''
  } catch (err) {
    error.value = errorMessage(err)
  }
}
onMounted(load)

const version = computed(() => item.value?.versions.find((v) => v.id === versionId.value) ?? null)
const hasOwnMetadata = computed(() => ['movie', 'series', 'video'].includes(item.value?.kind ?? ''))
const backdrop = computed(() => item.value?.backdropUrl ?? (item.value?.kind === 'episode' ? item.value.thumbUrl : null))
// Seasons and episodes have no backdrop of their own: the series' one is not loaded to keep a single request.
const poster = computed(() => (item.value?.kind === 'episode' || item.value?.kind === 'video' ? null : item.value?.posterUrl))

const displayTitle = computed(() => (item.value ? itemTitle(item.value) : ''))

function itemTitle(it: Pick<ItemDetail, 'kind' | 'title' | 'seasonNumber' | 'episodeNumber' | 'partNumber'>): string {
  if (it.kind === 'season') return it.seasonNumber === 0 ? t('catalog.specials') : it.title || t('catalog.season', { n: it.seasonNumber })
  if (it.kind === 'episode') return it.title || t('catalog.episode', { n: it.episodeNumber })
  if (it.kind === 'part') return t('catalog.part', { n: it.partNumber })
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
  switch (item.value?.kind) {
    case 'series':
      return t('catalog.seasons')
    case 'season':
      return t('catalog.episodes')
    default:
      return t('catalog.parts')
  }
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
                <RouterLink :to="{ name: 'item', params: { id: a.id } }" class="hover:text-white">
                  {{ a.kind === 'season' ? itemTitle({ ...a, episodeNumber: undefined, partNumber: undefined }) : a.title }}
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
              <BaseButton v-if="item.versions.length" variant="primary" disabled :title="t('catalog.playSoon')" class="px-6 py-2.5 text-base">
                ▶ {{ t('catalog.play') }}
              </BaseButton>
              <label v-if="item.versions.length > 1" class="flex items-center gap-2 text-sm text-slate-400">
                {{ t('catalog.version') }}
                <select
                  v-model="versionId"
                  class="rounded-lg border border-white/10 bg-surface-800/80 px-3 py-2 text-sm text-slate-100 focus:border-accent-500 focus:outline-none"
                >
                  <option v-for="v in item.versions" :key="v.id" :value="v.id">
                    {{ [v.label, formatBytes(v.size, locale)].filter(Boolean).join(' · ') }}
                  </option>
                </select>
              </label>
              <template v-if="auth.isAdmin && hasOwnMetadata">
                <BaseButton v-if="item.kind !== 'video'" @click="matchOpen = true">{{ t('catalog.fixMatch') }}</BaseButton>
                <BaseButton :loading="refreshing" @click="refresh">{{ t('catalog.refresh') }}</BaseButton>
              </template>
            </div>

            <dl v-if="version" class="mt-6 grid max-w-3xl gap-x-6 gap-y-2 text-sm sm:grid-cols-[auto_1fr]">
              <template v-if="item.versions.length === 1 && version.label">
                <dt class="text-slate-500">{{ t('catalog.version') }}</dt>
                <dd class="text-slate-300">{{ [version.label, formatBytes(version.size, locale)].join(' · ') }}</dd>
              </template>
              <dt class="text-slate-500">{{ t('catalog.audio') }}</dt>
              <dd class="text-slate-300">{{ version.audio.map(trackLabel).join(', ') || t('catalog.none') }}</dd>
              <dt class="text-slate-500">{{ t('catalog.subtitles') }}</dt>
              <dd class="text-slate-300">
                {{
                  version.subtitles
                    .map((s) => trackLabel(s) + (s.forced ? ` (${t('catalog.forced')})` : ''))
                    .join(', ') || t('catalog.none')
                }}
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
                :to="{ name: 'item', params: { id: c.id } }"
                :title="childTitle(c)"
                :subtitle="c.childCount ? t('catalog.episodeCount', c.childCount) : null"
                :image-url="c.posterUrl ?? item.posterUrl"
              />
            </li>
          </ul>

          <ul v-else class="divide-y divide-white/5 overflow-hidden rounded-2xl border border-white/5 bg-surface-900/60">
            <li v-for="c in item.children" :key="c.id">
              <RouterLink :to="{ name: 'item', params: { id: c.id } }" class="group flex gap-4 p-4 transition hover:bg-white/[0.03]">
                <div class="aspect-video w-40 shrink-0 overflow-hidden rounded-lg bg-surface-800 sm:w-56">
                  <img v-if="c.thumbUrl" :src="c.thumbUrl" alt="" loading="lazy" class="size-full object-cover transition duration-500 group-hover:scale-105" />
                </div>
                <div class="min-w-0 py-1">
                  <p class="font-semibold group-hover:text-white">{{ childTitle(c) }}</p>
                  <p class="mt-0.5 text-xs text-slate-500">
                    {{ [c.releaseDate ? d(new Date(c.releaseDate)) : null, formatRuntime(c.runtimeMinutes)].filter(Boolean).join(' · ') }}
                  </p>
                  <p v-if="c.overview" class="mt-2 line-clamp-2 text-sm text-slate-400">{{ c.overview }}</p>
                </div>
              </RouterLink>
            </li>
          </ul>
        </section>

        <section v-if="item.directors.length || item.cast.length">
          <h2 class="mb-4 text-xl font-semibold">{{ t('catalog.cast') }}</h2>
          <p v-if="item.directors.length" class="mb-4 text-sm text-slate-400">
            {{ item.kind === 'series' ? t('catalog.createdBy') : t('catalog.directedBy') }} :
            <span class="text-slate-200">{{ item.directors.join(', ') }}</span>
          </p>
          <ul class="flex gap-4 overflow-x-auto pb-2">
            <li v-for="c in item.cast" :key="c.name + c.character" class="w-28 shrink-0 text-center">
              <div class="mx-auto size-24 overflow-hidden rounded-full bg-surface-800 ring-1 ring-white/10">
                <img v-if="c.photoUrl" :src="c.photoUrl" :alt="c.name" loading="lazy" class="size-full object-cover" />
              </div>
              <p class="mt-2 truncate text-sm font-medium">{{ c.name }}</p>
              <p v-if="c.character" class="truncate text-xs text-slate-500">{{ c.character }}</p>
            </li>
          </ul>
        </section>
      </div>

      <MatchDialog v-if="auth.isAdmin" v-model:open="matchOpen" :item-id="item.id" :title="item.title" :year="item.year" @matched="onMatched" />
    </template>
  </main>
</template>
