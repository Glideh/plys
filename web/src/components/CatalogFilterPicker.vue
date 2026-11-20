<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, useId, useTemplateRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronDown, Search } from '@lucide/vue'
import { catalogApi } from '@/api/catalog'
import type { FilterCategory, FilterOption } from '@/api/catalogFilters'
import type { LibraryKind } from '@/api/libraries'
import { useErrorMessage } from '@/composables/useErrorMessage'

const props = defineProps<{
  kind?: LibraryKind
  library?: string
  category?: FilterCategory
  label: string
  labelAbove?: boolean
  disabledValues?: string[]
  staticOptions?: FilterOption[]
}>()
const model = defineModel<string[]>({ required: true })
const emit = defineEmits<{ options: [category: FilterCategory, options: FilterOption[]] }>()
const { t, locale } = useI18n()
const errorMessage = useErrorMessage()
const id = useId()
const searchInput = useTemplateRef<HTMLInputElement>('searchInput')
const root = useTemplateRef<HTMLElement>('root')
const query = ref('')
const open = ref(false)
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
// Single choice, like on Android TV: the API still takes a list, holding at most one value here.
const selected = computed(() => {
  const value = model.value[0]
  return value === undefined ? undefined : display({ id: value, label: known.value[value] ?? value })
})

async function load() {
  const current = ++generation
  error.value = ''
  if (props.staticOptions) {
    const term = query.value.trim().toLocaleLowerCase()
    options.value = props.staticOptions.filter((option) => !term || option.label.toLocaleLowerCase().includes(term))
    for (const option of props.staticOptions) known.value[option.id] = option.label
    loading.value = false
    return
  }
  if (!props.kind || !props.category) return
  loading.value = true
  try {
    const page = await catalogApi.filterOptions(props.kind, props.library, props.category, query.value.trim(), model.value, locale.value)
    if (current !== generation) return
    options.value = page.options[props.category] ?? []
    for (const option of options.value) known.value[option.id] = option.label
    emit('options', props.category, options.value)
  } catch (err) { if (current === generation) error.value = errorMessage(err) }
  finally { if (current === generation) loading.value = false }
}
function choose(value?: string) {
  model.value = value === undefined ? [] : [value]
  open.value = false
}
async function toggle() {
  open.value = !open.value
  if (open.value) { await nextTick(); searchInput.value?.focus() }
}
function closeOnOutsideClick(event: PointerEvent) {
  if (root.value && !root.value.contains(event.target as Node)) open.value = false
}
function closeOnEscape(event: KeyboardEvent) {
  if (event.key === 'Escape') open.value = false
}
watch(query, () => { ++generation; clearTimeout(timer); timer = setTimeout(load, 300) })
watch(() => [props.kind, props.library, props.category, locale.value, props.staticOptions], load, { immediate: true })
onMounted(() => {
  document.addEventListener('pointerdown', closeOnOutsideClick)
  document.addEventListener('keydown', closeOnEscape)
})
onUnmounted(() => {
  ++generation
  clearTimeout(timer)
  document.removeEventListener('pointerdown', closeOnOutsideClick)
  document.removeEventListener('keydown', closeOnEscape)
})
</script>

<template>
  <div ref="root" class="relative min-w-0">
    <span v-if="labelAbove" class="mb-1.5 block text-xs text-slate-400">{{ label }}</span>
    <button
      type="button" class="flex h-10 w-full items-center justify-between gap-2 rounded-lg border border-white/10 bg-surface-800 px-3 py-2 text-left text-sm text-slate-200"
      aria-haspopup="listbox" :aria-expanded="open" :aria-controls="id" @click="toggle"
    >
      <span class="min-w-0 truncate">
        <template v-if="labelAbove"><span :class="selected ? 'text-accent-400' : 'text-slate-400'">{{ selected ?? t('filters.none') }}</span></template>
        <template v-else>{{ label }}<template v-if="selected"> : <span class="text-accent-400">{{ selected }}</span></template></template>
      </span>
      <ChevronDown class="size-4 shrink-0 transition" :class="{ 'rotate-180': open }" aria-hidden="true" />
    </button>
    <div v-if="open" :id="id" class="absolute top-full right-0 left-0 z-30 mt-2 rounded-lg bg-surface-800 p-2 shadow-2xl ring-1 ring-white/10">
      <label :for="`${id}-search`" class="sr-only">{{ t('filters.searchIn', { label }) }}</label>
      <div class="relative">
        <Search class="pointer-events-none absolute top-3 left-2 size-4 text-slate-500" aria-hidden="true" />
        <input :id="`${id}-search`" ref="searchInput" v-model="query" type="search" maxlength="200" autocomplete="off" :placeholder="t('catalog.search')" class="h-10 w-full rounded-lg border border-white/10 bg-surface-900 pr-3 pl-8 text-sm" />
      </div>
      <p v-if="loading" class="px-2 py-2 text-xs text-slate-400" role="status">{{ t('common.loading') }}</p>
      <p v-if="error" class="px-2 py-2 text-xs text-red-300" role="alert">{{ error }}</p>
      <div class="mt-2 max-h-52 overflow-y-auto" role="listbox" :aria-label="label">
        <button type="button" role="option" :aria-selected="!model.length" class="block w-full rounded-md px-3 py-2 text-left text-sm text-slate-400 transition hover:bg-white/5 hover:text-slate-100" @click="choose()">
          {{ t('filters.none') }}
        </button>
        <button v-for="option in options" :key="option.id" type="button" role="option" :aria-selected="model.includes(option.id)" :disabled="disabledValues?.includes(option.id)" class="block w-full rounded-md px-3 py-2 text-left text-sm text-slate-300 transition hover:bg-white/5 hover:text-slate-100 disabled:cursor-not-allowed disabled:opacity-40" @click="choose(option.id)">
          {{ display(option) }}
        </button>
        <p v-if="!loading && !error && !options.length" class="px-3 py-2 text-xs text-slate-500">{{ t('catalog.noResults') }}</p>
      </div>
    </div>
  </div>
</template>
