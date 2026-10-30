<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { librariesApi, type Library } from '@/api/libraries'
import BaseButton from '@/components/BaseButton.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import ErrorAlert from '@/components/ErrorAlert.vue'
import LibraryFormDialog from '@/components/LibraryFormDialog.vue'
import { useErrorMessage } from '@/composables/useErrorMessage'

const { t } = useI18n()
const errorMessage = useErrorMessage()

const libraries = ref<Library[]>([])
const loading = ref(true)
const error = ref('')

const formOpen = ref(false)
const editing = ref<Library | null>(null)
const deleteOpen = ref(false)
const deleting = ref<Library | null>(null)

async function load() {
  loading.value = true
  error.value = ''
  try {
    libraries.value = await librariesApi.list()
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

function openForm(lib: Library | null) {
  editing.value = lib
  formOpen.value = true
}

function askDelete(lib: Library) {
  deleting.value = lib
  deleteOpen.value = true
}

async function remove() {
  await librariesApi.remove(deleting.value!.id)
  await load()
}

onMounted(load)
</script>

<template>
  <section>
    <div class="mb-6 flex items-center justify-between gap-4">
      <h2 class="text-lg font-semibold">{{ t('admin.libraries') }}</h2>
      <BaseButton variant="primary" @click="openForm(null)">+ {{ t('libraries.new') }}</BaseButton>
    </div>

    <ErrorAlert v-if="error" :message="error" />

    <div v-else-if="loading" class="grid gap-4 md:grid-cols-2">
      <div v-for="i in 2" :key="i" class="h-36 animate-pulse rounded-2xl bg-surface-800/60" />
    </div>

    <p v-else-if="libraries.length === 0" class="rounded-2xl border border-dashed border-white/10 px-6 py-14 text-center text-sm text-slate-400">
      {{ t('libraries.empty') }}
    </p>

    <ul v-else class="grid gap-4 md:grid-cols-2">
      <li
        v-for="lib in libraries"
        :key="lib.id"
        class="flex flex-col rounded-2xl border border-white/5 bg-surface-900 p-5 transition hover:border-white/10"
      >
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0">
            <h3 class="truncate text-base font-semibold">{{ lib.name }}</h3>
            <p class="mt-0.5 text-xs text-slate-500">{{ t('libraries.folderCount', lib.paths?.length ?? 0) }}</p>
          </div>
          <span class="rounded-full bg-accent-500/10 px-2.5 py-1 text-xs font-medium text-accent-300">
            {{ t(`libraries.kinds.${lib.kind}`) }}
          </span>
        </div>
        <ul class="mt-4 flex-1 space-y-1">
          <li v-for="p in lib.paths" :key="p" class="truncate font-mono text-xs text-slate-400" :title="p">{{ p }}</li>
        </ul>
        <div class="mt-5 flex justify-end gap-2">
          <BaseButton @click="openForm(lib)">{{ t('common.edit') }}</BaseButton>
          <BaseButton class="hover:!bg-red-500/15 hover:!text-red-300" @click="askDelete(lib)">{{ t('common.delete') }}</BaseButton>
        </div>
      </li>
    </ul>

    <LibraryFormDialog v-model:open="formOpen" :library="editing" @saved="load" />
    <ConfirmDialog
      v-model:open="deleteOpen"
      :title="t('libraries.deleteTitle')"
      :message="t('libraries.deleteConfirm', { name: deleting?.name ?? '' })"
      :confirm-label="t('common.delete')"
      :action="remove"
    />
  </section>
</template>
