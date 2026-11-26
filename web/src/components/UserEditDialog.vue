<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { User } from '@/api/client'
import { usersApi, type Role } from '@/api/users'
import { useErrorMessage } from '@/composables/useErrorMessage'
import BaseButton from './BaseButton.vue'
import BaseDialog from './BaseDialog.vue'
import ErrorAlert from './ErrorAlert.vue'
import TextField from './TextField.vue'

/**
 * Edits a user's alias, role and password in one place. Users synced from a paired server only take an
 * alias: their role and password belong to their own server.
 */
const props = defineProps<{ user: User | null }>()
const emit = defineEmits<{ saved: [user: User, passwordChanged: boolean] }>()
const open = defineModel<boolean>('open', { required: true })
const { t } = useI18n()
const errorMessage = useErrorMessage()

const alias = ref('')
const role = ref<Role>('user')
const password = ref('')
const error = ref('')
const saving = ref(false)

const selectClass =
  'block w-full rounded-lg border border-white/10 bg-surface-800 px-3.5 py-2.5 text-slate-100 focus:border-accent-500 focus:ring-2 focus:ring-accent-500/30 focus:outline-none'

watch(open, (isOpen) => {
  if (!isOpen || !props.user) return
  alias.value = props.user.alias ?? ''
  role.value = props.user.role
  password.value = ''
  error.value = ''
})

async function submit() {
  const user = props.user
  if (!user) return
  saving.value = true
  error.value = ''
  try {
    let updated = user
    const nextAlias = alias.value.trim() || null
    if (nextAlias !== (user.alias ?? null)) updated = await usersApi.setAlias(user.id, nextAlias)
    const changes: { role?: Role; password?: string } = {}
    if (!user.remote && role.value !== user.role) changes.role = role.value
    if (!user.remote && password.value) changes.password = password.value
    if (changes.role || changes.password) updated = await usersApi.update(user.id, changes)
    emit('saved', updated, !!changes.password)
    open.value = false
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <BaseDialog v-model:open="open" :title="t('users.edit', { name: user?.displayName ?? '' })">
    <form v-if="user" id="user-edit-form" class="space-y-5" @submit.prevent="submit">
      <TextField
        v-model="alias"
        :label="t('aliases.label')"
        :hint="t('aliases.hint', { name: user.username })"
        autocomplete="off"
        :maxlength="64"
      />
      <template v-if="!user.remote">
        <label class="block">
          <span class="mb-1.5 block text-sm font-medium text-slate-300">{{ t('users.role') }}</span>
          <select v-model="role" :class="selectClass">
            <option value="user">{{ t('users.roles.user') }}</option>
            <option value="admin">{{ t('users.roles.admin') }}</option>
          </select>
        </label>
        <TextField
          v-model="password"
          :label="t('users.newPassword')"
          :hint="t('users.newPasswordHint')"
          type="password"
          autocomplete="new-password"
          :minlength="8"
        />
      </template>
      <p v-else class="text-sm text-slate-400">{{ t('users.remoteHelp', { server: user.originServer ?? '' }) }}</p>
      <ErrorAlert v-if="error" :message="error" />
    </form>
    <template #footer>
      <BaseButton @click="open = false">{{ t('common.cancel') }}</BaseButton>
      <BaseButton type="submit" form="user-edit-form" variant="primary" :loading="saving">{{ t('common.save') }}</BaseButton>
    </template>
  </BaseDialog>
</template>
