<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { User } from '@/api/client'
import { libraryKinds, librariesApi, type Library, type LibraryKind } from '@/api/libraries'
import { metadataLanguageNames, metadataLanguages, settingsApi } from '@/api/settings'
import { usersApi } from '@/api/users'
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
const restricted = ref(false)
const userIds = ref<string[]>([])
/** Empty: the library follows the server-wide metadata language. */
const metadataLanguage = ref('')
const serverLanguage = ref('')
const users = ref<User[]>([])
const picking = ref(false)
const loading = ref(false)
const error = ref('')

// Admins always see every library, so only regular users can be granted access.
const grantableUsers = computed(() => users.value.filter((u) => u.role !== 'admin'))
const accessOptions = [
  { restricted: false, label: 'libraries.openAccess', help: 'libraries.openAccessHelp' },
  { restricted: true, label: 'libraries.restrictedAccess', help: 'libraries.restrictedAccessHelp' },
] as const

watch(open, async (isOpen) => {
  if (!isOpen) return
  name.value = props.library?.name ?? ''
  kind.value = props.library?.kind ?? 'movies'
  paths.value = [...(props.library?.paths ?? [])]
  restricted.value = props.library?.restricted ?? false
  userIds.value = [...(props.library?.userIds ?? [])]
  metadataLanguage.value = props.library?.metadataLanguage ?? ''
  picking.value = paths.value.length === 0
  error.value = ''
  try {
    const [userList, settings] = await Promise.all([usersApi.list(), settingsApi.get()])
    users.value = userList
    serverLanguage.value = settings.metadataLanguage
  } catch (err) {
    error.value = errorMessage(err)
  }
})

function addPath(path: string) {
  if (!paths.value.includes(path)) paths.value.push(path)
  picking.value = false
}

async function submit() {
  error.value = ''
  loading.value = true
  const access = { restricted: restricted.value, userIds: userIds.value, metadataLanguage: metadataLanguage.value }
  try {
    const saved = props.library
      ? await librariesApi.update(props.library.id, { name: name.value, paths: paths.value, ...access })
      : await librariesApi.create({ name: name.value, kind: kind.value, paths: paths.value, ...access })
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

      <!-- Personal videos get no TMDB metadata, so the language only matters for movies and series. -->
      <div v-if="kind !== 'other'">
        <label for="library-metadata-language" class="block text-sm font-medium text-slate-300">
          {{ t('libraries.metadataLanguage') }}
        </label>
        <select
          id="library-metadata-language"
          v-model="metadataLanguage"
          class="mt-1.5 block w-full rounded-lg border border-white/10 bg-surface-800 px-3.5 py-2.5 text-slate-100 focus:border-accent-500 focus:ring-2 focus:ring-accent-500/30 focus:outline-none sm:w-80"
        >
          <option value="">
            {{ t('libraries.metadataLanguageDefault', { language: metadataLanguageNames[serverLanguage] ?? serverLanguage }) }}
          </option>
          <option v-for="l in metadataLanguages" :key="l" :value="l">{{ metadataLanguageNames[l] }}</option>
        </select>
        <p class="mt-1.5 text-xs text-slate-500">{{ t('libraries.metadataLanguageHelp') }}</p>
      </div>

      <fieldset class="min-w-0">
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

      <fieldset>
        <legend class="mb-1.5 text-sm font-medium text-slate-300">{{ t('libraries.access') }}</legend>
        <div class="grid gap-3 sm:grid-cols-2">
          <label
            v-for="option in accessOptions"
            :key="String(option.restricted)"
            :class="restricted === option.restricted ? 'border-accent-500 bg-accent-500/10' : 'border-white/10 hover:border-white/20'"
            class="relative cursor-pointer rounded-xl border p-3.5 transition has-focus-visible:outline-2 has-focus-visible:outline-accent-400"
          >
            <input v-model="restricted" type="radio" name="access" :value="option.restricted" class="sr-only" />
            <span class="block font-semibold">{{ t(option.label) }}</span>
            <span class="mt-1 block text-xs leading-snug text-slate-400">{{ t(option.help) }}</span>
          </label>
        </div>
        <template v-if="restricted">
          <p v-if="grantableUsers.length === 0" class="mt-3 text-sm text-slate-500">{{ t('libraries.noUsers') }}</p>
          <ul v-else class="mt-3 grid gap-2 sm:grid-cols-2">
            <li v-for="u in grantableUsers" :key="u.id">
              <label class="flex items-center gap-3 rounded-lg bg-surface-800 px-3.5 py-2 text-sm">
                <input v-model="userIds" type="checkbox" :value="u.id" class="size-4 rounded accent-accent-500" />
                {{ u.displayName }}
              </label>
            </li>
          </ul>
        </template>
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
