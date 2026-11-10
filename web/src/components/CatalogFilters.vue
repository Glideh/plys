<script setup lang="ts">
import { computed, onMounted, ref, useId, useTemplateRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Check, Filter, RotateCcw, X } from '@lucide/vue'
import { catalogApi } from '@/api/catalog'
import { choiceKeys, choiceValues, filterCount, numberKeys, parseFilters, selectionKeys, serializeFilters, type CatalogFilters, type ChoiceKey, type FilterCategory, type FilterOption, type NumberKey, type SelectionKey } from '@/api/catalogFilters'
import type { LibraryKind } from '@/api/libraries'
import BaseButton from './BaseButton.vue'
import CatalogFilterPicker from './CatalogFilterPicker.vue'

const props = defineProps<{ kind: LibraryKind; library?: string }>()
const model = defineModel<CatalogFilters>({ required: true })
const { t, locale } = useI18n()
const id = useId()
const open = ref(false)
const draft = ref<CatalogFilters>({})
const validation = ref('')
const panel = useTemplateRef<HTMLDialogElement>('panel')
const labels = ref<Partial<Record<FilterCategory, Record<string, string>>>>({})
const count = computed(() => filterCount(model.value))
const pickerKeys = computed(() => selectionKeys)
// Related bounds are shown side by side under one heading.
interface NumberGroup { title: string; fields: { key: NumberKey; label: string }[] }
const numberGroups = computed<NumberGroup[]>(() => [
  { title: 'filters.yearGroup', fields: [{ key: 'yearMin', label: 'filters.from' }, { key: 'yearMax', label: 'filters.to' }] },
  { title: 'filters.ratingGroup', fields: [{ key: 'ratingMin', label: 'filters.minRating' }, { key: 'votesMin', label: 'filters.minVotes' }] },
  ...(props.kind === 'movies' ? [{ title: 'filters.runtimeGroup', fields: [{ key: 'runtimeMin', label: 'filters.runtimeFrom' }, { key: 'runtimeMax', label: 'filters.runtimeTo' }] } satisfies NumberGroup] : []),
])
const inputClassNoMarginTop = 'h-10 w-full rounded-lg border border-white/10 bg-surface-800 px-3 text-sm text-slate-100'
const inputClass = 'mt-1.5 ' + inputClassNoMarginTop

function category(key: SelectionKey): FilterCategory { return key === 'excludeGenres' ? 'genres' : key }
function fieldLabel(key: SelectionKey | NumberKey | ChoiceKey) {
  if (key === 'creators') return t(props.kind === 'movies' ? 'filters.directors' : 'filters.creators')
  if (key === 'countries') return t(props.kind === 'movies' ? 'filters.productionCountries' : 'filters.originCountries')
  return t(`filters.${key}`)
}
function values(key: SelectionKey) { return (draft.value[key] ?? []).map(String) }
function setValues(key: SelectionKey, selected: string[]) {
  if (key === 'countries' || key === 'languages') draft.value[key] = selected
  else draft.value[key] = selected.map(Number)
}
function remember(key: FilterCategory, options: FilterOption[]) {
  labels.value[key] = { ...labels.value[key], ...Object.fromEntries(options.map((option) => [option.id, option.label])) }
}
function optionLabel(key: SelectionKey, value: string | number) {
  if (key === 'countries' || key === 'languages') {
    try { return new Intl.DisplayNames([locale.value], { type: key === 'countries' ? 'region' : 'language' }).of(String(value)) ?? String(value) } catch { return String(value) }
  }
  return labels.value[category(key)]?.[String(value)] ?? `#${value}`
}
const active = computed(() => [
  ...selectionKeys.flatMap((key) => (model.value[key] ?? []).map((value) => ({ key, value, text: `${fieldLabel(key)}: ${optionLabel(key, value)}` }))),
  ...numberKeys.filter((key) => model.value[key] !== undefined).map((key) => ({ key, value: undefined, text: `${fieldLabel(key)}: ${model.value[key]}` })),
  ...choiceKeys.filter((key) => model.value[key]).map((key) => ({ key, value: undefined, text: `${fieldLabel(key)}: ${choiceLabel(key, model.value[key]!)}` })),
])
function choiceLabel(key: ChoiceKey, value: string) { return t(`filters.${key}Values.${value}`) }
function setChoice(key: ChoiceKey, event: Event) {
  const value = (event.target as HTMLSelectElement).value
  if (value) draft.value[key] = value as never
  else delete draft.value[key]
}
function remove(key: SelectionKey | NumberKey | ChoiceKey, value?: string | number) {
  const next = { ...model.value }
  if (selectionKeys.includes(key as SelectionKey)) {
    const selection = key as SelectionKey
    setModelSelection(next, selection, value)
  } else delete next[key as NumberKey | ChoiceKey]
  model.value = next
}
function setModelSelection(next: CatalogFilters, key: SelectionKey, value?: string | number) {
  if (key === 'languages' || key === 'countries') next[key] = next[key]?.filter((entry) => entry !== value)
  else next[key] = next[key]?.filter((entry) => entry !== value)
}
function show() {
  draft.value = parseFilters(serializeFilters(model.value))
  // One value per criterion, like on Android TV; older shared links may hold several.
  for (const key of selectionKeys) if (draft.value[key]?.length) draft.value[key] = draft.value[key].slice(0, 1) as never
  validation.value = ''
  open.value = true
}
function setNumber(key: NumberKey, event: Event) {
  const value = (event.target as HTMLInputElement).valueAsNumber
  if (Number.isFinite(value)) draft.value[key] = value
  else delete draft.value[key]
}
function apply() {
  for (const [minimum, maximum] of [['yearMin', 'yearMax'], ['runtimeMin', 'runtimeMax']] as const) {
    if (draft.value[minimum] !== undefined && draft.value[maximum] !== undefined && draft.value[minimum]! > draft.value[maximum]!) {
      validation.value = t('filters.invalidRange'); return
    }
  }
  if (props.kind !== 'movies') { delete draft.value.runtimeMin; delete draft.value.runtimeMax }
  model.value = parseFilters(serializeFilters(draft.value))
  open.value = false
}
watch([open, panel], ([visible, dialog]) => {
  if (!dialog) return
  if (visible && !dialog.open) {
    if (window.matchMedia('(max-width: 639px)').matches) dialog.showModal()
    else dialog.show()
  }
  if (!visible && dialog.open) dialog.close()
}, { flush: 'post' })
async function restoreLabels() {
  await Promise.all(selectionKeys.map(async (key) => {
    const selected = model.value[key]
    if (!selected?.length) return
    try {
      const page = await catalogApi.filterOptions(props.kind, props.library, category(key), '', selected.map(String), locale.value)
      remember(category(key), page.options[category(key)] ?? [])
    } catch {}
  }))
}
onMounted(restoreLabels)
watch(() => serializeFilters(model.value), restoreLabels)
</script>

<template>
  <section class="contents" :aria-label="t('filters.title')">
    <div class="flex flex-wrap items-center gap-3">
      <BaseButton :aria-expanded="open" :aria-controls="id" @click="open ? open = false : show()">
        <Filter class="size-4" aria-hidden="true" />{{ t('filters.title') }}<span v-if="count" class="text-accent-400">{{ count }}</span>
      </BaseButton>
      <button v-if="count" type="button" class="inline-flex size-9 items-center justify-center rounded-lg text-slate-400 hover:bg-white/5" :title="t('filters.reset')" :aria-label="t('filters.reset')" @click="model = {}"><RotateCcw class="size-4" aria-hidden="true" /></button>
    </div>
    <div v-if="active.length" class="flex w-full flex-wrap gap-2">
      <button v-for="entry in active" :key="`${entry.key}-${entry.value}`" type="button" class="inline-flex max-w-full items-center gap-2 rounded border border-white/10 px-2 py-1 text-xs text-slate-300 hover:bg-white/5" :aria-label="t('filters.remove', { label: entry.text })" @click="remove(entry.key, entry.value)">
        <span class="min-w-0 break-words">{{ entry.text }}</span><X class="size-3 shrink-0" aria-hidden="true" />
      </button>
    </div>
    <dialog :id="id" ref="panel" class="fixed inset-0 z-50 m-0 h-dvh max-h-dvh w-full max-w-none flex-col overflow-hidden border-0 bg-surface-900 p-4 text-slate-100 backdrop:bg-black/70 open:flex sm:static sm:z-auto sm:h-auto sm:max-h-none sm:overflow-visible sm:border-y sm:border-white/10 sm:bg-transparent sm:px-0 sm:py-5" :aria-labelledby="`${id}-title`" @close="open = false" @cancel="open = false">
      <form v-if="open" class="flex min-h-0 flex-1 flex-col" @submit.prevent="apply">
        <header class="mb-5 flex shrink-0 items-center justify-between gap-3">
          <h2 :id="`${id}-title`" class="text-base font-semibold">{{ t('filters.title') }}</h2>
          <button type="button" class="grid size-9 shrink-0 place-items-center rounded-lg hover:bg-white/5" :aria-label="t('common.close')" :title="t('common.close')" @click="open = false"><X class="size-5" aria-hidden="true" /></button>
        </header>
        <div class="grid min-h-0 min-w-0 flex-1 gap-5 overflow-y-auto sm:grid-cols-2 sm:overflow-visible lg:grid-cols-3">
          <CatalogFilterPicker v-for="key in pickerKeys" :key="key" :kind="kind" :library="library" :category="category(key)" :label="fieldLabel(key)" :model-value="values(key)" :disabled-values="key === 'genres' ? values('excludeGenres') : key === 'excludeGenres' ? values('genres') : undefined" @update:model-value="setValues(key, $event)" @options="remember" />
          <div v-for="key in choiceKeys" :key="key" class="min-w-0">
            <select :value="draft[key] ?? ''" :class="inputClassNoMarginTop" @change="setChoice(key, $event)">
              <option value="">{{ fieldLabel(key) }}</option>
              <option v-for="value in choiceValues[key]" :key="value" :value="value">{{ choiceLabel(key, value) }}</option>
            </select>
          </div>
          <fieldset v-for="group in numberGroups" :key="group.title" class="min-w-0 rounded-lg border border-white/10 px-3 pt-1 pb-3">
            <legend class="px-1 text-sm text-slate-300">{{ t(group.title) }}</legend>
            <div class="grid grid-cols-2 gap-3">
              <label v-for="field in group.fields" :key="field.key" class="min-w-0 text-xs text-slate-400" :for="`${id}-${field.key}`">
                {{ t(field.label) }}
                <input :id="`${id}-${field.key}`" type="number" :value="draft[field.key]" :min="field.key.startsWith('year') ? 1800 : 0" :max="field.key === 'ratingMin' ? 10 : field.key.startsWith('year') ? 9999 : field.key === 'votesMin' ? 2147483647 : 10000" :step="field.key === 'ratingMin' ? 0.1 : 1" :class="inputClass" @input="setNumber(field.key, $event)" />
              </label>
            </div>
          </fieldset>
        </div>
        <p v-if="validation" class="mt-4 text-sm text-red-300" role="alert">{{ validation }}</p>
        <footer class="mt-4 flex shrink-0 flex-wrap justify-end gap-2 border-t border-white/5 pt-4">
          <BaseButton :title="t('filters.reset')" :aria-label="t('filters.reset')" @click="draft = {}; validation = ''"><RotateCcw class="size-4" aria-hidden="true" /></BaseButton>
          <BaseButton @click="open = false">{{ t('common.cancel') }}</BaseButton>
          <BaseButton type="submit" variant="primary"><Check class="size-4" aria-hidden="true" />{{ t('filters.apply') }}</BaseButton>
        </footer>
      </form>
    </dialog>
  </section>
</template>