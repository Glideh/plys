<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { catalogApi, itemSorts } from '@/api/catalog'
import { librariesApi, type Library, type LibraryKind } from '@/api/libraries'
import { loadLocalServer } from '@/api/federation'
import CatalogGrid from '@/components/CatalogGrid.vue'
import CatalogFilters from '@/components/CatalogFilters.vue'
import ErrorAlert from '@/components/ErrorAlert.vue'
import LibrarySelect from '@/components/LibrarySelect.vue'
import PartialResultsNotice from '@/components/PartialResultsNotice.vue'
import { useCatalogList } from '@/composables/useCatalogList'
import { useErrorMessage } from '@/composables/useErrorMessage'

const props = defineProps<{ kind: LibraryKind }>()
const { t } = useI18n()
const router = useRouter()
const errorMessage = useErrorMessage()

const libraries = ref<Library[]>([])
const kindLibraries = computed(() => libraries.value.filter((l) => l.kind === props.kind))

const { items, total, loading, error, query, sort, filters, unavailableServers, unavailableServerNames, reload } = useCatalogList((req, signal) => catalogApi.listKind(props.kind, req, signal))

function openLibrary(id: string | null) {
  if (id) router.push({ name: 'library', params: { id } })
}

onMounted(async () => {
  try {
    await loadLocalServer()
  } catch (err) {
    error.value = errorMessage(err)
  }
  reload()
  try {
    libraries.value = await librariesApi.list()
  } catch (err) {
    error.value = errorMessage(err)
  }
})
</script>

<template>
  <main class="mx-auto max-w-screen-2xl px-4 py-8 sm:px-8">
    <header class="mb-8 flex flex-wrap items-end gap-4">
      <div class="mr-auto min-w-0">
        <h1>
          <LibrarySelect
            :model-value="null"
            :libraries="kindLibraries"
            :all-label="t(`libraries.kinds.${kind}`)"
            :label="t('catalog.libraryFilter')"
            @update:model-value="openLibrary"
          />
        </h1>
        <p class="mt-1 text-sm text-slate-500">{{ t('catalog.count', total) }}</p>
      </div>
      <input
        v-model="query"
        type="search"
        :placeholder="t('catalog.search')"
        :aria-label="t('catalog.search')"
        class="w-full rounded-lg border border-white/10 bg-surface-800 px-3.5 py-2 text-sm text-slate-100 placeholder-slate-500 focus:border-accent-500 focus:ring-2 focus:ring-accent-500/30 focus:outline-none sm:w-64"
      />
      <label class="flex items-center gap-2 text-sm text-slate-400">
        {{ t('catalog.sort') }}
        <select
          v-model="sort"
          class="rounded-lg border border-white/10 bg-surface-800 px-3 py-2 text-sm text-slate-100 focus:border-accent-500 focus:outline-none"
        >
          <option v-for="s in itemSorts" :key="s" :value="s">{{ t(`catalog.sorts.${s}`) }}</option>
        </select>
      </label>
      <CatalogFilters v-if="kind !== 'other'" v-model="filters" :kind="kind" />
    </header>

    <PartialResultsNotice :count="unavailableServers" :names="unavailableServerNames" class="mb-4" />
    <ErrorAlert v-if="error" class="mb-6" :message="error" />

    <p v-if="!loading && !error && items.length === 0" class="rounded-xl border border-dashed border-white/10 px-6 py-16 text-center text-slate-500">
      {{ query || Object.keys(filters).length ? t('catalog.noResults') : t('catalog.empty') }}
    </p>

    <CatalogGrid :items="items" :loading="loading" :kind="kind" />
    <div ref="sentinel" class="h-px" aria-hidden="true" />
  </main>
</template>
