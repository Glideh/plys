<script setup lang="ts">
import { computed, onUnmounted, ref, useId, useTemplateRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronDown, Search, X } from '@lucide/vue'
import { catalogApi } from '@/api/catalog'
import type { FilterCategory, FilterOption } from '@/api/catalogFilters'
import type { LibraryKind } from '@/api/libraries'
import { useErrorMessage } from '@/composables/useErrorMessage'

const props = defineProps<{ kind: LibraryKind; library?: string; category: FilterCategory; label: string; disabledValues?: string[] }>()
const model = defineModel<string[]>({ required: true })
const emit = defineEmits<{ options: [category: FilterCategory, options: FilterOption[]] }>()
const { t, locale } = useI18n()
const errorMessage = useErrorMessage()
const id = useId()
const searchInput = useTemplateRef<HTMLInputElement>('searchInput')
const query = ref('')
const options = ref<FilterOption[]>([])
const known = ref<Record<string, string>>({})
const loading = ref(false)
const error = ref('')
let generation = 0
let timer: ReturnType<typeof setTimeout> | undefined

function display(option: FilterOption) {
  if (props.category === 'countries' || props.category === 'languages') {
    try { return new Intl.DisplayNames([locale.value], { type: props.category === 'countries' ? 'region' : 'language' }).of(option.id) ?? option.label } catch { return option.label }
  }
  return option.label
}
const selected = computed(() => model.value.map((value) => ({ id: value, label: known.value[value] ?? value })))

async function load() {
  const current = ++generation
  loading.value = true
  error.value = ''
  try {
    const page = await catalogApi.filterOptions(props.kind, props.library, props.category, query.value.trim(), model.value, locale.value)
    if (current !== generation) return
    options.value = page.options[props.category] ?? []
    for (const option of options.value) known.value[option.id] = option.label
    emit('options', props.category, options.value)
  } catch (err) { if (current === generation) error.value = errorMessage(err) }
  finally { if (current === generation) loading.value = false }
}
function toggle(value: string) {
  model.value = model.value.includes(value) ? model.value.filter((entry) => entry !== value) : [...model.value, value]
}
function focusSearch(event: Event) {
  if ((event.currentTarget as HTMLDetailsElement).open) searchInput.value?.focus()
}
watch(query, () => { ++generation; clearTimeout(timer); timer = setTimeout(load, 300) })
watch(() => [props.kind, props.library, props.category, locale.value], load, { immediate: true })
onUnmounted(() => { ++generation; clearTimeout(timer) })
</script>

<template>
  <div class="min-w-0">
    <details class="group relative" @toggle="focusSearch">
      <summary class="flex min-h-10 cursor-pointer list-none items-center justify-between gap-2 rounded-lg border border-white/10 bg-surface-800 px-3 py-2 text-sm text-slate-200">
        <span>{{ label }}<span v-if="model.length" class="ml-2 text-accent-400">{{ model.length }}</span></span>
        <ChevronDown class="size-4 shrink-0 transition group-open:rotate-180" aria-hidden="true" />
      </summary>
      <div class="mt-2 border-l border-white/10 pl-3">
        <label :for="id" class="sr-only">{{ t('filters.searchIn', { label }) }}</label>
        <div class="relative">
          <Search class="pointer-events-none absolute top-3 left-2 size-4 text-slate-500" aria-hidden="true" />
          <input :id="id" ref="searchInput" v-model="query" type="search" maxlength="200" autocomplete="off" :placeholder="t('catalog.search')" class="h-10 w-full rounded-lg border border-white/10 bg-surface-800 pr-3 pl-8 text-sm" />
        </div>
        <p v-if="loading" class="py-2 text-xs text-slate-400" role="status">{{ t('common.loading') }}</p>
        <p v-if="error" class="py-2 text-xs text-red-300" role="alert">{{ error }}</p>
        <div class="mt-2 max-h-44 overflow-y-auto">
          <label v-for="option in options" :key="option.id" class="flex min-h-9 cursor-pointer items-start gap-2 py-1.5 text-sm text-slate-300">
            <input type="checkbox" class="mt-1 accent-accent-500" :checked="model.includes(option.id)" :disabled="disabledValues?.includes(option.id) || (!model.includes(option.id) && model.length >= 20)" @change="toggle(option.id)" />
            <span class="min-w-0 break-words">{{ display(option) }}</span>
          </label>
          <p v-if="!loading && !error && !options.length" class="py-2 text-xs text-slate-500">{{ t('catalog.noResults') }}</p>
        </div>
      </div>
    </details>
    <div v-if="selected.length" class="mt-2 flex flex-wrap gap-1.5">
      <button v-for="option in selected" :key="option.id" type="button" class="inline-flex max-w-full items-center gap-1 rounded border border-white/10 px-2 py-1 text-xs text-slate-300 hover:bg-white/5" :aria-label="t('filters.remove', { label: display(option) })" @click="toggle(option.id)">
        <span class="min-w-0 break-words">{{ display(option) }}</span><X class="size-3 shrink-0" aria-hidden="true" />
      </button>
    </div>
  </div>
</template>