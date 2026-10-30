<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { libraryKinds, librariesApi, type Library, type LibraryKind } from '@/api/libraries'
import { useErrorMessage } from '@/composables/useErrorMessage'
import BaseButton from './BaseButton.vue'
import BaseDialog from './BaseDialog.vue'
import ErrorAlert from './ErrorAlert.vue'
import FolderPicker from './FolderPicker.vue'
import TextField from './TextField.vue'

/** Edits the given library, or creates one when null. */
const props = defineProps<{ library: Library | null }>()
const emit = defineEmits<{ saved: [library: Library] }>()
const open = defineModel<boolean>('open', { required: true })
const { t } = useI18n()
const errorMessage = useErrorMessage()

const name = ref('')
const kind = ref<LibraryKind>('movies')
const paths = ref<string[]>([])
const picking = ref(false)
const loading = ref(false)
const error = ref('')

watch(open, (isOpen) => {
  if (!isOpen) return
  name.value = props.library?.name ?? ''
  kind.value = props.library?.kind ?? 'movies'
  paths.value = [...(props.library?.paths ?? [])]
  picking.value = paths.value.length === 0
  error.value = ''
})

function addPath(path: string) {
  if (!paths.value.includes(path)) paths.value.push(path)
  picking.value = false
}

async function submit() {
  error.value = ''
  loading.value = true
  try {
    const saved = props.library
      ? await librariesApi.update(props.library.id, { name: name.value, paths: paths.value })
      : await librariesApi.create({ name: name.value, kind: kind.value, paths: paths.value })
    emit('saved', saved)
    open.value = false
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <BaseDialog v-model:open="open" :title="library ? t('libraries.edit') : t('libraries.new')" wide>
    <form id="library-form" class="space-y-6" @submit.prevent="submit">
      <TextField v-model="name" :label="t('libraries.name')" required :maxlength="100" />

      <fieldset>
        <legend class="mb-1.5 text-sm font-medium text-slate-300">{{ t('libraries.kind') }}</legend>
        <div class="grid gap-3 sm:grid-cols-3">
          <label
            v-for="k in libraryKinds"
            :key="k"
            :class="[
              kind === k ? 'border-accent-500 bg-accent-500/10' : 'border-white/10 hover:border-white/20',
              library ? 'cursor-not-allowed opacity-60' : 'cursor-pointer',
            ]"
            class="relative rounded-xl border p-3.5 transition has-focus-visible:outline-2 has-focus-visible:outline-accent-400"
          >
            <input v-model="kind" type="radio" name="kind" :value="k" :disabled="!!library" class="sr-only" />
            <span class="block font-semibold">{{ t(`libraries.kinds.${k}`) }}</span>
            <span class="mt-1 block text-xs leading-snug text-slate-400">{{ t(`libraries.kindHints.${k}`) }}</span>
          </label>
        </div>
        <p v-if="library" class="mt-2 text-xs text-slate-500">{{ t('libraries.kindLocked') }}</p>
      </fieldset>

      <fieldset>
        <legend class="mb-1.5 text-sm font-medium text-slate-300">{{ t('libraries.folders') }}</legend>
        <ul v-if="paths.length" class="mb-3 space-y-2">
          <li v-for="p in paths" :key="p" class="flex items-center gap-3 rounded-lg bg-surface-800 px-3.5 py-2">
            <span class="flex-1 truncate font-mono text-sm" :title="p">{{ p }}</span>
            <button
              type="button"
              class="grid size-7 place-items-center rounded-md text-slate-400 transition hover:bg-red-500/15 hover:text-red-300"
              :aria-label="t('libraries.removeFolder', { path: p })"
              @click="paths = paths.filter((x) => x !== p)"
            >
              <svg viewBox="0 0 20 20" fill="currentColor" class="size-4" aria-hidden="true">
                <path d="M6.28 5.22a.75.75 0 0 0-1.06 1.06L8.94 10l-3.72 3.72a.75.75 0 1 0 1.06 1.06L10 11.06l3.72 3.72a.75.75 0 1 0 1.06-1.06L11.06 10l3.72-3.72a.75.75 0 0 0-1.06-1.06L10 8.94 6.28 5.22Z" />
              </svg>
            </button>
          </li>
        </ul>
        <p v-else-if="!picking" class="mb-3 text-sm text-slate-500">{{ t('libraries.noFolders') }}</p>

        <FolderPicker v-if="picking" @select="addPath" @cancel="picking = false" />
        <BaseButton v-else @click="picking = true">+ {{ t('libraries.addFolder') }}</BaseButton>
      </fieldset>

      <ErrorAlert v-if="error" :message="error" />
    </form>

    <template #footer>
      <BaseButton @click="open = false">{{ t('common.cancel') }}</BaseButton>
      <BaseButton type="submit" form="library-form" variant="primary" :loading="loading" :disabled="paths.length === 0">
        {{ library ? t('common.save') : t('common.create') }}
      </BaseButton>
    </template>
  </BaseDialog>
</template>
