<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import CatalogGrid from '@/components/CatalogGrid.vue'
import ErrorAlert from '@/components/ErrorAlert.vue'
import PartialResultsNotice from '@/components/PartialResultsNotice.vue'
import type { useGlobalSearch } from '@/composables/useGlobalSearch'

/** Results of a search across every kind and server, grouped by kind (home page and top bar). */
defineProps<{ search: ReturnType<typeof useGlobalSearch> }>()

const { t } = useI18n()
</script>

<template>
  <section class="space-y-10" :aria-label="t('home.searchResults')" aria-live="polite">
    <PartialResultsNotice :count="search.unavailableServers.value" :names="search.unavailableServerNames.value" />
    <ErrorAlert v-if="search.error.value" :message="search.error.value" />
    <CatalogGrid v-else-if="search.loading.value && search.groups.value.length === 0" :items="[]" loading :kind="null" />
    <p v-else-if="!search.loading.value && search.groups.value.length === 0" class="rounded-xl border border-dashed border-white/10 px-6 py-16 text-center text-slate-500">
      {{ t('catalog.noResults') }}
    </p>
    <div v-for="group in search.groups.value" :key="group.kind" :class="{ 'opacity-60 transition': search.loading.value }">
      <div class="mb-4 flex items-baseline gap-3">
        <h2 class="text-lg font-semibold">{{ t(`libraries.kinds.${group.kind}`) }}</h2>
        <span class="text-sm text-slate-500">{{ t('catalog.count', group.total) }}</span>
        <RouterLink
          v-if="group.total > group.items.length"
          :to="{ name: 'kind', params: { kind: group.kind }, query: { q: search.term.value } }"
          class="ml-auto text-sm font-medium text-accent-300 hover:text-accent-200"
        >
          {{ t('home.seeAll') }}
        </RouterLink>
      </div>
      <CatalogGrid :items="group.items" :loading="false" :kind="group.kind" />
    </div>
  </section>
</template>
