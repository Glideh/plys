<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Library, LibraryKind } from '@/api/libraries'

// Pills to switch between the libraries of one kind, "All" leading to the kind catalog.
// Hidden while there is a single library, where "All" and that library are the same list.
const props = defineProps<{ kind: LibraryKind; libraries: Library[]; current?: string }>()
const { t } = useI18n()

const kindLibraries = computed(() => props.libraries.filter((l) => l.kind === props.kind))

const pill = 'inline-flex max-w-full items-center rounded-full border px-3.5 py-1.5 text-sm font-medium transition focus-visible:ring-2 focus-visible:ring-accent-500/40 focus-visible:outline-none'
const idle = 'border-white/10 bg-surface-800 text-slate-300 hover:border-white/20 hover:text-slate-100'
const active = 'border-accent-500 bg-accent-500/15 text-accent-300'
</script>

<template>
  <nav v-if="kindLibraries.length > 1" :aria-label="t('catalog.libraryFilter')" class="mb-6">
    <ul class="flex flex-wrap gap-2">
      <li>
        <RouterLink
          :to="{ name: 'kind', params: { kind } }"
          :class="[pill, current ? idle : active]"
          :aria-current="current ? undefined : 'page'"
        >
          {{ t('catalog.allLibraries') }}
        </RouterLink>
      </li>
      <li v-for="lib in kindLibraries" :key="lib.id" class="min-w-0">
        <RouterLink
          :to="{ name: 'library', params: { id: lib.id } }"
          :class="[pill, lib.id === current ? active : idle]"
          :aria-current="lib.id === current ? 'page' : undefined"
        >
          <span class="truncate">{{ lib.name }}</span>
        </RouterLink>
      </li>
    </ul>
  </nav>
</template>
