<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import UserAvatar from '@/components/UserAvatar.vue'
import { libraryKinds } from '@/api/libraries'
import { serverApi } from '@/api/server'
import { useAuthStore } from '@/stores/auth'

const cachedViews = ['KindView', 'LibraryView']

const { t } = useI18n()
const auth = useAuthStore()

const menuOpen = ref(false)
const serverVersion = ref('')
const menu = ref<HTMLElement | null>(null)

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
      <nav class="mx-auto flex h-16 max-w-screen-2xl items-center gap-8 px-4 sm:px-8">
        <RouterLink :to="{ name: 'home' }" class="flex items-center gap-2.5">
          <span class="grid size-8 place-items-center rounded-lg bg-accent-500 text-sm font-black text-surface-950">▶</span>
          <span class="text-lg font-bold tracking-tight">{{ t('app.name') }}</span>
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

        <div ref="menu" class="relative ml-auto">
          <button
            type="button"
            class="flex items-center gap-2 rounded-lg px-2 py-1 text-sm text-slate-300 transition hover:bg-white/5"
            aria-haspopup="menu"
            :aria-expanded="menuOpen"
            @click="menuOpen = !menuOpen"
          >
            <UserAvatar :user="auth.user" class="size-8" />
            <span class="hidden sm:inline">{{ auth.user?.username }}</span>
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

    <div class="flex-1">
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
