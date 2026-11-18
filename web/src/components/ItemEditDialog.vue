<script setup lang="ts">
import { computed, onBeforeUnmount, ref, useId, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Plus, X } from '@lucide/vue'
import { catalogApi, type EditedField, type ItemDetail, type MetadataEdit } from '@/api/catalog'
import { useErrorMessage } from '@/composables/useErrorMessage'
import BaseButton from './BaseButton.vue'
import BaseDialog from './BaseDialog.vue'
import ErrorAlert from './ErrorAlert.vue'

const MAX_POSTER_BYTES = 10 * 1024 * 1024

const props = defineProps<{ item: ItemDetail }>()
const emit = defineEmits<{ saved: []; revert: [] }>()
const open = defineModel<boolean>('open', { required: true })
const { t } = useI18n()
const errorMessage = useErrorMessage()
const uid = useId()

interface CastRow {
  key: number
  name: string
  character: string
}

interface Form {
  title: string
  originalTitle: string
  releaseDate: string
  tagline: string
  overview: string
  directors: string
  cast: CastRow[]
}

let nextKey = 0
const form = ref<Form>(emptyForm())
let initial: Form = emptyForm()
const posterFile = ref<File | null>(null)
const posterPreview = ref<string | null>(null)
const posterInput = ref<HTMLInputElement | null>(null)
const saving = ref(false)
const error = ref('')

const hasPoster = computed(() => props.item.kind !== 'video')
const edited = computed(() => new Set<EditedField>(props.item.editedFields ?? []))
// Videos have no online metadata to go back to; neither have titles TMDB did not identify.
const canRevert = computed(() => edited.value.size > 0 && props.item.kind !== 'video' && props.item.tmdbId != null)
const directorsLabel = computed(() => (props.item.kind === 'series' ? t('catalog.createdBy') : t('catalog.directedBy')))

function emptyForm(): Form {
  return { title: '', originalTitle: '', releaseDate: '', tagline: '', overview: '', directors: '', cast: [] }
}

function formFrom(item: ItemDetail): Form {
  return {
    title: item.title,
    originalTitle: item.originalTitle ?? '',
    releaseDate: item.releaseDate?.slice(0, 10) ?? '',
    tagline: item.tagline ?? '',
    overview: item.overview ?? '',
    directors: item.directors.join('\n'),
    cast: item.cast.map((c) => ({ key: nextKey++, name: c.name, character: c.character ?? '' })),
  }
}

function setPosterPreview(url: string | null) {
  if (posterPreview.value) URL.revokeObjectURL(posterPreview.value)
  posterPreview.value = url
}
onBeforeUnmount(() => setPosterPreview(null))

watch(open, (isOpen) => {
  if (!isOpen) return
  initial = formFrom(props.item)
  form.value = formFrom(props.item)
  form.value.cast = initial.cast.map((c) => ({ ...c }))
  posterFile.value = null
  setPosterPreview(null)
  error.value = ''
})

function lines(text: string): string[] {
  return text
    .split('\n')
    .map((l) => l.trim())
    .filter(Boolean)
}

function castOf(rows: CastRow[]) {
  return rows.map((c) => ({ name: c.name.trim(), character: c.character.trim() })).filter((c) => c.name)
}

/** Only the fields that changed are sent, so that the others keep following TMDB. */
function changes(): MetadataEdit {
  const f = form.value
  const out: MetadataEdit = {}
  for (const key of ['title', 'originalTitle', 'releaseDate', 'tagline', 'overview'] as const) {
    if (f[key].trim() !== initial[key].trim()) out[key] = f[key].trim()
  }
  const directors = lines(f.directors)
  if (directors.join('\n') !== lines(initial.directors).join('\n')) out.directors = directors
  const cast = castOf(f.cast)
  if (JSON.stringify(cast) !== JSON.stringify(castOf(initial.cast))) out.cast = cast
  return out
}

function addCast() {
  form.value.cast.push({ key: nextKey++, name: '', character: '' })
}

function removeCast(key: number) {
  form.value.cast = form.value.cast.filter((c) => c.key !== key)
}

function onPosterPicked(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  if (file.size > MAX_POSTER_BYTES) {
    error.value = t('catalog.editPosterTooLarge')
    return
  }
  error.value = ''
  posterFile.value = file
  setPosterPreview(URL.createObjectURL(file))
}

async function save() {
  if (!form.value.title.trim()) {
    error.value = t('catalog.editTitleRequired')
    return
  }
  saving.value = true
  error.value = ''
  try {
    const edit = changes()
    if (Object.keys(edit).length) await catalogApi.editMetadata(props.item.id, edit)
    if (posterFile.value) await catalogApi.setPoster(props.item.id, posterFile.value)
    open.value = false
    emit('saved')
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    saving.value = false
  }
}

const inputClass =
  'block w-full rounded-lg border border-white/10 bg-surface-800 px-3.5 py-2 text-sm text-slate-100 placeholder-slate-500 transition focus:border-accent-500 focus:ring-2 focus:ring-accent-500/30 focus:outline-none'
const labelClass = 'mb-1.5 flex items-center gap-2 text-sm font-medium text-slate-300'
</script>

<template>
  <BaseDialog v-model:open="open" :title="t('catalog.editMetadataTitle')" wide>
    <form :id="`${uid}-form`" class="space-y-4" @submit.prevent="save">
      <p class="text-sm text-slate-400">{{ t('catalog.editHelp') }}</p>

      <div v-if="hasPoster" class="flex items-end gap-4">
        <img
          v-if="posterPreview ?? item.posterUrl"
          :src="posterPreview ?? item.posterUrl ?? ''"
          alt=""
          class="h-36 w-24 shrink-0 rounded-lg object-cover ring-1 ring-white/10"
        />
        <div v-else class="h-36 w-24 shrink-0 rounded-lg bg-surface-800 ring-1 ring-white/10" />
        <div>
          <p :class="labelClass">
            {{ t('catalog.editPoster') }}
            <span v-if="edited.has('poster')" class="rounded-full bg-accent-500/15 px-2 py-0.5 text-xs text-accent-300">{{ t('catalog.edited') }}</span>
          </p>
          <p class="mb-2 text-xs text-slate-500">{{ t('catalog.editPosterHelp') }}</p>
          <BaseButton @click="posterInput?.click()">{{ t('catalog.editPosterChoose') }}</BaseButton>
          <input ref="posterInput" type="file" accept="image/jpeg,image/png" class="hidden" @change="onPosterPicked" />
        </div>
      </div>

      <div>
        <label :for="`${uid}-title`" :class="labelClass">
          {{ t('catalog.editFieldTitle') }}
          <span v-if="edited.has('title')" class="rounded-full bg-accent-500/15 px-2 py-0.5 text-xs text-accent-300">{{ t('catalog.edited') }}</span>
        </label>
        <input :id="`${uid}-title`" v-model="form.title" required maxlength="300" :class="inputClass" />
      </div>

      <div class="grid gap-4 sm:grid-cols-[1fr_auto]">
        <div>
          <label :for="`${uid}-original`" :class="labelClass">
            {{ t('catalog.editFieldOriginalTitle') }}
            <span v-if="edited.has('originalTitle')" class="rounded-full bg-accent-500/15 px-2 py-0.5 text-xs text-accent-300">{{ t('catalog.edited') }}</span>
          </label>
          <input :id="`${uid}-original`" v-model="form.originalTitle" maxlength="300" :class="inputClass" />
        </div>
        <div>
          <label :for="`${uid}-date`" :class="labelClass">
            {{ t('catalog.editFieldReleaseDate') }}
            <span v-if="edited.has('releaseDate')" class="rounded-full bg-accent-500/15 px-2 py-0.5 text-xs text-accent-300">{{ t('catalog.edited') }}</span>
          </label>
          <input :id="`${uid}-date`" v-model="form.releaseDate" type="date" min="1800-01-01" max="2200-12-31" :class="inputClass" class="[color-scheme:dark]" />
        </div>
      </div>

      <div>
        <label :for="`${uid}-tagline`" :class="labelClass">
          {{ t('catalog.editFieldTagline') }}
          <span v-if="edited.has('tagline')" class="rounded-full bg-accent-500/15 px-2 py-0.5 text-xs text-accent-300">{{ t('catalog.edited') }}</span>
        </label>
        <input :id="`${uid}-tagline`" v-model="form.tagline" maxlength="500" :class="inputClass" />
      </div>

      <div>
        <label :for="`${uid}-overview`" :class="labelClass">
          {{ t('catalog.editFieldOverview') }}
          <span v-if="edited.has('overview')" class="rounded-full bg-accent-500/15 px-2 py-0.5 text-xs text-accent-300">{{ t('catalog.edited') }}</span>
        </label>
        <textarea :id="`${uid}-overview`" v-model="form.overview" rows="5" maxlength="10000" :class="inputClass" />
      </div>

      <div>
        <label :for="`${uid}-directors`" :class="labelClass">
          {{ directorsLabel }}
          <span v-if="edited.has('directors')" class="rounded-full bg-accent-500/15 px-2 py-0.5 text-xs text-accent-300">{{ t('catalog.edited') }}</span>
        </label>
        <textarea :id="`${uid}-directors`" v-model="form.directors" rows="2" :aria-describedby="`${uid}-directors-hint`" :class="inputClass" />
        <p :id="`${uid}-directors-hint`" class="mt-1.5 text-xs text-slate-500">{{ t('catalog.editOnePerLine') }}</p>
      </div>

      <fieldset>
        <legend :class="labelClass">
          {{ t('catalog.cast') }}
          <span v-if="edited.has('cast')" class="rounded-full bg-accent-500/15 px-2 py-0.5 text-xs text-accent-300">{{ t('catalog.edited') }}</span>
        </legend>
        <ul class="space-y-2">
          <li v-for="(c, i) in form.cast" :key="c.key" class="flex gap-2">
            <input v-model="c.name" maxlength="200" :aria-label="t('catalog.editCastName', { n: i + 1 })" :placeholder="t('catalog.editCastNamePlaceholder')" :class="inputClass" />
            <input
              v-model="c.character"
              maxlength="200"
              :aria-label="t('catalog.editCastCharacter', { n: i + 1 })"
              :placeholder="t('catalog.editCastCharacterPlaceholder')"
              :class="inputClass"
            />
            <button
              type="button"
              class="grid size-9 shrink-0 place-items-center rounded-lg text-slate-400 transition hover:bg-white/5 hover:text-slate-100"
              :aria-label="t('catalog.editCastRemove', { name: c.name || i + 1 })"
              :title="t('catalog.editCastRemove', { name: c.name || i + 1 })"
              @click="removeCast(c.key)"
            >
              <X class="size-4" aria-hidden="true" />
            </button>
          </li>
        </ul>
        <BaseButton class="mt-2" :disabled="form.cast.length >= 50" @click="addCast">
          <Plus class="size-4" aria-hidden="true" />
          {{ t('catalog.editCastAdd') }}
        </BaseButton>
      </fieldset>

      <ErrorAlert v-if="error" :message="error" />
    </form>

    <template #footer>
      <BaseButton v-if="canRevert" class="mr-auto" :disabled="saving" @click="emit('revert')">{{ t('catalog.editRevert') }}</BaseButton>
      <BaseButton :disabled="saving" @click="open = false">{{ t('common.cancel') }}</BaseButton>
      <BaseButton type="submit" variant="primary" :form="`${uid}-form`" :loading="saving">{{ t('common.save') }}</BaseButton>
    </template>
  </BaseDialog>
</template>
