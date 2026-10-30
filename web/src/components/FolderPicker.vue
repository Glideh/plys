<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { librariesApi, type BrowseResult } from '@/api/libraries'
import { useErrorMessage } from '@/composables/useErrorMessage'
import BaseButton from './BaseButton.vue'
import ErrorAlert from './ErrorAlert.vue'

const emit = defineEmits<{ select: [path: string]; cancel: [] }>()
const errorMessage = useErrorMessage()

const current = ref<BrowseResult | null>(null)
const loading = ref(false)
const error = ref('')

async function open(path: string | null) {
  loading.value = true
  error.value = ''
  try {
    current.value = await librariesApi.browse(path)
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

onMounted(() => open(null))
</script>

<template>
  <div class="overflow-hidden rounded-xl border border-white/10 bg-surface-950/60">
    <div class="flex items-center gap-2 border-b border-white/5 px-3 py-2">
      <button
        type="button"
        class="grid size-8 shrink-0 place-items-center rounded-lg text-slate-400 transition hover:bg-white/5 hover:text-slate-100 disabled:opacity-30"
        :disabled="!current?.path || loading"
        :aria-label="$t('picker.up')"
        :title="$t('picker.up')"
        @click="open(current?.parent ?? null)"
      >
        <svg viewBox="0 0 20 20" fill="currentColor" class="size-5" aria-hidden="true">
          <path fill-rule="evenodd" d="M10 17a.75.75 0 0 1-.75-.75V5.612L5.29 9.77a.75.75 0 0 1-1.08-1.04l5.25-5.5a.75.75 0 0 1 1.08 0l5.25 5.5a.75.75 0 1 1-1.08 1.04l-3.96-4.158V16.25A.75.75 0 0 1 10 17Z" clip-rule="evenodd" />
        </svg>
      </button>
      <span class="truncate font-mono text-xs text-slate-400" :title="current?.path ?? ''">
        {{ current?.path ?? $t('picker.roots') }}
      </span>
    </div>

    <ul class="h-64 overflow-y-auto py-1" :aria-busy="loading">
      <li v-if="!loading && current && current.entries.length === 0" class="px-4 py-8 text-center text-sm text-slate-500">
        {{ current.path ? $t('picker.empty') : $t('picker.noRoots') }}
      </li>
      <li v-for="entry in current?.entries ?? []" :key="entry.path">
        <button
          type="button"
          class="flex w-full items-center gap-3 px-4 py-2 text-left text-sm transition hover:bg-white/5"
          @click="open(entry.path)"
        >
          <svg viewBox="0 0 20 20" fill="currentColor" class="size-5 shrink-0 text-accent-400/80" aria-hidden="true">
            <path d="M3.75 3A1.75 1.75 0 0 0 2 4.75v3.26a3.235 3.235 0 0 1 1.75-.51h12.5c.644 0 1.245.188 1.75.51V6.75A1.75 1.75 0 0 0 16.25 5h-4.836a.25.25 0 0 1-.177-.073L9.823 3.513A1.75 1.75 0 0 0 8.586 3H3.75ZM3.75 9A1.75 1.75 0 0 0 2 10.75v4.5c0 .966.784 1.75 1.75 1.75h12.5A1.75 1.75 0 0 0 18 15.25v-4.5A1.75 1.75 0 0 0 16.25 9H3.75Z" />
          </svg>
          <span class="truncate">{{ entry.name }}</span>
        </button>
      </li>
    </ul>

    <div class="flex items-center justify-end gap-2 border-t border-white/5 px-3 py-2">
      <ErrorAlert v-if="error" class="mr-auto py-1.5" :message="error" />
      <BaseButton @click="emit('cancel')">{{ $t('common.cancel') }}</BaseButton>
      <BaseButton variant="primary" :disabled="!current?.path" @click="current?.path && emit('select', current.path)">
        {{ $t('picker.choose') }}
      </BaseButton>
    </div>
  </div>
</template>
