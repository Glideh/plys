<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { User } from '@/api/client'
import AliasDialog from '@/components/AliasDialog.vue'
import { invitationsApi, type Invitation } from '@/api/invitations'
import { settingsApi } from '@/api/settings'
import { usersApi, type Role } from '@/api/users'
import BaseButton from '@/components/BaseButton.vue'
import BaseDialog from '@/components/BaseDialog.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import ErrorAlert from '@/components/ErrorAlert.vue'
import InviteDialog from '@/components/InviteDialog.vue'
import TextField from '@/components/TextField.vue'
import UserAvatar from '@/components/UserAvatar.vue'
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

async function changeRole(user: User, role: Role) {
  error.value = ''
  notice.value = ''
  try {
    Object.assign(user, await usersApi.update(user.id, { role }))
  } catch (err) {
    error.value = errorMessage(err)
    await load()
  }
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

const passwordOpen = ref(false)
const passwordUser = ref<User | null>(null)
const newPassword = ref('')
const passwordError = ref('')
const savingPassword = ref(false)

function openPassword(user: User) {
  passwordUser.value = user
  newPassword.value = ''
  passwordError.value = ''
  passwordOpen.value = true
}

async function savePassword() {
  savingPassword.value = true
  passwordError.value = ''
  try {
    await usersApi.update(passwordUser.value!.id, { password: newPassword.value })
    passwordOpen.value = false
    notice.value = t('users.passwordChanged')
  } catch (err) {
    passwordError.value = errorMessage(err)
  } finally {
    savingPassword.value = false
  }
}

const aliasOpen = ref(false)
const aliasUser = ref<User | null>(null)

function openAlias(user: User) {
  aliasUser.value = user
  aliasOpen.value = true
}

async function saveAlias(alias: string | null) {
  const updated = await usersApi.setAlias(aliasUser.value!.id, alias)
  if (updated.id === auth.user?.id) auth.user = updated
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
    <div class="mb-6 flex items-center justify-between gap-4">
      <div class="flex items-center gap-3">
        <h2 class="text-lg font-semibold">{{ t('admin.users') }}</h2>
        <span v-if="loading || syncing" role="status" class="text-xs text-slate-500">{{ t('users.syncing') }}</span>
      </div>
      <div class="flex gap-2">
        <BaseButton v-if="canInvite" @click="inviteOpen = true">{{ t('invitations.invite') }}</BaseButton>
        <BaseButton variant="primary" @click="openCreate">+ {{ t('users.new') }}</BaseButton>
      </div>
    </div>

    <ErrorAlert v-if="error" class="mb-4" :message="error" />
    <p v-if="notice" role="status" class="mb-4 rounded-lg border border-emerald-500/30 bg-emerald-500/10 px-3.5 py-2.5 text-sm text-emerald-300">
      {{ notice }}
    </p>

    <div v-if="loading && users.length === 0" class="h-40 animate-pulse rounded-2xl bg-surface-800/60" />

    <div v-else class="overflow-x-auto rounded-2xl border border-white/5 bg-surface-900">
      <table class="w-full text-left text-sm">
        <thead class="border-b border-white/5 text-xs tracking-wide text-slate-500 uppercase">
          <tr>
            <th class="px-5 py-3 font-medium">{{ t('common.username') }}</th>
            <th class="px-5 py-3 font-medium">{{ t('users.role') }}</th>
            <th class="hidden px-5 py-3 font-medium sm:table-cell">{{ t('users.createdAt') }}</th>
            <th class="px-5 py-3"><span class="sr-only">Actions</span></th>
          </tr>
        </thead>
        <tbody class="divide-y divide-white/5">
          <tr v-for="user in users" :key="user.id">
            <td class="px-5 py-3">
              <div class="flex items-center gap-3">
                <UserAvatar :user="user" class="size-8 shrink-0" />
                <span class="font-medium">{{ user.displayName }}</span>
                <span v-if="user.alias" class="text-xs text-slate-500">{{ t('aliases.original', { name: user.username }) }}</span>
                <span v-if="user.id === auth.user?.id" class="text-xs text-slate-500">({{ t('users.you') }})</span>
                <span v-if="user.remote" class="text-xs text-slate-500">{{ t('users.remoteFrom', { server: user.originServer }) }}</span>
              </div>
            </td>
            <td class="px-5 py-3">
              <select
                :value="user.role"
                :class="selectClass"
                :disabled="user.remote"
                :aria-label="t('users.role')"
                @change="changeRole(user, ($event.target as HTMLSelectElement).value as Role)"
              >
                <option value="user">{{ t('users.roles.user') }}</option>
                <option value="admin">{{ t('users.roles.admin') }}</option>
              </select>
            </td>
            <td class="hidden px-5 py-3 text-slate-400 sm:table-cell">{{ d(new Date(user.createdAt)) }}</td>
            <td class="px-5 py-3">
              <div class="flex justify-end gap-2">
                <BaseButton @click="openAlias(user)">{{ t('aliases.action') }}</BaseButton>
                <BaseButton v-if="!user.remote" @click="openPassword(user)">{{ t('users.changePassword') }}</BaseButton>
                <BaseButton
                  v-if="!user.remote && user.id !== auth.user?.id"
                  class="hover:!bg-red-500/15 hover:!text-red-300"
                  @click="askDelete(user)"
                >
                  {{ t('common.delete') }}
                </BaseButton>
              </div>
            </td>
          </tr>
          <tr v-for="invitation in invitations" :key="invitation.id" data-testid="pending-invitation">
            <td class="px-5 py-3">
              <div class="flex items-center gap-3">
                <span class="grid size-8 shrink-0 place-items-center rounded-full border border-dashed border-white/20 text-slate-500" aria-hidden="true">?</span>
                <span class="font-medium text-slate-400 italic">{{ t('invitations.pending') }}</span>
                <span class="text-xs text-slate-500">{{ t('invitations.expires', { date: d(new Date(invitation.expiresAt), 'long') }) }}</span>
              </div>
            </td>
            <td class="px-5 py-3 text-slate-500">{{ t('users.roles.user') }}</td>
            <td class="hidden px-5 py-3 text-slate-400 sm:table-cell">{{ d(new Date(invitation.createdAt)) }}</td>
            <td class="px-5 py-3">
              <div class="flex justify-end gap-2">
                <BaseButton class="hover:!bg-red-500/15 hover:!text-red-300" @click="askRevoke(invitation)">
                  {{ t('common.delete') }}
                </BaseButton>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <BaseDialog v-model:open="createOpen" :title="t('users.new')">
      <form id="user-form" class="space-y-5" @submit.prevent="create">
        <TextField v-model="createForm.username" :label="t('common.username')" :hint="t('setup.usernameHint')" autocomplete="off" required :maxlength="32" />
        <TextField v-model="createForm.password" :label="t('common.password')" :hint="t('setup.passwordHint')" type="password" autocomplete="new-password" required :minlength="8" />
        <div class="grid grid-cols-2 gap-4">
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

    <BaseDialog v-model:open="passwordOpen" :title="`${t('users.changePassword')} · ${passwordUser?.displayName ?? ''}`">
      <form id="password-form" class="space-y-5" @submit.prevent="savePassword">
        <TextField v-model="newPassword" :label="t('users.newPassword')" :hint="t('setup.passwordHint')" type="password" autocomplete="new-password" required :minlength="8" />
        <ErrorAlert v-if="passwordError" :message="passwordError" />
      </form>
      <template #footer>
        <BaseButton @click="passwordOpen = false">{{ t('common.cancel') }}</BaseButton>
        <BaseButton type="submit" form="password-form" variant="primary" :loading="savingPassword">{{ t('common.save') }}</BaseButton>
      </template>
    </BaseDialog>

    <AliasDialog v-model:open="aliasOpen" :name="aliasUser?.username ?? ''" :alias="aliasUser?.alias ?? null" :save="saveAlias" />

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
