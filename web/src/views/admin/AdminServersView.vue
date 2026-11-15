<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { peersApi, type Peer } from '@/api/peers'
import AliasDialog from '@/components/AliasDialog.vue'
import BaseButton from '@/components/BaseButton.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import ErrorAlert from '@/components/ErrorAlert.vue'
import { useErrorMessage } from '@/composables/useErrorMessage'

const { t, locale } = useI18n()
const errorMessage = useErrorMessage()

const peers = ref<Peer[]>([])
const loading = ref(true)
const error = ref('')
const notice = ref('')

const pairingCode = ref('')
const codeError = ref('')

const addCode = ref('')
const adding = ref(false)
const addError = ref('')

const syncingId = ref<string | null>(null)
const pausingId = ref<string | null>(null)

const editingId = ref<string | null>(null)
const editUrl = ref('')
const savingEdit = ref(false)
const editError = ref('')

const removeOpen = ref(false)
const removing = ref<Peer | null>(null)

const textareaClass =
  'mt-1.5 block w-full rounded-lg border border-white/10 bg-surface-800 px-3.5 py-2.5 font-mono text-xs text-slate-100 placeholder-slate-500 focus:border-accent-500 focus:ring-2 focus:ring-accent-500/30 focus:outline-none'

async function load() {
  loading.value = true
  error.value = ''
  try {
    peers.value = await peersApi.list()
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

async function loadPairingCode() {
  codeError.value = ''
  try {
    pairingCode.value = (await peersApi.pairingCode()).code
  } catch (err) {
    codeError.value = errorMessage(err)
  }
}

async function add() {
  adding.value = true
  addError.value = ''
  notice.value = ''
  try {
    const peer = await peersApi.add(addCode.value.trim())
    addCode.value = ''
    notice.value = t(peer.pairingPending ? 'servers.addedPending' : 'servers.added', { name: peer.displayName })
    await load()
  } catch (err) {
    addError.value = errorMessage(err)
  } finally {
    adding.value = false
  }
}

async function sync(peer: Peer) {
  syncingId.value = peer.id
  error.value = ''
  notice.value = ''
  try {
    const res = await peersApi.sync(peer.id)
    notice.value = t('servers.synced', res.users)
    // A sync is also a manual check: an offline server that answers is back online.
    if (peer.status === 'offline') await load()
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
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

function startEdit(peer: Peer) {
  editingId.value = peer.id
  editUrl.value = peer.url
  editError.value = ''
  notice.value = ''
}

function cancelEdit() {
  editingId.value = null
  editError.value = ''
}

async function saveEdit(peer: Peer) {
  savingEdit.value = true
  editError.value = ''
  try {
    const updated = await peersApi.update(peer.id, editUrl.value.trim())
    editingId.value = null
    notice.value = t('servers.updated', { name: updated.displayName })
    await load()
  } catch (err) {
    editError.value = errorMessage(err)
  } finally {
    savingEdit.value = false
  }
}

const aliasOpen = ref(false)
const aliasPeer = ref<Peer | null>(null)

function openAlias(peer: Peer) {
  aliasPeer.value = peer
  aliasOpen.value = true
  notice.value = ''
}

async function saveAlias(alias: string | null) {
  await peersApi.setAlias(aliasPeer.value!.id, alias)
  await load()
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

onMounted(() => Promise.all([load(), loadPairingCode()]))
</script>

<template>
  <section class="space-y-10">
    <p v-if="notice" class="text-sm text-slate-300">{{ notice }}</p>

    <div class="space-y-4">
      <div>
        <h2 class="text-lg font-semibold">{{ t('servers.thisServer') }}</h2>
        <p class="mt-1 text-sm leading-relaxed text-slate-400">{{ t('servers.pairingHelp') }}</p>
      </div>
      <ErrorAlert v-if="codeError" :message="codeError" />
      <div v-if="pairingCode">
        <label for="pairing-code" class="block text-sm font-medium text-slate-300">{{ t('servers.codeLabel') }}</label>
        <textarea
          id="pairing-code"
          :value="pairingCode"
          readonly
          rows="3"
          :class="textareaClass"
          @focus="($event.target as HTMLTextAreaElement).select()"
        />
      </div>
    </div>

    <div class="space-y-4 border-t border-white/5 pt-8">
      <div>
        <h2 class="text-lg font-semibold">{{ t('servers.paired') }}</h2>
        <p class="mt-1 text-sm leading-relaxed text-slate-400">{{ t('servers.pairedHelp') }}</p>
      </div>
      <ErrorAlert v-if="error" :message="error" />
      <p v-if="!loading && peers.length === 0" class="text-sm text-slate-500">{{ t('servers.empty') }}</p>
      <ul v-else class="divide-y divide-white/5 rounded-2xl border border-white/5 bg-surface-900">
        <li v-for="peer in peers" :key="peer.id" class="px-5 py-4">
          <div v-if="editingId === peer.id" class="space-y-3">
            <div>
              <label :for="`peer-url-${peer.id}`" class="block text-sm font-medium text-slate-300">{{ t('servers.urlLabel') }}</label>
              <input
                :id="`peer-url-${peer.id}`"
                v-model="editUrl"
                type="url"
                required
                class="mt-1.5 block w-full rounded-lg border border-white/10 bg-surface-800 px-3.5 py-2.5 font-mono text-xs text-slate-100 focus:border-accent-500 focus:ring-2 focus:ring-accent-500/30 focus:outline-none"
              />
            </div>
            <ErrorAlert v-if="editError" :message="editError" />
            <div class="flex gap-2">
              <BaseButton variant="primary" :loading="savingEdit" :disabled="!editUrl.trim()" @click="saveEdit(peer)">{{ t('servers.save') }}</BaseButton>
              <BaseButton :disabled="savingEdit" @click="cancelEdit">{{ t('servers.cancel') }}</BaseButton>
            </div>
          </div>
          <div v-else class="flex flex-wrap items-center justify-between gap-4">
            <div class="min-w-0">
              <p class="font-medium text-slate-100">
                {{ peer.displayName }}
                <span v-if="peer.alias" class="ml-1 text-xs font-normal text-slate-500">{{ t('aliases.original', { name: peer.name }) }}</span>
              </p>
              <p class="truncate font-mono text-xs text-slate-500">{{ peer.url }}</p>
              <p class="mt-1 text-xs" :class="peer.status === 'compatible' ? 'text-emerald-400' : 'text-amber-400'">
                {{ peer.version ?? t('servers.unknownVersion') }} · {{ t(`servers.status.${peer.status}`) }}
              </p>
              <p v-if="peer.status === 'incompatible'" class="mt-1 text-xs text-slate-400">
                {{ t('servers.versionMismatch', { local: peer.localVersion }) }}
              </p>
              <p v-if="peer.paused" class="mt-1 text-xs text-slate-400">{{ t('servers.pausedHelp') }}</p>
              <p v-else-if="peer.offlineSince" class="mt-1 text-xs text-slate-400">
                {{ t('servers.offlineSince', { date: formatDate(peer.offlineSince) }) }}
              </p>
              <p v-if="peer.lastSeenAt && peer.status !== 'compatible'" class="mt-1 text-xs text-slate-500">
                {{ t('servers.lastSeen', { date: formatDate(peer.lastSeenAt) }) }}
              </p>
            </div>
            <div class="flex flex-wrap gap-2">
              <BaseButton :disabled="peer.status !== 'compatible' && peer.status !== 'offline'" :loading="syncingId === peer.id" @click="sync(peer)">{{ t('servers.sync') }}</BaseButton>
              <BaseButton :loading="pausingId === peer.id" @click="togglePause(peer)">{{ t(peer.paused ? 'servers.resume' : 'servers.pause') }}</BaseButton>
              <BaseButton @click="openAlias(peer)">{{ t('aliases.action') }}</BaseButton>
              <BaseButton @click="startEdit(peer)">{{ t('servers.edit') }}</BaseButton>
              <BaseButton class="hover:!bg-red-500/15 hover:!text-red-300" @click="askRemove(peer)">{{ t('servers.remove') }}</BaseButton>
            </div>
          </div>
        </li>
      </ul>
    </div>

    <form class="space-y-4 border-t border-white/5 pt-8" @submit.prevent="add">
      <div>
        <h2 class="text-lg font-semibold">{{ t('servers.addTitle') }}</h2>
        <p class="mt-1 text-sm leading-relaxed text-slate-400">{{ t('servers.addHelp') }}</p>
      </div>
      <div>
        <label for="peer-code" class="block text-sm font-medium text-slate-300">{{ t('servers.addCode') }}</label>
        <textarea id="peer-code" v-model="addCode" rows="3" required :class="textareaClass" />
      </div>
      <ErrorAlert v-if="addError" :message="addError" />
      <BaseButton type="submit" variant="primary" :loading="adding" :disabled="!addCode.trim()">{{ t('servers.add') }}</BaseButton>
    </form>

    <AliasDialog v-model:open="aliasOpen" :name="aliasPeer?.name ?? ''" :alias="aliasPeer?.alias ?? null" :save="saveAlias" />

    <ConfirmDialog
      v-model:open="removeOpen"
      :title="t('servers.removeTitle')"
      :message="t('servers.removeConfirm', { name: removing?.displayName ?? '' })"
      :confirm-label="t('servers.remove')"
      :action="remove"
    />
  </section>
</template>
