<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Plus, X } from '@lucide/vue'
import { catalogApi, type CollectionMember } from '@/api/catalog'
import { useErrorMessage } from '@/composables/useErrorMessage'
import BaseButton from './BaseButton.vue'
import BaseDialog from './BaseDialog.vue'
import ErrorAlert from './ErrorAlert.vue'

// Groups a movie or video with other titles: their pages then list one another.
const props = defineProps<{ itemId: string; title: string }>()
const emit = defineEmits<{ saved: [] }>()
const open = defineModel<boolean>('open', { required: true })
const { t } = useI18n()
const errorMessage = useErrorMessage()

const members = ref<CollectionMember[]>([])
const loading = ref(false)
const query = ref('')
const results = ref<CollectionMember[] | null>(null)
const saving = ref(false)
const error = ref('')
let searchTimer: ReturnType<typeof setTimeout> | undefined
let searchAbort: AbortController | undefined

const memberIds = computed(() => new Set(members.value.map((m) => m.id)))
const candidates = computed(() => results.value?.filter((r) => !memberIds.value.has(r.id)) ?? null)

watch(open, async (isOpen) => {
  if (!isOpen) return
  members.value = []
  query.value = ''
  results.value = null
  error.value = ''
  loading.value = true
  try {
    members.value = await catalogApi.collection(props.itemId)
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
})

watch(query, (value) => {
  clearTimeout(searchTimer)
  searchAbort?.abort()
  const q = value.trim()
  if (q.length < 2) {
    results.value = null
    return
  }
  searchTimer = setTimeout(async () => {
    const abort = (searchAbort = new AbortController())
    try {
      results.value = await catalogApi.searchCollection(props.itemId, q, abort.signal)
    } catch (err) {
      if (!abort.signal.aborted) error.value = errorMessage(err)
    }
  }, 250)
})

function add(m: CollectionMember) {
  members.value.push(m)
}
function remove(id: string) {
  members.value = members.value.filter((m) => m.id !== id)
}

async function save() {
  saving.value = true
  error.value = ''
  try {
    await catalogApi.setCollection(props.itemId, members.value.map((m) => m.id))
    open.value = false
    emit('saved')
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <BaseDialog v-model:open="open" :title="t('catalog.collectionTitle')" wide>
    <p class="mb-4 text-sm text-slate-400">{{ t('catalog.collectionHelp', { title }) }}</p>
    <ErrorAlert v-if="error" class="mb-4" :message="error" />

    <h3 class="mb-2 text-sm font-medium text-slate-300">{{ t('catalog.collectionLinked') }}</h3>
    <div v-if="loading" class="h-16 animate-pulse rounded-xl bg-surface-800/60" />
    <p v-else-if="!members.length" class="rounded-xl border border-dashed border-white/10 p-4 text-center text-sm text-slate-500">
      {{ t('catalog.collectionEmpty') }}
    </p>
    <ul v-else class="space-y-2">
      <li v-for="m in members" :key="m.id" class="flex items-center gap-3 rounded-xl border border-white/5 bg-surface-800/60 p-2">
        <img v-if="m.posterUrl" :src="m.posterUrl" alt="" loading="lazy" class="h-14 w-10 shrink-0 rounded object-cover" />
        <div v-else class="h-14 w-10 shrink-0 rounded bg-surface-700" />
        <p class="min-w-0 flex-1 truncate text-sm">
          {{ m.title }} <span v-if="m.year" class="text-slate-400">({{ m.year }})</span>
        </p>
        <button
          type="button"
          class="grid size-8 shrink-0 place-items-center rounded-lg text-slate-400 transition hover:bg-white/5 hover:text-slate-100"
          :aria-label="t('catalog.collectionRemove', { title: m.title })"
          :title="t('catalog.collectionRemove', { title: m.title })"
          @click="remove(m.id)"
        >
          <X class="size-4" aria-hidden="true" />
        </button>
      </li>
    </ul>

    <h3 class="mt-6 mb-2 text-sm font-medium text-slate-300">{{ t('catalog.collectionAdd') }}</h3>
    <input
      v-model="query"
      type="search"
      :aria-label="t('catalog.collectionSearch')"
      :placeholder="t('catalog.collectionSearch')"
      class="w-full rounded-lg border border-white/10 bg-surface-800 px-3.5 py-2 text-sm focus:border-accent-500 focus:ring-2 focus:ring-accent-500/30 focus:outline-none"
    />
    <p v-if="candidates && !candidates.length" class="mt-4 text-center text-sm text-slate-500">{{ t('catalog.collectionNoResults') }}</p>
    <ul v-if="candidates?.length" class="mt-3 space-y-2">
      <li v-for="c in candidates" :key="c.id">
        <button
          type="button"
          class="flex w-full items-center gap-3 rounded-xl border border-white/5 bg-surface-800/40 p-2 text-left transition hover:border-accent-500/60 hover:bg-surface-800"
          @click="add(c)"
        >
          <img v-if="c.posterUrl" :src="c.posterUrl" alt="" loading="lazy" class="h-14 w-10 shrink-0 rounded object-cover" />
          <div v-else class="h-14 w-10 shrink-0 rounded bg-surface-700" />
          <span class="min-w-0 flex-1 truncate text-sm">
            {{ c.title }} <span v-if="c.year" class="text-slate-400">({{ c.year }})</span>
          </span>
          <Plus class="mr-2 size-4 shrink-0 text-slate-400" aria-hidden="true" />
        </button>
      </li>
    </ul>

    <template #footer>
      <BaseButton :disabled="saving" @click="open = false">{{ t('common.cancel') }}</BaseButton>
      <BaseButton variant="primary" :loading="saving" :disabled="loading" @click="save">{{ t('common.save') }}</BaseButton>
    </template>
  </BaseDialog>
</template>
