<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { catalogApi, itemSorts } from '@/api/catalog'
import { librariesApi, type Library } from '@/api/libraries'
import CatalogGrid from '@/components/CatalogGrid.vue'
import CatalogFilters from '@/components/CatalogFilters.vue'
import ErrorAlert from '@/components/ErrorAlert.vue'
import LibrarySelect from '@/components/LibrarySelect.vue'
import { useCatalogList } from '@/composables/useCatalogList'
import { useErrorMessage } from '@/composables/useErrorMessage'

const props = defineProps<{ id: string }>()
const { t } = useI18n()
const router = useRouter()
const errorMessage = useErrorMessage()

const library = ref<Library | null>(null)
const libraries = ref<Library[]>([])
const sameKindLibraries = computed(() => libraries.value.filter((l) => l.kind === library.value?.kind))

const { items, total, loading, error, query, sort, filters, reload } = useCatalogList((req) => catalogApi.list(props.id, req))

function selectLibrary(id: string | null) {
  if (id) router.push({ name: 'library', params: { id } })
}

onMounted(async () => {
  try {
    const [current, all] = await Promise.all([librariesApi.get(props.id), librariesApi.list()])
    library.value = current
    libraries.value = all
  } catch (err) {
    error.value = errorMessage(err)
    return
  }
  reload()
})
</script>

<template>
  <main class="mx-auto max-w-screen-2xl px-4 py-8 sm:px-8">
    <header class="mb-8 flex flex-wrap items-end gap-4">
      <div class="mr-auto min-w-0">
        <RouterLink
          v-if="library"
          :to="{ name: 'kind', params: { kind: library.kind } }"
          class="text-xs font-medium tracking-widest text-accent-400 uppercase hover:underline"
        >
          {{ t(`libraries.kinds.${library.kind}`) }}
        </RouterLink>
        <h1 v-if="library">
          <LibrarySelect
            :model-value="library.id"
            :libraries="sameKindLibraries"
            :label="t('catalog.libraryFilter')"
            @update:model-value="selectLibrary"
          />
        </h1>
        <p v-if="library" class="mt-1 text-sm text-slate-500">{{ t('catalog.count', total) }}</p>
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
      <CatalogFilters v-if="library && library.kind !== 'other'" v-model="filters" :kind="library.kind" :library="id" />
    </header>

    <ErrorAlert v-if="error" class="mb-6" :message="error" />

    <p v-if="!loading && !error && items.length === 0" class="rounded-xl border border-dashed border-white/10 px-6 py-16 text-center text-slate-500">
      {{ query || Object.keys(filters).length ? t('catalog.noResults') : t('catalog.empty') }}
    </p>

    <CatalogGrid :items="items" :loading="loading" :kind="library?.kind ?? null" />
    <div ref="sentinel" class="h-px" aria-hidden="true" />
  </main>
</template>
