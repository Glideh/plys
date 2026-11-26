<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { User } from '@/api/client'
import { invitationsApi, type Invitation } from '@/api/invitations'
import { settingsApi } from '@/api/settings'
import { usersApi, type Role } from '@/api/users'
import ActionMenu from '@/components/ActionMenu.vue'
import AddButton from '@/components/AddButton.vue'
import AdminSectionHeader from '@/components/AdminSectionHeader.vue'
import BaseButton from '@/components/BaseButton.vue'
import BaseDialog from '@/components/BaseDialog.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import ErrorAlert from '@/components/ErrorAlert.vue'
import InviteDialog from '@/components/InviteDialog.vue'
import TextField from '@/components/TextField.vue'
import UserAvatar from '@/components/UserAvatar.vue'
import UserEditDialog from '@/components/UserEditDialog.vue'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { localeLabels, locales, type Locale } from '@/i18n'
import { useAuthStore } from '@/stores/auth'

const { t, d, locale: uiLocale } = useI18n()
const auth = useAuthStore()
const errorMessage = useErrorMessage()

const users = ref<User[]>([])
const invitations = ref<Invitation[]>([])
// Invitation links point at the public URL, so the button needs one.
const canInvite = ref(false)
const loading = ref(true)
const error = ref('')
const notice = ref('')

const selectClass =
  'rounded-lg border border-white/10 bg-surface-800 px-3 py-2 text-sm text-slate-100 focus:border-accent-500 focus:ring-2 focus:ring-accent-500/30 focus:outline-none'

const syncing = ref(false)
let syncRetries = 0
let syncTimer: ReturnType<typeof setTimeout> | undefined

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [page, pending] = await Promise.all([usersApi.listPage(), invitationsApi.list()])
    users.value = page.items
    syncing.value = page.syncing
    invitations.value = pending
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
  scheduleSyncReload()
}

// Re-reads the list while a peer sync runs in the background, up to a bounded number of tries.
function scheduleSyncReload() {
  if (!syncing.value || syncRetries >= 5) {
    syncRetries = 0
    return
  }
  syncRetries++
  syncTimer = setTimeout(load, 2000)
}

const createOpen = ref(false)
const createForm = ref({ username: '', password: '', role: 'user' as Role, locale: uiLocale.value as Locale })
const createError = ref('')
const creating = ref(false)

function openCreate() {
  createForm.value = { username: '', password: '', role: 'user', locale: uiLocale.value as Locale }
  createError.value = ''
  createOpen.value = true
}

async function create() {
  creating.value = true
  createError.value = ''
  try {
    await usersApi.create({ ...createForm.value, username: createForm.value.username.trim() })
    createOpen.value = false
    await load()
  } catch (err) {
    createError.value = errorMessage(err)
  } finally {
    creating.value = false
  }
}

const editOpen = ref(false)
const editUser = ref<User | null>(null)

function openEdit(user: User) {
  editUser.value = user
  notice.value = ''
  editOpen.value = true
}

async function onEdited(updated: User, passwordChanged: boolean) {
  if (updated.id === auth.user?.id) auth.user = updated
  if (passwordChanged) notice.value = t('users.passwordChanged')
  await load()
}

const inviteOpen = ref(false)

async function loadInvitations() {
  try {
    invitations.value = await invitationsApi.list()
  } catch (err) {
    error.value = errorMessage(err)
  }
}

const revokeOpen = ref(false)
const revoking = ref<Invitation | null>(null)

function askRevoke(invitation: Invitation) {
  revoking.value = invitation
  revokeOpen.value = true
}

async function revoke() {
  await invitationsApi.remove(revoking.value!.id)
  await loadInvitations()
}

async function loadCanInvite() {
  try {
    canInvite.value = (await settingsApi.get()).publicUrl !== ''
  } catch {
    canInvite.value = false
  }
}

const deleteOpen = ref(false)
const deleting = ref<User | null>(null)

function askDelete(user: User) {
  deleting.value = user
  deleteOpen.value = true
}

async function remove() {
  await usersApi.remove(deleting.value!.id)
  await load()
}

onMounted(() => {
  void load()
  void loadCanInvite()
})
onUnmounted(() => clearTimeout(syncTimer))
</script>

<template>
  <section>
    <AdminSectionHeader :title="t('admin.users')" :help="t('users.help')">
      <template #status>
        <span v-if="loading || syncing" role="status" class="text-xs text-slate-500">{{ t('users.syncing') }}</span>
      </template>
      <template #actions>
        <BaseButton v-if="canInvite" @click="inviteOpen = true">{{ t('invitations.invite') }}</BaseButton>
        <AddButton :label="t('users.new')" @click="openCreate" />
      </template>
    </AdminSectionHeader>

    <ErrorAlert v-if="error" class="mb-4" :message="error" />
    <p v-if="notice" role="status" class="mb-4 rounded-lg border border-emerald-500/30 bg-emerald-500/10 px-3.5 py-2.5 text-sm text-emerald-300">
      {{ notice }}
    </p>

    <div v-if="loading && users.length === 0" class="h-40 animate-pulse rounded-2xl bg-surface-800/60" />

    <!-- A list rather than a table: rows wrap on small viewports instead of scrolling sideways. -->
    <ul v-else class="divide-y divide-white/5 rounded-2xl border border-white/5 bg-surface-900">
      <li v-for="user in users" :key="user.id" class="flex items-center gap-3 px-4 py-3 sm:px-5">
        <UserAvatar :user="user" class="size-9 shrink-0" />
        <div class="min-w-0 flex-1">
          <p class="flex flex-wrap items-baseline gap-x-2">
            <span class="font-medium wrap-break-word">{{ user.displayName }}</span>
            <span v-if="user.id === auth.user?.id" class="text-xs text-slate-500">({{ t('users.you') }})</span>
          </p>
          <p class="mt-0.5 flex flex-wrap gap-x-2 text-xs text-slate-500">
            <span v-if="user.alias">{{ t('aliases.original', { name: user.username }) }}</span>
            <span v-if="user.remote">{{ t('users.remoteFrom', { server: user.originServer }) }}</span>
            <span>{{ t('users.createdOn', { date: d(new Date(user.createdAt)) }) }}</span>
          </p>
        </div>
        <span
          :class="user.role === 'admin' ? 'bg-accent-500/10 text-accent-300' : 'bg-white/5 text-slate-300'"
          class="shrink-0 rounded-full px-2.5 py-1 text-xs font-medium"
        >
          {{ t(`users.roles.${user.role}`) }}
        </span>
        <ActionMenu
          :items="[
            { label: t('common.edit'), action: () => openEdit(user) },
            ...(!user.remote && user.id !== auth.user?.id ? [{ label: t('common.delete'), action: () => askDelete(user), danger: true }] : []),
          ]"
        />
      </li>
      <li v-for="invitation in invitations" :key="invitation.id" data-testid="pending-invitation" class="flex items-center gap-3 px-4 py-3 sm:px-5">
        <span class="grid size-9 shrink-0 place-items-center rounded-full border border-dashed border-white/20 text-slate-500" aria-hidden="true">?</span>
        <div class="min-w-0 flex-1">
          <p class="font-medium text-slate-400 italic">{{ t('invitations.pending') }}</p>
          <p class="mt-0.5 text-xs text-slate-500">{{ t('invitations.expires', { date: d(new Date(invitation.expiresAt), 'long') }) }}</p>
        </div>
        <ActionMenu :items="[{ label: t('common.delete'), action: () => askRevoke(invitation), danger: true }]" />
      </li>
    </ul>

    <BaseDialog v-model:open="createOpen" :title="t('users.new')">
      <form id="user-form" class="space-y-5" @submit.prevent="create">
        <TextField v-model="createForm.username" :label="t('common.username')" :hint="t('setup.usernameHint')" autocomplete="off" required :maxlength="32" />
        <TextField v-model="createForm.password" :label="t('common.password')" :hint="t('setup.passwordHint')" type="password" autocomplete="new-password" required :minlength="8" />
        <div class="grid gap-4 sm:grid-cols-2">
          <label class="block">
            <span class="mb-1.5 block text-sm font-medium text-slate-300">{{ t('users.role') }}</span>
            <select v-model="createForm.role" :class="selectClass" class="w-full">
              <option value="user">{{ t('users.roles.user') }}</option>
              <option value="admin">{{ t('users.roles.admin') }}</option>
            </select>
          </label>
          <label class="block">
            <span class="mb-1.5 block text-sm font-medium text-slate-300">{{ t('common.language') }}</span>
            <select v-model="createForm.locale" :class="selectClass" class="w-full">
              <option v-for="l in locales" :key="l" :value="l">{{ localeLabels[l] }}</option>
            </select>
          </label>
        </div>
        <ErrorAlert v-if="createError" :message="createError" />
      </form>
      <template #footer>
        <BaseButton @click="createOpen = false">{{ t('common.cancel') }}</BaseButton>
        <BaseButton type="submit" form="user-form" variant="primary" :loading="creating">{{ t('common.create') }}</BaseButton>
      </template>
    </BaseDialog>

    <UserEditDialog v-model:open="editOpen" :user="editUser" @saved="onEdited" />

    <InviteDialog v-model:open="inviteOpen" @created="loadInvitations" />

    <ConfirmDialog
      v-model:open="revokeOpen"
      :title="t('invitations.revokeTitle')"
      :message="t('invitations.revokeConfirm')"
      :confirm-label="t('common.delete')"
      :action="revoke"
    />

    <ConfirmDialog
      v-model:open="deleteOpen"
      :title="t('users.deleteTitle')"
      :message="t('users.deleteConfirm', { name: deleting?.displayName ?? '' })"
      :confirm-label="t('common.delete')"
      :action="remove"
    />
  </section>
</template>
