<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const auth = useAuthStore()

const menuOpen = ref(false)
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
})

onUnmounted(() => {
  document.removeEventListener('pointerdown', closeOnOutsideClick)
  document.removeEventListener('keydown', closeOnEscape)
})
</script>

<template>
  <div class="min-h-full">
    <header class="sticky top-0 z-30 border-b border-white/5 bg-surface-950/80 backdrop-blur">
      <nav class="mx-auto flex h-16 max-w-screen-2xl items-center gap-8 px-4 sm:px-8">
        <RouterLink :to="{ name: 'home' }" class="flex items-center gap-2.5">
          <span class="grid size-8 place-items-center rounded-lg bg-accent-500 text-sm font-black text-surface-950">▶</span>
          <span class="text-lg font-bold tracking-tight">{{ t('app.name') }}</span>
        </RouterLink>

        <div ref="menu" class="relative ml-auto">
          <button
            type="button"
            class="flex items-center gap-2 rounded-lg px-2 py-1 text-sm text-slate-300 transition hover:bg-white/5"
            aria-haspopup="menu"
            :aria-expanded="menuOpen"
            @click="menuOpen = !menuOpen"
          >
            <span class="grid size-8 place-items-center rounded-full bg-surface-700 font-semibold uppercase">
              {{ auth.user?.username.charAt(0) }}
            </span>
            <span class="hidden sm:inline">{{ auth.user?.username }}</span>
          </button>
          <div
            v-if="menuOpen"
            role="menu"
            class="absolute right-0 mt-2 w-48 rounded-lg bg-surface-800 p-1 shadow-lg ring-1 ring-white/10"
            @click="menuOpen = false"
          >
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

    <RouterView :key="$route.path" />
  </div>
</template>
