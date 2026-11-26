<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const tabs = [
  { name: 'admin-libraries', label: 'admin.libraries' },
  { name: 'admin-users', label: 'admin.users' },
  { name: 'admin-playback', label: 'admin.playback' },
  { name: 'admin-servers', label: 'admin.servers' },
  { name: 'admin-settings', label: 'admin.settings' },
] as const
</script>

<template>
  <main class="mx-auto max-w-screen-xl px-4 py-10 sm:px-8">
    <h1 class="text-2xl font-bold tracking-tight">{{ t('nav.admin') }}</h1>
    <nav class="mt-6 flex flex-wrap gap-1 border-b border-white/5">
      <RouterLink
        v-for="tab in tabs"
        :key="tab.name"
        :to="{ name: tab.name }"
        class="-mb-px border-b-2 border-transparent px-4 py-2.5 text-sm font-medium text-slate-400 transition hover:text-slate-100"
        active-class="!border-accent-500 !text-slate-100"
      >
        {{ t(tab.label) }}
      </RouterLink>
    </nav>
    <div class="mt-8">
      <RouterView />
    </div>
  </main>
</template>
