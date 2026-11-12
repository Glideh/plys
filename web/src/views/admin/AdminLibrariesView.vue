<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { librariesApi, type Library, type ScanStatus } from '@/api/libraries'
import { metadataLanguageNames } from '@/api/settings'
import BaseButton from '@/components/BaseButton.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import ErrorAlert from '@/components/ErrorAlert.vue'
import LibraryFormDialog from '@/components/LibraryFormDialog.vue'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { useLibrarySummary } from '@/composables/useLibrarySummary'

const SCAN_POLL_MS = 1500

const { t, d } = useI18n()
const errorMessage = useErrorMessage()
const summary = useLibrarySummary()

const libraries = ref<Library[]>([])
const loading = ref(true)
const error = ref('')

const formOpen = ref(false)
const editing = ref<Library | null>(null)
const deleteOpen = ref(false)
const deleting = ref<Library | null>(null)

const scans = ref<Record<string, ScanStatus>>({})
let pollTimer: ReturnType<typeof setTimeout> | undefined
let scanning = false

async function load() {
  loading.value = libraries.value.length === 0
  error.value = ''
  try {
    libraries.value = await librariesApi.list()
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

const isActive = (s?: ScanStatus) => s?.state === 'queued' || s?.state === 'running'

// Polls only while a scan is queued or running, then reloads counts once everything is idle.
async function refreshScans() {
  clearTimeout(pollTimer)
  try {
    const items = await librariesApi.scans()
    scans.value = Object.fromEntries(items.map((s) => [s.libraryId, s]))
  } catch {
    // Transient polling errors are ignored; the next tick retries.
  }
  const active = Object.values(scans.value).some(isActive)
  if (scanning && !active) await load()
  scanning = active
  if (active) pollTimer = setTimeout(refreshScans, SCAN_POLL_MS)
}

async function startScan(lib: Library) {
  try {
    scans.value[lib.id] = await librariesApi.scan(lib.id)
  } catch (err) {
    error.value = errorMessage(err)
  }
  await refreshScans()
}

async function onSaved() {
  await load()
  await refreshScans()
}

function progressPercent(s: ScanStatus): number {
  const [done, total] = s.stage === 'metadata' ? [s.metaDone, s.metaTotal] : [s.processed, s.total]
  return total ? (100 * done) / total : 0
}

function scanResult(s: ScanStatus): string {
  const parts = (['added', 'missing', 'unmatched', 'failed'] as const)
    .filter((k) => s[k] > 0)
    .map((k) => t(`libraries.result.${k}`, s[k]))
  return parts.length ? parts.join(' · ') : t('libraries.upToDate')
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

onMounted(async () => {
  await load()
  await refreshScans()
})
onUnmounted(() => clearTimeout(pollTimer))
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
            <p class="mt-0.5 text-sm text-slate-300">{{ summary(lib) }}</p>
          </div>
          <span class="rounded-full bg-accent-500/10 px-2.5 py-1 text-xs font-medium text-accent-300">
            {{ t(`libraries.kinds.${lib.kind}`) }}
          </span>
        </div>
        <p v-if="lib.restricted" class="mt-2 text-xs text-slate-400">{{ t('libraries.restrictedBadge') }} · {{ lib.userIds?.length ?? 0 }}</p>
        <p v-if="lib.metadataLanguage && lib.kind !== 'other'" class="mt-2 text-xs text-slate-400">
          {{ t('libraries.metadataLanguage') }} · {{ metadataLanguageNames[lib.metadataLanguage] ?? lib.metadataLanguage }}
        </p>
        <ul class="mt-4 flex-1 space-y-1">
          <li v-for="p in lib.paths" :key="p" class="truncate font-mono text-xs text-slate-400" :title="p">{{ p }}</li>
        </ul>

        <div class="mt-4 rounded-lg bg-surface-950/60 px-3.5 py-2.5 text-xs" aria-live="polite">
          <template v-if="scans[lib.id]?.state === 'queued'">
            <p class="text-slate-300">{{ t('libraries.scanQueued') }}</p>
          </template>
          <template v-else-if="scans[lib.id]?.state === 'running'">
            <p class="text-slate-300">
              {{
                scans[lib.id]!.stage === 'metadata'
                  ? t('libraries.metaRunning', { done: scans[lib.id]!.metaDone, total: scans[lib.id]!.metaTotal })
                  : t('libraries.scanRunning', { processed: scans[lib.id]!.processed, total: scans[lib.id]!.total })
              }}
            </p>
            <div class="mt-2 h-1.5 overflow-hidden rounded-full bg-white/5">
              <div
                class="h-full rounded-full bg-accent-500 transition-[width] duration-500"
                :style="{ width: `${progressPercent(scans[lib.id]!)}%` }"
              />
            </div>
          </template>
          <template v-else>
            <p class="text-slate-500">
              {{ lib.lastScannedAt ? t('libraries.lastScan', { date: d(new Date(lib.lastScannedAt), 'long') }) : t('libraries.neverScanned') }}
            </p>
            <p v-if="scans[lib.id]?.state === 'failed'" class="mt-1 text-red-300">
              {{ t('libraries.scanFailed', { error: scans[lib.id]!.error ?? '' }) }}
            </p>
            <template v-else-if="scans[lib.id]?.state === 'done'">
              <p class="mt-1 text-slate-300">{{ scanResult(scans[lib.id]!) }}</p>
              <p v-if="scans[lib.id]!.incomplete" class="mt-1 text-amber-300">{{ t('libraries.scanIncomplete') }}</p>
              <p v-if="scans[lib.id]!.metaError" class="mt-1 text-amber-300">
                {{ t('libraries.metaFailed', { error: scans[lib.id]!.metaError ?? '' }) }}
              </p>
            </template>
            <p v-if="lib.matchCounts?.unmatched" class="mt-1 text-amber-300">
              {{ t('libraries.unmatchedItems', lib.matchCounts.unmatched) }}
            </p>
            <p v-if="lib.matchCounts?.pending && lib.kind !== 'other'" class="mt-1 text-slate-400">
              {{ t('libraries.pendingItems', { n: lib.matchCounts.pending }) }}
            </p>
          </template>
        </div>

        <div class="mt-4 flex justify-end gap-2">
          <BaseButton :disabled="isActive(scans[lib.id])" @click="startScan(lib)">{{ t('libraries.scan') }}</BaseButton>
          <BaseButton @click="openForm(lib)">{{ t('common.edit') }}</BaseButton>
          <BaseButton class="hover:!bg-red-500/15 hover:!text-red-300" @click="askDelete(lib)">{{ t('common.delete') }}</BaseButton>
        </div>
      </li>
    </ul>

    <LibraryFormDialog v-model:open="formOpen" :library="editing" @saved="onSaved" />
    <ConfirmDialog
      v-model:open="deleteOpen"
      :title="t('libraries.deleteTitle')"
      :message="t('libraries.deleteConfirm', { name: deleting?.name ?? '' })"
      :confirm-label="t('common.delete')"
      :action="remove"
    />
  </section>
</template>
