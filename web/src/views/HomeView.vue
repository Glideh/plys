<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { librariesApi, type Library, type LibraryKind } from '@/api/libraries'
import { progressApi, type HomeItem } from '@/api/progress'
import ErrorAlert from '@/components/ErrorAlert.vue'
import HomeRail from '@/components/HomeRail.vue'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { useLibrarySummary } from '@/composables/useLibrarySummary'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const auth = useAuthStore()
const errorMessage = useErrorMessage()
const summary = useLibrarySummary()

const libraries = ref<Library[]>([])
const loading = ref(true)
const error = ref('')

const gradients: Record<LibraryKind, string> = {
  movies: 'from-accent-500/30 via-sky-700/20 to-indigo-800/30',
  series: 'from-indigo-400/30 via-violet-600/20 to-slate-700/30',
  other: 'from-rose-400/25 via-pink-700/15 to-slate-700/30',
}

const continueItems = ref<HomeItem[]>([])
const nextItems = ref<HomeItem[]>([])
const railsLoading = ref(true)

onMounted(async () => {
  // A failing rail must not hide the libraries.
  progressApi
    .continueWatching()
    .then((items) => (continueItems.value = items))
    .catch(() => undefined)
    .then(() => progressApi.nextUp())
    .then((items) => (nextItems.value = items))
    .catch(() => undefined)
    .finally(() => (railsLoading.value = false))
  try {
    libraries.value = await librariesApi.list()
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <main class="mx-auto max-w-screen-2xl space-y-12 px-4 py-10 sm:px-8">
    <div class="grid gap-12 lg:grid-cols-2">
      <HomeRail :title="t('home.continueWatching')" :empty="t('home.emptyContinue')" :items="continueItems" :loading="railsLoading" resume />
      <HomeRail :title="t('home.nextUp')" :empty="t('home.emptyNextUp')" :items="nextItems" :loading="railsLoading" />
    </div>

    <section>
      <h2 class="mb-4 text-lg font-semibold">{{ t('home.libraries') }}</h2>
      <ErrorAlert v-if="error" :message="error" />
      <div v-else-if="loading" class="grid grid-cols-[repeat(auto-fill,minmax(12rem,1fr))] gap-4">
        <div v-for="i in 3" :key="i" class="aspect-video animate-pulse rounded-xl bg-surface-800/60" />
      </div>
      <div v-else-if="libraries.length === 0" class="rounded-xl border border-dashed border-white/10 px-6 py-10 text-center">
        <p class="text-sm text-slate-400">{{ t('home.emptyLibraries') }}</p>
        <template v-if="auth.isAdmin">
          <p class="mt-1 text-sm text-slate-500">{{ t('home.emptyLibrariesAdmin') }}</p>
          <RouterLink
            :to="{ name: 'admin-libraries' }"
            class="mt-4 inline-block rounded-lg bg-accent-500 px-4 py-2 text-sm font-semibold text-surface-950 transition hover:bg-accent-400"
          >
            {{ t('home.manageLibraries') }}
          </RouterLink>
        </template>
      </div>
      <ul v-else class="grid grid-cols-[repeat(auto-fill,minmax(12rem,1fr))] gap-4">
        <li v-for="lib in libraries" :key="lib.id">
          <RouterLink
            :to="{ name: 'library', params: { id: lib.id } }"
            :class="gradients[lib.kind]"
            class="flex aspect-video flex-col justify-end overflow-hidden rounded-xl border border-white/5 bg-gradient-to-br p-4 transition hover:-translate-y-0.5 hover:border-white/20 hover:shadow-xl hover:shadow-black/40"
          >
            <span class="text-xs font-medium tracking-wide text-white/60 uppercase">{{ t(`libraries.kinds.${lib.kind}`) }}</span>
            <span class="truncate text-lg font-semibold">{{ lib.name }}</span>
            <span class="truncate text-xs text-white/70">{{ summary(lib) }}</span>
          </RouterLink>
        </li>
      </ul>
    </section>
  </main>
</template>
