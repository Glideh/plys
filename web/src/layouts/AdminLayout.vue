<script setup lang="ts">
import { LibraryBig, MonitorPlay, Server, Settings, Users } from '@lucide/vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const tabs = [
  { name: 'admin-libraries', label: 'admin.libraries', icon: LibraryBig },
  { name: 'admin-users', label: 'admin.users', icon: Users },
  { name: 'admin-playback', label: 'admin.playback', icon: MonitorPlay },
  { name: 'admin-servers', label: 'admin.servers', icon: Server },
  { name: 'admin-settings', label: 'admin.settings', icon: Settings },
] as const
</script>

<template>
  <main class="mx-auto max-w-screen-xl px-4 py-10 sm:px-8">
    <h1 class="text-2xl font-bold tracking-tight">{{ t('nav.admin') }}</h1>
    <!-- Icons only on small viewports, so the five tabs share one row without wrapping or scrolling. -->
    <nav class="mt-6 grid grid-cols-5 border-b border-white/5 md:flex md:gap-1">
      <RouterLink
        v-for="tab in tabs"
        :key="tab.name"
        :to="{ name: tab.name }"
        :aria-label="t(tab.label)"
        :title="t(tab.label)"
        class="-mb-px flex items-center justify-center gap-2 border-b-2 border-transparent px-2 py-2.5 text-sm font-medium text-slate-400 transition hover:text-slate-100 md:px-4"
        active-class="!border-accent-500 !text-slate-100"
      >
        <component :is="tab.icon" class="size-5 shrink-0 md:size-4" aria-hidden="true" />
        <span class="hidden md:inline">{{ t(tab.label) }}</span>
      </RouterLink>
    </nav>
    <div class="mt-8">
      <RouterView />
    </div>
  </main>
</template>
