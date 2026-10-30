<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const auth = useAuthStore()

// Placeholder rails until the catalogue and progress endpoints exist.
const sections = [
  { title: 'home.continueWatching', empty: 'home.emptyContinue' },
  { title: 'home.nextUp', empty: 'home.emptyNextUp' },
] as const
</script>

<template>
  <main class="mx-auto max-w-screen-2xl space-y-12 px-4 py-10 sm:px-8">
    <section v-for="s in sections" :key="s.title">
      <h2 class="mb-4 text-lg font-semibold">{{ t(s.title) }}</h2>
      <p class="rounded-xl border border-dashed border-white/10 px-6 py-10 text-center text-sm text-slate-500">
        {{ t(s.empty) }}
      </p>
    </section>

    <section>
      <h2 class="mb-4 text-lg font-semibold">{{ t('home.libraries') }}</h2>
      <div class="rounded-xl border border-dashed border-white/10 px-6 py-10 text-center">
        <p class="text-sm text-slate-400">{{ t('home.emptyLibraries') }}</p>
        <p v-if="auth.isAdmin" class="mt-1 text-sm text-slate-500">{{ t('home.emptyLibrariesAdmin') }}</p>
      </div>
    </section>
  </main>
</template>
