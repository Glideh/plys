<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const auth = useAuthStore()
</script>

<template>
  <div class="min-h-full">
    <header class="sticky top-0 z-30 border-b border-white/5 bg-surface-950/80 backdrop-blur">
      <nav class="mx-auto flex h-16 max-w-screen-2xl items-center gap-8 px-4 sm:px-8">
        <RouterLink :to="{ name: 'home' }" class="flex items-center gap-2.5">
          <span class="grid size-8 place-items-center rounded-lg bg-accent-500 text-sm font-black text-surface-950">▶</span>
          <span class="text-lg font-bold tracking-tight">{{ t('app.name') }}</span>
        </RouterLink>

        <RouterLink
          :to="{ name: 'home' }"
          class="text-sm font-medium text-slate-400 transition hover:text-slate-100"
          exact-active-class="!text-slate-100"
        >
          {{ t('nav.home') }}
        </RouterLink>
        <RouterLink
          v-if="auth.isAdmin"
          :to="{ name: 'admin-libraries' }"
          class="-ml-4 text-sm font-medium text-slate-400 transition hover:text-slate-100"
          :class="{ '!text-slate-100': $route.path.startsWith('/admin') }"
        >
          {{ t('nav.admin') }}
        </RouterLink>

        <div class="ml-auto flex items-center gap-4">
          <span class="hidden items-center gap-2 text-sm text-slate-300 sm:flex">
            <span class="grid size-8 place-items-center rounded-full bg-surface-700 font-semibold uppercase">
              {{ auth.user?.username.charAt(0) }}
            </span>
            {{ auth.user?.username }}
          </span>
          <button
            type="button"
            class="rounded-lg px-3 py-1.5 text-sm text-slate-400 transition hover:bg-white/5 hover:text-slate-100"
            @click="auth.logout()"
          >
            {{ t('common.logout') }}
          </button>
        </div>
      </nav>
    </header>

    <RouterView :key="$route.path" />
  </div>
</template>
