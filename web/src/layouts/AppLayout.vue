<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import GlobalSearchResults from '@/components/GlobalSearchResults.vue'
import UserAvatar from '@/components/UserAvatar.vue'
import { libraryKinds } from '@/api/libraries'
import { serverApi } from '@/api/server'
import { useGlobalSearch } from '@/composables/useGlobalSearch'
import { useAuthStore } from '@/stores/auth'

const cachedViews = ['KindView', 'LibraryView']

const { t } = useI18n()
const auth = useAuthStore()

const menuOpen = ref(false)
const serverVersion = ref('')
const menu = ref<HTMLElement | null>(null)

// Top bar search. The home page has its own search field, so it is offered everywhere else. The
// query lives in the URL (`search`, apart from the `q` of catalog lists) so that coming back from
// a result shows the results again; following a result drops it and closes the search.
const route = useRoute()
const router = useRouter()
const onHome = computed(() => route.name === 'home')
const routeSearch = () => (typeof route.query.search === 'string' ? route.query.search : '')
const searchQuery = ref(routeSearch())
const searchOpen = ref(searchQuery.value !== '')
const searchInput = ref<HTMLInputElement | null>(null)
const search = useGlobalSearch(searchQuery)
const showSearchResults = computed(() => !onHome.value && search.active.value)

watch(search.term, (q) => {
  if (onHome.value || q === routeSearch()) return
  router.replace({ query: { ...route.query, search: q || undefined } })
})
watch([() => route.path, () => route.query.search], ([path], [previousPath]) => {
  const next = routeSearch()
  if (next !== search.term.value) searchQuery.value = next
  if (next) searchOpen.value = true
  else if (path !== previousPath) searchOpen.value = false
})

// Results replace the page, which stays mounted underneath: clearing the search shows it again
// where it was left.
let pageScroll = 0
watch(showSearchResults, async (shown) => {
  if (shown) {
    pageScroll = window.scrollY
    window.scrollTo({ top: 0 })
  } else {
    await nextTick()
    window.scrollTo({ top: pageScroll })
  }
})

async function openSearch() {
  searchOpen.value = true
  await nextTick()
  searchInput.value?.focus()
}

function closeSearch() {
  searchQuery.value = ''
  searchOpen.value = false
}

function closeSearchIfEmpty() {
  if (!searchQuery.value.trim()) closeSearch()
}

function closeOnOutsideClick(event: PointerEvent) {
  if (menu.value && !menu.value.contains(event.target as Node)) menuOpen.value = false
}

function closeOnEscape(event: KeyboardEvent) {
  if (event.key === 'Escape') menuOpen.value = false
}

onMounted(() => {
  document.addEventListener('pointerdown', closeOnOutsideClick)
  document.addEventListener('keydown', closeOnEscape)
  // The footer is informative only: without an answer it simply stays empty.
  serverApi
    .info()
    .then((info) => (serverVersion.value = info.version))
    .catch(() => {})
})

onUnmounted(() => {
  document.removeEventListener('pointerdown', closeOnOutsideClick)
  document.removeEventListener('keydown', closeOnEscape)
})
</script>

<template>
  <div class="flex min-h-full flex-col">
    <header class="sticky top-0 z-30 border-b border-white/5 bg-surface-950/80 backdrop-blur">
      <nav class="mx-auto flex h-16 max-w-screen-2xl items-center gap-4 px-4 sm:px-8 md:gap-8">
        <RouterLink :to="{ name: 'home' }" :aria-label="t('app.name')" class="flex items-center gap-2.5">
          <img src="/brand-mark.svg" alt="" class="size-8 object-contain" />
          <span class="text-lg font-extrabold tracking-tight" :class="{ 'hidden sm:inline': searchOpen && !onHome }">{{ t('app.name') }}</span>
        </RouterLink>

        <div class="hidden items-center gap-6 text-sm font-medium md:flex">
          <RouterLink
            v-for="kind in libraryKinds"
            :key="kind"
            :to="{ name: 'kind', params: { kind } }"
            class="text-slate-400 transition hover:text-slate-100"
            active-class="text-slate-100!"
          >
            {{ t(`libraries.kinds.${kind}`) }}
          </RouterLink>
        </div>

        <div v-if="!onHome" class="ml-auto flex min-w-0 items-center justify-end" :class="{ 'flex-1 sm:flex-none': searchOpen }">
          <div v-if="searchOpen" class="relative w-full sm:w-72">
            <svg viewBox="0 0 24 24" class="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-slate-500" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" />
            </svg>
            <input
              ref="searchInput"
              v-model="searchQuery"
              type="text"
              :placeholder="t('home.search')"
              :aria-label="t('home.search')"
              autocomplete="off"
              class="w-full rounded-lg border border-white/10 bg-surface-800 py-1.5 pr-9 pl-9 text-sm text-slate-100 placeholder-slate-500 focus:border-accent-500 focus:ring-2 focus:ring-accent-500/30 focus:outline-none"
              @keydown.esc="closeSearch"
              @blur="closeSearchIfEmpty"
            />
            <button
              type="button"
              class="absolute top-1/2 right-1.5 -translate-y-1/2 rounded-md p-1 text-slate-500 transition hover:bg-white/5 hover:text-slate-200"
              :aria-label="t('nav.closeSearch')"
              @pointerdown.prevent
              @click="closeSearch"
            >
              <svg viewBox="0 0 24 24" class="size-4" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                <path d="M18 6 6 18M6 6l12 12" />
              </svg>
            </button>
          </div>
          <button
            v-else
            type="button"
            class="rounded-lg p-2 text-slate-400 transition hover:bg-white/5 hover:text-slate-100"
            :aria-label="t('nav.search')"
            @click="openSearch"
          >
            <svg viewBox="0 0 24 24" class="size-5" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" />
            </svg>
          </button>
        </div>

        <div ref="menu" class="relative shrink-0" :class="{ 'ml-auto': onHome }">
          <button
            type="button"
            class="flex items-center gap-2 rounded-lg px-2 py-1 text-sm text-slate-300 transition hover:bg-white/5"
            aria-haspopup="menu"
            :aria-expanded="menuOpen"
            @click="menuOpen = !menuOpen"
          >
            <UserAvatar :user="auth.user" class="size-8" />
            <span class="hidden sm:inline">{{ auth.user?.displayName }}</span>
          </button>
          <div
            v-if="menuOpen"
            role="menu"
            class="absolute right-0 mt-2 w-48 rounded-lg bg-surface-800 p-1 shadow-lg ring-1 ring-white/10"
            @click="menuOpen = false"
          >
            <RouterLink
              v-for="kind in libraryKinds"
              :key="kind"
              :to="{ name: 'kind', params: { kind } }"
              role="menuitem"
              class="block rounded-md px-3 py-2 text-sm text-slate-300 transition hover:bg-white/5 hover:text-slate-100 md:hidden"
              active-class="text-slate-100!"
            >
              {{ t(`libraries.kinds.${kind}`) }}
            </RouterLink>
            <div class="my-1 h-px bg-white/10 md:hidden" role="separator" />
            <RouterLink
              :to="{ name: 'profile' }"
              role="menuitem"
              class="block rounded-md px-3 py-2 text-sm text-slate-300 transition hover:bg-white/5 hover:text-slate-100"
            >
              {{ t('nav.profile') }}
            </RouterLink>
            <RouterLink
              v-if="auth.isAdmin"
              :to="{ name: 'admin-libraries' }"
              role="menuitem"
              class="block rounded-md px-3 py-2 text-sm text-slate-300 transition hover:bg-white/5 hover:text-slate-100"
            >
              {{ t('nav.admin') }}
            </RouterLink>
            <button
              type="button"
              role="menuitem"
              class="block w-full rounded-md px-3 py-2 text-left text-sm text-slate-300 transition hover:bg-white/5 hover:text-slate-100"
              @click="auth.logout()"
            >
              {{ t('common.logout') }}
            </button>
          </div>
        </div>
      </nav>
    </header>

    <main v-if="showSearchResults" class="mx-auto w-full max-w-screen-2xl flex-1 px-4 py-10 sm:px-8">
      <GlobalSearchResults :search="search" />
    </main>

    <div v-show="!showSearchResults" class="flex-1">
      <!-- Catalog lists stay alive so that coming back from an item (or its seasons and episodes)
           finds them as they were left: same loaded pages, same scroll. -->
      <RouterView v-slot="{ Component, route: current }">
        <KeepAlive :include="cachedViews" :max="5">
          <component :is="Component" :key="current.path" />
        </KeepAlive>
      </RouterView>
    </div>

    <footer class="border-t border-white/5">
      <div class="mx-auto flex h-12 max-w-screen-2xl items-center justify-end px-4 text-xs text-slate-500 sm:px-8">
        <span v-if="serverVersion">{{ t('app.serverVersion', { version: serverVersion }) }}</span>
      </div>
    </footer>
  </div>
</template>
