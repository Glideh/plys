<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { peersApi, type Peer } from '@/api/peers'
import ActionMenu from '@/components/ActionMenu.vue'
import AddButton from '@/components/AddButton.vue'
import AdminSectionHeader from '@/components/AdminSectionHeader.vue'
import BaseButton from '@/components/BaseButton.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import ErrorAlert from '@/components/ErrorAlert.vue'
import PairServerDialog from '@/components/PairServerDialog.vue'
import PeerEditDialog from '@/components/PeerEditDialog.vue'
import { useErrorMessage } from '@/composables/useErrorMessage'

const { t, locale } = useI18n()
const errorMessage = useErrorMessage()

const peers = ref<Peer[]>([])
const loading = ref(true)
const error = ref('')
const notice = ref('')

const pairOpen = ref(false)
const syncingId = ref<string | null>(null)
const pausingId = ref<string | null>(null)

const editOpen = ref(false)
const editing = ref<Peer | null>(null)

const removeOpen = ref(false)
const removing = ref<Peer | null>(null)

async function load(preserveError = false) {
  loading.value = peers.value.length === 0
  if (!preserveError) error.value = ''
  try {
    peers.value = await peersApi.list()
  } catch (err) {
    error.value = [error.value, errorMessage(err)].filter(Boolean).join('\n')
  } finally {
    loading.value = false
  }
}

async function onPaired(peer: Peer & { pairingPending: boolean }) {
  notice.value = t(peer.pairingPending ? 'servers.addedPending' : 'servers.added', { name: peer.displayName })
  await load()
}

async function sync(peer: Peer) {
  syncingId.value = peer.id
  error.value = ''
  notice.value = ''
  try {
    const res = await peersApi.sync(peer.id)
    notice.value = t('servers.synced', res.users)
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    await load(true)
    syncingId.value = null
  }
}

async function togglePause(peer: Peer) {
  pausingId.value = peer.id
  error.value = ''
  notice.value = ''
  try {
    const updated = await peersApi.setPaused(peer.id, !peer.paused)
    notice.value = t(updated.paused ? 'servers.paused' : 'servers.resumed', { name: updated.displayName })
    await load()
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    pausingId.value = null
  }
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
}

function openEdit(peer: Peer) {
  editing.value = peer
  notice.value = ''
  editOpen.value = true
}

function askRemove(peer: Peer) {
  removing.value = peer
  removeOpen.value = true
}

async function remove() {
  await peersApi.remove(removing.value!.id)
  notice.value = ''
  await load()
}

onMounted(load)
</script>

<template>
  <section>
    <AdminSectionHeader :title="t('servers.paired')" :help="t('servers.pairedHelp')">
      <template #actions>
        <AddButton :label="t('servers.addTitle')" @click="pairOpen = true" />
      </template>
    </AdminSectionHeader>

    <ErrorAlert v-if="error" class="mb-4" :message="error" />
    <p v-if="notice" role="status" class="mb-4 rounded-lg border border-emerald-500/30 bg-emerald-500/10 px-3.5 py-2.5 text-sm text-emerald-300">
      {{ notice }}
    </p>

    <div v-if="loading" class="h-28 animate-pulse rounded-2xl bg-surface-800/60" />
    <p v-else-if="peers.length === 0" class="rounded-2xl border border-dashed border-white/10 px-6 py-14 text-center text-sm text-slate-400">
      {{ t('servers.empty') }}
    </p>
    <ul v-else class="divide-y divide-white/5 rounded-2xl border border-white/5 bg-surface-900">
      <li v-for="peer in peers" :key="peer.id" class="flex flex-wrap items-center gap-x-4 gap-y-3 px-4 py-4 sm:px-5">
        <div class="min-w-0 flex-1 basis-60">
          <p class="font-medium wrap-break-word text-slate-100">
            {{ peer.displayName }}
            <span v-if="peer.alias" class="ml-1 text-xs font-normal text-slate-500">{{ t('aliases.original', { name: peer.name }) }}</span>
          </p>
          <p class="truncate font-mono text-xs text-slate-500" :title="peer.url">{{ peer.url }}</p>
          <p class="mt-1 text-xs" :class="peer.status === 'compatible' ? 'text-emerald-400' : 'text-amber-400'">
            {{ peer.version ?? t('servers.unknownVersion') }} · {{ t(`servers.status.${peer.status}`) }}
          </p>
          <p v-if="peer.status === 'incompatible'" class="mt-1 text-xs text-slate-400">
            {{ t('servers.versionMismatch', { local: peer.localVersion }) }}
          </p>
          <p v-if="peer.paused" class="mt-1 text-xs text-slate-400">{{ t('servers.pausedHelp') }}</p>
          <p v-else-if="peer.status === 'offline'" class="mt-1 text-xs text-slate-400">
            {{ t('servers.offlineHelp') }}
          </p>
          <p v-if="peer.lastSeenAt && peer.status !== 'compatible'" class="mt-1 text-xs text-slate-500">
            {{ t('servers.lastSeen', { date: formatDate(peer.lastSeenAt) }) }}
          </p>
        </div>
        <div class="ml-auto flex gap-2">
          <BaseButton
            :disabled="peer.status !== 'compatible' && peer.status !== 'offline'"
            :loading="syncingId === peer.id"
            @click="sync(peer)"
          >
            {{ t('servers.sync') }}
          </BaseButton>
          <ActionMenu
            :items="[
              { label: t(peer.paused ? 'servers.resume' : 'servers.pause'), action: () => togglePause(peer), disabled: pausingId === peer.id },
              { label: t('common.edit'), action: () => openEdit(peer) },
              { label: t('servers.remove'), action: () => askRemove(peer), danger: true },
            ]"
          />
        </div>
      </li>
    </ul>

    <PairServerDialog v-model:open="pairOpen" @paired="onPaired" />
    <PeerEditDialog v-model:open="editOpen" :peer="editing" @saved="load" />

    <ConfirmDialog
      v-model:open="removeOpen"
      :title="t('servers.removeTitle')"
      :message="t('servers.removeConfirm', { name: removing?.displayName ?? '' })"
      :confirm-label="t('servers.remove')"
      :action="remove"
    />
  </section>
</template>
